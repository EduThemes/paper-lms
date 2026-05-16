package handlers_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// setupLTIHandlerForGate wires an LTIHandler with only the gate-relevant
// repos populated. The service slots stay nil; the gate runs before
// any service call, so OIDCLogin's BadRequest fallback is the success
// signal when the gate allows the request through.
func setupLTIHandlerForGate() (
	*fiber.App,
	*mocks.MockUserRepository,
	*mocks.MockAccountRepository,
	*mocks.MockParentalConsentRepository,
) {
	userRepo := new(mocks.MockUserRepository)
	accountRepo := new(mocks.MockAccountRepository)
	consentRepo := new(mocks.MockParentalConsentRepository)

	h := handlers.NewLTIHandler(nil, nil, nil, nil, nil, userRepo, accountRepo, consentRepo)

	app := testutil.SetupTestApp()
	// LTI launches are PUBLIC routes; no auth middleware.
	// Launch is the gate's primary mount; after the gate it hits a
	// BadRequest on empty lti_message_hint, which keeps the test path
	// safe (no nil-service panic) while still exercising the gate.
	app.Post("/lti/launch", h.Launch)
	return app, userRepo, accountRepo, consentRepo
}

// jsonLaunchBody returns a JSON body for a Launch call.
func jsonLaunchBody(loginHint string) interface{} {
	return map[string]interface{}{
		"client_id":  "abc",
		"login_hint": loginHint,
	}
}

// TestLTIOIDCLogin_HigherEdAllows — the gate is bypassed for non-COPPA
// tenants. The handler then drops to its service call (nil here), which
// would panic or 500 — but the gate test only needs NOT-403 to pass.
func TestLTIOIDCLogin_HigherEdAllows(t *testing.T) {
	app, userRepo, accountRepo, _ := setupLTIHandlerForGate()

	userRepo.On("FindByID", mock.Anything, uint(7)).Return(&models.User{ID: 7, AccountID: 1}, nil)
	accountRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.Account{
		ID:          1,
		TenantMode:  models.GamificationAudience("higher_ed"),
		CoppaStrict: false,
	}, nil)

	resp := testutil.MakeRequest(app, http.MethodPost, "/lti/launch", testutil.JSONBody(jsonLaunchBody("7")))
	// Gate let it through; downstream nil-service path produces a non-403.
	assert.NotEqual(t, http.StatusForbidden, resp.StatusCode)
}

// TestLTIOIDCLogin_K5RefusesWithoutConsent — k5 tenant, no granted
// third_party_sharing consent = 403.
func TestLTIOIDCLogin_K5RefusesWithoutConsent(t *testing.T) {
	app, userRepo, accountRepo, consentRepo := setupLTIHandlerForGate()

	userRepo.On("FindByID", mock.Anything, uint(7)).Return(&models.User{ID: 7, AccountID: 1}, nil)
	accountRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.Account{
		ID:          1,
		TenantMode:  models.GamificationAudience("k5"),
		CoppaStrict: false,
	}, nil)
	consentRepo.On("FindByStudentID", mock.Anything, uint(7)).Return([]models.ParentalConsent{}, nil)

	resp := testutil.MakeRequest(app, http.MethodPost, "/lti/launch", testutil.JSONBody(jsonLaunchBody("7")))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// TestLTIOIDCLogin_CoppaStrictRefuses — higher_ed tenant_mode but
// CoppaStrict=true = 403.
func TestLTIOIDCLogin_CoppaStrictRefuses(t *testing.T) {
	app, userRepo, accountRepo, consentRepo := setupLTIHandlerForGate()

	userRepo.On("FindByID", mock.Anything, uint(7)).Return(&models.User{ID: 7, AccountID: 1}, nil)
	accountRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.Account{
		ID:          1,
		TenantMode:  models.GamificationAudience("higher_ed"),
		CoppaStrict: true,
	}, nil)
	consentRepo.On("FindByStudentID", mock.Anything, uint(7)).Return([]models.ParentalConsent{}, nil)

	resp := testutil.MakeRequest(app, http.MethodPost, "/lti/launch", testutil.JSONBody(jsonLaunchBody("7")))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// TestLTIOIDCLogin_K5AllowsWithGrantedConsent — k5 tenant + granted
// third_party_sharing consent = bypass the 403.
func TestLTIOIDCLogin_K5AllowsWithGrantedConsent(t *testing.T) {
	app, userRepo, accountRepo, consentRepo := setupLTIHandlerForGate()

	userRepo.On("FindByID", mock.Anything, uint(7)).Return(&models.User{ID: 7, AccountID: 1}, nil)
	accountRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.Account{
		ID:          1,
		TenantMode:  models.GamificationAudience("k5"),
		CoppaStrict: false,
	}, nil)
	now := time.Now()
	consentRepo.On("FindByStudentID", mock.Anything, uint(7)).Return([]models.ParentalConsent{
		{StudentID: 7, ConsentType: "third_party_sharing", Status: "granted", ConsentedAt: &now},
	}, nil)

	resp := testutil.MakeRequest(app, http.MethodPost, "/lti/launch", testutil.JSONBody(jsonLaunchBody("7")))
	assert.NotEqual(t, http.StatusForbidden, resp.StatusCode)
}
