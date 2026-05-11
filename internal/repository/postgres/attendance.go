package postgres

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AttendanceRepository defines the interface for attendance record persistence.
type AttendanceRepository interface {
	Create(ctx context.Context, record *models.AttendanceRecord) error
	FindByID(ctx context.Context, id uint) (*models.AttendanceRecord, error)
	Update(ctx context.Context, record *models.AttendanceRecord) error
	Delete(ctx context.Context, id uint) error
	FindByCourseUserDate(ctx context.Context, courseID uint, userID uint, date time.Time) (*models.AttendanceRecord, error)
	ListByCourse(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AttendanceRecord], error)
	ListByCourseAndDate(ctx context.Context, courseID uint, date time.Time, params repository.PaginationParams) (*repository.PaginatedResult[models.AttendanceRecord], error)
	ListByUserAndCourse(ctx context.Context, userID uint, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AttendanceRecord], error)
	GetSummary(ctx context.Context, userID uint, courseID uint) (*models.AttendanceSummary, error)
	BulkCreate(ctx context.Context, records []models.AttendanceRecord) error
}

type attendanceRepo struct {
	db *gorm.DB
}

func NewAttendanceRepository(db *gorm.DB) AttendanceRepository {
	return &attendanceRepo{db: db}
}

func (r *attendanceRepo) Create(ctx context.Context, record *models.AttendanceRecord) error {
	return r.db.WithContext(ctx).Create(record).Error
}

func (r *attendanceRepo) FindByID(ctx context.Context, id uint) (*models.AttendanceRecord, error) {
	var record models.AttendanceRecord
	if err := r.db.WithContext(ctx).First(&record, id).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *attendanceRepo) Update(ctx context.Context, record *models.AttendanceRecord) error {
	return r.db.WithContext(ctx).Save(record).Error
}

func (r *attendanceRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.AttendanceRecord{}, id).Error
}

func (r *attendanceRepo) FindByCourseUserDate(ctx context.Context, courseID uint, userID uint, date time.Time) (*models.AttendanceRecord, error) {
	var record models.AttendanceRecord
	if err := r.db.WithContext(ctx).
		Where("course_id = ? AND user_id = ? AND date = ?", courseID, userID, date).
		First(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *attendanceRepo) ListByCourse(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AttendanceRecord], error) {
	var items []models.AttendanceRecord
	var totalCount int64

	query := r.db.WithContext(ctx).Model(&models.AttendanceRecord{}).Where("course_id = ?", courseID)
	query.Count(&totalCount)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Order("date DESC, user_id ASC").Offset(offset).Limit(params.PerPage).Find(&items).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.AttendanceRecord]{
		Items:      items,
		TotalCount: totalCount,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *attendanceRepo) ListByCourseAndDate(ctx context.Context, courseID uint, date time.Time, params repository.PaginationParams) (*repository.PaginatedResult[models.AttendanceRecord], error) {
	var items []models.AttendanceRecord
	var totalCount int64

	query := r.db.WithContext(ctx).Model(&models.AttendanceRecord{}).Where("course_id = ? AND date = ?", courseID, date)
	query.Count(&totalCount)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Order("user_id ASC").Offset(offset).Limit(params.PerPage).Find(&items).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.AttendanceRecord]{
		Items:      items,
		TotalCount: totalCount,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *attendanceRepo) ListByUserAndCourse(ctx context.Context, userID uint, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AttendanceRecord], error) {
	var items []models.AttendanceRecord
	var totalCount int64

	query := r.db.WithContext(ctx).Model(&models.AttendanceRecord{}).Where("user_id = ? AND course_id = ?", userID, courseID)
	query.Count(&totalCount)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Order("date DESC").Offset(offset).Limit(params.PerPage).Find(&items).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.AttendanceRecord]{
		Items:      items,
		TotalCount: totalCount,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *attendanceRepo) GetSummary(ctx context.Context, userID uint, courseID uint) (*models.AttendanceSummary, error) {
	summary := &models.AttendanceSummary{
		UserID:   userID,
		CourseID: courseID,
	}

	var totalCount int64
	r.db.WithContext(ctx).Model(&models.AttendanceRecord{}).
		Where("user_id = ? AND course_id = ?", userID, courseID).
		Count(&totalCount)
	summary.TotalDays = int(totalCount)

	var presentCount int64
	r.db.WithContext(ctx).Model(&models.AttendanceRecord{}).
		Where("user_id = ? AND course_id = ? AND status = ?", userID, courseID, "present").
		Count(&presentCount)
	summary.PresentDays = int(presentCount)

	var absentCount int64
	r.db.WithContext(ctx).Model(&models.AttendanceRecord{}).
		Where("user_id = ? AND course_id = ? AND status = ?", userID, courseID, "absent").
		Count(&absentCount)
	summary.AbsentDays = int(absentCount)

	var tardyCount int64
	r.db.WithContext(ctx).Model(&models.AttendanceRecord{}).
		Where("user_id = ? AND course_id = ? AND status = ?", userID, courseID, "tardy").
		Count(&tardyCount)
	summary.TardyDays = int(tardyCount)

	var excusedCount int64
	r.db.WithContext(ctx).Model(&models.AttendanceRecord{}).
		Where("user_id = ? AND course_id = ? AND status = ?", userID, courseID, "excused").
		Count(&excusedCount)
	summary.ExcusedDays = int(excusedCount)

	if summary.TotalDays > 0 {
		// Attendance rate: present + tardy (arrived) / total non-excused days
		nonExcused := summary.TotalDays - summary.ExcusedDays
		if nonExcused > 0 {
			summary.AttendanceRate = float64(summary.PresentDays+summary.TardyDays) / float64(nonExcused) * 100
		} else {
			summary.AttendanceRate = 100
		}
	}

	return summary, nil
}

func (r *attendanceRepo) BulkCreate(ctx context.Context, records []models.AttendanceRecord) error {
	if len(records) == 0 {
		return nil
	}
	// Upsert: if a record for the same course/user/date already exists, update the status and notes
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "course_id"}, {Name: "user_id"}, {Name: "date"}},
			DoUpdates: clause.AssignmentColumns([]string{"status", "notes", "marked_by_id", "updated_at"}),
		}).
		Create(&records).Error
}
