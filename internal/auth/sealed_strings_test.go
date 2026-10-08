package auth

import (
	"strings"
	"testing"
)

// B191: LTI 1.1 shared secrets are sealed before they reach the DB.
func TestSealString_RoundTripAndIdempotent(t *testing.T) {
	setKey(t, make([]byte, 32))

	sealed, err := SealString("lti-shared-secret")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if !IsSealedString(sealed) || strings.Contains(sealed, "lti-shared-secret") {
		t.Fatalf("expected sealed ciphertext, got %q", sealed)
	}
	again, err := SealString(sealed)
	if err != nil || again != sealed {
		t.Fatalf("sealing a sealed value must be a no-op; got %q, %v", again, err)
	}
	opened, err := OpenString(sealed)
	if err != nil || opened != "lti-shared-secret" {
		t.Fatalf("open: got %q, %v", opened, err)
	}
	if v, _ := SealString(""); v != "" {
		t.Fatalf("empty stays empty, got %q", v)
	}
	if v, _ := OpenString("legacy-plain"); v != "legacy-plain" {
		t.Fatalf("legacy plaintext passes through, got %q", v)
	}
}

func TestSealString_FailsClosedWithoutKey(t *testing.T) {
	t.Setenv(envKeyVar, "")
	resetKeys()
	if _, err := SealString("secret"); err == nil {
		t.Fatal("expected an error when no encryption key is configured")
	}
}
