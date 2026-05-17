package service

import (
	"context"
	"strings"
	"testing"
)

// Wave 5 — locks the SettingsLookupFunc-based SMTP resolution path.
// Verifies the new construction shape: empty lookup, valid lookup, and
// the int-port fallback when the catalog returns garbage.

func TestNotificationDeliveryService_NilLookup_ReturnsError(t *testing.T) {
	s := &NotificationDeliveryService{}
	_, err := s.resolveSMTPConfig(context.Background())
	if err == nil {
		t.Fatal("expected error when lookup is nil")
	}
	if !strings.Contains(err.Error(), "lookup not wired") {
		t.Errorf("expected 'lookup not wired' in error, got %q", err.Error())
	}
}

func TestNotificationDeliveryService_LookupAssemblesConfig(t *testing.T) {
	lookup := SettingsLookupFunc(func(_ context.Context, key string) (string, error) {
		switch key {
		case "smtp.host":
			return "mail.example.test", nil
		case "smtp.port":
			return "2525", nil
		case "smtp.username":
			return "noreply@example.test", nil
		case "smtp.password":
			return "hunter2", nil
		case "smtp.from":
			return "ops@example.test", nil
		case "smtp.enabled":
			return "true", nil
		}
		return "", nil
	})

	s := &NotificationDeliveryService{lookup: lookup}
	cfg, err := s.resolveSMTPConfig(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Host != "mail.example.test" {
		t.Errorf("host: got %q, want mail.example.test", cfg.Host)
	}
	if cfg.Port != 2525 {
		t.Errorf("port: got %d, want 2525", cfg.Port)
	}
	if cfg.Password != "hunter2" {
		t.Errorf("password: got %q, want hunter2", cfg.Password)
	}
	if !cfg.Enabled {
		t.Error("enabled should be true when lookup returns 'true'")
	}
}

func TestNotificationDeliveryService_UnparseablePortFallsBackTo587(t *testing.T) {
	lookup := SettingsLookupFunc(func(_ context.Context, key string) (string, error) {
		if key == "smtp.port" {
			return "not-a-number", nil // also covers ""
		}
		return "", nil
	})
	s := &NotificationDeliveryService{lookup: lookup}
	cfg, err := s.resolveSMTPConfig(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 587 {
		t.Errorf("port fallback: got %d, want 587", cfg.Port)
	}
}

// Wave 5 audit H1 regression — SMTP_ENABLED parse must be STRICT
// (literal lowercase "true" only). The previous draft accepted
// "True", "TRUE", " true " etc. Operators who used non-canonical
// casing as a soft-disable previously had SMTP off; the audit
// caught this drift before it shipped. Values from the super-admin
// UI are normalized to lowercase by the catalog validator, so this
// only matters for env-driven deployments.
func TestNotificationDeliveryService_EnabledIsStrictTrue(t *testing.T) {
	for _, raw := range []string{"true"} {
		t.Run("on/"+raw, func(t *testing.T) {
			lookup := SettingsLookupFunc(func(_ context.Context, key string) (string, error) {
				if key == "smtp.enabled" {
					return raw, nil
				}
				return "", nil
			})
			s := &NotificationDeliveryService{lookup: lookup}
			cfg, _ := s.resolveSMTPConfig(context.Background())
			if !cfg.Enabled {
				t.Errorf("enabled=%q should parse as true", raw)
			}
		})
	}
	// Every non-canonical form must parse OFF — these are the cases
	// the audit H1 fix locks down.
	for _, raw := range []string{"", "false", "FALSE", "True", "TRUE", " true ", " true", "true ", "yes", "1", "0"} {
		t.Run("off/"+raw, func(t *testing.T) {
			lookup := SettingsLookupFunc(func(_ context.Context, key string) (string, error) {
				if key == "smtp.enabled" {
					return raw, nil
				}
				return "", nil
			})
			s := &NotificationDeliveryService{lookup: lookup}
			cfg, _ := s.resolveSMTPConfig(context.Background())
			if cfg.Enabled {
				t.Errorf("enabled=%q must NOT parse as true (audit H1: strict literal lowercase only)", raw)
			}
		})
	}
}

func TestNotificationDeliveryService_LookupErrorPropagates(t *testing.T) {
	lookup := SettingsLookupFunc(func(_ context.Context, _ string) (string, error) {
		return "", &lookupError{msg: "DB down"}
	})
	s := &NotificationDeliveryService{lookup: lookup}
	_, err := s.resolveSMTPConfig(context.Background())
	if err == nil {
		t.Fatal("expected error to propagate")
	}
	if !strings.Contains(err.Error(), "DB down") {
		t.Errorf("expected wrapped error, got %q", err.Error())
	}
}

type lookupError struct{ msg string }

func (e *lookupError) Error() string { return e.msg }
