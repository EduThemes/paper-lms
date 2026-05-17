package initialpassword

import (
	"encoding/hex"
	"testing"
)

// TestGenerateInitialPassword_Length asserts the helper produces a
// 64-character output (32 bytes hex-encoded). Length is part of the
// public contract: callers and bcrypt downstream rely on it.
func TestGenerateInitialPassword_Length(t *testing.T) {
	pw, err := GenerateInitialPassword()
	if err != nil {
		t.Fatalf("GenerateInitialPassword: %v", err)
	}
	if got, want := len(pw), 64; got != want {
		t.Fatalf("length: got %d want %d", got, want)
	}
}

// TestGenerateInitialPassword_HexEncoded asserts the output is valid
// hex. bcrypt is binary-safe so this is belt-and-suspenders, but it
// also documents the encoding for callers grepping for the format.
func TestGenerateInitialPassword_HexEncoded(t *testing.T) {
	pw, err := GenerateInitialPassword()
	if err != nil {
		t.Fatalf("GenerateInitialPassword: %v", err)
	}
	if _, err := hex.DecodeString(pw); err != nil {
		t.Fatalf("output is not valid hex: %v (got %q)", err, pw)
	}
}

// TestGenerateInitialPassword_Distinct asserts two consecutive calls
// produce different values. This is the security-critical property:
// the prior code path generated "OneRoster-<sourcedId>-changeme" /
// "changeme" — deterministic and recoverable. The whole point of
// this helper is that an attacker who watches one provisioning run
// cannot predict the next.
func TestGenerateInitialPassword_Distinct(t *testing.T) {
	a, err := GenerateInitialPassword()
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	b, err := GenerateInitialPassword()
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if a == b {
		t.Fatalf("two calls returned the same password — entropy source is broken")
	}
}

// TestGenerateInitialPassword_NotLegacyLeakStrings is the regression
// lock for the two known recoverable-credential vulnerabilities this
// helper replaces:
//
//   - internal/service/oneroster_service.go used
//     "OneRoster-" + SourcedID + "-changeme" — derivable from a
//     public SIS identifier.
//   - internal/service/sis_import_service.go fell back to "changeme"
//     literally for any CSV row that omitted the password column.
//
// We can't iterate the SourcedID universe, but we can prove that for
// a fresh GenerateInitialPassword output, the static-literal "changeme"
// and a plausible derived string of the form
// "OneRoster-<any-sourcedid>-changeme" are NOT equal to the helper
// output. Equality probability under crypto/rand is ~2^-256; a test
// failure here means the helper is broken.
func TestGenerateInitialPassword_NotLegacyLeakStrings(t *testing.T) {
	pw, err := GenerateInitialPassword()
	if err != nil {
		t.Fatalf("GenerateInitialPassword: %v", err)
	}
	if pw == "changeme" {
		t.Fatalf("generated password equals the legacy SIS default 'changeme'")
	}
	for _, sid := range []string{
		"user-1", "user-42", "stu-abc-123", "12345", "alice@school.edu",
	} {
		guess := "OneRoster-" + sid + "-changeme"
		if pw == guess {
			t.Fatalf("generated password equals the legacy OneRoster derived string %q", guess)
		}
	}
}
