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

func TestAssignmentCreate_Success(t *testing.T) {
	mockRepo := new(mocks.MockAssignmentRepository)
	svc := service.NewAssignmentService(mockRepo)
	ctx := context.Background()

	points := 100.0
	assignment := &models.Assignment{
		CourseID:       1,
		Name:           "Homework 1",
		PointsPossible: &points,
	}

	mockRepo.On("Create", ctx, mock.AnythingOfType("*models.Assignment")).Return(nil)

	err := svc.Create(ctx, assignment)

	assert.NoError(t, err)
	assert.Equal(t, "unpublished", assignment.WorkflowState)
	mockRepo.AssertExpectations(t)
}

func TestAssignmentCreate_MissingName(t *testing.T) {
	mockRepo := new(mocks.MockAssignmentRepository)
	svc := service.NewAssignmentService(mockRepo)
	ctx := context.Background()

	assignment := &models.Assignment{
		CourseID: 1,
		Name:     "",
	}

	err := svc.Create(ctx, assignment)

	assert.EqualError(t, err, "assignment name is required")
	mockRepo.AssertExpectations(t)
}

func TestAssignmentCreate_PreservesState(t *testing.T) {
	mockRepo := new(mocks.MockAssignmentRepository)
	svc := service.NewAssignmentService(mockRepo)
	ctx := context.Background()

	assignment := &models.Assignment{
		CourseID:      1,
		Name:          "Published Assignment",
		WorkflowState: "published",
	}

	mockRepo.On("Create", ctx, mock.AnythingOfType("*models.Assignment")).Return(nil)

	err := svc.Create(ctx, assignment)

	assert.NoError(t, err)
	assert.Equal(t, "published", assignment.WorkflowState)
	mockRepo.AssertExpectations(t)
}

func TestAssignmentCreate_DefaultsUnpublished(t *testing.T) {
	mockRepo := new(mocks.MockAssignmentRepository)
	svc := service.NewAssignmentService(mockRepo)
	ctx := context.Background()

	assignment := &models.Assignment{
		CourseID:      1,
		Name:          "New Assignment",
		WorkflowState: "",
	}

	mockRepo.On("Create", ctx, mock.AnythingOfType("*models.Assignment")).Return(nil)

	err := svc.Create(ctx, assignment)

	assert.NoError(t, err)
	assert.Equal(t, "unpublished", assignment.WorkflowState)
	mockRepo.AssertExpectations(t)
}

func TestAssignmentCreate_RepoError(t *testing.T) {
	mockRepo := new(mocks.MockAssignmentRepository)
	svc := service.NewAssignmentService(mockRepo)
	ctx := context.Background()

	assignment := &models.Assignment{
		CourseID: 1,
		Name:     "Error Assignment",
	}

	mockRepo.On("Create", ctx, mock.AnythingOfType("*models.Assignment")).Return(errors.New("db error"))

	err := svc.Create(ctx, assignment)

	assert.EqualError(t, err, "db error")
	mockRepo.AssertExpectations(t)
}

func TestAssignmentGetByID(t *testing.T) {
	mockRepo := new(mocks.MockAssignmentRepository)
	svc := service.NewAssignmentService(mockRepo)
	ctx := context.Background()

	points := 50.0
	expected := &models.Assignment{
		ID:             3,
		CourseID:       1,
		Name:           "Midterm Exam",
		PointsPossible: &points,
		WorkflowState:  "published",
	}

	mockRepo.On("FindByID", ctx, uint(3), uint(0)).Return(expected, nil)

	result, err := svc.GetByID(ctx, 3, 0)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, uint(3), result.ID)
	assert.Equal(t, "Midterm Exam", result.Name)
	assert.Equal(t, 50.0, *result.PointsPossible)
	mockRepo.AssertExpectations(t)
}

func TestAssignmentGetByID_NotFound(t *testing.T) {
	mockRepo := new(mocks.MockAssignmentRepository)
	svc := service.NewAssignmentService(mockRepo)
	ctx := context.Background()

	mockRepo.On("FindByID", ctx, uint(999), uint(0)).Return(nil, errors.New("record not found"))

	result, err := svc.GetByID(ctx, 999, 0)

	assert.Nil(t, result)
	assert.EqualError(t, err, "record not found")
	mockRepo.AssertExpectations(t)
}

func TestAssignmentUpdate(t *testing.T) {
	mockRepo := new(mocks.MockAssignmentRepository)
	svc := service.NewAssignmentService(mockRepo)
	ctx := context.Background()

	assignment := &models.Assignment{
		ID:            1,
		CourseID:      1,
		Name:          "Updated Assignment",
		WorkflowState: "published",
	}

	mockRepo.On("Update", ctx, assignment).Return(nil)

	err := svc.Update(ctx, assignment)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestAssignmentDelete(t *testing.T) {
	mockRepo := new(mocks.MockAssignmentRepository)
	svc := service.NewAssignmentService(mockRepo)
	ctx := context.Background()

	mockRepo.On("Delete", ctx, uint(5)).Return(nil)

	err := svc.Delete(ctx, 5)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestAssignmentListByCourse(t *testing.T) {
	mockRepo := new(mocks.MockAssignmentRepository)
	svc := service.NewAssignmentService(mockRepo)
	ctx := context.Background()

	params := repository.PaginationParams{Page: 1, PerPage: 10}
	points1 := 100.0
	points2 := 50.0
	expectedResult := &repository.PaginatedResult[models.Assignment]{
		Items: []models.Assignment{
			{ID: 1, CourseID: 2, Name: "Assignment 1", PointsPossible: &points1, WorkflowState: "published"},
			{ID: 2, CourseID: 2, Name: "Assignment 2", PointsPossible: &points2, WorkflowState: "unpublished"},
		},
		TotalCount: 2,
		Page:       1,
		PerPage:    10,
	}

	mockRepo.On("ListByCourseID", ctx, uint(2), params).Return(expectedResult, nil)

	result, err := svc.ListByCourse(ctx, 2, params)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Items, 2)
	assert.Equal(t, int64(2), result.TotalCount)
	assert.Equal(t, "Assignment 1", result.Items[0].Name)
	assert.Equal(t, "Assignment 2", result.Items[1].Name)
	assert.Equal(t, 100.0, *result.Items[0].PointsPossible)
	assert.Equal(t, 50.0, *result.Items[1].PointsPossible)
	mockRepo.AssertExpectations(t)
}
