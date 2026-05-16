package auth

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"gorm.io/datatypes"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// LoginPipeline matrix tests (Sprint 10-A.6).
//
// Covers the convergence-point logic across all five real
// credential types plus the future passkey case:
//
//   axis         | values
//   -------------+--------------------------------------------------
//   ProviderType | local / saml / ldap / cas / oidc / passkey
//   user state   | new / existing-local / existing-federated
//   email_verified | true / false
//   auto_provision | on / off
//   mfa_policy   | off / optional / required_admin / required_all
//   enrolled     | yes / no (totp_verified_at set or not)
//
// Not every combination is meaningful — `local` + `EmailVerified=false`
// is impossible by construction, JIT semantics only matter for
// federated paths, etc. The test file picks the ~25 representative
// cases that exercise distinct branches in `Execute` and
// `resolveUser`.
//
// In-test fakes (vs. testify Mock) keep the matrix readable. The
// existing MockUserRepository requires per-call argument expectations;
// for 25 cases that would dominate the test file. The fakes here
// track state in plain maps + slices so each case is one sentence
// of setup + one assertion.

// ----- fakes -----

type fakeUserRepo struct {
	byID    map[uint]*models.User
	byEmail map[string]*models.User
	created []*models.User
	nextID  uint
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		byID:    map[uint]*models.User{},
		byEmail: map[string]*models.User{},
		nextID:  100,
	}
}

func (f *fakeUserRepo) put(u *models.User) {
	if u.ID == 0 {
		f.nextID++
		u.ID = f.nextID
	}
	f.byID[u.ID] = u
	if u.Email != "" {
		f.byEmail[u.Email] = u
	}
}

func (f *fakeUserRepo) Create(ctx context.Context, u *models.User) error {
	f.nextID++
	u.ID = f.nextID
	f.byID[u.ID] = u
	if u.Email != "" {
		f.byEmail[u.Email] = u
	}
	f.created = append(f.created, u)
	return nil
}

func (f *fakeUserRepo) FindByID(ctx context.Context, id uint) (*models.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return nil, errors.New("user not found")
	}
	return u, nil
}

func (f *fakeUserRepo) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	u, ok := f.byEmail[email]
	if !ok {
		return nil, errors.New("user not found")
	}
	return u, nil
}

// Unused-but-required interface methods. The pipeline only calls the
// three above; everything else returns zero-values so the interface
// is satisfied for typing.
func (f *fakeUserRepo) FindByLoginID(context.Context, string) (*models.User, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeUserRepo) FindBySISUserID(context.Context, string) (*models.User, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeUserRepo) FindByIDs(context.Context, []uint) ([]models.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) Update(context.Context, *models.User) error                 { return nil }
func (f *fakeUserRepo) List(context.Context, repository.PaginationParams) (*repository.PaginatedResult[models.User], error) {
	return nil, nil
}
func (f *fakeUserRepo) FindByResetToken(context.Context, string) (*models.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) Search(context.Context, string, repository.PaginationParams) (*repository.PaginatedResult[models.User], error) {
	return nil, nil
}
func (f *fakeUserRepo) FilterPublicLeaderboardCandidates(context.Context, []uint) ([]uint, error) {
	return nil, nil
}

type fakeFederationRepo struct {
	bySubject map[string]*models.FederatedIdentity // key = providerID + ":" + subject
	created   []*models.FederatedIdentity
}

func newFakeFederationRepo() *fakeFederationRepo {
	return &fakeFederationRepo{bySubject: map[string]*models.FederatedIdentity{}}
}

func fedKey(providerID uint, subject string) string {
	return fmt.Sprintf("%d:%s", providerID, subject)
}

func (f *fakeFederationRepo) FindByProviderAndSubject(ctx context.Context, providerID uint, subject string) (*models.FederatedIdentity, error) {
	return f.bySubject[fedKey(providerID, subject)], nil
}

func (f *fakeFederationRepo) Create(ctx context.Context, fi *models.FederatedIdentity) error {
	fi.ID = uint(len(f.created) + 1)
	f.bySubject[fedKey(fi.ProviderID, fi.ExternalSubject)] = fi
	f.created = append(f.created, fi)
	return nil
}

func (f *fakeFederationRepo) TouchLastSeen(context.Context, uint, []byte) error { return nil }
func (f *fakeFederationRepo) ListForUser(context.Context, uint) ([]models.FederatedIdentity, error) {
	return nil, nil
}

type fakeProviderRepo struct {
	byID map[uint]*models.AuthenticationProvider
}

func newFakeProviderRepo() *fakeProviderRepo {
	return &fakeProviderRepo{byID: map[uint]*models.AuthenticationProvider{}}
}

func (f *fakeProviderRepo) FindByID(ctx context.Context, id uint) (*models.AuthenticationProvider, error) {
	return f.byID[id], nil
}

type fakeAccountRepo struct {
	policy string
}

func (f *fakeAccountRepo) FindByID(ctx context.Context, id uint) (*models.Account, error) {
	return &models.Account{ID: id, MFAPolicy: f.policy}, nil
}
func (f *fakeAccountRepo) Create(context.Context, *models.Account) error { return nil }
func (f *fakeAccountRepo) Update(context.Context, *models.Account) error { return nil }
func (f *fakeAccountRepo) List(context.Context, repository.PaginationParams) (*repository.PaginatedResult[models.Account], error) {
	return nil, nil
}

// ----- harness -----

type harness struct {
	users     *fakeUserRepo
	feds      *fakeFederationRepo
	providers *fakeProviderRepo
	accounts  *fakeAccountRepo
	pipeline  *LoginPipeline
}

func newHarness(t *testing.T, mfaPolicy string) *harness {
	t.Helper()
	// Encryption key for any path that touches secretbox (none here,
	// but Encrypt/Decrypt may be called by future pipeline code).
	setKey(t, make([]byte, 32))

	users := newFakeUserRepo()
	feds := newFakeFederationRepo()
	providers := newFakeProviderRepo()
	accounts := &fakeAccountRepo{policy: mfaPolicy}

	pipeline := NewLoginPipeline(
		users,
		feds,
		providers,
		accounts,
		NewAuthAudit(nil), // nil-svc shim: audit is no-op
		"test-jwt-secret",
	)
	return &harness{
		users:     users,
		feds:      feds,
		providers: providers,
		accounts:  accounts,
		pipeline:  pipeline,
	}
}

func enrolledUser(id uint, email, role string) *models.User {
	now := time.Now()
	return &models.User{ID: id, Email: email, LoginID: email, Name: "User " + email, Role: role, TOTPVerifiedAt: &now}
}

func unenrolledUser(id uint, email, role string) *models.User {
	return &models.User{ID: id, Email: email, LoginID: email, Name: "User " + email, Role: role}
}

func provider(id uint, ptype string, autoProvision bool) *models.AuthenticationProvider {
	return &models.AuthenticationProvider{ID: id, AuthType: ptype, AutoProvision: autoProvision}
}

// ----- the matrix -----

// Local-password path: caller resolved the user; pipeline just
// re-fetches by id, applies the gate, and mints.
func TestPipeline_Local_NoMFAPolicy_MintsSession(t *testing.T) {
	h := newHarness(t, "off")
	u := enrolledUser(1, "alice@paper.test", "user")
	h.users.put(u)

	res, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "local", ExternalSubject: "1", Email: u.Email, EmailVerified: true,
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.Token == "" || res.PendingToken != "" {
		t.Errorf("expected real Token, got %+v", res)
	}
}

func TestPipeline_Local_OptionalPolicyEnrolled_GatesToPending(t *testing.T) {
	h := newHarness(t, "optional")
	u := enrolledUser(1, "alice@paper.test", "user")
	h.users.put(u)

	res, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "local", ExternalSubject: "1", Email: u.Email, EmailVerified: true,
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.PendingToken == "" || res.Token != "" {
		t.Errorf("expected PendingToken, got %+v", res)
	}
}

func TestPipeline_Local_OptionalPolicyUnenrolled_MintsSession(t *testing.T) {
	h := newHarness(t, "optional")
	u := unenrolledUser(1, "alice@paper.test", "user")
	h.users.put(u)

	res, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "local", ExternalSubject: "1", Email: u.Email, EmailVerified: true,
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.Token == "" || res.PendingToken != "" {
		t.Errorf("optional + unenrolled should mint session, got %+v", res)
	}
}

func TestPipeline_Local_RequiredAllUnenrolled_MustEnrollFlag(t *testing.T) {
	h := newHarness(t, "required_all")
	u := unenrolledUser(1, "alice@paper.test", "user")
	h.users.put(u)

	res, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "local", ExternalSubject: "1", Email: u.Email, EmailVerified: true,
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !res.MustEnroll {
		t.Errorf("expected MustEnroll, got %+v", res)
	}
	if res.Token == "" {
		t.Error("expected session token even when MustEnroll (so user can reach /mfa/enroll)")
	}
}

func TestPipeline_Local_RequiredAdminEnrolled_PendingForAdmin(t *testing.T) {
	h := newHarness(t, "required_admin")
	u := enrolledUser(1, "admin@paper.test", "admin")
	h.users.put(u)

	res, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "local", ExternalSubject: "1", Email: u.Email, EmailVerified: true,
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.PendingToken == "" {
		t.Errorf("admin should hit MFA gate under required_admin policy, got %+v", res)
	}
}

func TestPipeline_Local_RequiredAdminEnrolled_NonAdminSkipped(t *testing.T) {
	h := newHarness(t, "required_admin")
	u := unenrolledUser(1, "alice@paper.test", "user")
	h.users.put(u)

	res, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "local", ExternalSubject: "1", Email: u.Email, EmailVerified: true,
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.Token == "" {
		t.Errorf("non-admin under required_admin should mint session, got %+v", res)
	}
}

// SAML / LDAP / CAS / OIDC — federated paths. Their semantics are
// identical from the pipeline's point of view; we vary ProviderType
// to make the test names speak.

func TestPipeline_OIDC_NewUser_AutoProvisionOn_Creates(t *testing.T) {
	h := newHarness(t, "off")
	h.providers.byID[5] = provider(5, "oidc", true)

	res, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType:    "oidc",
		ProviderID:      5,
		ExternalSubject: "google-sub-abc",
		Email:           "newuser@paper.test",
		EmailVerified:   true,
		Name:            "New User",
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.Token == "" {
		t.Error("expected session token")
	}
	if len(h.users.created) != 1 {
		t.Errorf("expected user to be JIT-created, got %d users", len(h.users.created))
	}
	if len(h.feds.created) != 1 {
		t.Errorf("expected federation row to be created, got %d", len(h.feds.created))
	}
}

func TestPipeline_OIDC_NewUser_AutoProvisionOff_Refused(t *testing.T) {
	h := newHarness(t, "off")
	h.providers.byID[5] = provider(5, "oidc", false)

	_, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType:    "oidc",
		ProviderID:      5,
		ExternalSubject: "google-sub-abc",
		Email:           "newuser@paper.test",
		EmailVerified:   true,
	}, RequestMeta{})
	if err == nil {
		t.Fatal("expected refusal when auto-provision is off and no existing user")
	}
}

func TestPipeline_OIDC_ExistingFederatedUser_Resolves(t *testing.T) {
	h := newHarness(t, "off")
	u := enrolledUser(50, "alice@paper.test", "user")
	h.users.put(u)
	h.providers.byID[5] = provider(5, "oidc", false)
	h.feds.bySubject[fedKey(5, "google-sub-abc")] = &models.FederatedIdentity{
		ID: 1, UserID: u.ID, ProviderID: 5, ExternalSubject: "google-sub-abc",
		ClaimsSnapshot: datatypes.JSON([]byte(`{}`)),
	}

	res, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "oidc", ProviderID: 5, ExternalSubject: "google-sub-abc",
		Email: u.Email, EmailVerified: true,
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.User.ID != u.ID {
		t.Errorf("expected resolved user %d, got %d", u.ID, res.User.ID)
	}
	if len(h.users.created) != 0 {
		t.Error("must not create a new user when federation row already binds")
	}
}

func TestPipeline_OIDC_EmailAutoLink_OnlyWhenEmailVerified(t *testing.T) {
	// Existing local user with the same email; auto_provision OFF.
	// EmailVerified=true → links. EmailVerified=false → refused.
	for _, verified := range []bool{true, false} {
		t.Run(fmt.Sprintf("verified=%v", verified), func(t *testing.T) {
			h := newHarness(t, "off")
			u := enrolledUser(50, "alice@paper.test", "user")
			h.users.put(u)
			h.providers.byID[5] = provider(5, "oidc", false)

			_, err := h.pipeline.Execute(context.Background(), SSOOutcome{
				ProviderType: "oidc", ProviderID: 5, ExternalSubject: "google-sub-new",
				Email: u.Email, EmailVerified: verified,
			}, RequestMeta{})
			if verified && err != nil {
				t.Fatalf("verified email should auto-link: %v", err)
			}
			if !verified && err == nil {
				t.Fatal("unverified email + auto_provision=off must refuse")
			}
			if verified && len(h.feds.created) != 1 {
				t.Errorf("expected federation binding created on auto-link, got %d", len(h.feds.created))
			}
		})
	}
}

func TestPipeline_SAML_EmailVerifiedAlwaysTrue_AutoLinks(t *testing.T) {
	// SAML handlers set EmailVerified=true unconditionally (the IdP
	// attested via signed assertion). Pipeline behavior is identical
	// to OIDC w/ verified email.
	h := newHarness(t, "off")
	u := enrolledUser(50, "alice@paper.test", "user")
	h.users.put(u)
	h.providers.byID[7] = provider(7, "saml", false)

	_, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "saml", ProviderID: 7, ExternalSubject: "alice@idp",
		Email: u.Email, EmailVerified: true,
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("saml auto-link: %v", err)
	}
	if len(h.feds.created) != 1 {
		t.Errorf("expected SAML auto-link to create federation row")
	}
}

func TestPipeline_LDAP_NewUser_JITCreates(t *testing.T) {
	h := newHarness(t, "off")
	h.providers.byID[8] = provider(8, "ldap", true)

	res, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "ldap", ProviderID: 8, ExternalSubject: "uid=alice,dc=paper,dc=test",
		Email: "alice@paper.test", EmailVerified: true, Name: "Alice",
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("ldap jit: %v", err)
	}
	if res.Token == "" {
		t.Error("expected session for JIT-created LDAP user")
	}
}

func TestPipeline_CAS_ExistingFederation_NoJIT(t *testing.T) {
	h := newHarness(t, "off")
	u := enrolledUser(60, "bob@paper.test", "user")
	h.users.put(u)
	h.providers.byID[9] = provider(9, "cas", true) // auto-provision irrelevant — federation hit
	h.feds.bySubject[fedKey(9, "bob")] = &models.FederatedIdentity{
		ID: 2, UserID: u.ID, ProviderID: 9, ExternalSubject: "bob",
	}

	_, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "cas", ProviderID: 9, ExternalSubject: "bob",
		Email: u.Email, EmailVerified: true,
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("cas existing: %v", err)
	}
	if len(h.users.created) != 0 {
		t.Error("CAS with existing federation must not create a user")
	}
}

// Passkey path (Sprint 10-B): ExternalSubject is user_id, no
// federation lookup, no JIT, and the MFA gate is skipped entirely.

func TestPipeline_Passkey_SkipsMFAGate_EvenUnderRequiredAll(t *testing.T) {
	h := newHarness(t, "required_all")
	u := unenrolledUser(1, "alice@paper.test", "user") // NOT enrolled
	h.users.put(u)

	res, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "passkey", ExternalSubject: "1", Email: u.Email, EmailVerified: true,
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("passkey: %v", err)
	}
	if res.Token == "" {
		t.Error("passkey login should produce session even under required_all + unenrolled")
	}
	if res.PendingToken != "" {
		t.Error("passkey path MUST NOT issue a pending-MFA token")
	}
	if res.MustEnroll {
		t.Error("passkey path MUST NOT set MustEnroll")
	}
}

func TestPipeline_Passkey_AdminEnrolled_StillSkipsGate(t *testing.T) {
	h := newHarness(t, "required_admin")
	u := enrolledUser(1, "admin@paper.test", "admin")
	h.users.put(u)

	res, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "passkey", ExternalSubject: "1", Email: u.Email, EmailVerified: true,
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("passkey admin: %v", err)
	}
	if res.PendingToken != "" {
		t.Error("passkey is itself two-factor — pipeline MUST short-circuit the gate")
	}
}

func TestPipeline_Passkey_MissingUserID_Errors(t *testing.T) {
	h := newHarness(t, "off")
	_, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "passkey", ExternalSubject: "", Email: "x@paper.test",
	}, RequestMeta{})
	if err == nil {
		t.Fatal("expected error when passkey outcome carries no user id")
	}
}

// Local path edge cases.

func TestPipeline_Local_MissingUserID_Errors(t *testing.T) {
	h := newHarness(t, "off")
	_, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "local", ExternalSubject: "", Email: "x@paper.test",
	}, RequestMeta{})
	if err == nil {
		t.Fatal("expected error when local outcome carries no user id")
	}
}

// JIT semantics: provider context required when no federation/auto-link.

func TestPipeline_Federated_NoProviderContext_Refused(t *testing.T) {
	h := newHarness(t, "off")
	_, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "oidc", ProviderID: 0, ExternalSubject: "stranger",
		Email: "stranger@paper.test", EmailVerified: false,
	}, RequestMeta{})
	if err == nil {
		t.Fatal("expected refusal when no provider context AND no federation/auto-link path")
	}
}

// Enrollment touching: required_admin + admin + unenrolled should
// mint a session + flag MustEnroll (not block — block-and-redirect
// is the frontend's job, but the pipeline's contract is "mint, flag").
func TestPipeline_Local_RequiredAdminAdminUnenrolled_MustEnroll(t *testing.T) {
	h := newHarness(t, "required_admin")
	u := unenrolledUser(1, "admin@paper.test", "admin")
	h.users.put(u)

	res, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "local", ExternalSubject: "1", Email: u.Email, EmailVerified: true,
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !res.MustEnroll {
		t.Error("required_admin + admin + unenrolled → MustEnroll")
	}
	if res.Token == "" {
		t.Error("MustEnroll path still mints session so the user can reach /mfa/enroll")
	}
}

// Sprint 10-C — lock the SSOOutcome shapes the refactored SAML, LDAP,
// and CAS handlers emit. These tests don't run the protocol-side code
// (XML parsing, LDAP bind, ticket validation); they assert that the
// outcome a passing protocol verifier would hand the pipeline is
// well-formed and produces the right downstream behavior.
//
// The protocol-side tests still live in saml/ldap/cas test files (or
// will, when integration coverage is added) — those exercise the
// cryptographic and bind layers, which 10-C did not touch.

func TestPipeline_SAML_NewUser_JITCreates_FromNameID(t *testing.T) {
	h := newHarness(t, "off")
	h.providers.byID[7] = provider(7, "saml", true)

	res, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType:    "saml",
		ProviderID:      7,
		ExternalSubject: "alice@idp",
		Email:           "alice@paper.test",
		EmailVerified:   true, // SAML IdP attested via signed assertion
		Name:            "Alice Smith",
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("saml jit: %v", err)
	}
	if res.Token == "" {
		t.Error("expected session token")
	}
	if len(h.users.created) != 1 {
		t.Errorf("expected JIT-created user, got %d", len(h.users.created))
	}
	if len(h.feds.created) != 1 {
		t.Errorf("expected federation row for SAML NameID, got %d", len(h.feds.created))
	}
	if h.feds.created[0].ExternalSubject != "alice@idp" {
		t.Errorf("federation ExternalSubject should match SAML NameID, got %q", h.feds.created[0].ExternalSubject)
	}
}

func TestPipeline_LDAP_NewUser_JITCreates_FromDN(t *testing.T) {
	h := newHarness(t, "off")
	h.providers.byID[8] = provider(8, "ldap", true)

	// The LDAP authenticator passes the full DN as ExternalSubject.
	res, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType:    "ldap",
		ProviderID:      8,
		ExternalSubject: "uid=alice,ou=people,dc=paper,dc=test",
		Email:           "alice@paper.test",
		EmailVerified:   true,
		Name:            "Alice Smith",
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("ldap jit: %v", err)
	}
	if res.Token == "" {
		t.Error("expected session token")
	}
	if h.feds.created[0].ExternalSubject != "uid=alice,ou=people,dc=paper,dc=test" {
		t.Error("LDAP federation must carry the full DN as ExternalSubject")
	}
}

func TestPipeline_CAS_NewUser_JITCreates_FromPrincipal(t *testing.T) {
	h := newHarness(t, "off")
	h.providers.byID[9] = provider(9, "cas", true)

	res, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType:    "cas",
		ProviderID:      9,
		ExternalSubject: "alice",
		Email:           "alice@paper.test",
		EmailVerified:   true,
		Name:            "Alice Smith",
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("cas jit: %v", err)
	}
	if res.Token == "" {
		t.Error("expected session token")
	}
	if h.feds.created[0].ExternalSubject != "alice" {
		t.Error("CAS federation must carry the principal name as ExternalSubject")
	}
}

func TestPipeline_SAML_ExistingFederatedUser_NoJIT(t *testing.T) {
	h := newHarness(t, "off")
	u := enrolledUser(50, "alice@paper.test", "user")
	h.users.put(u)
	h.providers.byID[7] = provider(7, "saml", false) // auto-provision off; should still resolve via federation
	h.feds.bySubject[fedKey(7, "alice@idp")] = &models.FederatedIdentity{
		ID: 1, UserID: u.ID, ProviderID: 7, ExternalSubject: "alice@idp",
	}

	res, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "saml", ProviderID: 7, ExternalSubject: "alice@idp",
		Email: u.Email, EmailVerified: true,
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("saml federated: %v", err)
	}
	if res.User.ID != u.ID {
		t.Errorf("expected resolved user %d, got %d", u.ID, res.User.ID)
	}
	if len(h.users.created) != 0 {
		t.Error("SAML with existing federation must not JIT")
	}
}

func TestPipeline_LDAP_AdminUnderRequiredAdmin_MFAGate(t *testing.T) {
	// LDAP login by an admin under required_admin policy should
	// gate into MFA just like local-password — the pipeline doesn't
	// care which credential type opened the door, only the user's
	// role + tenant policy.
	h := newHarness(t, "required_admin")
	u := enrolledUser(60, "admin@paper.test", "admin")
	h.users.put(u)
	h.providers.byID[8] = provider(8, "ldap", false)
	h.feds.bySubject[fedKey(8, "uid=admin,dc=paper,dc=test")] = &models.FederatedIdentity{
		ID: 1, UserID: u.ID, ProviderID: 8, ExternalSubject: "uid=admin,dc=paper,dc=test",
	}

	res, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "ldap", ProviderID: 8,
		ExternalSubject: "uid=admin,dc=paper,dc=test",
		Email:           u.Email, EmailVerified: true,
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("ldap admin: %v", err)
	}
	if res.PendingToken == "" {
		t.Error("LDAP login by admin under required_admin must hit MFA gate")
	}
}

func TestPipeline_CAS_EmailAutoLink_OnlyWhenEmailVerified(t *testing.T) {
	// CAS always sets EmailVerified=true (ticket validated against
	// the directory). Auto-link path should succeed.
	h := newHarness(t, "off")
	u := enrolledUser(70, "carol@paper.test", "user")
	h.users.put(u)
	h.providers.byID[9] = provider(9, "cas", false) // auto-provision off — relying on auto-link

	res, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "cas", ProviderID: 9, ExternalSubject: "carol",
		Email: u.Email, EmailVerified: true,
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("cas auto-link: %v", err)
	}
	if res.User.ID != u.ID {
		t.Errorf("expected auto-link to existing user, got %d", res.User.ID)
	}
	if len(h.feds.created) != 1 {
		t.Error("CAS auto-link must create the federation row for next time")
	}
}

// JIT-created user's role default.
func TestPipeline_JIT_DefaultsToUserRole(t *testing.T) {
	h := newHarness(t, "off")
	h.providers.byID[5] = provider(5, "oidc", true)

	_, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "oidc", ProviderID: 5, ExternalSubject: "sub",
		Email: "fresh@paper.test", EmailVerified: true, Name: "Fresh User",
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("jit: %v", err)
	}
	if h.users.created[0].Role != "user" {
		t.Errorf("JIT default role should be 'user', got %q", h.users.created[0].Role)
	}
}
