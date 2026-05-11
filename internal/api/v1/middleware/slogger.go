package middleware

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
)

// StructuredLogger is a Fiber middleware that logs requests using log/slog.
// It replaces the default Fiber logger with structured JSON output suitable
// for production log aggregation (ELK, Datadog, CloudWatch, etc.).
func StructuredLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Process request
		err := c.Next()

		duration := time.Since(start)
		status := c.Response().StatusCode()

		attrs := []slog.Attr{
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.Int("status", status),
			slog.Duration("duration", duration),
			slog.String("ip", c.IP()),
		}

		// Add request ID if present
		if reqID, ok := c.Locals("request_id").(string); ok && reqID != "" {
			attrs = append(attrs, slog.String("request_id", reqID))
		}

		// Add user ID if authenticated
		if userID, ok := c.Locals("user_id").(uint); ok && userID > 0 {
			attrs = append(attrs, slog.Uint64("user_id", uint64(userID)))
		}

		// Add response size
		attrs = append(attrs, slog.Int("bytes", len(c.Response().Body())))

		// Log level based on status code
		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		} else if status >= 400 {
			level = slog.LevelWarn
		}

		slog.LogAttrs(c.Context(), level, "http request", attrs...)

		return err
	}
}
