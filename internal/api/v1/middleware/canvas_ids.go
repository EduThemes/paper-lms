package middleware

import (
	"context"
	"encoding/hex"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/gofiber/fiber/v2"
)

// ErrCanvasIDNotFound is returned by a CanvasIDLookup when an alternate
// identifier names nothing visible to the caller.
var ErrCanvasIDNotFound = errors.New("canvas id not found")

// CanvasIDLookup resolves a Canvas alternate identifier ("sis_course_id",
// "sis_user_id", "sis_login_id", "sis_section_id", "sis_account_id",
// "sis_term_id") to the Paper row id. accountID scopes the lookup to one
// tenant; 0 means unscoped (platform super admins only).
type CanvasIDLookup interface {
	ResolveCanvasID(ctx context.Context, kind, value string, accountID uint) (uint, error)
}

// canvasIDKindsByCollection lists, per URL collection segment, the
// alternate id kinds Canvas accepts in the segment that follows it.
var canvasIDKindsByCollection = map[string]map[string]bool{
	"courses":  {"sis_course_id": true},
	"users":    {"sis_user_id": true, "sis_login_id": true},
	"students": {"sis_user_id": true, "sis_login_id": true},
	"sections": {"sis_section_id": true},
	"accounts": {"sis_account_id": true},
	"terms":    {"sis_term_id": true},
}

// selfUserCollections are the collections whose next segment is a user id,
// so "self" there means the caller (Canvas: /users/self/...,
// .../submissions/self, /courses/:id/students/self).
var selfUserCollections = map[string]bool{
	"users":       true,
	"submissions": true,
	"students":    true,
}

// CanvasIDs rewrites Canvas identifier conventions in the request path to
// numeric ids before routing continues, so every handler and permission
// gate downstream sees the plain numeric form (G137):
//
//   - "self" after users/, submissions/ or students/ becomes the caller's
//     user id, and after accounts/ the caller's account id. A route that
//     names "self" literally (GET /users/self, /users/self/groups, ...)
//     keeps it, so those dedicated handlers still run.
//   - "sis_course_id:X", "sis_user_id:X", "sis_login_id:X",
//     "sis_section_id:X", "sis_account_id:X" and "sis_term_id:X" (also in
//     Canvas's "hex:sis_user_id:<hex>" form, and URL-encoded) are looked up
//     in the caller's tenant. A miss is a 404, like Canvas's api_find.
//
// It must be mounted on the protected group AFTER authentication (it needs
// the caller) and BEFORE the routes are registered (Fiber matches the
// rewritten path against the routes that follow).
func CanvasIDs(lookup CanvasIDLookup, users repository.UserRepository) fiber.Handler {
	var (
		once   sync.Once
		static map[string][][]string
	)
	return func(c *fiber.Ctx) error {
		path := c.Path()
		if !strings.HasPrefix(path, "/api/v1/") || (!strings.Contains(path, "/self") && !strings.Contains(path, "sis_")) {
			return c.Next()
		}
		once.Do(func() { static = staticSelfRoutes(c.App()) })

		segs := strings.Split(path, "/")
		changed := false
		for i := 2; i < len(segs); i++ {
			prev := segs[i-1]
			seg := segs[i]
			if seg == "self" {
				replacement, ok := selfReplacement(c, prev)
				if !ok || routeKeepsLiteralSelf(static[c.Method()], segs, i) {
					continue
				}
				segs[i] = replacement
				changed = true
				continue
			}
			kinds, ok := canvasIDKindsByCollection[prev]
			if !ok || !strings.Contains(seg, "sis_") {
				continue
			}
			kind, value, ok := parseCanvasAltID(seg)
			if !ok || !kinds[kind] {
				continue
			}
			if lookup == nil {
				return notFound(c, "resource")
			}
			scope, _ := c.Locals("account_id").(uint)
			if EffectiveRole(c, users) == RoleSuperAdmin {
				scope = 0
			} else if scope == 0 {
				return notFound(c, "resource")
			}
			id, err := lookup.ResolveCanvasID(c.Context(), kind, value, scope)
			if err != nil || id == 0 {
				return notFound(c, "resource")
			}
			segs[i] = strconv.FormatUint(uint64(id), 10)
			changed = true
		}
		if changed {
			c.Path(strings.Join(segs, "/"))
		}
		return c.Next()
	}
}

// selfReplacement returns what "self" stands for after collection prev.
func selfReplacement(c *fiber.Ctx, prev string) (string, bool) {
	if prev == "accounts" {
		if acct, ok := c.Locals("account_id").(uint); ok && acct > 0 {
			return strconv.FormatUint(uint64(acct), 10), true
		}
		return "", false
	}
	if selfUserCollections[prev] {
		if uid, ok := c.Locals("user_id").(uint); ok && uid > 0 {
			return strconv.FormatUint(uint64(uid), 10), true
		}
	}
	return "", false
}

// parseCanvasAltID splits "sis_course_id:ABC" (optionally URL-encoded, or
// "hex:sis_user_id:414243") into its kind and value.
func parseCanvasAltID(seg string) (kind, value string, ok bool) {
	decoded, err := url.PathUnescape(seg)
	if err != nil {
		return "", "", false
	}
	hexForm := false
	if rest, found := strings.CutPrefix(decoded, "hex:"); found {
		decoded = rest
		hexForm = true
	}
	kind, value, found := strings.Cut(decoded, ":")
	if !found || value == "" {
		return "", "", false
	}
	if hexForm {
		raw, err := hex.DecodeString(value)
		if err != nil || len(raw) == 0 {
			return "", "", false
		}
		value = string(raw)
	}
	return kind, value, true
}

// staticSelfRoutes indexes, per method, the split paths of every registered
// route that contains a literal "self" segment.
func staticSelfRoutes(app *fiber.App) map[string][][]string {
	out := map[string][][]string{}
	for _, routes := range app.Stack() {
		for _, r := range routes {
			if r == nil || !strings.Contains(r.Path, "/self") {
				continue
			}
			out[r.Method] = append(out[r.Method], strings.Split(r.Path, "/"))
		}
	}
	return out
}

// routeKeepsLiteralSelf reports whether some registered route matches the
// request path with its "self" segment at index i consumed literally.
func routeKeepsLiteralSelf(patterns [][]string, req []string, i int) bool {
	for _, p := range patterns {
		if i < len(p) && p[i] == "self" && routePatternMatches(p, req) {
			return true
		}
	}
	return false
}

// routePatternMatches is a small Fiber-style matcher: ":param" matches one
// non-empty segment, "*" / "+" match the rest, anything else must be equal.
func routePatternMatches(pattern, req []string) bool {
	for i, seg := range pattern {
		if seg == "*" || seg == "+" {
			return true
		}
		if i >= len(req) {
			return strings.HasPrefix(seg, ":") && strings.HasSuffix(seg, "?") && i == len(pattern)-1
		}
		if strings.HasPrefix(seg, ":") {
			if req[i] == "" {
				return false
			}
			continue
		}
		if seg != req[i] {
			return false
		}
	}
	return len(pattern) == len(req)
}
