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

func TestPageCreate_Success(t *testing.T) {
	repo := new(mocks.MockPageRepository)
	svc := service.NewPageService(repo)

	page := &models.WikiPage{
		CourseID: 1,
		Title:    "Getting Started",
		Body:     "<p>Welcome to the course.</p>",
	}

	repo.On("Create", mock.Anything, page).Return(nil)

	err := svc.Create(context.Background(), page)

	assert.NoError(t, err)
	assert.Equal(t, "getting-started", page.URL)
	assert.Equal(t, "unpublished", page.WorkflowState)
	repo.AssertExpectations(t)
}

func TestPageCreate_MissingTitle(t *testing.T) {
	repo := new(mocks.MockPageRepository)
	svc := service.NewPageService(repo)

	page := &models.WikiPage{
		CourseID: 1,
		Title:    "",
		Body:     "<p>Some content</p>",
	}

	err := svc.Create(context.Background(), page)

	assert.Error(t, err)
	assert.Equal(t, "page title is required", err.Error())
	repo.AssertNotCalled(t, "Create")
}

func TestPageCreate_URLGenerated(t *testing.T) {
	repo := new(mocks.MockPageRepository)
	svc := service.NewPageService(repo)

	page := &models.WikiPage{
		CourseID: 1,
		Title:    "My Test Page",
		Body:     "<p>Test content</p>",
	}

	repo.On("Create", mock.Anything, page).Return(nil)

	err := svc.Create(context.Background(), page)

	assert.NoError(t, err)
	assert.Equal(t, "my-test-page", page.URL)
	repo.AssertExpectations(t)
}

func TestPageCreate_PreservesURL(t *testing.T) {
	repo := new(mocks.MockPageRepository)
	svc := service.NewPageService(repo)

	page := &models.WikiPage{
		CourseID: 1,
		Title:    "My Test Page",
		URL:      "custom-slug",
		Body:     "<p>Test content</p>",
	}

	repo.On("Create", mock.Anything, page).Return(nil)

	err := svc.Create(context.Background(), page)

	assert.NoError(t, err)
	assert.Equal(t, "custom-slug", page.URL, "URL should not be overwritten when already set")
	repo.AssertExpectations(t)
}

func TestPageGetByID(t *testing.T) {
	repo := new(mocks.MockPageRepository)
	svc := service.NewPageService(repo)

	expected := &models.WikiPage{
		ID:            1,
		CourseID:      1,
		Title:         "Welcome Page",
		URL:           "welcome-page",
		Body:          "<p>Welcome!</p>",
		WorkflowState: "unpublished",
	}

	repo.On("FindByID", mock.Anything, uint(1), uint(0)).Return(expected, nil)

	result, err := svc.GetByID(context.Background(), 1)

	assert.NoError(t, err)
	assert.Equal(t, expected.ID, result.ID)
	assert.Equal(t, expected.Title, result.Title)
	repo.AssertExpectations(t)
}

func TestPageGetByURL(t *testing.T) {
	repo := new(mocks.MockPageRepository)
	svc := service.NewPageService(repo)

	expected := &models.WikiPage{
		ID:            1,
		CourseID:      1,
		Title:         "Welcome Page",
		URL:           "welcome-page",
		Body:          "<p>Welcome!</p>",
		WorkflowState: "unpublished",
	}

	repo.On("FindByCourseAndURL", mock.Anything, uint(1), "welcome-page").Return(expected, nil)

	result, err := svc.GetByURL(context.Background(), 1, "welcome-page")

	assert.NoError(t, err)
	assert.Equal(t, expected.ID, result.ID)
	assert.Equal(t, "welcome-page", result.URL)
	repo.AssertExpectations(t)
}

func TestPageUpdate(t *testing.T) {
	repo := new(mocks.MockPageRepository)
	svc := service.NewPageService(repo)

	page := &models.WikiPage{
		ID:            1,
		CourseID:      1,
		Title:         "Updated Title",
		URL:           "updated-title",
		Body:          "<p>Updated body</p>",
		WorkflowState: "active",
	}

	repo.On("Update", mock.Anything, page).Return(nil)

	err := svc.Update(context.Background(), page)

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestPageDelete(t *testing.T) {
	repo := new(mocks.MockPageRepository)
	svc := service.NewPageService(repo)

	repo.On("Delete", mock.Anything, uint(1)).Return(nil)

	err := svc.Delete(context.Background(), 1)

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestPageListByCourse(t *testing.T) {
	repo := new(mocks.MockPageRepository)
	svc := service.NewPageService(repo)

	params := repository.PaginationParams{Page: 1, PerPage: 10}
	expected := &repository.PaginatedResult[models.WikiPage]{
		Items: []models.WikiPage{
			{ID: 1, CourseID: 1, Title: "Page 1", URL: "page-1", WorkflowState: "unpublished"},
			{ID: 2, CourseID: 1, Title: "Page 2", URL: "page-2", WorkflowState: "active"},
		},
		TotalCount: 2,
		Page:       1,
		PerPage:    10,
	}

	repo.On("ListByCourseID", mock.Anything, uint(1), params).Return(expected, nil)

	result, err := svc.ListByCourse(context.Background(), 1, params)

	assert.NoError(t, err)
	assert.Len(t, result.Items, 2)
	assert.Equal(t, int64(2), result.TotalCount)
	repo.AssertExpectations(t)
}

func TestPageGetByID_NotFound(t *testing.T) {
	repo := new(mocks.MockPageRepository)
	svc := service.NewPageService(repo)

	repo.On("FindByID", mock.Anything, uint(999), uint(0)).Return(nil, errors.New("record not found"))

	result, err := svc.GetByID(context.Background(), 999)

	assert.Error(t, err)
	assert.Nil(t, result)
	repo.AssertExpectations(t)
}
