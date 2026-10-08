package postgres

import (
	"context"

	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type accountReportRepo struct {
	db *gorm.DB
}

// NewAccountReportRepository builds the account report repository (G138).
func NewAccountReportRepository(db *gorm.DB) repository.AccountReportRepository {
	return &accountReportRepo{db: db}
}

func (r *accountReportRepo) Create(ctx context.Context, rep *models.AccountReport) error {
	return r.db.WithContext(ctx).Create(rep).Error
}

func (r *accountReportRepo) Update(ctx context.Context, rep *models.AccountReport) error {
	return r.db.WithContext(ctx).Save(rep).Error
}

func (r *accountReportRepo) FindByID(ctx context.Context, id, accountID uint) (*models.AccountReport, error) {
	var rep models.AccountReport
	q := r.db.WithContext(ctx).Where("workflow_state <> ?", models.AccountReportDeleted)
	if accountID != 0 {
		q = q.Where("account_id = ?", accountID)
	}
	if err := q.First(&rep, id).Error; err != nil {
		return nil, err
	}
	return &rep, nil
}

func (r *accountReportRepo) List(ctx context.Context, reportType string, params repository.PaginationParams, accountID uint) (*repository.PaginatedResult[models.AccountReport], error) {
	q := r.db.WithContext(ctx).Model(&models.AccountReport{}).Where("workflow_state <> ?", models.AccountReportDeleted)
	if accountID != 0 {
		q = q.Where("account_id = ?", accountID)
	}
	if reportType != "" {
		q = q.Where("report_type = ?", reportType)
	}
	return canvasPaginate[models.AccountReport](q, params, "created_at DESC, id DESC")
}

func (r *accountReportRepo) LastRuns(ctx context.Context, accountID uint) (map[string]models.AccountReport, error) {
	var rows []models.AccountReport
	q := r.db.WithContext(ctx).
		Raw(`SELECT DISTINCT ON (report_type) * FROM account_reports
		     WHERE account_id = ? AND workflow_state <> ?
		     ORDER BY report_type, created_at DESC, id DESC`, accountID, models.AccountReportDeleted)
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]models.AccountReport, len(rows))
	for _, rw := range rows {
		out[rw.ReportType] = rw
	}
	return out, nil
}

func (r *accountReportRepo) GradeExportRows(ctx context.Context, termID *uint, accountID uint) ([]repository.GradeExportRow, error) {
	q := r.db.WithContext(ctx).Table("enrollments e").
		Select(`e.user_id AS user_id, u.name AS user_name, u.sis_user_id AS sis_user_id,
		        c.id AS course_id, c.name AS course_name, c.sis_course_id AS sis_course_id,
		        s.id AS section_id, s.name AS section_name, s.sis_section_id AS sis_section_id,
		        t.id AS term_id, t.name AS term_name, t.sis_term_id AS sis_term_id,
		        e.workflow_state AS enrollment_state`).
		Joins("JOIN users u ON u.id = e.user_id").
		Joins("JOIN courses c ON c.id = e.course_id").
		Joins("LEFT JOIN course_sections s ON s.id = e.course_section_id").
		Joins("LEFT JOIN enrollment_terms t ON t.id = c.enrollment_term_id").
		Where("e.type = ? AND e.workflow_state IN ? AND c.workflow_state <> ? AND c.account_id = ?",
			"StudentEnrollment", []string{string(models.EnrollmentActive), string(models.EnrollmentCompleted)},
			models.CourseDeleted, accountID)
	if termID != nil {
		q = q.Where("c.enrollment_term_id = ?", *termID)
	}
	var rows []repository.GradeExportRow
	err := q.Order("c.id ASC, LOWER(u.sortable_name) ASC, u.id ASC, e.id ASC").Scan(&rows).Error
	return rows, err
}

func (r *accountReportRepo) LastAccessRows(ctx context.Context, accountID uint) ([]repository.LastAccessRow, error) {
	var rows []repository.LastAccessRow
	err := r.db.WithContext(ctx).Table("users u").
		Select(`u.id AS user_id, u.sis_user_id AS sis_user_id, u.name AS name, u.sortable_name AS sortable_name,
		        (SELECT MAX(pv.created_at) FROM page_views pv WHERE pv.user_id = u.id) AS last_access_at`).
		Where("u.account_id = ?", accountID).
		Order("LOWER(COALESCE(NULLIF(u.sortable_name, ''), u.name)) ASC, u.id ASC").
		Scan(&rows).Error
	return rows, err
}

func (r *accountReportRepo) UnpublishedCourses(ctx context.Context, termID *uint, accountID uint) ([]models.Course, error) {
	q := r.db.WithContext(ctx).Where("account_id = ? AND workflow_state IN ?", accountID,
		[]models.CourseWorkflow{models.CourseCreated, models.CourseClaimed, models.CourseUnpublished})
	if termID != nil {
		q = q.Where("enrollment_term_id = ?", *termID)
	}
	var out []models.Course
	err := q.Order("id ASC").Find(&out).Error
	return out, err
}
