package handlers_test

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// setupConversationHandler wires the ConversationHandler against mocks
// for the 13.4 COPPA gate test matrix. callerID is the authenticated
// user; accountID drives the tenant-mode lookup.
func setupConversationHandler(callerID, accountID uint) (
	*fiber.App,
	*mocks.MockConversationRepository,
	*mocks.MockConversationParticipantRepository,
	*mocks.MockConversationMessageRepository,
	*mocks.MockUserRepository,
	*mocks.MockAccountRepository,
	*mocks.MockEnrollmentRepository,
) {
	convRepo := new(mocks.MockConversationRepository)
	partRepo := new(mocks.MockConversationParticipantRepository)
	msgRepo := new(mocks.MockConversationMessageRepository)
	userRepo := new(mocks.MockUserRepository)
	accountRepo := new(mocks.MockAccountRepository)
	enrollRepo := new(mocks.MockEnrollmentRepository)

	conversationService := service.NewConversationService(convRepo, partRepo, msgRepo)
	userService := service.NewUserService(userRepo)
	h := handlers.NewConversationHandler(conversationService, userService, accountRepo, enrollRepo, nil)

	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", callerID)
		c.Locals("account_id", accountID)
		return c.Next()
	})
	app.Post("/conversations", h.CreateConversation)
	return app, convRepo, partRepo, msgRepo, userRepo, accountRepo, enrollRepo
}

// TestCreateConversation_HigherEdAllows — no COPPA gate applies, the
// conversation is created (200/201).
func TestCreateConversation_HigherEdAllows(t *testing.T) {
	app, convRepo, partRepo, _, _, accountRepo, _ := setupConversationHandler(10, 1)

	accountRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.Account{
		ID:          1,
		TenantMode:  models.GamificationAudience("higher_ed"),
		CoppaStrict: false,
	}, nil)
	convRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Conversation")).Return(nil)
	partRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.ConversationParticipant")).Return(nil)

	body := testutil.JSONBody(map[string]interface{}{
		"conversation": map[string]interface{}{
			"subject":    "hi",
			"recipients": []uint{20, 30},
		},
	})
	resp := testutil.MakeRequest(app, http.MethodPost, "/conversations", body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

// TestCreateConversation_K5RefusesStudentToStudent — k5 tenant + no
// teacher enrollment in a shared course = 403.
func TestCreateConversation_K5RefusesStudentToStudent(t *testing.T) {
	app, _, _, _, _, accountRepo, enrollRepo := setupConversationHandler(10, 1)

	accountRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.Account{
		ID:          1,
		TenantMode:  models.GamificationAudience("k5"),
		CoppaStrict: false,
	}, nil)
	// Sender 10 has only StudentEnrollment in course 5.
	enrollRepo.On("ListByUserID", mock.Anything, uint(10), mock.AnythingOfType("uint")).Return([]models.Enrollment{
		{UserID: 10, CourseID: 5, Type: "StudentEnrollment", WorkflowState: "active"},
	}, nil)

	body := testutil.JSONBody(map[string]interface{}{
		"conversation": map[string]interface{}{
			"subject":    "hi",
			"recipients": []uint{20},
		},
	})
	resp := testutil.MakeRequest(app, http.MethodPost, "/conversations", body)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// TestCreateConversation_CoppaStrictRefuses — tenant_mode higher_ed but
// CoppaStrict=true = 403 (the AND-of-modes rule).
func TestCreateConversation_CoppaStrictRefuses(t *testing.T) {
	app, _, _, _, _, accountRepo, enrollRepo := setupConversationHandler(10, 1)

	accountRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.Account{
		ID:          1,
		TenantMode:  models.GamificationAudience("higher_ed"),
		CoppaStrict: true,
	}, nil)
	enrollRepo.On("ListByUserID", mock.Anything, uint(10), mock.AnythingOfType("uint")).Return([]models.Enrollment{
		{UserID: 10, CourseID: 5, Type: "StudentEnrollment", WorkflowState: "active"},
	}, nil)

	body := testutil.JSONBody(map[string]interface{}{
		"conversation": map[string]interface{}{
			"subject":    "hi",
			"recipients": []uint{20},
		},
	})
	resp := testutil.MakeRequest(app, http.MethodPost, "/conversations", body)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// TestCreateConversation_K5AllowsTeacherToStudent — k5 tenant, sender
// is a teacher in course 5 where recipient 20 is enrolled = 201.
func TestCreateConversation_K5AllowsTeacherToStudent(t *testing.T) {
	app, convRepo, partRepo, _, _, accountRepo, enrollRepo := setupConversationHandler(10, 1)

	accountRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.Account{
		ID:          1,
		TenantMode:  models.GamificationAudience("k5"),
		CoppaStrict: false,
	}, nil)
	// Sender 10 is Teacher in course 5.
	enrollRepo.On("ListByUserID", mock.Anything, uint(10), mock.AnythingOfType("uint")).Return([]models.Enrollment{
		{UserID: 10, CourseID: 5, Type: "TeacherEnrollment", WorkflowState: "active"},
	}, nil)
	// Recipient 20 is Student in course 5 — shared.
	enrollRepo.On("ListByUserID", mock.Anything, uint(20), mock.AnythingOfType("uint")).Return([]models.Enrollment{
		{UserID: 20, CourseID: 5, Type: "StudentEnrollment", WorkflowState: "active"},
	}, nil)
	convRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Conversation")).Return(nil)
	partRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.ConversationParticipant")).Return(nil)

	body := testutil.JSONBody(map[string]interface{}{
		"conversation": map[string]interface{}{
			"subject":    "hi",
			"recipients": []uint{20},
		},
	})
	resp := testutil.MakeRequest(app, http.MethodPost, "/conversations", body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}
