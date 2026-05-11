package middleware

import (
	"context"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// EnforceUploadSize rejects requests whose Content-Length exceeds the
// account-configured max upload size (Account.MaxUploadSizeMB).
//
// The Fiber-level BodyLimit acts as a hard ceiling (5 GB by default); this
// middleware enforces the admin-tunable cap on top of it so the limit can be
// changed at runtime without restarting the server.
//
// The default account (id=1) is treated as the authority. We cache the value
// in-memory for a short TTL to avoid hitting the DB on every upload.
func EnforceUploadSize(accountRepo repository.AccountRepository) fiber.Handler {
	var (
		cachedMB    atomic.Uint64
		cachedAt    atomic.Int64 // unix nano
		cacheTTL    = 30 * time.Second
		fallbackMB  = uint64(500)
	)

	return func(c *fiber.Ctx) error {
		// Only enforce for methods that carry a body
		method := c.Method()
		if method != fiber.MethodPost && method != fiber.MethodPut && method != fiber.MethodPatch {
			return c.Next()
		}

		// Resolve the cap (cache for 30s)
		maxMB := cachedMB.Load()
		now := time.Now().UnixNano()
		if maxMB == 0 || now-cachedAt.Load() > int64(cacheTTL) {
			ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
			defer cancel()
			account, err := accountRepo.FindByID(ctx, 1)
			if err != nil || account == nil || account.MaxUploadSizeMB == 0 {
				maxMB = fallbackMB
			} else {
				maxMB = uint64(account.MaxUploadSizeMB)
			}
			cachedMB.Store(maxMB)
			cachedAt.Store(now)
		}

		maxBytes := maxMB * 1024 * 1024

		// Prefer Content-Length header if the client sent it.
		if cl := c.Get(fiber.HeaderContentLength); cl != "" {
			if n, err := strconv.ParseInt(cl, 10, 64); err == nil && uint64(n) > maxBytes {
				return responses.Error(c, fiber.StatusRequestEntityTooLarge,
					"File exceeds the configured upload limit ("+strconv.FormatUint(maxMB, 10)+" MB). An admin can raise this in Admin → Settings.")
			}
		}

		// Fall back to actual body length already buffered by Fiber.
		if uint64(len(c.Body())) > maxBytes {
			return responses.Error(c, fiber.StatusRequestEntityTooLarge,
				"File exceeds the configured upload limit ("+strconv.FormatUint(maxMB, 10)+" MB). An admin can raise this in Admin → Settings.")
		}

		return c.Next()
	}
}
