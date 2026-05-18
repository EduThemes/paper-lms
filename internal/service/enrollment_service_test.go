package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestEnrollmentCreate_Valid(t *testing.T) {
	mockRepo := new(mocks.MockEnrollmentRepository)
	svc := service.NewEnrollmentService(mockRepo)
	ctx := context.Background()

	enrollment := &models.Enrollment{
		UserID:   1,
		CourseID: 1,
		Type:     "StudentEnrollment",
	}

	// No existing enrollment
	mockRepo.On("FindByUserAndCourse", ctx, uint(1), uint(1)).Return(nil, errors.New("not found"))
	mockRepo.On("Create", ctx, mock.AnythingOfType("*models.Enrollment")).Return(nil)

	err := svc.Create(ctx, enrollment)

	assert.NoError(t, err)
	assert.Equal(t, "StudentEnrollment", enrollment.Role)
	assert.Equal(t, models.EnrollmentActive, enrollment.WorkflowState)
	mockRepo.AssertExpectations(t)
}

func TestEnrollmentCreate_InvalidType(t *testing.T) {
	mockRepo := new(mocks.MockEnrollmentRepository)
	svc := service.NewEnrollmentService(mockRepo)
	ctx := context.Background()

	enrollment := &models.Enrollment{
		UserID:   1,
		CourseID: 1,
		Type:     "BadType",
	}

	err := svc.Create(ctx, enrollment)

	assert.EqualError(t, err, "invalid enrollment type")
	mockRepo.AssertExpectations(t)
}

func TestEnrollmentCreate_DefaultState(t *testing.T) {
	mockRepo := new(mocks.MockEnrollmentRepository)
	svc := service.NewEnrollmentService(mockRepo)
	ctx := context.Background()

	enrollment := &models.Enrollment{
		UserID:        1,
		CourseID:      1,
		Type:          "TeacherEnrollment",
		WorkflowState: "",
	}

	mockRepo.On("FindByUserAndCourse", ctx, uint(1), uint(1)).Return(nil, errors.New("not found"))
	mockRepo.On("Create", ctx, mock.AnythingOfType("*models.Enrollment")).Return(nil)

	err := svc.Create(ctx, enrollment)

	assert.NoError(t, err)
	assert.Equal(t, models.EnrollmentActive, enrollment.WorkflowState)
	mockRepo.AssertExpectations(t)
}

func TestEnrollmentCreate_Duplicate(t *testing.T) {
	mockRepo := new(mocks.MockEnrollmentRepository)
	svc := service.NewEnrollmentService(mockRepo)
	ctx := context.Background()

	existing := &models.Enrollment{
		ID:       99,
		UserID:   1,
		CourseID: 1,
		Type:     "StudentEnrollment",
		Role:     "StudentEnrollment",
	}

	enrollment := &models.Enrollment{
		UserID:   1,
		CourseID: 1,
		Type:     "StudentEnrollment",
	}

	mockRepo.On("FindByUserAndCourse", ctx, uint(1), uint(1)).Return(existing, nil)

	err := svc.Create(ctx, enrollment)

	assert.EqualError(t, err, "user is already enrolled in this course")
	mockRepo.AssertExpectations(t)
}

func TestEnrollmentCreate_AllTypes(t *testing.T) {
	validTypes := []string{
		"StudentEnrollment",
		"TeacherEnrollment",
		"TaEnrollment",
		"ObserverEnrollment",
		"DesignerEnrollment",
	}

	for _, enrollType := range validTypes {
		t.Run(enrollType, func(t *testing.T) {
			mockRepo := new(mocks.MockEnrollmentRepository)
			svc := service.NewEnrollmentService(mockRepo)
			ctx := context.Background()

			enrollment := &models.Enrollment{
				UserID:   1,
				CourseID: 1,
				Type:     enrollType,
			}

			mockRepo.On("FindByUserAndCourse", ctx, uint(1), uint(1)).Return(nil, errors.New("not found"))
			mockRepo.On("Create", ctx, mock.AnythingOfType("*models.Enrollment")).Return(nil)

			err := svc.Create(ctx, enrollment)

			assert.NoError(t, err)
			assert.Equal(t, enrollType, enrollment.Role)
			assert.Equal(t, models.EnrollmentActive, enrollment.WorkflowState)
			mockRepo.AssertExpectations(t)
		})
	}
}


func TestEnrollmentCreate_PreservesExistingState(t *testing.T) {
	mockRepo := new(mocks.MockEnrollmentRepository)
	svc := service.NewEnrollmentService(mockRepo)
	ctx := context.Background()

	enrollment := &models.Enrollment{
		UserID:        1,
		CourseID:      1,
		Type:          "StudentEnrollment",
		WorkflowState: "invited",
	}

	mockRepo.On("FindByUserAndCourse", ctx, uint(1), uint(1)).Return(nil, errors.New("not found"))
	mockRepo.On("Create", ctx, mock.AnythingOfType("*models.Enrollment")).Return(nil)

	err := svc.Create(ctx, enrollment)

	assert.NoError(t, err)
	assert.Equal(t, models.EnrollmentInvited, enrollment.WorkflowState)
	mockRepo.AssertExpectations(t)
}

func TestEnrollmentCreate_RepoError(t *testing.T) {
	mockRepo := new(mocks.MockEnrollmentRepository)
	svc := service.NewEnrollmentService(mockRepo)
	ctx := context.Background()

	enrollment := &models.Enrollment{
		UserID:   1,
		CourseID: 1,
		Type:     "StudentEnrollment",
	}

	mockRepo.On("FindByUserAndCourse", ctx, uint(1), uint(1)).Return(nil, errors.New("not found"))
	mockRepo.On("Create", ctx, mock.AnythingOfType("*models.Enrollment")).Return(errors.New("db error"))

	err := svc.Create(ctx, enrollment)

	assert.EqualError(t, err, "db error")
	mockRepo.AssertExpectations(t)
}

func TestEnrollmentListByCourse(t *testing.T) {
	mockRepo := new(mocks.MockEnrollmentRepository)
	svc := service.NewEnrollmentService(mockRepo)
	ctx := context.Background()

	params := repository.PaginationParams{Page: 1, PerPage: 10}
	expectedResult := &repository.PaginatedResult[models.Enrollment]{
		Items: []models.Enrollment{
			{ID: 1, UserID: 1, CourseID: 5, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: "active"},
			{ID: 2, UserID: 2, CourseID: 5, Type: "TeacherEnrollment", Role: "TeacherEnrollment", WorkflowState: "active"},
		},
		TotalCount: 2,
		Page:       1,
		PerPage:    10,
	}

	mockRepo.On("ListByCourseID", ctx, uint(5), params).Return(expectedResult, nil)

	result, err := svc.ListByCourse(ctx, 5, params)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Items, 2)
	assert.Equal(t, int64(2), result.TotalCount)
	assert.Equal(t, "StudentEnrollment", result.Items[0].Type)
	assert.Equal(t, "TeacherEnrollment", result.Items[1].Type)
	mockRepo.AssertExpectations(t)
}

func TestEnrollmentListByUser(t *testing.T) {
	mockRepo := new(mocks.MockEnrollmentRepository)
	svc := service.NewEnrollmentService(mockRepo)
	ctx := context.Background()

	expectedEnrollments := []models.Enrollment{
		{ID: 1, UserID: 10, CourseID: 1, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: "active"},
		{ID: 2, UserID: 10, CourseID: 2, Type: "TaEnrollment", Role: "TaEnrollment", WorkflowState: "active"},
	}

	mockRepo.On("ListByUserID", ctx, uint(10)).Return(expectedEnrollments, nil)

	result, err := svc.ListByUser(ctx, 10)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, uint(1), result[0].CourseID)
	assert.Equal(t, uint(2), result[1].CourseID)
	mockRepo.AssertExpectations(t)
}

func TestEnrollmentGetUserRole(t *testing.T) {
	mockRepo := new(mocks.MockEnrollmentRepository)
	svc := service.NewEnrollmentService(mockRepo)
	ctx := context.Background()

	enrollment := &models.Enrollment{
		ID:       1,
		UserID:   10,
		CourseID: 5,
		Type:     "TeacherEnrollment",
		Role:     "TeacherEnrollment",
	}

	mockRepo.On("FindByUserAndCourse", ctx, uint(10), uint(5)).Return(enrollment, nil)

	role, err := svc.GetUserRole(ctx, 10, 5)

	assert.NoError(t, err)
	assert.Equal(t, "TeacherEnrollment", role)
	mockRepo.AssertExpectations(t)
}

func TestEnrollmentGetUserRole_NotFound(t *testing.T) {
	mockRepo := new(mocks.MockEnrollmentRepository)
	svc := service.NewEnrollmentService(mockRepo)
	ctx := context.Background()

	mockRepo.On("FindByUserAndCourse", ctx, uint(10), uint(5)).Return(nil, errors.New("not found"))

	role, err := svc.GetUserRole(ctx, 10, 5)

	assert.EqualError(t, err, "not found")
	assert.Equal(t, "", role)
	mockRepo.AssertExpectations(t)
}
