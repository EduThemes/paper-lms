package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// SAML provider attribution (B061 follow-up). The ACS verifies a response
// only against account 1's SAML providers, but the provider it hands the
// LoginPipeline decides the tenant the user is resolved in. A request-
// supplied provider_id must not be able to point that at another tenant's
// provider, and an unverified response must never reach the pipeline.
// All data is made up.

type fakeSAMLProviderRepo struct {
	items []models.AuthenticationProvider
}

func (f *fakeSAMLProviderRepo) Create(context.Context, *models.AuthenticationProvider) error {
	return nil
}
func (f *fakeSAMLProviderRepo) FindByID(_ context.Context, id uint) (*models.AuthenticationProvider, error) {
	for i := range f.items {
		if f.items[i].ID == id {
			return &f.items[i], nil
		}
	}
	return nil, nil
}
func (f *fakeSAMLProviderRepo) Update(context.Context, *models.AuthenticationProvider) error {
	return nil
}
func (f *fakeSAMLProviderRepo) Delete(context.Context, uint) error { return nil }
func (f *fakeSAMLProviderRepo) ListByAccountID(_ context.Context, accountID uint, _ repository.PaginationParams) (*repository.PaginatedResult[models.AuthenticationProvider], error) {
	var out []models.AuthenticationProvider
	for _, p := range f.items {
		if p.AccountID == accountID {
			out = append(out, p)
		}
	}
	return &repository.PaginatedResult[models.AuthenticationProvider]{Items: out, TotalCount: int64(len(out)), Page: 1, PerPage: 100}, nil
}
func (f *fakeSAMLProviderRepo) FindByAccountAndType(_ context.Context, accountID uint, authType string) ([]models.AuthenticationProvider, error) {
	var out []models.AuthenticationProvider
	for _, p := range f.items {
		if p.AccountID == accountID && p.AuthType == authType && p.WorkflowState != "deleted" {
			out = append(out, p)
		}
	}
	return out, nil
}

func resolveProviderIDFor(t *testing.T, repo *fakeSAMLProviderRepo, target string) uint {
	t.Helper()
	h := &SAMLHandler{authProviderRepo: repo}
	app := fiber.New()
	app.Post("/acs", func(c *fiber.Ctx) error {
		return c.SendString(fmt.Sprint(h.resolveProviderID(c)))
	})
	resp, err := app.Test(httptest.NewRequest(http.MethodPost, target, nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	var id uint
	_, _ = fmt.Sscan(string(body), &id)
	return id
}

func TestSAMLResolveProviderID_OnlyAccountOneSAMLProviders(t *testing.T) {
	repo := &fakeSAMLProviderRepo{items: []models.AuthenticationProvider{
		{ID: 3, AccountID: 1, AuthType: "saml", WorkflowState: "active"},
		{ID: 4, AccountID: 1, AuthType: "saml", WorkflowState: "active"},
		{ID: 7, AccountID: 2, AuthType: "saml", WorkflowState: "active"}, // another tenant
		{ID: 8, AccountID: 1, AuthType: "oidc", WorkflowState: "active"}, // wrong type
		{ID: 9, AccountID: 1, AuthType: "saml", WorkflowState: "deleted"},
	}}

	cases := []struct {
		target string
		want   uint
	}{
		{"/acs", 3},                 // default: first active account-1 SAML provider
		{"/acs?provider_id=4", 4},   // explicit, eligible
		{"/acs?provider_id=7", 0},   // other tenant's provider: refused
		{"/acs?provider_id=8", 0},   // not a SAML provider: refused
		{"/acs?provider_id=9", 0},   // deleted: refused
		{"/acs?provider_id=999", 0}, // unknown: refused (no silent fallback)
		{"/acs?provider_id=abc", 0}, // malformed: refused
	}
	for _, tc := range cases {
		if got := resolveProviderIDFor(t, repo, tc.target); got != tc.want {
			t.Errorf("%s: resolveProviderID = %d, want %d", tc.target, got, tc.want)
		}
	}
}

// buildUnsignedSAMLResponse returns a well-formed but UNSIGNED response for
// the given email, with a valid Conditions window.
func buildUnsignedSAMLResponse(email string) string {
	now := time.Now().UTC()
	xml := fmt.Sprintf(`<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" ID="_forged-%d" Version="2.0" IssueInstant="%s">`+
		`<samlp:Status><samlp:StatusCode Value="urn:oasis:names:tc:SAML:2.0:status:Success"/></samlp:Status>`+
		`<Assertion xmlns="urn:oasis:names:tc:SAML:2.0:assertion" ID="_forged-a-%d" Version="2.0" IssueInstant="%s">`+
		`<Issuer>https://idp.test</Issuer><Subject><NameID>%s</NameID></Subject>`+
		`<Conditions NotBefore="%s" NotOnOrAfter="%s"></Conditions>`+
		`</Assertion></samlp:Response>`,
		now.UnixNano(), now.Format(time.RFC3339), now.UnixNano(), now.Format(time.RFC3339), email,
		now.Add(-time.Minute).Format(time.RFC3339), now.Add(10*time.Minute).Format(time.RFC3339))
	return base64.StdEncoding.EncodeToString([]byte(xml))
}

func postACS(t *testing.T, h *SAMLHandler, query, samlResponse string) *http.Response {
	t.Helper()
	app := fiber.New()
	app.Post("/acs", h.HandleACS)
	form := url.Values{"SAMLResponse": {samlResponse}}
	req := httptest.NewRequest(http.MethodPost, "/acs"+query, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// Account 1 has no SAML provider (signature check is skipped), tenant 2
// has one. A forged, unsigned response that names tenant 2's provider must
// not log in as tenant 2's admin.
func TestSAMLACS_ForgedResponseCannotTargetAnotherTenantsProvider(t *testing.T) {
	h := newHarness(t, "off")
	victim := enrolledUser(60, "admin@school-b.test", "admin")
	victim.AccountID = 2
	h.users.put(victim)
	tenantB := provider(7, "saml", false)
	tenantB.AccountID = 2
	h.providers.byID[7] = tenantB

	repo := &fakeSAMLProviderRepo{items: []models.AuthenticationProvider{
		{ID: 7, AccountID: 2, AuthType: "saml", WorkflowState: "active"},
	}}
	sh := &SAMLHandler{authProviderRepo: repo, loginPipeline: h.pipeline}

	resp := postACS(t, sh, "?provider_id=7", buildUnsignedSAMLResponse(victim.Email))
	if resp.StatusCode == http.StatusFound {
		t.Fatalf("forged unsigned assertion produced a session (status %d)", resp.StatusCode)
	}
	for _, ck := range resp.Cookies() {
		if ck.Name == "paper_session" && ck.Value != "" {
			t.Fatal("forged unsigned assertion set a session cookie")
		}
	}
	if len(h.feds.created) != 0 {
		t.Errorf("no federation binding may be created, got %d", len(h.feds.created))
	}
}

// Account 1 has a SAML provider but no IdP certificate (and no SP entity
// id), so verifyResponseSignature returns no signed id. Such an unverified
// response must be refused before the pipeline, not used to log in.
func TestSAMLACS_UnverifiedResponseRefusedEvenForAccountOneProvider(t *testing.T) {
	h := newHarness(t, "off")
	victim := enrolledUser(61, "teacher@school-a.test", "user")
	h.users.put(victim)
	h.providers.byID[3] = provider(3, "saml", false)

	repo := &fakeSAMLProviderRepo{items: []models.AuthenticationProvider{
		{ID: 3, AccountID: 1, AuthType: "saml", WorkflowState: "active"}, // no IDPCertificate
	}}
	sh := &SAMLHandler{authProviderRepo: repo, loginPipeline: h.pipeline}

	resp := postACS(t, sh, "", buildUnsignedSAMLResponse(victim.Email))
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unverified response: status = %d, want 401", resp.StatusCode)
	}
	if len(h.feds.created) != 0 {
		t.Errorf("no federation binding may be created, got %d", len(h.feds.created))
	}
}

// The pipeline itself refuses a provider whose auth type differs from the
// verified credential's type (defense in depth for any handler that lets
// the caller name the provider).
func TestPipeline_ProviderTypeMismatchRefused(t *testing.T) {
	h := newHarness(t, "off")
	u := enrolledUser(62, "student@school-b.test", "user")
	u.AccountID = 2
	h.users.put(u)
	p := provider(8, "oidc", true)
	p.AccountID = 2
	h.providers.byID[8] = p

	_, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "saml", ProviderID: 8, ExternalSubject: u.Email,
		Email: u.Email, EmailVerified: true,
	}, RequestMeta{})
	if err == nil {
		t.Fatal("a saml outcome attributed to an oidc provider must be refused")
	}
}

func TestPipeline_InactiveProviderRefused(t *testing.T) {
	h := newHarness(t, "off")
	u := enrolledUser(63, "student@school-a.test", "user")
	h.users.put(u)
	p := provider(5, "oidc", false)
	p.WorkflowState = "deleted"
	h.providers.byID[5] = p

	_, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "oidc", ProviderID: 5, ExternalSubject: "sub-63",
		Email: u.Email, EmailVerified: true,
	}, RequestMeta{})
	if err == nil {
		t.Fatal("a deleted provider must not authenticate anyone")
	}
}
