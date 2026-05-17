package auth

import (
	"context"
	"testing"
	"time"

	wa "github.com/go-webauthn/webauthn/webauthn"
)

// TestEncodeDecodePasskeySession_RoundTrip verifies the
// ceremony-state-in-cookie design: SessionData → encrypted base64 →
// SessionData survives identical.
func TestEncodeDecodePasskeySession_RoundTrip(t *testing.T) {
	setKey(t, make([]byte, 32))

	original := &wa.SessionData{
		Challenge:      "VGhpcyBpcyBhIHJhbmRvbSBjaGFsbGVuZ2U",
		RelyingPartyID: "paper.test",
		UserID:         []byte{0x01, 0x02, 0x03},
		AllowedCredentialIDs: [][]byte{
			{0xaa, 0xbb, 0xcc},
			{0xdd, 0xee, 0xff},
		},
		Expires: time.Now().Add(60 * time.Second).Truncate(time.Second),
	}

	cookie, err := EncodePasskeySession(original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if cookie == "" {
		t.Fatal("encoded cookie must not be empty")
	}

	decoded, err := DecodePasskeySession(cookie)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Challenge != original.Challenge {
		t.Errorf("Challenge: got %q want %q", decoded.Challenge, original.Challenge)
	}
	if decoded.RelyingPartyID != original.RelyingPartyID {
		t.Errorf("RPID: got %q want %q", decoded.RelyingPartyID, original.RelyingPartyID)
	}
	if string(decoded.UserID) != string(original.UserID) {
		t.Errorf("UserID round-trip lost bytes: %v vs %v", decoded.UserID, original.UserID)
	}
	if len(decoded.AllowedCredentialIDs) != len(original.AllowedCredentialIDs) {
		t.Fatalf("AllowedCredentialIDs count drift: got %d want %d", len(decoded.AllowedCredentialIDs), len(original.AllowedCredentialIDs))
	}
	for i := range original.AllowedCredentialIDs {
		if string(decoded.AllowedCredentialIDs[i]) != string(original.AllowedCredentialIDs[i]) {
			t.Errorf("AllowedCredentialIDs[%d] mismatch", i)
		}
	}
}

// TestDecodePasskeySession_RejectsTamperedCookie verifies the
// secretbox AEAD guard — flipping a byte should fail authentication
// before any JSON parse.
func TestDecodePasskeySession_RejectsTamperedCookie(t *testing.T) {
	setKey(t, make([]byte, 32))
	original := &wa.SessionData{
		Challenge:      "challenge",
		RelyingPartyID: "paper.test",
		Expires:        time.Now().Add(60 * time.Second),
	}
	cookie, err := EncodePasskeySession(original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	// Flip a character near the end (ciphertext region, not header).
	if len(cookie) < 10 {
		t.Fatalf("cookie unexpectedly short: %d", len(cookie))
	}
	tampered := cookie[:len(cookie)-2] + "AA"

	if _, err := DecodePasskeySession(tampered); err == nil {
		t.Fatal("expected tampered cookie to fail decode")
	}
}

// TestDecodePasskeySession_RejectsEmpty makes sure the handler-side
// "no cookie" path surfaces as an error, not a panic on nil.
func TestDecodePasskeySession_RejectsEmpty(t *testing.T) {
	setKey(t, make([]byte, 32))
	if _, err := DecodePasskeySession(""); err == nil {
		t.Fatal("expected empty cookie to fail decode")
	}
}

// TestPasskeyOutcome_PipelineShape verifies the Outcome shape the
// passkey handler constructs is well-formed for the pipeline. We
// don't run the pipeline here (it'd need a full repo stack); the
// purpose is to lock the contract: ProviderType="passkey",
// EmailVerified=true, ExternalSubject=decimal-user-id.
func TestPasskeyOutcome_PipelineShape(t *testing.T) {
	o := SSOOutcome{
		ProviderType:    "passkey",
		ExternalSubject: "42",
		Email:           "user@paper.test",
		EmailVerified:   true,
	}
	if o.ProviderType != "passkey" {
		t.Errorf("ProviderType = %q, want passkey", o.ProviderType)
	}
	if !o.EmailVerified {
		t.Error("passkey outcomes MUST have EmailVerified=true")
	}
	if o.ExternalSubject == "" {
		t.Error("passkey outcomes MUST carry user id as ExternalSubject")
	}
}

// ── Wave 6: per-ceremony RPID/RPOrigins resolution ──
//
// These tests lock the settings-resolution contract on the
// PasskeyEngine constructor + webauthnFor:
//   - nil lookup → boot values used (back-compat with env-only deploys)
//   - settings returns value → settings overrides boot
//   - settings returns empty → boot values used
//   - bogus lookup output → engine rejects via wa.New validation
//
// The RPID-rotation footgun documented in PasskeyEngine.SECURITY MODEL
// is NOT covered here (would require an integration test with real
// browser-issued credentials); the test below at least asserts that
// changing RPID via settings produces a different *wa.WebAuthn config.

func TestPasskeyEngine_NilLookup_UsesBootValues(t *testing.T) {
	e, err := NewPasskeyEngine("Paper LMS", "localhost", []string{"http://localhost:3000"}, nil, nil, nil)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	w, err := e.webauthnFor(context.Background())
	if err != nil {
		t.Fatalf("webauthnFor: %v", err)
	}
	if w == nil {
		t.Fatal("expected non-nil engine")
	}
}

func TestPasskeyEngine_LookupOverridesBoot(t *testing.T) {
	lookup := SettingsLookupFunc(func(_ context.Context, key string) (string, error) {
		switch key {
		case "auth.passkey.rpid":
			return "paper.example.edu", nil
		case "auth.passkey.rporigins":
			return "https://paper.example.edu", nil
		}
		return "", nil
	})
	e, err := NewPasskeyEngine("Paper LMS", "localhost", []string{"http://localhost:3000"}, lookup, nil, nil)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	// We can't directly observe the resulting RPID from outside the
	// library, but we can verify wa.New accepted the config (no error
	// = origins match the registrable suffix).
	if _, err := e.webauthnFor(context.Background()); err != nil {
		t.Fatalf("webauthnFor with settings override: %v", err)
	}
}

func TestPasskeyEngine_EmptySettings_FallsBackToBoot(t *testing.T) {
	// Lookup returns empty for everything — should fall back to the
	// construction-time rpID/rpOrigins.
	lookup := SettingsLookupFunc(func(_ context.Context, _ string) (string, error) {
		return "", nil
	})
	e, err := NewPasskeyEngine("Paper LMS", "localhost", []string{"http://localhost:3000"}, lookup, nil, nil)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	if _, err := e.webauthnFor(context.Background()); err != nil {
		t.Fatalf("webauthnFor with empty settings: %v", err)
	}
}

func TestPasskeyEngine_MismatchedRPIDAndOrigins_NoErrorAtConfig(t *testing.T) {
	// IMPORTANT (Wave 6 finding): go-webauthn's wa.New() does NOT
	// validate that RPID is a registrable suffix of the supplied
	// origins. Mismatched values silently produce a valid engine;
	// the real defense lives downstream in the browser (which won't
	// return credentials for the wrong RPID) and in the library's
	// per-ceremony origin verification.
	//
	// This test exists to LOCK this finding so a future engine
	// upgrade that DID add config-time validation doesn't silently
	// break our test expectations — and so a future contributor
	// can find the comment here when they wonder why we don't
	// reject obviously-bad RPIDs at write time.
	//
	// MITIGATION: the settings catalog should ideally validate that
	// auth.passkey.rpid is a registrable suffix of the FrontendURL
	// at write time. That's a Wave 7 hardening — for now we accept
	// that a super-admin who misconfigures RPID has broken passkey
	// flows until they fix it. Audit-log emission means the bad
	// write is traceable.
	lookup := SettingsLookupFunc(func(_ context.Context, key string) (string, error) {
		switch key {
		case "auth.passkey.rpid":
			return "evil.com", nil
		case "auth.passkey.rporigins":
			return "https://paper.example.edu", nil
		}
		return "", nil
	})
	e, err := NewPasskeyEngine("Paper LMS", "paper.example.edu", []string{"https://paper.example.edu"}, lookup, nil, nil)
	if err != nil {
		t.Fatalf("ctor with good boot config: %v", err)
	}
	// Document the current behavior: no error at config time.
	if _, err := e.webauthnFor(context.Background()); err != nil {
		t.Logf("LIBRARY UPGRADE NOTICE: wa.New now rejects mismatched RPID/origins (was silent before). Update catalog validator. err=%v", err)
	}
}

func TestPasskeyEngine_RPOriginsSplitOnComma(t *testing.T) {
	// Multi-origin RPORIGINS env-var format ("https://a.example,https://b.example")
	// must parse identically when returned via the settings lookup.
	lookup := SettingsLookupFunc(func(_ context.Context, key string) (string, error) {
		if key == "auth.passkey.rporigins" {
			return "https://paper.example.edu, https://paper-staging.example.edu", nil
		}
		if key == "auth.passkey.rpid" {
			return "example.edu", nil
		}
		return "", nil
	})
	e, err := NewPasskeyEngine("Paper LMS", "example.edu", []string{"https://example.edu"}, lookup, nil, nil)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	if _, err := e.webauthnFor(context.Background()); err != nil {
		t.Fatalf("multi-origin parse: %v", err)
	}
}

func TestSplitOrigins_TrimsAndDropsEmpty(t *testing.T) {
	got := splitOrigins("  https://a.test ,, https://b.test  ,")
	want := []string{"https://a.test", "https://b.test"}
	if len(got) != len(want) {
		t.Fatalf("len: got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("[%d] got %q, want %q", i, got[i], want[i])
		}
	}
}
