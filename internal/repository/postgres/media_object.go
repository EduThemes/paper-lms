package postgres

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type mediaObjectRepo struct {
	db *gorm.DB
}

// NewMediaObjectRepository returns the Postgres media object repository.
func NewMediaObjectRepository(db *gorm.DB) repository.MediaObjectRepository {
	return &mediaObjectRepo{db: db}
}

func (r *mediaObjectRepo) Create(ctx context.Context, m *models.MediaObject) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// FindByMediaID — accountID==0 disables the tenant scope (auth-internal
// callers only; none today).
func (r *mediaObjectRepo) FindByMediaID(ctx context.Context, mediaID string, accountID uint) (*models.MediaObject, error) {
	var m models.MediaObject
	q := r.db.WithContext(ctx).Where("media_id = ? AND workflow_state = ?", mediaID, models.MediaObjectActive)
	if accountID != 0 {
		q = q.Where("account_id = ?", accountID)
	}
	if err := q.First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *mediaObjectRepo) Update(ctx context.Context, m *models.MediaObject) error {
	return r.db.WithContext(ctx).Save(m).Error
}

func (r *mediaObjectRepo) ListByUser(ctx context.Context, userID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.MediaObject], error) {
	var items []models.MediaObject
	var count int64
	q := r.db.WithContext(ctx).Model(&models.MediaObject{}).
		Where("user_id = ? AND workflow_state = ?", userID, models.MediaObjectActive)
	if accountID != 0 {
		q = q.Where("account_id = ?", accountID)
	}
	if err := q.Count(&count).Error; err != nil {
		return nil, err
	}
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PerPage < 1 {
		params.PerPage = 20
	}
	if err := q.Order("created_at DESC, id DESC").Offset((params.Page - 1) * params.PerPage).Limit(params.PerPage).Find(&items).Error; err != nil {
		return nil, err
	}
	return &repository.PaginatedResult[models.MediaObject]{Items: items, TotalCount: count, Page: params.Page, PerPage: params.PerPage}, nil
}

func (r *mediaObjectRepo) CoursesTaughtToViewer(ctx context.Context, instructorID, viewerID, accountID uint, limit int) ([]uint, error) {
	if instructorID == 0 || viewerID == 0 || accountID == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	var ids []uint
	err := r.db.WithContext(ctx).Table("enrollments AS ie").
		Distinct("ie.course_id").
		Joins("JOIN enrollments ve ON ve.course_id = ie.course_id").
		Joins("JOIN courses c ON c.id = ie.course_id").
		Where("ie.user_id = ? AND ie.workflow_state = ? AND ie.type IN ?", instructorID, models.EnrollmentActive,
			[]string{"TeacherEnrollment", "TaEnrollment", "DesignerEnrollment"}).
		Where("ve.user_id = ? AND ve.workflow_state = ?", viewerID, models.EnrollmentActive).
		Where("c.account_id = ? AND c.workflow_state <> ?", accountID, models.CourseDeleted).
		Order("ie.course_id").
		Limit(limit).
		Pluck("ie.course_id", &ids).Error
	return ids, err
}

type mediaTrackRepo struct {
	db *gorm.DB
}

// NewMediaTrackRepository returns the Postgres media track repository.
func NewMediaTrackRepository(db *gorm.DB) repository.MediaTrackRepository {
	return &mediaTrackRepo{db: db}
}

func (r *mediaTrackRepo) scoped(ctx context.Context, accountID uint) *gorm.DB {
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		q = q.Where("account_id = ?", accountID)
	}
	return q
}

func (r *mediaTrackRepo) ListByMediaObject(ctx context.Context, mediaObjectID, accountID uint) ([]models.MediaTrack, error) {
	var tracks []models.MediaTrack
	err := r.scoped(ctx, accountID).Where("media_object_id = ?", mediaObjectID).Order("locale ASC").Find(&tracks).Error
	return tracks, err
}

func (r *mediaTrackRepo) FindByID(ctx context.Context, id, accountID uint) (*models.MediaTrack, error) {
	var t models.MediaTrack
	if err := r.scoped(ctx, accountID).First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *mediaTrackRepo) Upsert(ctx context.Context, t *models.MediaTrack) error {
	if t.Kind == "" {
		t.Kind = "subtitles"
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "media_object_id"}, {Name: "locale"}},
		DoUpdates: clause.AssignmentColumns([]string{"kind", "label", "content", "webvtt_content", "user_id", "updated_at"}),
	}).Create(t).Error
}

func (r *mediaTrackRepo) Delete(ctx context.Context, id, accountID uint) error {
	res := r.scoped(ctx, accountID).Delete(&models.MediaTrack{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
