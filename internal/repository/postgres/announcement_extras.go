package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

// AnnouncementExtrasRepository stores announcement replies and the
// section list of a section-targeted announcement (G090).
type AnnouncementExtrasRepository struct {
	db *gorm.DB
}

// NewAnnouncementExtrasRepository creates the postgres-backed store.
func NewAnnouncementExtrasRepository(db *gorm.DB) *AnnouncementExtrasRepository {
	return &AnnouncementExtrasRepository{db: db}
}

// CreateReply inserts a reply.
func (r *AnnouncementExtrasRepository) CreateReply(ctx context.Context, reply *models.AnnouncementReply) error {
	return r.db.WithContext(ctx).Create(reply).Error
}

// FindReply loads a live reply of the announcement (gorm.ErrRecordNotFound
// otherwise, so a reply id of another announcement never resolves).
func (r *AnnouncementExtrasRepository) FindReply(ctx context.Context, announcementID, replyID uint) (*models.AnnouncementReply, error) {
	var reply models.AnnouncementReply
	err := r.db.WithContext(ctx).
		Where("id = ? AND announcement_id = ? AND workflow_state = ?", replyID, announcementID, models.AnnouncementReplyActive).
		First(&reply).Error
	if err != nil {
		return nil, err
	}
	return &reply, nil
}

// UpdateReply saves a reply.
func (r *AnnouncementExtrasRepository) UpdateReply(ctx context.Context, reply *models.AnnouncementReply) error {
	return r.db.WithContext(ctx).Save(reply).Error
}

// ListReplies pages through an announcement's live replies, oldest first.
func (r *AnnouncementExtrasRepository) ListReplies(ctx context.Context, announcementID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AnnouncementReply], error) {
	q := r.db.WithContext(ctx).Model(&models.AnnouncementReply{}).
		Where("announcement_id = ? AND workflow_state = ?", announcementID, models.AnnouncementReplyActive)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	if params.PerPage <= 0 {
		params.PerPage = 50
	}
	if params.Page <= 0 {
		params.Page = 1
	}
	var rows []models.AnnouncementReply
	if err := q.Order("created_at ASC, id ASC").Offset((params.Page - 1) * params.PerPage).Limit(params.PerPage).Find(&rows).Error; err != nil {
		return nil, err
	}
	return &repository.PaginatedResult[models.AnnouncementReply]{Items: rows, TotalCount: total, Page: params.Page, PerPage: params.PerPage}, nil
}

// ReplyCounts returns the live reply count of each announcement.
func (r *AnnouncementExtrasRepository) ReplyCounts(ctx context.Context, announcementIDs []uint) (map[uint]int64, error) {
	out := map[uint]int64{}
	if len(announcementIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		AnnouncementID uint
		N              int64
	}
	err := r.db.WithContext(ctx).Model(&models.AnnouncementReply{}).
		Select("announcement_id, COUNT(*) AS n").
		Where("announcement_id IN ? AND workflow_state = ?", announcementIDs, models.AnnouncementReplyActive).
		Group("announcement_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.AnnouncementID] = row.N
	}
	return out, nil
}

// SetSections replaces the section list of an announcement.
func (r *AnnouncementExtrasRepository) SetSections(ctx context.Context, announcementID uint, sectionIDs []uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("announcement_id = ?", announcementID).Delete(&models.AnnouncementSectionVisibility{}).Error; err != nil {
			return err
		}
		if len(sectionIDs) == 0 {
			return nil
		}
		rows := make([]models.AnnouncementSectionVisibility, 0, len(sectionIDs))
		for _, id := range sectionIDs {
			rows = append(rows, models.AnnouncementSectionVisibility{AnnouncementID: announcementID, CourseSectionID: id})
		}
		return tx.Create(&rows).Error
	})
}

// SectionsFor returns the targeted sections of each announcement.
func (r *AnnouncementExtrasRepository) SectionsFor(ctx context.Context, announcementIDs []uint) (map[uint][]uint, error) {
	out := map[uint][]uint{}
	if len(announcementIDs) == 0 {
		return out, nil
	}
	var rows []models.AnnouncementSectionVisibility
	if err := r.db.WithContext(ctx).Where("announcement_id IN ?", announcementIDs).Order("course_section_id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.AnnouncementID] = append(out[row.AnnouncementID], row.CourseSectionID)
	}
	return out, nil
}

// SectionsInCourse returns which of sectionIDs are live sections of the
// course (the parent tie for section targeting).
func (r *AnnouncementExtrasRepository) SectionsInCourse(ctx context.Context, courseID uint, sectionIDs []uint) ([]uint, error) {
	if len(sectionIDs) == 0 {
		return nil, nil
	}
	var ids []uint
	err := r.db.WithContext(ctx).Model(&models.CourseSection{}).
		Where("course_id = ? AND id IN ? AND COALESCE(workflow_state, 'active') <> 'deleted'", courseID, sectionIDs).
		Pluck("id", &ids).Error
	return ids, err
}
