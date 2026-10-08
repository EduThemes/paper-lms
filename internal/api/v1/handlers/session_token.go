package handlers

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/auth"
	"github.com/EduThemes/paper-lms/internal/service"
)

// SessionTokenHandler implements Canvas's GET /login/session_token (G136):
// an app holding an API access token asks for a session_url; opening that
// URL in a browser signs the same user into a web session once, within
// service.LoginSessionTokenTTL. Redemption is a new credential path, so it
// funnels through auth.LoginPipeline (ProviderType "session_token") like
// every other login.
type SessionTokenHandler struct {
	tokens      *service.LoginSessionTokenService
	pipeline    *auth.LoginPipeline
	frontendURL string
}

// NewSessionTokenHandler wires the handler. frontendURL is the SPA origin
// the browser lands on after redemption ("" = same origin).
func NewSessionTokenHandler(tokens *service.LoginSessionTokenService, pipeline *auth.LoginPipeline, frontendURL string) *SessionTokenHandler {
	return &SessionTokenHandler{tokens: tokens, pipeline: pipeline, frontendURL: strings.TrimSuffix(frontendURL, "/")}
}

// sanitizeReturnTo turns a return_to value into a same-site path+query.
// Absolute URLs are accepted only for the request's own host or the SPA's
// host (Canvas requires the same host); anything else is refused so the
// endpoint can't be used as an open redirect.
func sanitizeReturnTo(raw, requestHost, frontendURL string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "/", true
	}
	if strings.ContainsAny(raw, "\\\r\n") {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	if u.IsAbs() || u.Host != "" {
		if u.Scheme != "http" && u.Scheme != "https" {
			return "", false
		}
		allowed := strings.EqualFold(u.Host, requestHost)
		if fu, err := url.Parse(frontendURL); err == nil && fu.Host != "" && strings.EqualFold(u.Host, fu.Host) {
			allowed = true
		}
		if !allowed {
			return "", false
		}
	} else if !strings.HasPrefix(u.Path, "/") || strings.HasPrefix(raw, "//") {
		return "", false
	}
	q := u.Query()
	q.Del("session_token")
	out := u.EscapedPath()
	if out == "" {
		out = "/"
	}
	if enc := q.Encode(); enc != "" {
		out += "?" + enc
	}
	if u.Fragment != "" {
		out += "#" + u.EscapedFragment()
	}
	return out, true
}

// Mint returns a single-use session_url for the caller.
// GET /login/session_token and GET /api/v1/login/session_token
// (access token required; a cookie session gets 401, as in Canvas, which
// only allows this from the API).
func (h *SessionTokenHandler) Mint(c *fiber.Ctx) error {
	if tokenID, ok := c.Locals("access_token_id").(uint); !ok || tokenID == 0 {
		return responses.Error(c, fiber.StatusUnauthorized, "session_token must be requested with an API access token")
	}
	// A masquerade session has no access token, so this is the real user.
	userID, err := getUserID(c)
	if err != nil {
		return responses.Unauthorized(c)
	}
	returnTo, ok := sanitizeReturnTo(c.Query("return_to"), c.Get(fiber.HeaderHost), h.frontendURL)
	if !ok {
		return responses.BadRequest(c, "Invalid return_to: it must be a path or a URL on this site")
	}
	raw, err := h.tokens.Mint(c.Context(), userID, callerAccountID(c), returnTo)
	if err != nil {
		return responses.InternalError(c, "Could not create a session token")
	}
	return c.JSON(fiber.Map{
		"session_url":               c.BaseURL() + "/login/session_token?session_token=" + url.QueryEscape(raw),
		"requires_terms_acceptance": false,
		"expires_in":                int(service.LoginSessionTokenTTL.Seconds()),
	})
}

// Redeem consumes ?session_token=... and signs the browser in. Mounted as
// the first handler of GET /login/session_token: without the parameter it
// passes on to the authenticated Mint chain.
func (h *SessionTokenHandler) Redeem(c *fiber.Ctx) error {
	raw := c.Query("session_token")
	if raw == "" {
		return c.Next()
	}
	fail := func() error {
		return c.Redirect(h.frontendURL+"/login?error=session_token", fiber.StatusFound)
	}
	row, err := h.tokens.Consume(c.Context(), raw)
	if err != nil || h.pipeline == nil {
		return fail()
	}
	outcome := auth.SSOOutcome{
		ProviderType:    auth.ProviderTypeSessionToken,
		ExternalSubject: strconv.FormatUint(uint64(row.UserID), 10),
		EmailVerified:   true,
	}
	meta := auth.RequestMeta{IPAddress: c.IP(), UserAgent: c.Get(fiber.HeaderUserAgent)}
	result, err := h.pipeline.Execute(c.Context(), outcome, meta)
	if err != nil || result == nil || result.Token == "" || result.User == nil || result.User.AccountID != row.AccountID {
		return fail()
	}
	c.Cookie(&fiber.Cookie{
		Name:     "paper_session",
		Value:    result.Token,
		Path:     "/",
		HTTPOnly: true,
		Secure:   auth.SecureCookies(),
		SameSite: "Lax",
		MaxAge:   86400,
		Expires:  time.Now().Add(24 * time.Hour),
	})
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Redirect(h.frontendURL+row.ReturnTo, fiber.StatusFound)
}
