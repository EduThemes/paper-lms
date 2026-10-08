package service

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// AdminToolsService backs the account Admin Tools page (G119): finding and
// restoring deleted courses. Logs by user live on AdminUserService.
type AdminToolsService struct {
	db *gorm.DB
}

// NewAdminToolsService builds the service.
func NewAdminToolsService(db *gorm.DB) *AdminToolsService {
	return &AdminToolsService{db: db}
}

// ErrDeletedCourseNotFound is returned when the course is not a deleted
// course of the account.
var ErrDeletedCourseNotFound = errors.New("deleted course not found")

// ListDeletedCourses returns an account's deleted courses, most recently
// deleted first, optionally filtered by name, code, SIS id or numeric id.
func (s *AdminToolsService) ListDeletedCourses(ctx context.Context, accountID uint, search string, limit int) ([]models.Course, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q := s.db.WithContext(ctx).Where("account_id = ? AND workflow_state = ?", accountID, models.CourseDeleted)
	if term := strings.TrimSpace(search); term != "" {
		like := "%" + strings.ToLower(term) + "%"
		q = q.Where("(LOWER(name) LIKE ? OR LOWER(course_code) LIKE ? OR sis_course_id = ? OR CAST(id AS text) = ?)", like, like, term, term)
	}
	var out []models.Course
	err := q.Order("updated_at DESC, id DESC").Limit(limit).Find(&out).Error
	return out, err
}

// RestoreCourse brings a deleted course of accountID back as unpublished
// (Canvas restores to "claimed" so students don't see it until a teacher
// republishes). Enrollments, content and grades were never removed by the
// soft delete, so they come back with it.
func (s *AdminToolsService) RestoreCourse(ctx context.Context, courseID, accountID uint) (*models.Course, error) {
	var c models.Course
	if err := s.db.WithContext(ctx).
		Where("id = ? AND account_id = ? AND workflow_state = ?", courseID, accountID, models.CourseDeleted).
		First(&c).Error; err != nil {
		return nil, ErrDeletedCourseNotFound
	}
	c.WorkflowState = models.CourseClaimed
	if err := s.db.WithContext(ctx).Model(&c).Update("workflow_state", models.CourseClaimed).Error; err != nil {
		return nil, err
	}
	return &c, nil
}
