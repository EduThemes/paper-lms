package postgres

import (
	"context"

	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// BlackoutDateRepository persists course pacing blackout dates.
type BlackoutDateRepository struct {
	db *gorm.DB
}

func NewBlackoutDateRepository(db *gorm.DB) *BlackoutDateRepository {
	return &BlackoutDateRepository{db: db}
}

// ListByCourse returns the course's blackout dates by start date.
func (r *BlackoutDateRepository) ListByCourse(ctx context.Context, courseID uint) ([]models.BlackoutDate, error) {
	var rows []models.BlackoutDate
	err := r.db.WithContext(ctx).Where("course_id = ?", courseID).Order("start_date ASC, id ASC").Find(&rows).Error
	return rows, err
}

func (r *BlackoutDateRepository) Create(ctx context.Context, b *models.BlackoutDate) error {
	return r.db.WithContext(ctx).Create(b).Error
}

// Delete removes blackout date id when it belongs to courseID; deleted
// is false when no such row exists (parent tie).
func (r *BlackoutDateRepository) Delete(ctx context.Context, id, courseID uint) (bool, error) {
	res := r.db.WithContext(ctx).Where("id = ? AND course_id = ?", id, courseID).Delete(&models.BlackoutDate{})
	return res.RowsAffected > 0, res.Error
}
