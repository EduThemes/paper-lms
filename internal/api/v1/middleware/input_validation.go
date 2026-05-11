package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// MaxBodySize is the maximum allowed request body size for non-upload
// endpoints (10 MB) — caps JSON / form-encoded payloads to prevent DoS.
//
// File uploads (multipart/form-data) are exempt from this cap and instead
// enforced by EnforceUploadSize, which reads the admin-configured
// Account.MaxUploadSizeMB. The Fiber-level BodyLimit (5 GB) is the absolute
// ceiling for both paths.
const MaxBodySize = 10 * 1024 * 1024

// InputValidation returns middleware that enforces a body size cap on
// non-upload requests. Multipart requests are skipped so file uploads can
// flow through EnforceUploadSize without being rejected here first.
func InputValidation() fiber.Handler {
	return func(c *fiber.Ctx) error {
		method := c.Method()
		if method != "POST" && method != "PUT" && method != "PATCH" {
			return c.Next()
		}
		// Skip file uploads — they have their own admin-tunable cap.
		if ct := c.Get(fiber.HeaderContentType); strings.HasPrefix(strings.ToLower(ct), "multipart/form-data") {
			return c.Next()
		}
		if len(c.Body()) > MaxBodySize {
			return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
				"errors": []fiber.Map{{"message": "Request body too large"}},
			})
		}
		return c.Next()
	}
}
