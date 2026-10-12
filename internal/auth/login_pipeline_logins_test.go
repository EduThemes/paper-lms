package auth

// G115 — an admin-created SSO login (Canvas pseudonym with an
// authentication provider) maps the provider's subject to a user before
// the email auto-link. All data is made up.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

type fakeProviderLogins struct {
	logins  []models.UserLogin
	touched []uint
}

func (f *fakeProviderLogins) FindForProvider(_ context.Context, providerID, accountID uint, subject string) (*models.UserLogin, error) {
	for i := range f.logins {
		l := &f.logins[i]
		if l.AuthenticationProviderID != nil && *l.AuthenticationProviderID == providerID &&
			l.AccountID == accountID && strings.EqualFold(l.UniqueID, subject) {
			return l, nil
		}
	}
	return nil, errors.New("not found")
}

func (f *fakeProviderLogins) TouchLastLogin(_ context.Context, id uint) {
	f.touched = append(f.touched, id)
}

func TestPipeline_ProviderLogin_ResolvesBySubjectOrLoginID(t *testing.T) {
	h := newHarness(t, "off")
	// The user's email differs from what the IdP asserts, and the IdP
	// doesn't vouch for email at all: only the explicit login matches.
	u := enrolledUser(7, "jane@school.test", "user")
	h.users.put(u)
	h.providers.byID[3] = provider(3, "ldap", false)
	pid := uint(3)
	logins := &fakeProviderLogins{logins: []models.UserLogin{{ID: 11, UserID: 7, AccountID: 1, UniqueID: "jdoe", AuthenticationProviderID: &pid}}}
	h.pipeline.SetProviderLogins(logins)

	res, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "ldap", ProviderID: 3, ExternalSubject: "uid=jdoe,ou=people,dc=school,dc=test",
		Email: "someone-else@school.test", EmailVerified: false,
		Attributes: map[string]any{"login_id": "JDOE"},
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.User.ID != 7 {
		t.Fatalf("resolved user %d, want 7", res.User.ID)
	}
	if len(h.feds.created) != 1 || h.feds.created[0].UserID != 7 {
		t.Fatalf("expected a federation binding for the login, got %+v", h.feds.created)
	}
	if len(logins.touched) != 1 {
		t.Fatal("login last_login_at not touched")
	}
}

func TestPipeline_ProviderLogin_NeverCrossesTenantsOrSignsInSuperAdmin(t *testing.T) {
	h := newHarness(t, "off")
	h.providers.byID[3] = provider(3, "cas", false)
	pid := uint(3)

	// A login row pointing at a user of another tenant is refused.
	other := enrolledUser(8, "x@other.test", "user")
	other.AccountID = 2
	h.users.put(other)
	h.pipeline.SetProviderLogins(&fakeProviderLogins{logins: []models.UserLogin{{ID: 1, UserID: 8, AccountID: 1, UniqueID: "x", AuthenticationProviderID: &pid}}})
	if _, err := h.pipeline.Execute(context.Background(), SSOOutcome{ProviderType: "cas", ProviderID: 3, ExternalSubject: "x"}, RequestMeta{}); !errors.Is(err, ErrCrossTenantIdentity) {
		t.Fatalf("want ErrCrossTenantIdentity, got %v", err)
	}

	root := enrolledUser(9, "root@school.test", "super_admin")
	h.users.put(root)
	h.pipeline.SetProviderLogins(&fakeProviderLogins{logins: []models.UserLogin{{ID: 2, UserID: 9, AccountID: 1, UniqueID: "root", AuthenticationProviderID: &pid}}})
	if _, err := h.pipeline.Execute(context.Background(), SSOOutcome{ProviderType: "cas", ProviderID: 3, ExternalSubject: "root"}, RequestMeta{}); !errors.Is(err, ErrPrivilegedAutoLink) {
		t.Fatalf("want ErrPrivilegedAutoLink, got %v", err)
	}
}
