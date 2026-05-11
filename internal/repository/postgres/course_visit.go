package postgres

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type courseVisitRepo struct {
	db *gorm.DB
}

func NewCourseVisitRepository(db *gorm.DB) repository.CourseVisitRepository {
	return &courseVisitRepo{db: db}
}

func (r *courseVisitRepo) Upsert(ctx context.Context, visit *models.CourseVisit) error {
	now := time.Now()
	return r.db.WithContext(ctx).Exec(
		"INSERT INTO course_visits (user_id, course_id, last_url, last_title, updated_at) VALUES (?, ?, ?, ?, ?) ON CONFLICT (user_id, course_id) DO UPDATE SET last_url = ?, last_title = ?, updated_at = ?",
		visit.UserID, visit.CourseID, visit.LastURL, visit.LastTitle, now,
		visit.LastURL, visit.LastTitle, now,
	).Error
}

func (r *courseVisitRepo) FindByUserAndCourse(ctx context.Context, userID, courseID uint) (*models.CourseVisit, error) {
	var visit models.CourseVisit
	if err := r.db.WithContext(ctx).Where("user_id = ? AND course_id = ?", userID, courseID).First(&visit).Error; err != nil {
		return nil, err
	}
	return &visit, nil
}
