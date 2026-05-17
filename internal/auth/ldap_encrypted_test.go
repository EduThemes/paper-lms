package auth

import (
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// TestResolveLDAPBindPassword_PrefersEncrypted is the load-bearing
// assertion for Phase 9-PRE: when both columns are populated (e.g.
// during the rolling backfill) the read path takes the ciphertext
// branch, never the plaintext branch. Drift here would silently undo
// the encryption-at-rest contract.
func TestResolveLDAPBindPassword_PrefersEncrypted(t *testing.T) {
	setKey(t, make([]byte, 32))

	want := "rotated-bind-secret"
	ct, err := Encrypt([]byte(want))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	provider := &models.AuthenticationProvider{
		AuthType:                  "ldap",
		LDAPBindPassword:          "STALE-PLAINTEXT-MUST-NOT-WIN",
		LDAPBindPasswordEncrypted: ct,
	}

	got, err := resolveLDAPBindPassword(provider)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != want {
		t.Errorf("resolveLDAPBindPassword returned %q, want %q (the encrypted column must win when both are populated)", got, want)
	}
}

// TestResolveLDAPBindPassword_FallsBackToPlaintext covers the
// backward-compat bridge: a row that predates the encryption rotation
// still has its plaintext column populated and the encrypted column
// empty. Authentication must keep working until Wave-B drops the
// plaintext column.
func TestResolveLDAPBindPassword_FallsBackToPlaintext(t *testing.T) {
	setKey(t, make([]byte, 32))

	provider := &models.AuthenticationProvider{
		AuthType:         "ldap",
		LDAPBindPassword: "legacy-plaintext",
		// LDAPBindPasswordEncrypted left nil
	}

	got, err := resolveLDAPBindPassword(provider)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != "legacy-plaintext" {
		t.Errorf("resolveLDAPBindPassword fallback returned %q, want %q", got, "legacy-plaintext")
	}
}

// TestResolveLDAPBindPassword_EmptyBothReturnsEmpty covers the
// anonymous-search path: a provider configured without a service
// account binds anonymously. resolveLDAPBindPassword returns "" with
// no error so the LDAP authenticator's `bindDN != "" && password != ""`
// guard correctly skips the bind step.
func TestResolveLDAPBindPassword_EmptyBothReturnsEmpty(t *testing.T) {
	setKey(t, make([]byte, 32))

	provider := &models.AuthenticationProvider{AuthType: "ldap"}
	got, err := resolveLDAPBindPassword(provider)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty bind password for anonymous-bind provider, got %q", got)
	}
}

// TestResolveLDAPBindPassword_DecryptError surfaces a corrupt ciphertext
// as an error rather than silently falling back to the plaintext column
// — the fallback is a backward-compat bridge for un-rotated rows, not
// a recovery path for ciphertext corruption.
func TestResolveLDAPBindPassword_DecryptError(t *testing.T) {
	setKey(t, make([]byte, 32))

	provider := &models.AuthenticationProvider{
		AuthType:                  "ldap",
		LDAPBindPasswordEncrypted: []byte{0x01, 0x02, 0x03}, // too short to be a valid ciphertext
	}
	if _, err := resolveLDAPBindPassword(provider); err == nil {
		t.Error("expected decrypt error for malformed ciphertext, got nil")
	}
}

// TestEncryptLDAPBindPassword_RoundTrip is the end-to-end assertion of
// the Phase 9-PRE contract: a plaintext bind password sealed via
// secretbox.Encrypt and stored on the model returns the original
// plaintext when resolveLDAPBindPassword reads it back. Mirrors the
// handler's Create path.
func TestEncryptLDAPBindPassword_RoundTrip(t *testing.T) {
	setKey(t, make([]byte, 32))

	plaintext := "service-account-password-for-ldap"
	ct, err := Encrypt([]byte(plaintext))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	// Verify the stored bytes are NOT the plaintext — a regression
	// check against accidentally writing plaintext into the
	// "encrypted" column.
	if string(ct) == plaintext {
		t.Fatal("ciphertext bytes equal plaintext — encryption is a no-op")
	}

	// Simulate the post-Create row shape: encrypted column populated,
	// plaintext column blanked out by the handler.
	provider := &models.AuthenticationProvider{
		AuthType:                  "ldap",
		LDAPBindPassword:          "",
		LDAPBindPasswordEncrypted: ct,
	}

	got, err := resolveLDAPBindPassword(provider)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != plaintext {
		t.Errorf("round-trip: got %q, want %q", got, plaintext)
	}
}
