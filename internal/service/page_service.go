package service

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type PageService struct {
	repo repository.PageRepository
}

func NewPageService(repo repository.PageRepository) *PageService {
	return &PageService{repo: repo}
}

func (s *PageService) Create(ctx context.Context, page *models.WikiPage) error {
	if page.Title == "" {
		return errors.New("page title is required")
	}
	if page.URL == "" {
		page.URL = slugify(page.Title)
	}
	if page.WorkflowState == "" {
		page.WorkflowState = "unpublished"
	}
	return s.repo.Create(ctx, page)
}

func (s *PageService) GetByID(ctx context.Context, id uint) (*models.WikiPage, error) {
	return s.repo.FindByID(ctx, id, 0)
}

func (s *PageService) GetByURL(ctx context.Context, courseID uint, url string) (*models.WikiPage, error) {
	return s.repo.FindByCourseAndURL(ctx, courseID, url)
}

func (s *PageService) GetPublicPage(ctx context.Context, courseID uint, slug string) (*models.WikiPage, error) {
	return s.repo.FindPublicByCourseAndURL(ctx, courseID, slug)
}

func (s *PageService) Update(ctx context.Context, page *models.WikiPage) error {
	return s.repo.Update(ctx, page)
}

func (s *PageService) Delete(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}

func (s *PageService) ListByCourse(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.WikiPage], error) {
	return s.repo.ListByCourseID(ctx, courseID, params)
}

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(title string) string {
	slug := strings.ToLower(title)
	slug = nonAlphanumeric.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	return slug
}
