package service_test

import (
	"context"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestModuleCreate_Success(t *testing.T) {
	moduleRepo := new(mocks.MockModuleRepository)
	itemRepo := new(mocks.MockModuleItemRepository)
	svc := service.NewModuleService(moduleRepo, itemRepo)

	module := &models.ContextModule{
		CourseID: 1,
		Name:     "Week 1 Introduction",
		Position: 1,
	}

	moduleRepo.On("Create", mock.Anything, module).Return(nil)

	err := svc.Create(context.Background(), module)

	assert.NoError(t, err)
	assert.Equal(t, "active", module.WorkflowState)
	moduleRepo.AssertExpectations(t)
}

func TestModuleCreate_MissingName(t *testing.T) {
	moduleRepo := new(mocks.MockModuleRepository)
	itemRepo := new(mocks.MockModuleItemRepository)
	svc := service.NewModuleService(moduleRepo, itemRepo)

	module := &models.ContextModule{
		CourseID: 1,
		Name:     "",
	}

	err := svc.Create(context.Background(), module)

	assert.Error(t, err)
	assert.Equal(t, "module name is required", err.Error())
	moduleRepo.AssertNotCalled(t, "Create")
}

func TestModuleCreateItem_Success(t *testing.T) {
	moduleRepo := new(mocks.MockModuleRepository)
	itemRepo := new(mocks.MockModuleItemRepository)
	svc := service.NewModuleService(moduleRepo, itemRepo)

	item := &models.ContentTag{
		ContextModuleID: 1,
		ContentType:     "Assignment",
		Title:           "Homework 1",
		Position:        1,
	}

	itemRepo.On("Create", mock.Anything, item).Return(nil)

	err := svc.CreateItem(context.Background(), item)

	assert.NoError(t, err)
	assert.Equal(t, models.ContentTagActive, item.WorkflowState)
	itemRepo.AssertExpectations(t)
}

func TestModuleCreateItem_MissingTitle(t *testing.T) {
	moduleRepo := new(mocks.MockModuleRepository)
	itemRepo := new(mocks.MockModuleItemRepository)
	svc := service.NewModuleService(moduleRepo, itemRepo)

	item := &models.ContentTag{
		ContextModuleID: 1,
		ContentType:     "Assignment",
		Title:           "",
	}

	err := svc.CreateItem(context.Background(), item)

	assert.Error(t, err)
	assert.Equal(t, "item title is required", err.Error())
	itemRepo.AssertNotCalled(t, "Create")
}

func TestModuleGetByID(t *testing.T) {
	moduleRepo := new(mocks.MockModuleRepository)
	itemRepo := new(mocks.MockModuleItemRepository)
	svc := service.NewModuleService(moduleRepo, itemRepo)

	expected := &models.ContextModule{
		ID:            1,
		CourseID:      1,
		Name:          "Week 1 Introduction",
		Position:      1,
		WorkflowState: "active",
	}

	moduleRepo.On("FindByID", mock.Anything, uint(1), uint(0)).Return(expected, nil)

	result, err := svc.GetByID(context.Background(), 1, 0)

	assert.NoError(t, err)
	assert.Equal(t, expected.ID, result.ID)
	assert.Equal(t, expected.Name, result.Name)
	moduleRepo.AssertExpectations(t)
}

func TestModuleUpdate(t *testing.T) {
	moduleRepo := new(mocks.MockModuleRepository)
	itemRepo := new(mocks.MockModuleItemRepository)
	svc := service.NewModuleService(moduleRepo, itemRepo)

	module := &models.ContextModule{
		ID:            1,
		CourseID:      1,
		Name:          "Week 1 - Updated",
		Position:      1,
		WorkflowState: "active",
	}

	moduleRepo.On("Update", mock.Anything, module).Return(nil)

	err := svc.Update(context.Background(), module)

	assert.NoError(t, err)
	moduleRepo.AssertExpectations(t)
}

func TestModuleDelete(t *testing.T) {
	moduleRepo := new(mocks.MockModuleRepository)
	itemRepo := new(mocks.MockModuleItemRepository)
	svc := service.NewModuleService(moduleRepo, itemRepo)

	moduleRepo.On("Delete", mock.Anything, uint(1)).Return(nil)

	err := svc.Delete(context.Background(), 1)

	assert.NoError(t, err)
	moduleRepo.AssertExpectations(t)
}

func TestModuleListByCourse(t *testing.T) {
	moduleRepo := new(mocks.MockModuleRepository)
	itemRepo := new(mocks.MockModuleItemRepository)
	svc := service.NewModuleService(moduleRepo, itemRepo)

	params := repository.PaginationParams{Page: 1, PerPage: 10}
	expected := &repository.PaginatedResult[models.ContextModule]{
		Items: []models.ContextModule{
			{ID: 1, CourseID: 1, Name: "Week 1", Position: 1, WorkflowState: "active"},
			{ID: 2, CourseID: 1, Name: "Week 2", Position: 2, WorkflowState: "active"},
		},
		TotalCount: 2,
		Page:       1,
		PerPage:    10,
	}

	moduleRepo.On("ListByCourseID", mock.Anything, uint(1), params).Return(expected, nil)

	result, err := svc.ListByCourse(context.Background(), 1, params)

	assert.NoError(t, err)
	assert.Len(t, result.Items, 2)
	assert.Equal(t, int64(2), result.TotalCount)
	moduleRepo.AssertExpectations(t)
}

func TestModuleGetItem(t *testing.T) {
	moduleRepo := new(mocks.MockModuleRepository)
	itemRepo := new(mocks.MockModuleItemRepository)
	svc := service.NewModuleService(moduleRepo, itemRepo)

	expected := &models.ContentTag{
		ID:              1,
		ContextModuleID: 1,
		ContentType:     "Assignment",
		Title:           "Homework 1",
		Position:        1,
		WorkflowState:   "active",
	}

	itemRepo.On("FindByID", mock.Anything, uint(1)).Return(expected, nil)

	result, err := svc.GetItem(context.Background(), 1)

	assert.NoError(t, err)
	assert.Equal(t, expected.ID, result.ID)
	assert.Equal(t, expected.Title, result.Title)
	itemRepo.AssertExpectations(t)
}

func TestModuleListItems(t *testing.T) {
	moduleRepo := new(mocks.MockModuleRepository)
	itemRepo := new(mocks.MockModuleItemRepository)
	svc := service.NewModuleService(moduleRepo, itemRepo)

	params := repository.PaginationParams{Page: 1, PerPage: 10}
	expected := &repository.PaginatedResult[models.ContentTag]{
		Items: []models.ContentTag{
			{ID: 1, ContextModuleID: 1, ContentType: "Assignment", Title: "Homework 1", Position: 1, WorkflowState: "active"},
			{ID: 2, ContextModuleID: 1, ContentType: "WikiPage", Title: "Reading Notes", Position: 2, WorkflowState: "active"},
		},
		TotalCount: 2,
		Page:       1,
		PerPage:    10,
	}

	itemRepo.On("ListByModuleID", mock.Anything, uint(1), params).Return(expected, nil)

	result, err := svc.ListItems(context.Background(), 1, params)

	assert.NoError(t, err)
	assert.Len(t, result.Items, 2)
	assert.Equal(t, int64(2), result.TotalCount)
	itemRepo.AssertExpectations(t)
}
