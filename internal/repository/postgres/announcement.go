package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

// AnnouncementRepository defines the persistence methods for announcements.
type AnnouncementRepository interface {
	Create(ctx context.Context, announcement *models.Announcement) error
	FindByID(ctx context.Context, id uint) (*models.Announcement, error)
	Update(ctx context.Context, announcement *models.Announcement) error
	Delete(ctx context.Context, id uint) error
	ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Announcement], error)
	ListByAccountID(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Announcement], error)
	ListGlobal(ctx context.Context, params repository.PaginationParams) (*repository.PaginatedResult[models.Announcement], error)
	ListScheduledReady(ctx context.Context) ([]models.Announcement, error)
	// ListForViewer lists a course's announcements as one viewer sees
	// them. Visibility (draft/scheduled rows, target audience, section
	// targeting) is applied in SQL so TotalCount and page sizes are
	// exact.
	ListForViewer(ctx context.Context, courseID uint, viewer AnnouncementViewer, params repository.PaginationParams) (*repository.PaginatedResult[models.Announcement], error)
}

// AnnouncementViewer describes who is listing a course's announcements.
//
//   - Instructor (teacher, TA, admin) sees every non-deleted row:
//     drafts and delayed (scheduled) posts included, every audience.
//   - Everyone else sees only published rows whose posted_at has
//     passed, restricted to target_audience 'all' (or empty), the
//     audiences in Audiences, and 'section:<id>' for SectionIDs.
type AnnouncementViewer struct {
	Instructor bool
	Audiences  []string
	SectionIDs []uint
}

type announcementRepo struct {
	db *gorm.DB
}

// NewAnnouncementRepository creates a new postgres-backed announcement repository.
func NewAnnouncementRepository(db *gorm.DB) AnnouncementRepository {
	return &announcementRepo{db: db}
}

func (r *announcementRepo) Create(ctx context.Context, announcement *models.Announcement) error {
	return r.db.WithContext(ctx).Create(announcement).Error
}

func (r *announcementRepo) FindByID(ctx context.Context, id uint) (*models.Announcement, error) {
	var announcement models.Announcement
	if err := r.db.WithContext(ctx).First(&announcement, id).Error; err != nil {
		return nil, err
	}
	return &announcement, nil
}

func (r *announcementRepo) Update(ctx context.Context, announcement *models.Announcement) error {
	return r.db.WithContext(ctx).Save(announcement).Error
}

func (r *announcementRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.Announcement{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *announcementRepo) ListForViewer(ctx context.Context, courseID uint, viewer AnnouncementViewer, params repository.PaginationParams) (*repository.PaginatedResult[models.Announcement], error) {
	var announcements []models.Announcement
	var count int64

	query := r.db.WithContext(ctx).Model(&models.Announcement{}).
		Where("course_id = ? AND workflow_state != ?", courseID, "deleted")
	if !viewer.Instructor {
		query = query.
			Where("workflow_state = ?", "published").
			Where("(posted_at IS NULL OR posted_at <= ?)", time.Now())
		audiences := append([]string{"all", ""}, viewer.Audiences...)
		sections := make([]string, 0, len(viewer.SectionIDs))
		for _, id := range viewer.SectionIDs {
			sections = append(sections, fmt.Sprintf("section:%d", id))
		}
		if len(sections) > 0 {
			query = query.Where("(target_audience IS NULL OR target_audience IN ? OR target_audience IN ?)", audiences, sections)
		} else {
			query = query.Where("(target_audience IS NULL OR target_audience IN ?)", audiences)
		}
	}
	if err := query.Count(&count).Error; err != nil {
		return nil, err
	}

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&announcements).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Announcement]{
		Items:      announcements,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *announcementRepo) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Announcement], error) {
	var announcements []models.Announcement
	var count int64

	now := time.Now()
	query := r.db.WithContext(ctx).Model(&models.Announcement{}).
		Where("course_id = ? AND workflow_state != ?", courseID, "deleted").
		Where("workflow_state = ? AND (posted_at IS NULL OR posted_at <= ?)", "published", now)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&announcements).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Announcement]{
		Items:      announcements,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *announcementRepo) ListByAccountID(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Announcement], error) {
	var announcements []models.Announcement
	var count int64

	now := time.Now()
	query := r.db.WithContext(ctx).Model(&models.Announcement{}).
		Where("account_id = ? AND workflow_state != ?", accountID, "deleted").
		Where("workflow_state = ? AND (posted_at IS NULL OR posted_at <= ?)", "published", now)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&announcements).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Announcement]{
		Items:      announcements,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *announcementRepo) ListGlobal(ctx context.Context, params repository.PaginationParams) (*repository.PaginatedResult[models.Announcement], error) {
	var announcements []models.Announcement
	var count int64

	now := time.Now()
	query := r.db.WithContext(ctx).Model(&models.Announcement{}).
		Where("is_global = ? AND workflow_state != ?", true, "deleted").
		Where("workflow_state = ? AND (posted_at IS NULL OR posted_at <= ?)", "published", now)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&announcements).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Announcement]{
		Items:      announcements,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *announcementRepo) ListScheduledReady(ctx context.Context) ([]models.Announcement, error) {
	var announcements []models.Announcement
	now := time.Now()
	if err := r.db.WithContext(ctx).
		Where("workflow_state = ? AND delayed_post_at <= ?", "scheduled", now).
		Find(&announcements).Error; err != nil {
		return nil, err
	}
	return announcements, nil
}
