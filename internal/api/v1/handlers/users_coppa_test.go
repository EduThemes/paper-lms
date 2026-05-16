package handlers_test

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// setupUserHandlerCOPPA wires the user handler with the 13.4 COPPA
// signup gate dependencies plus the existing user repo. The age
// verification + parental consent repos are independent mocks so each
// test configures them on its own.
func setupUserHandlerCOPPA() (
	*fiber.App,
	*mocks.MockUserRepository,
	*mocks.MockAccountRepository,
	*mocks.MockAgeVerificationRepository,
	*mocks.MockParentalConsentRepository,
) {
	userRepo := new(mocks.MockUserRepository)
	accountRepo := new(mocks.MockAccountRepository)
	ageVerifyRepo := new(mocks.MockAgeVerificationRepository)
	consentRepo := new(mocks.MockParentalConsentRepository)

	userService := service.NewUserService(userRepo)
	h := handlers.NewUserHandler(userService, "test-jwt-secret", "test", nil, nil, nil).
		WithCOPPADeps(accountRepo, ageVerifyRepo, consentRepo, nil)

	app := testutil.SetupTestApp()
	app.Post("/register", h.Register)
	return app, userRepo, accountRepo, ageVerifyRepo, consentRepo
}

// TestRegister_HigherEdActiveSession — non-coppa tenant: no gate, the
// new row signs in with a token immediately (201 + token).
func TestRegister_HigherEdActiveSession(t *testing.T) {
	app, userRepo, accountRepo, _, _ := setupUserHandlerCOPPA()

	userRepo.On("FindByEmail", mock.Anything, "kid@example.com").Return(nil, errors.New("not found"))
	userRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil).Run(func(args mock.Arguments) {
		u := args.Get(1).(*models.User)
		u.ID = 100
		u.AccountID = 1
	})
	accountRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.Account{
		ID:          1,
		TenantMode:  models.GamificationAudience("higher_ed"),
		CoppaStrict: false,
	}, nil)

	body := testutil.JSONBody(map[string]string{
		"name":     "Kid Person",
		"email":    "kid@example.com",
		"password": "password123",
	})
	resp := testutil.MakeRequest(app, http.MethodPost, "/register", body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.NotEmpty(t, result["token"])
}

// TestRegister_CoppaStrictUnder13Pending — coppa_strict tenant + under
// 13 + no consent token = 201 with pending status, NO session token.
func TestRegister_CoppaStrictUnder13Pending(t *testing.T) {
	app, userRepo, accountRepo, ageVerifyRepo, _ := setupUserHandlerCOPPA()

	userRepo.On("FindByEmail", mock.Anything, "kid@example.com").Return(nil, errors.New("not found"))
	userRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil).Run(func(args mock.Arguments) {
		u := args.Get(1).(*models.User)
		u.ID = 100
		u.AccountID = 1
	})
	accountRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.Account{
		ID:          1,
		TenantMode:  models.GamificationAudience("higher_ed"),
		CoppaStrict: true,
	}, nil)
	ageVerifyRepo.On("FindByUserID", mock.Anything, uint(100)).Return(&models.AgeVerification{
		UserID: 100, IsUnder13: true,
	}, nil)
	// Update is called to set RequiresParentalConsent=true on the row.
	userRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)

	body := testutil.JSONBody(map[string]string{
		"name":     "Kid Person",
		"email":    "kid@example.com",
		"password": "password123",
	})
	resp := testutil.MakeRequest(app, http.MethodPost, "/register", body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.Empty(t, result["token"], "pending-consent registration must NOT mint a session token")
	userMap, ok := result["user"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, true, userMap["requires_parental_consent"])
}

// TestRegister_CoppaStrictUnder13WithValidToken — coppa_strict tenant
// + under 13 + valid granted consent token = active session.
func TestRegister_CoppaStrictUnder13WithValidToken(t *testing.T) {
	app, userRepo, accountRepo, ageVerifyRepo, consentRepo := setupUserHandlerCOPPA()

	userRepo.On("FindByEmail", mock.Anything, "kid@example.com").Return(nil, errors.New("not found"))
	userRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil).Run(func(args mock.Arguments) {
		u := args.Get(1).(*models.User)
		u.ID = 100
		u.AccountID = 1
	})
	accountRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.Account{
		ID:          1,
		TenantMode:  models.GamificationAudience("higher_ed"),
		CoppaStrict: true,
	}, nil)
	ageVerifyRepo.On("FindByUserID", mock.Anything, uint(100)).Return(&models.AgeVerification{
		UserID: 100, IsUnder13: true,
	}, nil)
	now := time.Now()
	consentRepo.On("FindByToken", mock.Anything, "tok-good").Return(&models.ParentalConsent{
		StudentID:   100,
		Status:      "granted",
		ConsentedAt: &now,
	}, nil)

	body := testutil.JSONBody(map[string]string{
		"name":                    "Kid Person",
		"email":                   "kid@example.com",
		"password":                "password123",
		"parental_consent_token":  "tok-good",
	})
	resp := testutil.MakeRequest(app, http.MethodPost, "/register", body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.NotEmpty(t, result["token"])
}

// TestRegister_K5TenantNoStrictAllows — tenant_mode k5 alone (without
// CoppaStrict) does NOT gate signup. The signup gate keys on
// CoppaStrict per the spec — tenant_mode independent rule applies to
// Conversations/LTI but not to public Register (where the user is
// presumed adult unless age verification says otherwise AND
// CoppaStrict is set).
func TestRegister_K5TenantNoStrictAllows(t *testing.T) {
	app, userRepo, accountRepo, _, _ := setupUserHandlerCOPPA()

	userRepo.On("FindByEmail", mock.Anything, "kid@example.com").Return(nil, errors.New("not found"))
	userRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil).Run(func(args mock.Arguments) {
		u := args.Get(1).(*models.User)
		u.ID = 100
		u.AccountID = 1
	})
	accountRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.Account{
		ID:          1,
		TenantMode:  models.GamificationAudience("k5"),
		CoppaStrict: false,
	}, nil)

	body := testutil.JSONBody(map[string]string{
		"name":     "Kid Person",
		"email":    "kid@example.com",
		"password": "password123",
	})
	resp := testutil.MakeRequest(app, http.MethodPost, "/register", body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.NotEmpty(t, result["token"])
}
