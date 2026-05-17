package auth

import (
	"strings"
	"testing"
)

// Wave 7 audit M1 regression — extractCertBase64 must refuse to embed
// arbitrary file contents into SAML metadata. A super-admin who points
// auth.saml.cert_file at /etc/shadow or a private-key file must NOT
// see those bytes echoed in the public /saml/metadata response.

func TestExtractCertBase64_RejectsNonPEM(t *testing.T) {
	// Garbage bytes — could be /etc/shadow, a binary file, anything.
	_, err := extractCertBase64("not even close to PEM\n")
	if err == nil {
		t.Fatal("expected error for non-PEM input")
	}
	if !strings.Contains(err.Error(), "PEM block") {
		t.Errorf("error should mention PEM block: %v", err)
	}
}

func TestExtractCertBase64_RejectsPrivateKey(t *testing.T) {
	// A PEM-encoded RSA PRIVATE KEY block (header bytes valid PEM
	// structure, but the wrong type). A super-admin who flipped
	// cert_file/key_file paths would hit this.
	keyPEM := `-----BEGIN RSA PRIVATE KEY-----
MIIBOgIBAAJBAKi4mGTGQXkPpL5LjK1bN4kxnT9rJDeKQ8FvKBV3HJh0EhAUhPp1
-----END RSA PRIVATE KEY-----`
	_, err := extractCertBase64(keyPEM)
	if err == nil {
		t.Fatal("expected error for PRIVATE KEY block")
	}
	if !strings.Contains(err.Error(), "CERTIFICATE") {
		t.Errorf("error should mention CERTIFICATE: %v", err)
	}
}

func TestExtractCertBase64_RejectsInvalidCertBody(t *testing.T) {
	// PEM block typed CERTIFICATE but with garbage body (not a real
	// X.509 DER). x509.ParseCertificate refuses.
	badCert := `-----BEGIN CERTIFICATE-----
bm90LXJlYWxseS1hLWNlcnQ=
-----END CERTIFICATE-----`
	_, err := extractCertBase64(badCert)
	if err == nil {
		t.Fatal("expected error for non-X.509 body")
	}
	if !strings.Contains(err.Error(), "X.509") {
		t.Errorf("error should mention X.509: %v", err)
	}
}

func TestExtractCertBase64_AcceptsValidCertificate(t *testing.T) {
	// Self-signed minimal cert generated for this test (SHA-256/RSA-
	// 2048). PEM-decodable, X.509-parseable.
	certPEM := `-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----`
	out, err := extractCertBase64(certPEM)
	if err != nil {
		t.Fatalf("valid certificate should be accepted: %v", err)
	}
	if out == "" {
		t.Error("expected non-empty base64 output")
	}
	// The output must NOT contain PEM headers — those are stripped.
	if strings.Contains(out, "BEGIN") || strings.Contains(out, "-----") {
		t.Error("output should be raw base64, not PEM")
	}
}

func TestExtractCertBase64_RejectsEmptyInput(t *testing.T) {
	_, err := extractCertBase64("")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestExtractCertBase64_RejectsBinaryGarbage(t *testing.T) {
	// Pretend the file at the configured path is a binary file
	// (e.g. /usr/bin/ls). pem.Decode returns nil block.
	_, err := extractCertBase64("\x00\x01\x02\x03binary garbage\xff\xfe")
	if err == nil {
		t.Fatal("expected error for binary input")
	}
}
