// Package auth's webauthn surface implements the four passkey
// ceremonies — register/login × begin/finish — atop the
// go-webauthn library. Sprint 10-B introduces passkeys as
// **primary** credentials: a registered passkey replaces password +
// TOTP (the device's biometric/PIN is itself the second factor by
// definition).
//
// Why these wrappers exist instead of calling the library from the
// handlers directly:
//
//   - The library accepts a `User` interface that returns the user
//     handle + the user's full credential list. The pipeline needs
//     to resolve the user FROM the assertion (discoverable login),
//     not from a logged-in session. The library's
//     `FinishPasskeyLogin` takes a `DiscoverableUserHandler` for
//     exactly this — but our repos return models, not webauthn.User
//     impls. We wrap.
//
//   - The ceremony's `SessionData` (challenge + RP ID + allowed
//     credentials + expiration) needs to survive between the two
//     halves of the ceremony. Per the plan, v1 keeps this stateless:
//     marshal-encrypt-encode into a short-lived HttpOnly cookie via
//     `secretbox.Encrypt`. No DB table, no Redis dependency. The
//     cookie's 60-second TTL matches the library's challenge TTL.
//
//   - Encoding the SessionData involves JSON-marshaling a struct
//     with `[]byte` fields. Go's JSON encoder base64-encodes them
//     correctly, so round-trip is lossless.
package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-webauthn/webauthn/protocol"
	wa "github.com/go-webauthn/webauthn/webauthn"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

func b64EncodeURL(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
func b64DecodeURL(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

// PasskeyEngine bundles the library config + the repos it needs to
// resolve users and store/lookup credentials. Constructed once at
// boot.
type PasskeyEngine struct {
	w     *wa.WebAuthn
	users repository.UserRepository
	creds repository.UserWebauthnCredentialRepository
}

// NewPasskeyEngine builds the WebAuthn instance and binds the repos.
// rpID is the registrable suffix of the site origin (e.g. "localhost"
// for dev, "paper.example.edu" for prod). rpOrigins are the full
// origins the user agent connects from.
func NewPasskeyEngine(rpDisplayName, rpID string, rpOrigins []string, users repository.UserRepository, creds repository.UserWebauthnCredentialRepository) (*PasskeyEngine, error) {
	cfg := &wa.Config{
		RPDisplayName: rpDisplayName,
		RPID:          rpID,
		RPOrigins:     rpOrigins,
	}
	w, err := wa.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("webauthn config: %w", err)
	}
	return &PasskeyEngine{w: w, users: users, creds: creds}, nil
}

// ----- User adapter -----

// webauthnUser adapts a Paper LMS user + its credentials to the
// library's `User` interface.
type webauthnUser struct {
	id          []byte
	name        string
	displayName string
	credentials []wa.Credential
}

func (u *webauthnUser) WebAuthnID() []byte                  { return u.id }
func (u *webauthnUser) WebAuthnName() string                { return u.name }
func (u *webauthnUser) WebAuthnDisplayName() string         { return u.displayName }
func (u *webauthnUser) WebAuthnCredentials() []wa.Credential { return u.credentials }

func (e *PasskeyEngine) buildWebauthnUser(ctx context.Context, user *models.User) (*webauthnUser, error) {
	rows, err := e.creds.ListForUser(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	creds := make([]wa.Credential, 0, len(rows))
	for _, row := range rows {
		creds = append(creds, credentialFromModel(row))
	}
	display := user.Name
	if display == "" {
		display = user.Email
	}
	return &webauthnUser{
		id:          user.WebauthnUserHandle,
		name:        user.LoginID,
		displayName: display,
		credentials: creds,
	}, nil
}

func credentialFromModel(row models.UserWebauthnCredential) wa.Credential {
	transports := make([]protocol.AuthenticatorTransport, 0, len(row.Transports))
	for _, t := range row.Transports {
		transports = append(transports, protocol.AuthenticatorTransport(t))
	}
	return wa.Credential{
		ID:        row.CredentialID,
		PublicKey: row.PublicKeyCOSE,
		Transport: transports,
		Flags: wa.CredentialFlags{
			BackupEligible: row.BackupEligible,
			BackupState:    row.BackupState,
		},
		Authenticator: wa.Authenticator{
			AAGUID:    row.AAGUID,
			SignCount: row.SignCount,
		},
	}
}

// ----- Session encoding (stateless via secretbox + cookie) -----

// encodeSession marshals + encrypts the ceremony's SessionData so it
// can ride in a short-lived HttpOnly cookie. No server-side store.
func EncodePasskeySession(s *wa.SessionData) (string, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	ct, err := Encrypt(raw)
	if err != nil {
		return "", err
	}
	return b64EncodeURL(ct), nil
}

// decodeSession reverses EncodePasskeySession. Returns
// ErrPasskeySessionInvalid for any failure: forged cookie, expired
// key id, malformed JSON.
func DecodePasskeySession(cookie string) (*wa.SessionData, error) {
	if cookie == "" {
		return nil, ErrPasskeySessionInvalid
	}
	ct, err := b64DecodeURL(cookie)
	if err != nil {
		return nil, ErrPasskeySessionInvalid
	}
	raw, err := Decrypt(ct)
	if err != nil {
		return nil, ErrPasskeySessionInvalid
	}
	var s wa.SessionData
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, ErrPasskeySessionInvalid
	}
	return &s, nil
}

// ErrPasskeySessionInvalid is returned by DecodePasskeySession for
// any reason the cookie can't be trusted. The handler maps it to a
// 400 "ceremony expired or invalid; please retry" without leaking
// which specific failure happened.
var ErrPasskeySessionInvalid = errors.New("passkey ceremony session invalid or expired")

// ----- Registration -----

// BeginRegistration creates the PublicKeyCredentialCreationOptions
// for the user. Caller stores the returned session-cookie value (via
// EncodePasskeySession) on the response and sends the options to the
// browser.
//
// Excludes the user's existing credentials so they can't accidentally
// register the same authenticator twice.
func (e *PasskeyEngine) BeginRegistration(ctx context.Context, user *models.User) (*protocol.CredentialCreation, *wa.SessionData, error) {
	u, err := e.buildWebauthnUser(ctx, user)
	if err != nil {
		return nil, nil, err
	}
	opts := []wa.RegistrationOption{
		wa.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
		wa.WithExclusions(wa.Credentials(u.WebAuthnCredentials()).CredentialDescriptors()),
	}
	return e.w.BeginRegistration(u, opts...)
}

// FinishRegistration validates the browser's response against the
// session, and on success builds a UserWebauthnCredential row ready
// for insert. Caller persists the row.
//
// nickname is the user-facing label ("MacBook Touch ID"). It's
// optional but recommended — the management UI gets ugly without
// labels.
func (e *PasskeyEngine) FinishRegistration(ctx context.Context, user *models.User, session *wa.SessionData, r *http.Request, nickname string) (*models.UserWebauthnCredential, error) {
	u, err := e.buildWebauthnUser(ctx, user)
	if err != nil {
		return nil, err
	}
	cred, err := e.w.FinishRegistration(u, *session, r)
	if err != nil {
		return nil, err
	}
	row := credentialToModel(user.ID, cred, nickname)
	return row, nil
}

func credentialToModel(userID uint, cred *wa.Credential, nickname string) *models.UserWebauthnCredential {
	transports := make([]string, 0, len(cred.Transport))
	for _, t := range cred.Transport {
		transports = append(transports, string(t))
	}
	return &models.UserWebauthnCredential{
		UserID:         userID,
		CredentialID:   cred.ID,
		PublicKeyCOSE:  cred.PublicKey,
		SignCount:      cred.Authenticator.SignCount,
		AAGUID:         cred.Authenticator.AAGUID,
		Transports:     transports,
		Nickname:       nickname,
		BackupEligible: cred.Flags.BackupEligible,
		BackupState:    cred.Flags.BackupState,
	}
}

// ----- Login (discoverable — no username needed) -----

// BeginLogin issues the PublicKeyCredentialRequestOptions for a
// discoverable login. The user picks a passkey in the browser
// dialog; the assertion comes back with the credential_id, which we
// resolve to a user.
func (e *PasskeyEngine) BeginLogin(ctx context.Context) (*protocol.CredentialAssertion, *wa.SessionData, error) {
	return e.w.BeginDiscoverableLogin()
}

// FinishLogin verifies the assertion, resolves the user via the
// stored credential_id, and updates SignCount + LastUsedAt. Returns
// the user and the verified credential row.
//
// The library's DiscoverableUserHandler is how it asks us "given
// this user handle and raw credential id, who is the user?" — we
// look up the credential, then the user.
func (e *PasskeyEngine) FinishLogin(ctx context.Context, session *wa.SessionData, r *http.Request) (*models.User, *models.UserWebauthnCredential, error) {
	var (
		matchedUser *models.User
		matchedCred *models.UserWebauthnCredential
	)
	handler := func(rawID, userHandle []byte) (wa.User, error) {
		row, err := e.creds.FindByCredentialID(ctx, rawID)
		if err != nil {
			return nil, err
		}
		if row == nil {
			return nil, fmt.Errorf("unknown credential")
		}
		user, err := e.users.FindByID(ctx, row.UserID)
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, fmt.Errorf("user not found for credential")
		}
		// The user handle MUST match — defense in depth against a
		// credential pointing at a missing/rotated user.
		if len(userHandle) != 0 && !bytesEqual(userHandle, user.WebauthnUserHandle) {
			return nil, fmt.Errorf("user handle mismatch")
		}
		matchedUser = user
		matchedCred = row
		// Build the User adapter the library expects.
		return e.buildWebauthnUser(ctx, user)
	}

	verifiedCred, err := e.w.FinishDiscoverableLogin(handler, *session, r)
	if err != nil {
		return nil, nil, err
	}
	if matchedUser == nil || matchedCred == nil {
		return nil, nil, errors.New("passkey login: user resolution did not complete")
	}
	// Update the stored sign_count + last_used_at. The library has
	// already rejected the assertion if the counter regressed.
	if err := e.creds.UpdateSignCount(ctx, matchedCred.ID, verifiedCred.Authenticator.SignCount); err != nil {
		// Non-fatal: we still mint the session. SignCount drift on
		// the next login will be detected even without this row
		// update.
		_ = err
	}
	return matchedUser, matchedCred, nil
}

// ----- small helpers -----

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
