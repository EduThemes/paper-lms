package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/EduThemes/paper-lms/internal/obs"
)

// Observability is the per-request span + histogram middleware.
//
// What it does, in order:
//
//  1. Extracts inbound W3C TraceContext / Baggage headers so this server
//     is a downstream link in a longer distributed trace.
//  2. Starts a SERVER-kind span named "<METHOD> <route-pattern>" — using
//     the Fiber route pattern (e.g. "/api/v1/courses/:id"), not the
//     concrete URL, so cardinality stays manageable.
//  3. Annotates the span with the existing X-Request-ID (the
//     `request_id` Local set by middleware.RequestID) so the OTEL trace
//     and the slog request_id can be joined.
//  4. Propagates the span context into c.UserContext() so downstream
//     code (handlers, repos) can attach child spans without re-extracting
//     context manually.
//  5. After c.Next() returns, records the HTTP status onto the span +
//     observes the request duration into the Prom histogram.
//
// This middleware MUST be registered AFTER middleware.RequestID so the
// request_id Local is populated when this code reads it. The slog
// logger should also run after this so it can log the trace_id.
func Observability() fiber.Handler {
	propagator := otel.GetTextMapPropagator()

	return func(c *fiber.Ctx) error {
		// (1) Extract upstream trace context.
		ctx := propagator.Extract(c.UserContext(), fiberHeaderCarrier{c: c})

		// (2) Start the server span on the route pattern, not the URL.
		route := c.Route().Path
		if route == "" {
			route = c.Path()
		}
		spanName := c.Method() + " " + route

		ctx, span := obs.Tracer().Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("http.method", c.Method()),
				attribute.String("http.route", route),
				attribute.String("http.target", c.OriginalURL()),
				attribute.String("http.scheme", c.Protocol()),
				attribute.String("net.host.name", c.Hostname()),
				attribute.String("net.peer.ip", c.IP()),
				attribute.String("http.user_agent", c.Get("User-Agent")),
			),
		)
		defer span.End()

		// (3) Cross-link with the existing request_id pipeline.
		if reqID, ok := c.Locals("request_id").(string); ok && reqID != "" {
			span.SetAttributes(attribute.String("request.id", reqID))
		}

		// (4) Propagate the context for downstream span children + log
		//     correlation. Handlers that call out to repos via
		//     c.Context() will already get the right ctx because Fiber
		//     wires UserContext into Context() when set.
		c.SetUserContext(ctx)

		// (5) Run the handler chain.
		start := time.Now()
		err := c.Next()
		duration := time.Since(start)

		status := c.Response().StatusCode()
		span.SetAttributes(attribute.Int("http.status_code", status))
		switch {
		case err != nil:
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		case status >= 500:
			span.SetStatus(codes.Error, "server error")
		case status >= 400:
			// 4xx is "client did something we rejected" — not a server
			// fault. Leave the status code on the span; do not flag as
			// span-error.
		default:
			span.SetStatus(codes.Ok, "")
		}

		obs.RecordHTTPDuration(c.Method(), route, status, duration)
		return err
	}
}

// fiberHeaderCarrier adapts Fiber's request headers to the OTEL
// TextMapCarrier interface. Read-only: writing isn't needed because
// we're extracting upstream context, not injecting downstream context.
type fiberHeaderCarrier struct {
	c *fiber.Ctx
}

func (h fiberHeaderCarrier) Get(key string) string {
	return h.c.Get(key)
}

func (h fiberHeaderCarrier) Set(key, value string) {
	h.c.Set(key, value)
}

func (h fiberHeaderCarrier) Keys() []string {
	var keys []string
	h.c.Request().Header.VisitAll(func(k, _ []byte) {
		keys = append(keys, string(k))
	})
	return keys
}

// compile-time check
var _ propagation.TextMapCarrier = fiberHeaderCarrier{}
