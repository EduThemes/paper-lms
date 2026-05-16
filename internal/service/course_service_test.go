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

func TestCourseCreate_Success(t *testing.T) {
	mockCourseRepo := new(mocks.MockCourseRepository)
	mockEnrollmentRepo := new(mocks.MockEnrollmentRepository)
	mockSectionRepo := new(mocks.MockSectionRepository)
	svc := service.NewCourseService(mockCourseRepo, mockEnrollmentRepo, mockSectionRepo)
	ctx := context.Background()

	course := &models.Course{
		Name:       "Intro to CS",
		CourseCode: "CS101",
	}

	mockCourseRepo.On("Create", ctx, mock.AnythingOfType("*models.Course")).Return(nil)
	mockSectionRepo.On("Create", ctx, mock.AnythingOfType("*models.CourseSection")).Return(nil)
	mockEnrollmentRepo.On("Create", ctx, mock.AnythingOfType("*models.Enrollment")).Return(nil)

	err := svc.Create(ctx, course, 1)

	assert.NoError(t, err)
	mockCourseRepo.AssertExpectations(t)
	mockSectionRepo.AssertExpectations(t)
	mockEnrollmentRepo.AssertExpectations(t)
}

func TestCourseCreate_MissingName(t *testing.T) {
	mockCourseRepo := new(mocks.MockCourseRepository)
	mockEnrollmentRepo := new(mocks.MockEnrollmentRepository)
	mockSectionRepo := new(mocks.MockSectionRepository)
	svc := service.NewCourseService(mockCourseRepo, mockEnrollmentRepo, mockSectionRepo)
	ctx := context.Background()

	course := &models.Course{
		Name:       "",
		CourseCode: "CS101",
	}

	err := svc.Create(ctx, course, 1)

	assert.EqualError(t, err, "course name is required")
	mockCourseRepo.AssertExpectations(t)
}

func TestCourseCreate_MissingCode(t *testing.T) {
	mockCourseRepo := new(mocks.MockCourseRepository)
	mockEnrollmentRepo := new(mocks.MockEnrollmentRepository)
	mockSectionRepo := new(mocks.MockSectionRepository)
	svc := service.NewCourseService(mockCourseRepo, mockEnrollmentRepo, mockSectionRepo)
	ctx := context.Background()

	course := &models.Course{
		Name:       "Intro to CS",
		CourseCode: "",
	}

	err := svc.Create(ctx, course, 1)

	assert.EqualError(t, err, "course code is required")
	mockCourseRepo.AssertExpectations(t)
}

func TestCourseCreate_SectionCreated(t *testing.T) {
	mockCourseRepo := new(mocks.MockCourseRepository)
	mockEnrollmentRepo := new(mocks.MockEnrollmentRepository)
	mockSectionRepo := new(mocks.MockSectionRepository)
	svc := service.NewCourseService(mockCourseRepo, mockEnrollmentRepo, mockSectionRepo)
	ctx := context.Background()

	course := &models.Course{
		Name:       "Intro to CS",
		CourseCode: "CS101",
	}

	mockCourseRepo.On("Create", ctx, mock.AnythingOfType("*models.Course")).Return(nil)

	// Capture the section argument to verify its name matches the course name
	var capturedSection *models.CourseSection
	mockSectionRepo.On("Create", ctx, mock.AnythingOfType("*models.CourseSection")).
		Run(func(args mock.Arguments) {
			capturedSection = args.Get(1).(*models.CourseSection)
		}).
		Return(nil)

	mockEnrollmentRepo.On("Create", ctx, mock.AnythingOfType("*models.Enrollment")).Return(nil)

	err := svc.Create(ctx, course, 1)

	assert.NoError(t, err)
	assert.NotNil(t, capturedSection)
	assert.Equal(t, "Intro to CS", capturedSection.Name)
	assert.Equal(t, "active", capturedSection.WorkflowState)
	mockSectionRepo.AssertExpectations(t)
}

func TestCourseCreate_TeacherEnrolled(t *testing.T) {
	mockCourseRepo := new(mocks.MockCourseRepository)
	mockEnrollmentRepo := new(mocks.MockEnrollmentRepository)
	mockSectionRepo := new(mocks.MockSectionRepository)
	svc := service.NewCourseService(mockCourseRepo, mockEnrollmentRepo, mockSectionRepo)
	ctx := context.Background()

	course := &models.Course{
		Name:       "Intro to CS",
		CourseCode: "CS101",
	}

	mockCourseRepo.On("Create", ctx, mock.AnythingOfType("*models.Course")).Return(nil)
	mockSectionRepo.On("Create", ctx, mock.AnythingOfType("*models.CourseSection")).Return(nil)

	// Capture the enrollment argument to verify the creator is enrolled as teacher
	var capturedEnrollment *models.Enrollment
	mockEnrollmentRepo.On("Create", ctx, mock.AnythingOfType("*models.Enrollment")).
		Run(func(args mock.Arguments) {
			capturedEnrollment = args.Get(1).(*models.Enrollment)
		}).
		Return(nil)

	err := svc.Create(ctx, course, 42)

	assert.NoError(t, err)
	assert.NotNil(t, capturedEnrollment)
	assert.Equal(t, uint(42), capturedEnrollment.UserID)
	assert.Equal(t, "TeacherEnrollment", capturedEnrollment.Type)
	assert.Equal(t, "TeacherEnrollment", capturedEnrollment.Role)
	assert.Equal(t, "active", capturedEnrollment.WorkflowState)
	mockEnrollmentRepo.AssertExpectations(t)
}

func TestCourseCreate_CourseRepoError(t *testing.T) {
	mockCourseRepo := new(mocks.MockCourseRepository)
	mockEnrollmentRepo := new(mocks.MockEnrollmentRepository)
	mockSectionRepo := new(mocks.MockSectionRepository)
	svc := service.NewCourseService(mockCourseRepo, mockEnrollmentRepo, mockSectionRepo)
	ctx := context.Background()

	course := &models.Course{
		Name:       "Intro to CS",
		CourseCode: "CS101",
	}

	mockCourseRepo.On("Create", ctx, mock.AnythingOfType("*models.Course")).Return(errors.New("db error"))

	err := svc.Create(ctx, course, 1)

	assert.EqualError(t, err, "db error")
	mockCourseRepo.AssertExpectations(t)
}

func TestCourseGetByID(t *testing.T) {
	mockCourseRepo := new(mocks.MockCourseRepository)
	mockEnrollmentRepo := new(mocks.MockEnrollmentRepository)
	mockSectionRepo := new(mocks.MockSectionRepository)
	svc := service.NewCourseService(mockCourseRepo, mockEnrollmentRepo, mockSectionRepo)
	ctx := context.Background()

	expectedCourse := &models.Course{
		ID:         5,
		Name:       "Biology 101",
		CourseCode: "BIO101",
	}

	mockCourseRepo.On("FindByID", ctx, uint(5), uint(0)).Return(expectedCourse, nil)

	result, err := svc.GetByID(ctx, 5, 0)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, uint(5), result.ID)
	assert.Equal(t, "Biology 101", result.Name)
	assert.Equal(t, "BIO101", result.CourseCode)
	mockCourseRepo.AssertExpectations(t)
}

func TestCourseUpdate(t *testing.T) {
	mockCourseRepo := new(mocks.MockCourseRepository)
	mockEnrollmentRepo := new(mocks.MockEnrollmentRepository)
	mockSectionRepo := new(mocks.MockSectionRepository)
	svc := service.NewCourseService(mockCourseRepo, mockEnrollmentRepo, mockSectionRepo)
	ctx := context.Background()

	course := &models.Course{
		ID:         1,
		Name:       "Updated Course",
		CourseCode: "UC101",
	}

	mockCourseRepo.On("Update", ctx, course).Return(nil)

	err := svc.Update(ctx, course)

	assert.NoError(t, err)
	mockCourseRepo.AssertExpectations(t)
}

func TestCourseDelete(t *testing.T) {
	mockCourseRepo := new(mocks.MockCourseRepository)
	mockEnrollmentRepo := new(mocks.MockEnrollmentRepository)
	mockSectionRepo := new(mocks.MockSectionRepository)
	svc := service.NewCourseService(mockCourseRepo, mockEnrollmentRepo, mockSectionRepo)
	ctx := context.Background()

	mockCourseRepo.On("Delete", ctx, uint(7)).Return(nil)

	err := svc.Delete(ctx, 7)

	assert.NoError(t, err)
	mockCourseRepo.AssertExpectations(t)
}

func TestCourseList(t *testing.T) {
	mockCourseRepo := new(mocks.MockCourseRepository)
	mockEnrollmentRepo := new(mocks.MockEnrollmentRepository)
	mockSectionRepo := new(mocks.MockSectionRepository)
	svc := service.NewCourseService(mockCourseRepo, mockEnrollmentRepo, mockSectionRepo)
	ctx := context.Background()

	params := repository.PaginationParams{Page: 1, PerPage: 10}
	expectedResult := &repository.PaginatedResult[models.Course]{
		Items: []models.Course{
			{ID: 1, Name: "Course A", CourseCode: "CA101"},
			{ID: 2, Name: "Course B", CourseCode: "CB201"},
		},
		TotalCount: 2,
		Page:       1,
		PerPage:    10,
	}

	mockCourseRepo.On("List", ctx, uint(0), params).Return(expectedResult, nil)

	result, err := svc.List(ctx, params)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Items, 2)
	assert.Equal(t, int64(2), result.TotalCount)
	assert.Equal(t, "Course A", result.Items[0].Name)
	assert.Equal(t, "Course B", result.Items[1].Name)
	mockCourseRepo.AssertExpectations(t)
}

func TestCourseListForUser(t *testing.T) {
	mockCourseRepo := new(mocks.MockCourseRepository)
	mockEnrollmentRepo := new(mocks.MockEnrollmentRepository)
	mockSectionRepo := new(mocks.MockSectionRepository)
	svc := service.NewCourseService(mockCourseRepo, mockEnrollmentRepo, mockSectionRepo)
	ctx := context.Background()

	params := repository.PaginationParams{Page: 1, PerPage: 10}
	expectedResult := &repository.PaginatedResult[models.Course]{
		Items: []models.Course{
			{ID: 3, Name: "My Course", CourseCode: "MC301"},
		},
		TotalCount: 1,
		Page:       1,
		PerPage:    10,
	}

	mockCourseRepo.On("ListByUserID", ctx, uint(10), uint(0), params).Return(expectedResult, nil)

	result, err := svc.ListForUser(ctx, 10, params)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Items, 1)
	assert.Equal(t, int64(1), result.TotalCount)
	assert.Equal(t, "My Course", result.Items[0].Name)
	mockCourseRepo.AssertExpectations(t)
}
