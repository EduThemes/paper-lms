package postgres

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type pageRepo struct {
	db *gorm.DB
}

func NewPageRepository(db *gorm.DB) repository.PageRepository {
	return &pageRepo{db: db}
}

// Create inserts a page, first making its URL slug unique within the
// course (B044). Canvas semantics: the slug derives from the title and a
// collision gets a numeric suffix ("week-1", "week-1-2", "week-1-3", …),
// so two pages titled "Week 1" never share /pages/week-1. Every creation
// path (handler, module-item auto-create, blueprint sync, Commons import,
// IMSCC import, course copy) goes through here, so the guarantee holds
// for all of them; callers read the final slug back from page.URL.
func (r *pageRepo) Create(ctx context.Context, page *models.WikiPage) error {
	db := r.db.WithContext(ctx)
	base := page.URL
	if base == "" {
		base = slugifyPageTitle(page.Title)
	}
	if base == "" {
		base = "page"
	}
	candidate := base
	for n := 2; ; n++ {
		var count int64
		if err := db.Model(&models.WikiPage{}).
			Where("course_id = ? AND url = ? AND workflow_state != ?", page.CourseID, candidate, "deleted").
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			break
		}
		candidate = fmt.Sprintf("%s-%d", base, n)
	}
	page.URL = candidate
	return db.Create(page).Error
}

var pageSlugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// slugifyPageTitle mirrors service.slugify (lower-case, non-alphanumeric
// runs collapsed to '-', trimmed).
func slugifyPageTitle(title string) string {
	slug := pageSlugNonAlnum.ReplaceAllString(strings.ToLower(title), "-")
	return strings.Trim(slug, "-")
}

func (r *pageRepo) FindByID(ctx context.Context, id, accountID uint) (*models.WikiPage, error) {
	var page models.WikiPage
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		q = q.Where("course_id IN (SELECT id FROM courses WHERE account_id = ?)", accountID)
	}
	if err := q.First(&page, id).Error; err != nil {
		return nil, err
	}
	return &page, nil
}

func (r *pageRepo) FindByCourseAndURL(ctx context.Context, courseID uint, url string) (*models.WikiPage, error) {
	var page models.WikiPage
	if err := r.db.WithContext(ctx).Where("course_id = ? AND url = ? AND workflow_state != ?", courseID, url, "deleted").First(&page).Error; err != nil {
		return nil, err
	}
	return &page, nil
}

func (r *pageRepo) FindPublicByCourseAndURL(ctx context.Context, courseID uint, url string) (*models.WikiPage, error) {
	var page models.WikiPage
	if err := r.db.WithContext(ctx).Where("course_id = ? AND url = ? AND public = ? AND workflow_state = ?", courseID, url, true, "active").First(&page).Error; err != nil {
		return nil, err
	}
	return &page, nil
}

func (r *pageRepo) Update(ctx context.Context, page *models.WikiPage) error {
	return r.db.WithContext(ctx).Save(page).Error
}

func (r *pageRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.WikiPage{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *pageRepo) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.WikiPage], error) {
	return r.list(r.db.WithContext(ctx).Model(&models.WikiPage{}).Where("course_id = ? AND workflow_state != ?", courseID, "deleted"), params)
}

// ListPublishedByCourseID returns only published (active) pages — the
// student-facing list (B045).
func (r *pageRepo) ListPublishedByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.WikiPage], error) {
	return r.list(r.db.WithContext(ctx).Model(&models.WikiPage{}).Where("course_id = ? AND workflow_state = ?", courseID, "active"), params)
}

func (r *pageRepo) FindFrontPage(ctx context.Context, courseID uint) (*models.WikiPage, error) {
	var page models.WikiPage
	if err := r.db.WithContext(ctx).
		Where("course_id = ? AND front_page = ? AND workflow_state != ?", courseID, true, "deleted").
		Order("updated_at DESC").
		First(&page).Error; err != nil {
		return nil, err
	}
	return &page, nil
}

func (r *pageRepo) ClearFrontPage(ctx context.Context, courseID, exceptID uint) error {
	return r.db.WithContext(ctx).Model(&models.WikiPage{}).
		Where("course_id = ? AND id <> ? AND front_page = ?", courseID, exceptID, true).
		Update("front_page", false).Error
}

func (r *pageRepo) list(query *gorm.DB, params repository.PaginationParams) (*repository.PaginatedResult[models.WikiPage], error) {
	var pages []models.WikiPage
	var count int64

	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("title ASC").Find(&pages).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.WikiPage]{
		Items:      pages,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
