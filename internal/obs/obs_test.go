package obs

import (
	"context"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"
)

// TestInit_NoOTLPExporter_StillServesMetrics is the smoke test for the
// "plumbing only" v1 path: no OTLP endpoint configured → spans are
// no-op → /metrics still returns Prom exposition.
func TestInit_NoOTLPExporter_StillServesMetrics(t *testing.T) {
	resetForTest(t)

	shutdown, err := Init(context.Background(), Config{
		ServiceName: "paper-lms-test",
		Version:     "test",
		Environment: "test",
		SampleRatio: 1.0,
	})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = shutdown(ctx)
	})

	// Emit a histogram observation so /metrics has content to report.
	RecordHTTPDuration("GET", "/api/v1/courses/:id", 200, 12*time.Millisecond)

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	MetricsHandler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("metrics endpoint: want 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "paper_lms_http_request_duration_seconds_count") {
		t.Errorf("metrics body missing the http duration histogram\n%s", body)
	}
	if !strings.Contains(body, `route="/api/v1/courses/:id"`) {
		t.Errorf("metrics body missing the route label\n%s", body)
	}
	// Process collector should be registered by default.
	if !strings.Contains(body, "process_cpu_seconds_total") {
		t.Errorf("metrics body missing process collector output")
	}
}

// TestTracer_DefaultIsNoop verifies that calling Tracer() before Init
// returns a usable tracer (the no-op one), so dependent code never
// panics even if Init is skipped (e.g. in unit tests of handler code
// that imports obs transitively).
func TestTracer_DefaultIsNoop(t *testing.T) {
	resetForTest(t)
	tr := Tracer()
	if tr == nil {
		t.Fatalf("Tracer() returned nil before Init")
	}
	_, span := tr.Start(context.Background(), "test-span")
	span.End()
}

// TestLoadConfig_DefaultsAndOverrides pins the env-var contract.
func TestLoadConfig_DefaultsAndOverrides(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		t.Setenv("OBSERVABILITY_OTLP_ENDPOINT", "")
		t.Setenv("OBSERVABILITY_TRACES_STDOUT", "")
		t.Setenv("OBSERVABILITY_SERVICE_NAME", "")
		t.Setenv("OBSERVABILITY_SAMPLE_RATIO", "")
		cfg := LoadConfig("v1.2.3", "production")
		if cfg.ServiceName != "paper-lms" {
			t.Errorf("default service name: want paper-lms, got %q", cfg.ServiceName)
		}
		if cfg.SampleRatio != 1.0 {
			t.Errorf("default sample ratio: want 1.0, got %v", cfg.SampleRatio)
		}
		if cfg.TracesStdout {
			t.Errorf("default stdout traces: want false")
		}
		if cfg.Version != "v1.2.3" || cfg.Environment != "production" {
			t.Errorf("version/env not propagated: %+v", cfg)
		}
	})

	t.Run("overrides", func(t *testing.T) {
		t.Setenv("OBSERVABILITY_OTLP_ENDPOINT", "http://collector.test:4318")
		t.Setenv("OBSERVABILITY_TRACES_STDOUT", "true")
		t.Setenv("OBSERVABILITY_SERVICE_NAME", "paper-lms-staging")
		t.Setenv("OBSERVABILITY_SAMPLE_RATIO", "0.25")
		cfg := LoadConfig("v1.2.3", "staging")
		if cfg.OTLPEndpoint != "http://collector.test:4318" {
			t.Errorf("OTLP endpoint not picked up: %q", cfg.OTLPEndpoint)
		}
		if !cfg.TracesStdout {
			t.Errorf("stdout traces: want true")
		}
		if cfg.ServiceName != "paper-lms-staging" {
			t.Errorf("service name: %q", cfg.ServiceName)
		}
		if cfg.SampleRatio != 0.25 {
			t.Errorf("sample ratio: want 0.25, got %v", cfg.SampleRatio)
		}
	})

	t.Run("invalid sample ratio is dropped", func(t *testing.T) {
		t.Setenv("OBSERVABILITY_SAMPLE_RATIO", "not-a-number")
		cfg := LoadConfig("v1", "test")
		if cfg.SampleRatio != 1.0 {
			t.Errorf("invalid ratio should fall back to default 1.0; got %v", cfg.SampleRatio)
		}
	})

	t.Run("out-of-range sample ratio is dropped", func(t *testing.T) {
		t.Setenv("OBSERVABILITY_SAMPLE_RATIO", "2.0")
		cfg := LoadConfig("v1", "test")
		if cfg.SampleRatio != 1.0 {
			t.Errorf("out-of-range ratio should fall back to 1.0; got %v", cfg.SampleRatio)
		}
	})
}

// resetForTest re-initialises the package-level sync.Once + state so
// tests can call Init multiple times against fresh state. Only valid
// inside the obs package because it touches unexported globals.
func resetForTest(t *testing.T) {
	t.Helper()
	initOnce = sync.Once{}
	initErr = nil
	tracerProvider = noop.NewTracerProvider()
	promRegistry = nil
	httpReqDuration = nil
}
