package middleware

import (
	"context"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/settingsctx"
)

// UploadSizeLookupFunc resolves the catalog key `quotas.max_upload_size_mb`
// through the Settings Engine. It mirrors service.SettingsLookupFunc /
// auth.SettingsLookupFunc — function-typed so this middleware package
// doesn't have to import internal/service/settings (which would pull in
// internal/auth via the secretbox path and short-circuit through the
// service → auth_audit → service cycle this codebase has carefully
// avoided since Wave 5).
//
// Empty return + nil error means "no value in the resolution chain" —
// the middleware falls back to fallbackMB. A non-nil error also falls
// back to fallbackMB (defensive: a transient settings repo failure
// must NOT 500 every upload).
type UploadSizeLookupFunc func(ctx context.Context, key string) (string, error)

// fallbackUploadSizeMB matches the catalog default (5120 MB / 5 GB) and
// is the last-resort cap when the lookup returns empty/unparseable.
// Kept in sync with internal/service/settings/catalog.go's
// quotas.max_upload_size_mb Default. Owner directive 2026-05-17:
// "we don't want teachers suffering from max upload size issues."
const fallbackUploadSizeMB uint64 = 5120

// EnforceUploadSize rejects requests whose Content-Length exceeds the
// per-tenant configured max upload size, resolved through the Settings
// Engine catalog (key: `quotas.max_upload_size_mb`).
//
// The Fiber-level BodyLimit (100 MB on non-upload routes, but the
// upload routes get a higher per-mount cap upstream) acts as a hard
// ceiling; this middleware enforces the admin-tunable cap on top of it
// so the limit can be changed at runtime without restarting the server.
//
// Resolution chain (via the shared settingsLookup closure declared in
// cmd/server/main.go):
//
//  1. user scope — not allowed for this key, skipped
//  2. account scope (via settingsctx.WithAccountID + c.Locals("account_id"))
//  3. instance scope
//  4. env var MAX_UPLOAD_SIZE_MB
//  5. catalog default (5120 MB)
//
// If lookup fails or returns an unparseable value, falls back to the
// catalog default — never 500s an upload over a settings-store hiccup.
//
// Wave 4 (chore/wave4-upload-size-catalog) wired this through the
// catalog. The previous implementation read account.MaxUploadSizeMB
// directly, which made the catalog entry a documented no-op. The
// Account column is preserved for backward compat but the middleware
// no longer reads it; a follow-up PR can drop the column after a
// release cycle confirms nothing else depends on it.
func EnforceUploadSize(lookup UploadSizeLookupFunc) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Only enforce for methods that carry a body.
		method := c.Method()
		if method != fiber.MethodPost && method != fiber.MethodPut && method != fiber.MethodPatch {
			return c.Next()
		}

		maxMB := resolveUploadCapMB(c, lookup)
		maxBytes := maxMB * 1024 * 1024

		// Prefer Content-Length header if the client sent it. fasthttp's
		// `c.Body()` may be empty before the body has streamed in, so
		// the header check is the cheap fast-path.
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

// resolveUploadCapMB walks the per-tenant settings resolution chain.
// Pulled out so the test suite can exercise the lookup→fallback edges
// without spinning up a full Fiber chain. The defensive fallback on
// error / empty / unparseable value is the load-bearing invariant:
// uploads must NEVER 500 because the settings store hiccuped.
func resolveUploadCapMB(c *fiber.Ctx, lookup UploadSizeLookupFunc) uint64 {
	if lookup == nil {
		return fallbackUploadSizeMB
	}

	// Stamp the per-tenant scope hint on the ctx so the lookup walks
	// the account chain (then instance → env → default). Caller is
	// post-auth so account_id Locals is populated by AuthMiddleware.
	// Type the local as context.Context (not *fasthttp.RequestCtx) so
	// the settingsctx.WithAccountID return value (context.Context) can
	// be assigned back.
	var ctx context.Context = c.Context()
	if acctRaw := c.Locals("account_id"); acctRaw != nil {
		if acct, ok := acctRaw.(uint); ok && acct != 0 {
			ctx = settingsctx.WithAccountID(ctx, acct)
		}
	}

	raw, err := lookup(ctx, "quotas.max_upload_size_mb")
	if err != nil || raw == "" {
		return fallbackUploadSizeMB
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallbackUploadSizeMB
	}
	return uint64(n)
}
