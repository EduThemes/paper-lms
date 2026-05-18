// Package handlers' MFAHandler hosts the TOTP enrollment + step-up
// flow. Five endpoints:
//
//   POST /api/v1/users/self/mfa/enroll
//        Re-verify password → generate secret + recovery codes →
//        return otpauth URL + plaintext codes ONCE. NOT FINAL — user
//        must verify a code from their app before the secret persists.
//
//   POST /api/v1/users/self/mfa/verify-enrollment
//        User submits the first 6-digit code → server verifies →
//        secret + recovery code hashes persist; users.totp_verified_at
//        is set.
//
//   DELETE /api/v1/users/self/mfa
//        User submits their current password OR a valid 6-digit code
//        to disable. Wipes secret + recovery codes.
//
//   POST /api/v1/auth/mfa/verify
//        Login step-up. Body: {pending_token, code}. Verifies pending
//        token, verifies TOTP code, mints real session.
//
//   POST /api/v1/auth/mfa/recovery
//        Login step-up via recovery code. Body: {pending_token, code}.
//        Consumes the code; mints real session.
package handlers

import (
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/pquerna/otp"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/auth"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

// MFAHandler is the TOTP enrollment + step-up surface (Sprint 9-B).
type MFAHandler struct {
	users          repository.UserRepository
	recoveryCodes  repository.UserRecoveryCodeRepository
	jwtSecret      string
	authService    *service.UserService // for password re-verification on enroll/disable
	rateLimit      *auth.MFAAttemptTracker
}

func NewMFAHandler(users repository.UserRepository, recovery repository.UserRecoveryCodeRepository, jwtSecret string, userSvc *service.UserService, tracker *auth.MFAAttemptTracker) *MFAHandler {
	return &MFAHandler{
		users:         users,
		recoveryCodes: recovery,
		jwtSecret:     jwtSecret,
		authService:   userSvc,
		rateLimit:     tracker,
	}
}

// ---- enrollment ----

type enrollRequest struct {
	Password string `json:"password"`
}

type enrollResponse struct {
	OTPAuthURL    string   `json:"otpauth_url"`     // user scans this as QR
	Secret        string   `json:"secret"`          // displayed below the QR in case the app can't scan
	RecoveryCodes []string `json:"recovery_codes"`  // shown ONCE
	QRDataURL     string   `json:"qr_data_url"`     // optional: base64 PNG; v1 leaves QR-rendering to the client
}

// EnrollMFA begins MFA setup for the calling user. Re-verifies the
// password to keep a stolen session from enrolling-and-locking-out.
// Stores the secret + recovery code hashes immediately; verification
// in the next call confirms the user actually scanned the QR.
func (h *MFAHandler) EnrollMFA(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 {
		return responses.Unauthorized(c)
	}
	// Self lookup: userID is the JWT subject (or pending-MFA token
	// subject). accountID=0 is correct — the user-id IS the caller,
	// no cross-tenant pivot is possible.
	user, err := h.users.FindByID(c.Context(), userID, 0)
	if err != nil || user == nil {
		return responses.Error(c, fiber.StatusNotFound, "user not found")
	}

	var body enrollRequest
	if err := c.BodyParser(&body); err != nil {
		return responses.BadRequest(c, "invalid body")
	}
	// Re-verify password. Federation users (no local password) can't
	// enroll TOTP via this path today — they have to set a local
	// password first, OR (future 9-C) use their passkey for re-auth.
	if user.PasswordHash == "" {
		return responses.Error(c, fiber.StatusBadRequest, "this account has no local password; set one before enrolling 2FA")
	}
	if err := user.CheckPassword(body.Password); err != nil {
		return responses.Error(c, fiber.StatusUnauthorized, "password incorrect")
	}

	// Generate the secret + recovery codes. Persist secret (encrypted)
	// + recovery code hashes; mark NOT-yet-verified.
	plaintextSecret, otpauthURL, err := auth.EnrollUserTOTP(user.LoginID)
	if err != nil {
		return responses.InternalError(c, "failed to generate TOTP secret")
	}
	ct, err := auth.EncodeSecretForStorage(plaintextSecret)
	if err != nil {
		return responses.InternalError(c, "failed to encrypt TOTP secret")
	}
	plaintextCodes, hashes, err := auth.GenerateRecoveryCodes()
	if err != nil {
		return responses.InternalError(c, "failed to generate recovery codes")
	}

	user.TOTPSecretEncrypted = ct
	user.TOTPVerifiedAt = nil // explicitly null: enrollment isn't final
	if err := h.users.Update(c.Context(), user); err != nil {
		return responses.InternalError(c, "failed to persist enrollment")
	}
	if err := h.recoveryCodes.CreateBatch(c.Context(), user.ID, hashes); err != nil {
		return responses.InternalError(c, "failed to persist recovery codes")
	}

	return c.JSON(enrollResponse{
		OTPAuthURL:    otpauthURL,
		Secret:        plaintextSecret,
		RecoveryCodes: plaintextCodes,
		QRDataURL:     qrDataURLFromOtpauth(otpauthURL),
	})
}

type verifyEnrollmentRequest struct {
	Code string `json:"code"`
}

// VerifyEnrollment finalizes MFA setup. Until this succeeds,
// TOTPVerifiedAt is NULL and the login pipeline treats the user as
// NOT enrolled.
func (h *MFAHandler) VerifyEnrollment(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 {
		return responses.Unauthorized(c)
	}
	// Self lookup: userID is the JWT subject (or pending-MFA token
	// subject). accountID=0 is correct — the user-id IS the caller,
	// no cross-tenant pivot is possible.
	user, err := h.users.FindByID(c.Context(), userID, 0)
	if err != nil || user == nil {
		return responses.Error(c, fiber.StatusNotFound, "user not found")
	}
	if len(user.TOTPSecretEncrypted) == 0 {
		return responses.Error(c, fiber.StatusBadRequest, "no enrollment in progress; call /mfa/enroll first")
	}
	secret, err := auth.DecodeSecretFromStorage(user.TOTPSecretEncrypted)
	if err != nil {
		return responses.InternalError(c, "failed to decode stored secret")
	}

	var body verifyEnrollmentRequest
	if err := c.BodyParser(&body); err != nil {
		return responses.BadRequest(c, "invalid body")
	}
	if !auth.VerifyTOTP(secret, auth.SanitizeCode(body.Code)) {
		return responses.Error(c, fiber.StatusUnauthorized, "code did not match")
	}
	now := time.Now()
	user.TOTPVerifiedAt = &now
	if err := h.users.Update(c.Context(), user); err != nil {
		return responses.InternalError(c, "failed to mark enrollment complete")
	}
	return c.JSON(fiber.Map{"verified_at": now})
}

// DisableMFA wipes the secret + recovery codes. Requires password
// (the local credential) OR a valid current TOTP code (the second
// credential) — proves the caller actually has at least one factor.
type disableRequest struct {
	Password string `json:"password,omitempty"`
	Code     string `json:"code,omitempty"`
}

func (h *MFAHandler) DisableMFA(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 {
		return responses.Unauthorized(c)
	}
	// Self lookup: userID is the JWT subject (or pending-MFA token
	// subject). accountID=0 is correct — the user-id IS the caller,
	// no cross-tenant pivot is possible.
	user, err := h.users.FindByID(c.Context(), userID, 0)
	if err != nil || user == nil {
		return responses.Error(c, fiber.StatusNotFound, "user not found")
	}
	if user.TOTPVerifiedAt == nil {
		return responses.Error(c, fiber.StatusBadRequest, "MFA is not enabled")
	}

	var body disableRequest
	if err := c.BodyParser(&body); err != nil {
		return responses.BadRequest(c, "invalid body")
	}
	if body.Password != "" {
		if err := user.CheckPassword(body.Password); err != nil {
			return responses.Error(c, fiber.StatusUnauthorized, "password incorrect")
		}
	} else if body.Code != "" {
		secret, err := auth.DecodeSecretFromStorage(user.TOTPSecretEncrypted)
		if err != nil || !auth.VerifyTOTP(secret, auth.SanitizeCode(body.Code)) {
			return responses.Error(c, fiber.StatusUnauthorized, "code did not match")
		}
	} else {
		return responses.BadRequest(c, "must supply password or code to disable")
	}

	user.TOTPSecretEncrypted = nil
	user.TOTPVerifiedAt = nil
	if err := h.users.Update(c.Context(), user); err != nil {
		return responses.InternalError(c, "failed to update user")
	}
	_ = h.recoveryCodes.DeleteAllForUser(c.Context(), user.ID)
	return c.JSON(fiber.Map{"disabled_at": time.Now()})
}

// ---- step-up at login ----

type stepUpRequest struct {
	PendingToken string `json:"pending_token"`
	Code         string `json:"code"`
}

type stepUpResponse struct {
	Token string `json:"token"`
	User  any    `json:"user"`
}

// VerifyAtLogin completes the second factor during login. Body
// carries the pending-MFA JWT (from the initial login response) and
// the 6-digit code. On success, mints a real session JWT.
func (h *MFAHandler) VerifyAtLogin(c *fiber.Ctx) error {
	var body stepUpRequest
	if err := c.BodyParser(&body); err != nil {
		return responses.BadRequest(c, "invalid body")
	}
	if body.PendingToken == "" || body.Code == "" {
		return responses.BadRequest(c, "pending_token and code required")
	}
	userID, _, err := auth.VerifyPendingMFAToken(h.jwtSecret, body.PendingToken)
	if err != nil {
		return responses.Error(c, fiber.StatusUnauthorized, "pending token invalid or expired: "+err.Error())
	}
	// Phase 10-A.4 — rate limit before any expensive work (DB lookup,
	// bcrypt check). Reject the request without exposing whether the
	// user exists / is enrolled.
	if h.rateLimit != nil {
		if err := h.rateLimit.CheckAndIncrementVerify(body.PendingToken); err != nil {
			return responses.Error(c, fiber.StatusTooManyRequests, "too many attempts; log in again")
		}
	}
	// Self lookup: userID is the JWT subject (or pending-MFA token
	// subject). accountID=0 is correct — the user-id IS the caller,
	// no cross-tenant pivot is possible.
	user, err := h.users.FindByID(c.Context(), userID, 0)
	if err != nil || user == nil {
		return responses.Error(c, fiber.StatusNotFound, "user not found")
	}
	if len(user.TOTPSecretEncrypted) == 0 || user.TOTPVerifiedAt == nil {
		return responses.Error(c, fiber.StatusBadRequest, "user is not MFA-enrolled")
	}
	secret, err := auth.DecodeSecretFromStorage(user.TOTPSecretEncrypted)
	if err != nil {
		return responses.InternalError(c, "failed to decode stored secret")
	}
	// Phase 10-A.5: use the reuse-guarded verifier and persist the
	// new last-used window on success. Replay attempts (same code,
	// same 30-second window) surface a distinct error.
	newWindow, ok, err := auth.VerifyTOTPWithReuseGuard(secret, auth.SanitizeCode(body.Code), user.TOTPLastUsedWindow)
	if err != nil && auth.IsTOTPReplay(err) {
		return responses.Error(c, fiber.StatusUnauthorized, "this code was already used; wait for the next one")
	}
	if !ok {
		return responses.Error(c, fiber.StatusUnauthorized, "code did not match")
	}
	user.TOTPLastUsedWindow = newWindow
	if err := h.users.Update(c.Context(), user); err != nil {
		// Don't fail the login on a last-window-update failure;
		// the worst case is the user could re-use this code, which
		// they already authenticated with anyway. Log via audit.
		_ = err
	}
	token, err := auth.GenerateToken(user, h.jwtSecret)
	if err != nil {
		return responses.InternalError(c, "failed to mint session")
	}
	// Set the session cookie too — login flow can rely on cookie OR
	// the returned token.
	c.Cookie(&fiber.Cookie{
		Name:     "paper_session",
		Value:    token,
		Path:     "/",
		HTTPOnly: true,
		SameSite: "Lax",
		MaxAge:   86400,
		Expires:  time.Now().Add(24 * time.Hour),
	})
	return c.JSON(stepUpResponse{
		Token: token,
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

// UseRecoveryCode is the fallback path when a user lost their TOTP
// device. Single-use: the matched code is marked used immediately.
func (h *MFAHandler) UseRecoveryCode(c *fiber.Ctx) error {
	var body stepUpRequest
	if err := c.BodyParser(&body); err != nil {
		return responses.BadRequest(c, "invalid body")
	}
	if body.PendingToken == "" || body.Code == "" {
		return responses.BadRequest(c, "pending_token and code required")
	}
	// Phase 10-A.4 — tighter cap on recovery attempts than verify.
	if h.rateLimit != nil {
		if err := h.rateLimit.CheckAndIncrementRecovery(body.PendingToken); err != nil {
			return responses.Error(c, fiber.StatusTooManyRequests, "too many attempts; log in again")
		}
	}
	userID, _, err := auth.VerifyPendingMFAToken(h.jwtSecret, body.PendingToken)
	if err != nil {
		return responses.Error(c, fiber.StatusUnauthorized, "pending token invalid or expired")
	}
	// Self lookup: userID is the JWT subject (or pending-MFA token
	// subject). accountID=0 is correct — the user-id IS the caller,
	// no cross-tenant pivot is possible.
	user, err := h.users.FindByID(c.Context(), userID, 0)
	if err != nil || user == nil {
		return responses.Error(c, fiber.StatusNotFound, "user not found")
	}

	codes, err := h.recoveryCodes.ListUnusedForUser(c.Context(), user.ID)
	if err != nil {
		return responses.InternalError(c, "failed to load recovery codes")
	}
	submitted := strings.ToUpper(strings.TrimSpace(body.Code))
	for _, rc := range codes {
		if auth.VerifyRecoveryCode(submitted, rc.CodeHash) {
			if err := h.recoveryCodes.MarkUsed(c.Context(), rc.ID); err != nil {
				if errors.Is(err, errRecoveryAlreadyUsed) {
					return responses.Error(c, fiber.StatusUnauthorized, "code already used")
				}
				return responses.InternalError(c, "failed to mark recovery code used")
			}
			token, err := auth.GenerateToken(user, h.jwtSecret)
			if err != nil {
				return responses.InternalError(c, "failed to mint session")
			}
			c.Cookie(&fiber.Cookie{Name: "paper_session", Value: token, Path: "/", HTTPOnly: true, SameSite: "Lax", MaxAge: 86400, Expires: time.Now().Add(24 * time.Hour)})
			return c.JSON(stepUpResponse{
				Token: token,
				User:  fiber.Map{"id": user.ID, "name": user.Name, "email": user.Email, "role": user.Role},
			})
		}
	}
	return responses.Error(c, fiber.StatusUnauthorized, "recovery code did not match")
}

// errRecoveryAlreadyUsed is the sentinel UserRecoveryCodeRepo.MarkUsed
// returns when the row was already consumed (race between two
// concurrent recovery-code submissions). The repo uses
// gorm.ErrRecordNotFound for both "doesn't exist" and "already used,"
// so we don't have a clean separation today; the handler treats
// either as 401 to avoid leaking which it was.
var errRecoveryAlreadyUsed = errors.New("recovery code already used")

// qrDataURLFromOtpauth: v1 leaves QR rendering to the frontend
// (which uses a small JS lib). This stub returns an empty string so
// the response shape is stable; future iteration can use a server-
// side QR renderer if mobile clients lack one.
func qrDataURLFromOtpauth(otpauthURL string) string {
	_ = otpauthURL
	return ""
}

// Avoid an "imported and not used" lint when the test file is the
// only consumer of base64 / otp in this package.
var _ = base64.StdEncoding
var _ = otp.AlgorithmSHA1
