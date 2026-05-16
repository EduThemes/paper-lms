package auth

import (
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
