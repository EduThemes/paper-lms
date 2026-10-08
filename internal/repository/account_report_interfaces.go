package repository

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// GradeExportRow is one student enrollment for the grade_export_csv
// account report.
type GradeExportRow struct {
	UserID          uint
	UserName        string
	SISUserID       *string
	CourseID        uint
	CourseName      string
	SISCourseID     *string
	SectionID       *uint
	SectionName     *string
	SISSectionID    *string
	TermID          *uint
	TermName        *string
	SISTermID       *string
	EnrollmentState string
}

// LastAccessRow is one user for the last_user_access_csv report.
type LastAccessRow struct {
	UserID       uint
	SISUserID    *string
	Name         string
	SortableName string
	LastAccessAt *time.Time
}

// AccountReportRepository stores account report runs and supplies the
// report data queries (G138). Reads take a trailing accountID.
type AccountReportRepository interface {
	Create(ctx context.Context, r *models.AccountReport) error
	Update(ctx context.Context, r *models.AccountReport) error
	FindByID(ctx context.Context, id, accountID uint) (*models.AccountReport, error)
	// List returns the runs of reportType (all types when "") newest first,
	// excluding deleted runs.
	List(ctx context.Context, reportType string, params PaginationParams, accountID uint) (*PaginatedResult[models.AccountReport], error)
	// LastRuns returns the newest non-deleted run per report type.
	LastRuns(ctx context.Context, accountID uint) (map[string]models.AccountReport, error)

	GradeExportRows(ctx context.Context, termID *uint, accountID uint) ([]GradeExportRow, error)
	LastAccessRows(ctx context.Context, accountID uint) ([]LastAccessRow, error)
	UnpublishedCourses(ctx context.Context, termID *uint, accountID uint) ([]models.Course, error)
}

// LoginSessionTokenRepository stores the single-use credentials behind
// GET /login/session_token (G136).
type LoginSessionTokenRepository interface {
	Create(ctx context.Context, t *models.LoginSessionToken) error
	// Consume atomically deletes and returns the row; a miss is an error.
	Consume(ctx context.Context, tokenHash string) (*models.LoginSessionToken, error)
	DeleteExpired(ctx context.Context) error
}
