// Package handlers' PasskeyHandler hosts the WebAuthn (passkey)
// ceremonies — register/login × begin/finish — plus the
// authenticated management routes (list + rename + revoke).
//
// Sprint 10-B locked UX: **passkey-as-primary**. A passkey login
// produces a real session immediately; no password, no TOTP step.
// The LoginPipeline branches on `outcome.ProviderType=="passkey"`
// to skip the MFA gate (the device biometric IS the second factor).
//
// Ceremony state is stored in a short-lived HttpOnly cookie:
// `passkey_ceremony`. The cookie carries `secretbox`-encrypted
// SessionData (challenge + RP + allowed credentials + expiry). No
// Redis dependency, no DB table — the cookie's 60-second TTL matches
// the library's challenge TTL.
package handlers

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/auth"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// fiberToHTTPRequest builds a minimal *http.Request from the fiber
// context's body — enough for the webauthn library, which only
// reads request.Body. We don't translate every header because the
// library doesn't consult them.
func fiberToHTTPRequest(c *fiber.Ctx) (*http.Request, error) {
	body := c.Body()
	req, err := http.NewRequest(http.MethodPost, c.OriginalURL(), io.NopCloser(bytes.NewReader(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

const passkeyCeremonyCookie = "passkey_ceremony"

// PasskeyHandler wires the engine + the login pipeline. The engine
// owns the library config + repo references; the pipeline produces
// the session on a successful login.
type PasskeyHandler struct {
	engine    *auth.PasskeyEngine
	users     repository.UserRepository
	creds     repository.UserWebauthnCredentialRepository
	pipeline  *auth.LoginPipeline
	audit     *auth.AuthAudit
	jwtSecret string
}

func NewPasskeyHandler(engine *auth.PasskeyEngine, users repository.UserRepository, creds repository.UserWebauthnCredentialRepository, pipeline *auth.LoginPipeline, audit *auth.AuthAudit, jwtSecret string) *PasskeyHandler {
	return &PasskeyHandler{engine: engine, users: users, creds: creds, pipeline: pipeline, audit: audit, jwtSecret: jwtSecret}
}

// ----- Registration -----

type beginRegistrationRequest struct {
	Nickname string `json:"nickname"`
}

// BeginRegistration starts a passkey enrollment for the
// authenticated user. Returns the PublicKeyCredentialCreationOptions
// the browser will pass to `navigator.credentials.create()`.
//
// The optional nickname is stashed in the ceremony cookie so the
// finish step can persist it without a second client-side trip.
func (h *PasskeyHandler) BeginRegistration(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 {
		return responses.Unauthorized(c)
	}
	// Self lookup: userID is the JWT subject. accountID=0 is correct.
	user, err := h.users.FindByID(c.Context(), userID, 0)
	if err != nil || user == nil {
		return responses.Error(c, fiber.StatusNotFound, "user not found")
	}

	var body beginRegistrationRequest
	_ = c.BodyParser(&body)

	creation, session, err := h.engine.BeginRegistration(c.Context(), user)
	if err != nil {
		return responses.InternalError(c, "failed to begin passkey registration: "+err.Error())
	}
	cookieValue, err := auth.EncodePasskeySession(session)
	if err != nil {
		return responses.InternalError(c, "failed to encode ceremony")
	}
	// Store the nickname alongside the ceremony cookie. A second
	// cookie keeps the SessionData encoding independent of our
	// per-flow extras.
	nickCookieValue := strings.TrimSpace(body.Nickname)
	setCeremonyCookies(c, cookieValue, nickCookieValue)

	return c.JSON(fiber.Map{"options": creation})
}

// FinishRegistration validates the browser's attestation, persists
// the new credential row, and clears the ceremony cookie.
func (h *PasskeyHandler) FinishRegistration(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 {
		return responses.Unauthorized(c)
	}
	// Self lookup: userID is the JWT subject. accountID=0 is correct.
	user, err := h.users.FindByID(c.Context(), userID, 0)
	if err != nil || user == nil {
		return responses.Error(c, fiber.StatusNotFound, "user not found")
	}

	cookieValue := c.Cookies(passkeyCeremonyCookie)
	session, err := auth.DecodePasskeySession(cookieValue)
	if err != nil {
		return responses.Error(c, fiber.StatusBadRequest, "passkey ceremony expired; please retry")
	}
	nickname := c.Cookies(passkeyCeremonyCookie + "_nick")

	// Reconstruct a stdlib *http.Request from the fiber.Ctx so the
	// library can read the response body. Fiber's adapter handles
	// this for us.
	httpReq, herr := fiberToHTTPRequest(c)
	if herr != nil {
		return responses.InternalError(c, "failed to adapt request")
	}

	row, err := h.engine.FinishRegistration(c.Context(), user, session, httpReq, nickname)
	if err != nil {
		return responses.Error(c, fiber.StatusBadRequest, "passkey registration failed: "+err.Error())
	}
	if err := h.creds.Create(c.Context(), row); err != nil {
		return responses.InternalError(c, "failed to persist passkey: "+err.Error())
	}
	clearCeremonyCookies(c)
	meta := auth.RequestMeta{IPAddress: c.IP(), UserAgent: c.Get("User-Agent")}
	h.audit.PasskeyRegistered(c.Context(), user.ID, row.ID, nickname, meta)

	return c.JSON(fiber.Map{
		"id":       row.ID,
		"nickname": row.Nickname,
	})
}

// ----- Management (authenticated) -----

type passkeyView struct {
	ID             uint       `json:"id"`
	Nickname       string     `json:"nickname"`
	BackupEligible bool       `json:"backup_eligible"`
	BackupState    bool       `json:"backup_state"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// List returns the calling user's registered passkeys (no key
// material — just the management view).
func (h *PasskeyHandler) List(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 {
		return responses.Unauthorized(c)
	}
	rows, err := h.creds.ListForUser(c.Context(), userID)
	if err != nil {
		return responses.InternalError(c, "failed to list passkeys")
	}
	out := make([]passkeyView, 0, len(rows))
	for _, r := range rows {
		out = append(out, passkeyView{
			ID:             r.ID,
			Nickname:       r.Nickname,
			BackupEligible: r.BackupEligible,
			BackupState:    r.BackupState,
			LastUsedAt:     r.LastUsedAt,
			CreatedAt:      r.CreatedAt,
		})
	}
	return c.JSON(fiber.Map{"passkeys": out})
}

type renameRequest struct {
	Nickname string `json:"nickname"`
}

// Rename updates the user-facing label on a passkey. Scoped to the
// caller's user_id by the repo so a stolen id can't rename someone
// else's passkey.
func (h *PasskeyHandler) Rename(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 {
		return responses.Unauthorized(c)
	}
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return responses.BadRequest(c, "invalid passkey id")
	}
	var body renameRequest
	if err := c.BodyParser(&body); err != nil {
		return responses.BadRequest(c, "invalid body")
	}
	if err := h.creds.UpdateNickname(c.Context(), uint(id), userID, strings.TrimSpace(body.Nickname)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return responses.NotFound(c, "passkey")
		}
		return responses.InternalError(c, "failed to rename passkey")
	}
	return c.JSON(fiber.Map{"id": id, "nickname": body.Nickname})
}

// Revoke deletes the passkey. The CASCADE on user_id in the table
// definition handles user-delete; this handles per-credential revoke
// from the management UI.
func (h *PasskeyHandler) Revoke(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 {
		return responses.Unauthorized(c)
	}
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return responses.BadRequest(c, "invalid passkey id")
	}
	if err := h.creds.Delete(c.Context(), uint(id), userID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return responses.NotFound(c, "passkey")
		}
		return responses.InternalError(c, "failed to revoke passkey")
	}
	meta := auth.RequestMeta{IPAddress: c.IP(), UserAgent: c.Get("User-Agent")}
	h.audit.PasskeyRevoked(c.Context(), userID, uint(id), meta)
	return c.SendStatus(fiber.StatusNoContent)
}

// ----- Login (anonymous; passkey IS the credential) -----

// BeginLogin issues PublicKeyCredentialRequestOptions for a
// discoverable login. No username required — the user picks their
// passkey in the browser dialog.
func (h *PasskeyHandler) BeginLogin(c *fiber.Ctx) error {
	assertion, session, err := h.engine.BeginLogin(c.Context())
	if err != nil {
		return responses.InternalError(c, "failed to begin passkey login: "+err.Error())
	}
	cookieValue, err := auth.EncodePasskeySession(session)
	if err != nil {
		return responses.InternalError(c, "failed to encode ceremony")
	}
	setCeremonyCookies(c, cookieValue, "")
	return c.JSON(fiber.Map{"options": assertion})
}

type finishLoginResponse struct {
	Token string   `json:"token"`
	User  fiber.Map `json:"user"`
}

// FinishLogin verifies the assertion, resolves the credential to a
// user, runs the login pipeline (which short-circuits the MFA gate
// for passkeys), and mints a session.
func (h *PasskeyHandler) FinishLogin(c *fiber.Ctx) error {
	cookieValue := c.Cookies(passkeyCeremonyCookie)
	session, err := auth.DecodePasskeySession(cookieValue)
	if err != nil {
		return responses.Error(c, fiber.StatusBadRequest, "passkey ceremony expired; please retry")
	}
	httpReq, herr := fiberToHTTPRequest(c)
	if herr != nil {
		return responses.InternalError(c, "failed to adapt request")
	}
	user, cred, err := h.engine.FinishLogin(c.Context(), session, httpReq)
	if err != nil {
		return responses.Error(c, fiber.StatusUnauthorized, "passkey login failed: "+err.Error())
	}
	clearCeremonyCookies(c)

	// Hand off to the pipeline so the session-mint + audit path is
	// shared with every other credential type. ProviderType=="passkey"
	// makes the pipeline skip auto-link, JIT, and the MFA gate.
	outcome := auth.SSOOutcome{
		ProviderType:    "passkey",
		ExternalSubject: fmt.Sprintf("%d", user.ID),
		Email:           user.Email,
		EmailVerified:   true,
	}
	meta := auth.RequestMeta{IPAddress: c.IP(), UserAgent: c.Get("User-Agent")}
	result, err := h.pipeline.Execute(c.Context(), outcome, meta)
	if err != nil {
		return responses.Error(c, fiber.StatusForbidden, err.Error())
	}
	h.audit.PasskeyUsed(c.Context(), user.ID, cred.ID, meta)

	c.Cookie(&fiber.Cookie{
		Name:     "paper_session",
		Value:    result.Token,
		Path:     "/",
		HTTPOnly: true,
		SameSite: "Lax",
		MaxAge:   86400,
		Expires:  time.Now().Add(24 * time.Hour),
	})
	return c.JSON(finishLoginResponse{
		Token: result.Token,
		User: fiber.Map{
			"id":         user.ID,
			"name":       user.Name,
			"login_id":   user.LoginID,
			"email":      user.Email,
			"role":       user.Role,
			"short_name": user.ShortName,
		},
	})
}

// ----- helpers -----

func setCeremonyCookies(c *fiber.Ctx, ceremony, nickname string) {
	expiry := time.Now().Add(75 * time.Second)
	c.Cookie(&fiber.Cookie{
		Name:     passkeyCeremonyCookie,
		Value:    ceremony,
		Path:     "/",
		HTTPOnly: true,
		SameSite: "Lax",
		Expires:  expiry,
	})
	if nickname != "" {
		c.Cookie(&fiber.Cookie{
			Name:     passkeyCeremonyCookie + "_nick",
			Value:    nickname,
			Path:     "/",
			HTTPOnly: true,
			SameSite: "Lax",
			Expires:  expiry,
		})
	}
}

func clearCeremonyCookies(c *fiber.Ctx) {
	for _, name := range []string{passkeyCeremonyCookie, passkeyCeremonyCookie + "_nick"} {
		c.Cookie(&fiber.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			HTTPOnly: true,
			MaxAge:   -1,
			Expires:  time.Now().Add(-1 * time.Hour),
		})
	}
}
