package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// Tenant scoping of the LoginPipeline's federated paths (B061 / B067).
// Uses the fakes and harness from login_pipeline_test.go.

// B061: an IdP configured on tenant 2 asserts the verified email of a
// tenant-1 user. The email auto-link must NOT bind or log in as that user;
// with auto-provision off the login is refused outright.
func TestPipeline_EmailAutoLink_RefusesUserInAnotherTenant(t *testing.T) {
	h := newHarness(t, "off")
	victim := enrolledUser(50, "victim@school-a.test", "user") // account 1
	h.users.put(victim)
	p := provider(5, "oidc", false)
	p.AccountID = 2 // IdP controlled by tenant 2's admin
	h.providers.byID[5] = p

	res, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "oidc", ProviderID: 5, ExternalSubject: "other-sub",
		Email: victim.Email, EmailVerified: true,
	}, RequestMeta{})
	if err == nil {
		t.Fatalf("cross-tenant email auto-link must be refused, got session for user %d", res.User.ID)
	}
	if len(h.feds.created) != 0 {
		t.Errorf("no federation binding may be created across tenants, got %d", len(h.feds.created))
	}
}

// B061: the same assertion with auto-provision ON must provision a NEW
// tenant-2 user, never resolve to the tenant-1 user.
func TestPipeline_EmailAutoLink_CrossTenant_JITCreatesInProviderTenant(t *testing.T) {
	h := newHarness(t, "off")
	victim := enrolledUser(50, "victim@school-a.test", "user")
	h.users.put(victim)
	p := provider(5, "oidc", true)
	p.AccountID = 2
	h.providers.byID[5] = p

	res, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "oidc", ProviderID: 5, ExternalSubject: "other-sub",
		Email: victim.Email, EmailVerified: true,
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.User.ID == victim.ID {
		t.Fatal("cross-tenant IdP resolved to the other tenant's user")
	}
	if res.User.AccountID != 2 {
		t.Errorf("JIT user must be created in the provider's account 2, got %d", res.User.AccountID)
	}
}

// B061: same-tenant auto-link still works.
func TestPipeline_EmailAutoLink_SameTenantLinks(t *testing.T) {
	h := newHarness(t, "off")
	u := enrolledUser(50, "alice@school-b.test", "user")
	u.AccountID = 2
	h.users.put(u)
	p := provider(5, "oidc", false)
	p.AccountID = 2
	h.providers.byID[5] = p

	res, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "oidc", ProviderID: 5, ExternalSubject: "alice-sub",
		Email: u.Email, EmailVerified: true,
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("same-tenant auto-link: %v", err)
	}
	if res.User.ID != u.ID || len(h.feds.created) != 1 {
		t.Errorf("expected link to user %d with one binding, got user %d / %d bindings", u.ID, res.User.ID, len(h.feds.created))
	}
}

// B061: a super_admin is never auto-linked by email, even from an IdP in
// their own account.
func TestPipeline_EmailAutoLink_RefusesSuperAdmin(t *testing.T) {
	h := newHarness(t, "off")
	sa := enrolledUser(1, "ops@paper.test", "super_admin")
	h.users.put(sa)
	h.providers.byID[5] = provider(5, "oidc", true)

	_, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "oidc", ProviderID: 5, ExternalSubject: "idp-sub",
		Email: sa.Email, EmailVerified: true,
	}, RequestMeta{})
	if !errors.Is(err, ErrPrivilegedAutoLink) {
		t.Fatalf("expected ErrPrivilegedAutoLink, got %v", err)
	}
	if len(h.feds.created) != 0 {
		t.Errorf("no binding may be created for a super_admin")
	}
}

// Defense in depth: an existing federation row that points at a user in
// another tenant is not honored.
func TestPipeline_Federation_CrossTenantBindingRefused(t *testing.T) {
	h := newHarness(t, "off")
	u := enrolledUser(60, "bob@school-a.test", "user") // account 1
	h.users.put(u)
	p := provider(9, "cas", false)
	p.AccountID = 2
	h.providers.byID[9] = p
	h.feds.bySubject[fedKey(9, "bob")] = &models.FederatedIdentity{
		ID: 3, UserID: u.ID, ProviderID: 9, ExternalSubject: "bob",
	}

	_, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "cas", ProviderID: 9, ExternalSubject: "bob",
	}, RequestMeta{})
	if !errors.Is(err, ErrCrossTenantIdentity) {
		t.Fatalf("expected ErrCrossTenantIdentity, got %v", err)
	}
}

// B067: a JIT-provisioned user lands in the provider's tenant, not
// account 1.
func TestPipeline_JIT_CreatesUserInProviderAccount(t *testing.T) {
	h := newHarness(t, "off")
	p := provider(7, "saml", true)
	p.AccountID = 5
	h.providers.byID[7] = p

	res, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "saml", ProviderID: 7, ExternalSubject: "new-student@idp",
		Email: "new-student@school-e.test", EmailVerified: true, Name: "New Student",
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("jit: %v", err)
	}
	if len(h.users.created) != 1 || h.users.created[0].AccountID != 5 {
		t.Fatalf("JIT user must be created in account 5, got %+v", h.users.created)
	}
	if res.User.AccountID != 5 {
		t.Errorf("session user account = %d, want 5", res.User.AccountID)
	}
}
