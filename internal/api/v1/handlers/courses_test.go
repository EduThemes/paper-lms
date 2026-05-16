package handlers_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// setupCourseHandler creates a CourseHandler wired to mock repositories and
// registers routes on a fresh test Fiber app.
func setupCourseHandler() (*fiber.App, *mocks.MockCourseRepository, *mocks.MockEnrollmentRepository, *mocks.MockSectionRepository) {
	mockCourseRepo := new(mocks.MockCourseRepository)
	mockEnrollmentRepo := new(mocks.MockEnrollmentRepository)
	mockSectionRepo := new(mocks.MockSectionRepository)

	courseService := service.NewCourseService(mockCourseRepo, mockEnrollmentRepo, mockSectionRepo)
	enrollmentService := service.NewEnrollmentService(mockEnrollmentRepo)
	handler := handlers.NewCourseHandler(courseService, enrollmentService)

	app := testutil.SetupTestApp()

	// All course routes require an authenticated user; inject user_id into Locals
	api := app.Group("", func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(1))
		c.Locals("account_id", uint(1))
		return c.Next()
	}, middleware.PaginationParams())

	api.Post("/courses", handler.CreateCourse)
	api.Get("/courses/:id", handler.GetCourse)
	api.Get("/courses", handler.ListCourses)
	api.Put("/courses/:id", handler.UpdateCourse)
	api.Delete("/courses/:id", handler.DeleteCourse)

	return app, mockCourseRepo, mockEnrollmentRepo, mockSectionRepo
}

// ---------------------------------------------------------------------------
// CreateCourse
// ---------------------------------------------------------------------------

func TestCreateCourse_Success(t *testing.T) {
	app, mockCourseRepo, mockEnrollmentRepo, mockSectionRepo := setupCourseHandler()

	// courseRepo.Create succeeds
	mockCourseRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Course")).Return(nil)
	// sectionRepo.Create succeeds (default section)
	mockSectionRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.CourseSection")).Return(nil)
	// enrollmentRepo.Create succeeds (creator enrolled as teacher)
	mockEnrollmentRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Enrollment")).Return(nil)

	body := testutil.JSONBody(map[string]interface{}{
		"course": map[string]string{
			"name":        "Intro to Testing",
			"course_code": "TEST101",
		},
	})

	resp := testutil.MakeRequest(app, http.MethodPost, "/courses", body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.Equal(t, "Intro to Testing", result["name"])
	assert.Equal(t, "TEST101", result["course_code"])

	mockCourseRepo.AssertExpectations(t)
	mockSectionRepo.AssertExpectations(t)
	mockEnrollmentRepo.AssertExpectations(t)
}

func TestCreateCourse_MissingName(t *testing.T) {
	app, _, _, _ := setupCourseHandler()

	// Omit name; the service should reject this
	body := testutil.JSONBody(map[string]interface{}{
		"course": map[string]string{
			"course_code": "TEST101",
		},
	})

	resp := testutil.MakeRequest(app, http.MethodPost, "/courses", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.NotNil(t, result["errors"])
}

func TestCreateCourse_MissingCourseCode(t *testing.T) {
	app, _, _, _ := setupCourseHandler()

	body := testutil.JSONBody(map[string]interface{}{
		"course": map[string]string{
			"name": "Intro to Testing",
		},
	})

	resp := testutil.MakeRequest(app, http.MethodPost, "/courses", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.NotNil(t, result["errors"])
}

// ---------------------------------------------------------------------------
// GetCourse
// ---------------------------------------------------------------------------

func TestGetCourse_Success(t *testing.T) {
	app, mockCourseRepo, _, _ := setupCourseHandler()

	course := &models.Course{
		ID:            1,
		AccountID:     1,
		Name:          "Intro to Testing",
		CourseCode:    "TEST101",
		WorkflowState: "available",
		DefaultView:   "modules",
	}
	mockCourseRepo.On("FindByID", mock.Anything, uint(1), uint(0)).Return(course, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/1", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(1), result["id"])
	assert.Equal(t, "Intro to Testing", result["name"])
	assert.Equal(t, "TEST101", result["course_code"])
	assert.Equal(t, "available", result["workflow_state"])

	mockCourseRepo.AssertExpectations(t)
}

func TestGetCourse_NotFound(t *testing.T) {
	app, mockCourseRepo, _, _ := setupCourseHandler()

	mockCourseRepo.On("FindByID", mock.Anything, uint(999), uint(0)).Return(nil, errors.New("not found"))

	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/999", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.NotNil(t, result["errors"])

	mockCourseRepo.AssertExpectations(t)
}

func TestGetCourse_InvalidID(t *testing.T) {
	app, _, _, _ := setupCourseHandler()

	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/abc", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.NotNil(t, result["errors"])
}

// ---------------------------------------------------------------------------
// ListCourses
// ---------------------------------------------------------------------------

func TestListCourses_Default(t *testing.T) {
	app, mockCourseRepo, mockEnrollmentRepo, _ := setupCourseHandler()

	// Default (no ?scope=all) returns user-enrolled courses via ListByUserID
	expectedResult := &repository.PaginatedResult[models.Course]{
		Items: []models.Course{
			{ID: 1, Name: "Course One", CourseCode: "C1", WorkflowState: "available", DefaultView: "modules"},
			{ID: 2, Name: "Course Two", CourseCode: "C2", WorkflowState: "available", DefaultView: "modules"},
		},
		TotalCount: 2,
		Page:       1,
		PerPage:    10,
	}
	mockCourseRepo.On("ListByUserID", mock.Anything, uint(1), uint(0), repository.PaginationParams{Page: 1, PerPage: 10}).Return(expectedResult, nil)
	mockEnrollmentRepo.On("CountByCourseIDs", mock.Anything, mock.AnythingOfType("[]uint")).Return(map[uint]int64{1: 25, 2: 30}, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/courses", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	assert.NotEmpty(t, resp.Header.Get("Link"))

	result, err := testutil.ParseJSONArray(resp)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "Course One", result[0]["name"])
	assert.Equal(t, "Course Two", result[1]["name"])
	assert.Equal(t, float64(25), result[0]["total_students"])
	assert.Equal(t, float64(30), result[1]["total_students"])

	mockCourseRepo.AssertExpectations(t)
}

func TestListCourses_WithEnrollmentType(t *testing.T) {
	app, mockCourseRepo, mockEnrollmentRepo, _ := setupCourseHandler()

	// When enrollment_type query param is set, handler calls ListForUser
	expectedResult := &repository.PaginatedResult[models.Course]{
		Items: []models.Course{
			{ID: 1, Name: "My Course", CourseCode: "MC1", WorkflowState: "available", DefaultView: "modules"},
		},
		TotalCount: 1,
		Page:       1,
		PerPage:    10,
	}
	mockCourseRepo.On("ListByUserID", mock.Anything, uint(1), uint(0), repository.PaginationParams{Page: 1, PerPage: 10}).Return(expectedResult, nil)
	mockEnrollmentRepo.On("CountByCourseIDs", mock.Anything, mock.AnythingOfType("[]uint")).Return(map[uint]int64{1: 15}, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/courses?enrollment_type=TeacherEnrollment", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result, err := testutil.ParseJSONArray(resp)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "My Course", result[0]["name"])

	mockCourseRepo.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// UpdateCourse
// ---------------------------------------------------------------------------

func TestUpdateCourse(t *testing.T) {
	app, mockCourseRepo, _, _ := setupCourseHandler()

	course := &models.Course{
		ID:            1,
		AccountID:     1,
		Name:          "Old Name",
		CourseCode:    "OLD101",
		WorkflowState: "available",
		DefaultView:   "modules",
	}
	mockCourseRepo.On("FindByID", mock.Anything, uint(1), uint(0)).Return(course, nil)
	mockCourseRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.Course")).Return(nil)

	newName := "New Name"
	body := testutil.JSONBody(map[string]interface{}{
		"course": map[string]*string{
			"name": &newName,
		},
	})

	resp := testutil.MakeRequest(app, http.MethodPut, "/courses/1", body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.Equal(t, "New Name", result["name"])

	mockCourseRepo.AssertExpectations(t)
}

func TestUpdateCourse_NotFound(t *testing.T) {
	app, mockCourseRepo, _, _ := setupCourseHandler()

	mockCourseRepo.On("FindByID", mock.Anything, uint(999), uint(0)).Return(nil, errors.New("not found"))

	body := testutil.JSONBody(map[string]interface{}{
		"course": map[string]string{
			"name": "New Name",
		},
	})

	resp := testutil.MakeRequest(app, http.MethodPut, "/courses/999", body)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	mockCourseRepo.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// DeleteCourse
// ---------------------------------------------------------------------------

func TestDeleteCourse(t *testing.T) {
	app, mockCourseRepo, _, _ := setupCourseHandler()

	mockCourseRepo.On("Delete", mock.Anything, uint(1)).Return(nil)

	resp := testutil.MakeRequest(app, http.MethodDelete, "/courses/1", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.Equal(t, true, result["delete"])

	mockCourseRepo.AssertExpectations(t)
}

func TestDeleteCourse_InvalidID(t *testing.T) {
	app, _, _, _ := setupCourseHandler()

	resp := testutil.MakeRequest(app, http.MethodDelete, "/courses/abc", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.NotNil(t, result["errors"])
}
