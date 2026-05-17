package handlers

// SSRF guard table-driven tests. We test the *unexported*
// validateExternalURL directly here (same package, no _test suffix)
// so we can cover the IP-classification logic without HTTP plumbing.
// The handler-level integration is covered in
// super_admin_settings_write_test.go via TestTestOIDC_Rejects*.

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestValidateExternalURL_RejectsScheme(t *testing.T) {
	for _, u := range []string{
		"http://example.com",
		"ftp://example.com",
		"file:///etc/passwd",
		"gopher://example.com",
		"//example.com",
		"javascript:alert(1)",
	} {
		t.Run(u, func(t *testing.T) {
			err := validateExternalURL(context.Background(), u)
			if !errors.Is(err, ErrSSRFBlocked) {
				t.Errorf("expected ErrSSRFBlocked for %q, got %v", u, err)
			}
		})
	}
}

func TestValidateExternalURL_RejectsLoopbackIP(t *testing.T) {
	for _, u := range []string{
		"https://127.0.0.1",
		"https://127.1.2.3",
		"https://[::1]",
	} {
		t.Run(u, func(t *testing.T) {
			err := validateExternalURL(context.Background(), u)
			if !errors.Is(err, ErrSSRFBlocked) {
				t.Errorf("expected ErrSSRFBlocked for %q, got %v", u, err)
			}
		})
	}
}

func TestValidateExternalURL_RejectsPrivateRanges(t *testing.T) {
	for _, u := range []string{
		"https://10.0.0.1",
		"https://10.255.255.255",
		"https://172.16.0.1",
		"https://172.31.255.255",
		"https://192.168.0.1",
		"https://[fc00::1]",
		"https://[fd00::1]",
	} {
		t.Run(u, func(t *testing.T) {
			err := validateExternalURL(context.Background(), u)
			if !errors.Is(err, ErrSSRFBlocked) {
				t.Errorf("expected ErrSSRFBlocked for %q, got %v", u, err)
			}
		})
	}
}

func TestValidateExternalURL_RejectsLinkLocalAndMetadata(t *testing.T) {
	// 169.254.169.254 is the canonical AWS/GCP/Azure instance
	// metadata endpoint — leaking this would expose IAM creds in
	// the cloud-hosted deployment topology.
	for _, u := range []string{
		"https://169.254.169.254",
		"https://169.254.0.1",
		"https://[fe80::1]",
	} {
		t.Run(u, func(t *testing.T) {
			err := validateExternalURL(context.Background(), u)
			if !errors.Is(err, ErrSSRFBlocked) {
				t.Errorf("expected ErrSSRFBlocked for %q, got %v", u, err)
			}
		})
	}
}

func TestValidateExternalURL_RejectsCGNATAndDocRanges(t *testing.T) {
	for _, u := range []string{
		"https://100.64.0.1",   // CGNAT
		"https://100.127.0.1",  // CGNAT
		"https://0.0.0.0",      // unspecified
		"https://192.0.2.1",    // TEST-NET-1
		"https://198.51.100.1", // TEST-NET-2
		"https://203.0.113.1",  // TEST-NET-3
	} {
		t.Run(u, func(t *testing.T) {
			err := validateExternalURL(context.Background(), u)
			if !errors.Is(err, ErrSSRFBlocked) {
				t.Errorf("expected ErrSSRFBlocked for %q, got %v", u, err)
			}
		})
	}
}

func TestValidateExternalURL_RejectsBlockedHostSuffixes(t *testing.T) {
	for _, u := range []string{
		"https://oidc.internal",
		"https://idp.local",
		"https://admin.intranet",
		"https://api.lan",
		"https://something.localhost",
		"https://broken.corp",
	} {
		t.Run(u, func(t *testing.T) {
			err := validateExternalURL(context.Background(), u)
			if !errors.Is(err, ErrSSRFBlocked) {
				t.Errorf("expected ErrSSRFBlocked for %q, got %v", u, err)
			}
		})
	}
}

func TestValidateExternalURL_RejectsNonStandardPort(t *testing.T) {
	for _, u := range []string{
		"https://example.com:80",
		"https://example.com:8443",
		"https://example.com:22",
		"https://example.com:3306",
	} {
		t.Run(u, func(t *testing.T) {
			err := validateExternalURL(context.Background(), u)
			if !errors.Is(err, ErrSSRFBlocked) {
				t.Errorf("expected ErrSSRFBlocked for %q, got %v", u, err)
			}
			if !strings.Contains(err.Error(), "443") {
				t.Errorf("error should mention 443: %v", err)
			}
		})
	}
}

func TestValidateExternalURL_RejectsBadInput(t *testing.T) {
	for _, u := range []string{
		"",
		"   ",
		"https://",       // no host
		"https:///path",  // no host
		"://example.com", // no scheme — but url.Parse may accept; the scheme check catches it
		"not-a-url-at-all",
	} {
		t.Run(u, func(t *testing.T) {
			err := validateExternalURL(context.Background(), u)
			if !errors.Is(err, ErrSSRFBlocked) {
				t.Errorf("expected ErrSSRFBlocked for %q, got %v", u, err)
			}
		})
	}
}

// Note: we don't include a positive test (a real public URL) because
// it would hit DNS at test time and flake on offline machines. The
// integration of the guard is covered by the handler-level tests in
// super_admin_settings_write_test.go where a localhost stub server
// is correctly rejected — proving the guard runs in the actual
// handler path.
