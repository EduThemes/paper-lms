package middleware

import (
	"context"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/service"
)

// AuditEventEmitter is the narrow surface AuditWrites needs from the
// audit service. Extracted so the middleware is mockable in tests
// without having to spin up a real *postgres.AuditLogRepo.
type AuditEventEmitter interface {
	LogEvent(ctx context.Context, eventType string, userID uint, courseID, accountID *uint, contextType string, contextID uint, action, payload, ipAddress, userAgent string) error
}

// AuditWrites mounts on every write-emitting protected route group
// and emits an audit_log row when the response is 2xx and the
// request method is a writing method (POST/PUT/PATCH/DELETE).
//
// 13.5 — `LogPIIAccess` and `LogGradeChange` were defined but never
// called pre-13.5; this middleware is the systematic wrapping piece
// for the write side. A single `protected.Use(...)` in the router
// covers every authenticated write route (~333 of them) without
// per-route plumbing. The read side is handled per-handler with
// `auditService.LogPIIAccess`.
func AuditWrites(emitter AuditEventEmitter, eventType string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if err := c.Next(); err != nil {
			return err
		}
		status := c.Response().StatusCode()
		if status < 200 || status >= 300 {
			return nil
		}
		method := c.Method()
		if method != fiber.MethodPost && method != fiber.MethodPut &&
			method != fiber.MethodPatch && method != fiber.MethodDelete {
			return nil
		}
		// Nil-emitter guard. Production wires a *service.AuditService;
		// tests can pass nil to bypass emission, or a fake to assert.
		if emitter == nil {
			return nil
		}
		userID, _ := c.Locals("user_id").(uint)
		var courseID *uint
		if cid, err := c.ParamsInt("course_id"); err == nil && cid > 0 {
			u := uint(cid)
			courseID = &u
		}
		var accountID *uint
		if aid, ok := c.Locals("account_id").(uint); ok && aid > 0 {
			accountID = &aid
		}
		payload := method + " " + c.Path()
		// Fire-and-forget — the request has already succeeded; an
		// audit-write failure should NOT 5xx the client.
		_ = emitter.LogEvent(
			c.Context(),
			eventType,
			userID,
			courseID,
			accountID,
			"http",
			0,
			method+" "+c.Route().Path,
			payload,
			c.IP(),
			c.Get("User-Agent"),
		)
		return nil
	}
}

// Compile-time check that *service.AuditService satisfies AuditEventEmitter.
var _ AuditEventEmitter = (*service.AuditService)(nil)
