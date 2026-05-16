// Package obs sets up the observability plumbing — distributed tracing
// (OpenTelemetry) and Prometheus metrics — for the Paper LMS server.
//
// Phase 11 / G2 "observability skeleton": this is the plumbing layer.
// It configures the global TracerProvider + a Prometheus registry,
// exposes the per-request middleware that records spans and the
// http-server-request-duration histogram, and serves /metrics. It
// does NOT instrument individual handlers, repositories, or service
// functions — that comes in follow-up PRs as we measure what's worth
// tracing.
//
// Env vars:
//
//	OBSERVABILITY_OTLP_ENDPOINT
//	    If set (e.g. "http://otel-collector:4318" or
//	    "https://api.honeycomb.io"), traces are exported via OTLP/HTTP.
//	    If empty, the TracerProvider is wired but spans are dropped on
//	    flush — useful for development without a collector.
//	OBSERVABILITY_TRACES_STDOUT
//	    "true" routes spans to stdout (JSON), in addition to OTLP if
//	    enabled. Handy for local "did the span fire?" debugging.
//	OBSERVABILITY_SERVICE_NAME
//	    Defaults to "paper-lms". Set per-environment if you run multiple
//	    instances against the same collector (e.g. "paper-lms-staging").
//	OBSERVABILITY_SAMPLE_RATIO
//	    Float in [0.0, 1.0]. Default 1.0 (sample everything). Drop below
//	    0.1 in high-traffic production to control collector cost.
//
// Prometheus is always on: /metrics is served unconditionally. The
// registry is package-global so a no-arg call to obs.HTTPRequestDuration
// returns the canonical histogram registered at Init time.
package obs

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// TracerName is the name to pass into otel.Tracer / TracerProvider.Tracer
// from any instrumentation site. Centralized so renames are atomic.
const TracerName = "github.com/EduThemes/paper-lms"

var (
	initOnce sync.Once
	initErr  error

	tracerProvider trace.TracerProvider = noop.NewTracerProvider()
	promRegistry   *prometheus.Registry

	httpReqDuration *prometheus.HistogramVec
)

// Config is the resolved-from-env configuration the Init step uses.
// Exposed for tests that want to override individual fields.
type Config struct {
	OTLPEndpoint  string
	TracesStdout  bool
	ServiceName   string
	Version       string
	Environment   string
	SampleRatio   float64
}

// LoadConfig reads obs config from environment.
//
// Version is supplied by the caller (it's a build-time -ldflags value
// in cmd/server, not an env var). Environment is the existing config
// system's environment flag — passed in so we don't reach into the
// config package and create a dep cycle.
func LoadConfig(version, environment string) Config {
	cfg := Config{
		OTLPEndpoint: os.Getenv("OBSERVABILITY_OTLP_ENDPOINT"),
		TracesStdout: os.Getenv("OBSERVABILITY_TRACES_STDOUT") == "true",
		ServiceName:  os.Getenv("OBSERVABILITY_SERVICE_NAME"),
		Version:      version,
		Environment:  environment,
		SampleRatio:  1.0,
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = "paper-lms"
	}
	if raw := os.Getenv("OBSERVABILITY_SAMPLE_RATIO"); raw != "" {
		if f, err := strconv.ParseFloat(raw, 64); err == nil && f >= 0 && f <= 1 {
			cfg.SampleRatio = f
		}
	}
	return cfg
}

// Init configures the global TracerProvider, Prometheus registry, and
// process collectors. Returns a shutdown function that flushes exporters
// and tears down the provider. Idempotent: subsequent calls are no-ops.
//
// The returned shutdown must be invoked at server stop (defer in main).
// Without it, spans buffered at exit are silently dropped.
func Init(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error) {
	initOnce.Do(func() {
		shutdown, initErr = doInit(ctx, cfg)
	})
	if shutdown == nil {
		shutdown = func(context.Context) error { return nil }
	}
	return shutdown, initErr
}

func doInit(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	// --- Prometheus side: always on. ---
	promRegistry = prometheus.NewRegistry()
	promRegistry.MustRegister(collectors.NewGoCollector())
	promRegistry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	httpReqDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "paper_lms",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "Server-side HTTP request duration, labelled by method/route-pattern/status.",
			// Buckets tuned for an LMS: most reads sub-100ms; gradebook
			// writes can reach the seconds range during bulk ops.
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "route", "status"},
	)
	promRegistry.MustRegister(httpReqDuration)

	// --- OTEL side: optional, controlled by env. ---
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.Version),
			semconv.DeploymentEnvironmentName(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("obs: build resource: %w", err)
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.SampleRatio)),
	}

	var shutdownFns []func(context.Context) error

	if cfg.OTLPEndpoint != "" {
		exp, err := otlptrace.New(ctx,
			otlptracehttp.NewClient(
				otlptracehttp.WithEndpointURL(cfg.OTLPEndpoint),
			),
		)
		if err != nil {
			return nil, fmt.Errorf("obs: build OTLP exporter: %w", err)
		}
		opts = append(opts, sdktrace.WithBatcher(exp))
		shutdownFns = append(shutdownFns, exp.Shutdown)
	}
	if cfg.TracesStdout {
		exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("obs: build stdout exporter: %w", err)
		}
		opts = append(opts, sdktrace.WithBatcher(exp))
		shutdownFns = append(shutdownFns, exp.Shutdown)
	}

	tp := sdktrace.NewTracerProvider(opts...)
	tracerProvider = tp
	shutdownFns = append(shutdownFns, tp.Shutdown)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	shutdown := func(ctx context.Context) error {
		var firstErr error
		for _, fn := range shutdownFns {
			if err := fn(ctx); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	}
	return shutdown, nil
}

// Tracer returns the package-level tracer. Always safe to call — pre-Init
// it returns a noop tracer.
func Tracer() trace.Tracer {
	return tracerProvider.Tracer(TracerName)
}

// RecordHTTPDuration is the per-request histogram observation. The
// middleware in internal/api/v1/middleware/observability.go is the only
// expected caller. Exposed (not unexported) so a future external
// transport (gRPC, GraphQL subscriptions) can record into the same
// histogram if appropriate.
func RecordHTTPDuration(method, route string, status int, d time.Duration) {
	if httpReqDuration == nil {
		return
	}
	httpReqDuration.WithLabelValues(method, route, strconv.Itoa(status)).Observe(d.Seconds())
}

// MetricsHandler returns the http.Handler that serves /metrics in
// Prometheus exposition format. Mount on the server next to /healthz.
//
// Returns a 503 handler if Init has not been called — this is a misuse
// signal, not a fallback to silently work.
func MetricsHandler() http.Handler {
	if promRegistry == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "obs.Init not called", http.StatusServiceUnavailable)
		})
	}
	return promhttp.HandlerFor(promRegistry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
		Registry:          promRegistry,
	})
}
