package auth

import (
	"encoding/base64"
	"os"
	"sync"
	"testing"
)

// resetKeys lets each test reset the sync.Once-protected key cache.
// Production code calls loadKeys() once at startup; tests need to
// re-load with different env values.
func resetKeys() {
	keysOnce = sync.Once{}
	keys = nil
	keysErr = nil
}

func setKey(t *testing.T, raw32 []byte) {
	t.Helper()
	t.Setenv(envKeyVar, base64.StdEncoding.EncodeToString(raw32))
	resetKeys()
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	setKey(t, make([]byte, 32)) // all zeros — fine for a test
	plaintext := []byte("super-secret-totp-seed-MFRGGZDF")

	ct, err := Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if len(ct) <= len(plaintext) {
		t.Errorf("ciphertext should be longer than plaintext (header + nonce + tag); got %d vs %d", len(ct), len(plaintext))
	}
	if ct[0] != currentKeyID {
		t.Errorf("ciphertext header byte: want key_id %d, got %d", currentKeyID, ct[0])
	}

	pt, err := Decrypt(ct)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(pt) != string(plaintext) {
		t.Errorf("round-trip lost data: got %q, want %q", pt, plaintext)
	}
}

func TestEncrypt_NonceIsRandomEachCall(t *testing.T) {
	setKey(t, make([]byte, 32))
	plaintext := []byte("same input")
	a, err := Encrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Encrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) == string(b) {
		t.Error("two encrypts of the same plaintext produced identical ciphertexts — nonce reuse is catastrophic for GCM")
	}
}

func TestDecrypt_TamperedCiphertextRejected(t *testing.T) {
	setKey(t, make([]byte, 32))
	ct, err := Encrypt([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	// Flip a bit in the body.
	ct[len(ct)-1] ^= 0x01
	if _, err := Decrypt(ct); err == nil {
		t.Error("expected AEAD tag to reject tampered ciphertext, got nil")
	}
}

func TestDecrypt_UnknownKeyIDRejected(t *testing.T) {
	setKey(t, make([]byte, 32))
	ct, err := Encrypt([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	// Pretend this was written under key id 0x99 — no such key configured.
	ct[0] = 0x99
	if _, err := Decrypt(ct); err == nil {
		t.Error("expected decrypt to fail on unknown key id")
	}
}

func TestLoadKeys_FailsOnMissingEnv(t *testing.T) {
	t.Setenv(envKeyVar, "")
	resetKeys()
	if err := EnsureKeysLoaded(); err == nil {
		t.Error("expected error when MFA_ENCRYPTION_KEY is unset")
	}
}

func TestLoadKeys_FailsOnWrongLength(t *testing.T) {
	// 16 bytes is AES-128's size; we require 32 for AES-256.
	t.Setenv(envKeyVar, base64.StdEncoding.EncodeToString(make([]byte, 16)))
	resetKeys()
	if err := EnsureKeysLoaded(); err == nil {
		t.Error("expected error when key is not 32 bytes")
	}
}

func TestGenerateKey_Produces32ByteBase64(t *testing.T) {
	k, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(k)
	if err != nil {
		t.Fatalf("GenerateKey did not produce valid base64: %v", err)
	}
	if len(decoded) != 32 {
		t.Errorf("GenerateKey output: want 32 bytes, got %d", len(decoded))
	}
}

// Avoid an "imported and not used" lint when the test file is the only
// consumer of os in this package.
var _ = os.Getenv
