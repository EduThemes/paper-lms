package postgres

import (
	"context"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type enrollmentRepo struct {
	db *gorm.DB
}

func NewEnrollmentRepository(db *gorm.DB) repository.EnrollmentRepository {
	return &enrollmentRepo{db: db}
}

// enrollments has no direct account_id column; tenant scope is enforced
// by joining through course_id → courses.account_id. Pattern mirrors
// submission.go's deep subquery used to scope submissions through
// assignment → course → account. See Phase 13.1.D for the convention.
// accountID==0 means "no tenant scope" — only internal callers
// (background workers, seed scripts, gamification wiring) may pass 0.
// Handler-routed callers MUST pass callerAccountID(c).
func tenantScopedByCourse(q *gorm.DB, accountID uint) *gorm.DB {
	if accountID == 0 {
		return q
	}
	return q.Where("course_id IN (SELECT id FROM courses WHERE account_id = ?)", accountID)
}

func (r *enrollmentRepo) Create(ctx context.Context, enrollment *models.Enrollment) error {
	return r.db.WithContext(ctx).Create(enrollment).Error
}

func (r *enrollmentRepo) FindByID(ctx context.Context, id, accountID uint) (*models.Enrollment, error) {
	var enrollment models.Enrollment
	q := r.db.WithContext(ctx)
	q = tenantScopedByCourse(q, accountID)
	if err := q.Preload("User").First(&enrollment, id).Error; err != nil {
		return nil, err
	}
	return &enrollment, nil
}

func (r *enrollmentRepo) ListByCourseID(ctx context.Context, courseID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Enrollment], error) {
	var enrollments []models.Enrollment
	var count int64

	query := r.db.WithContext(ctx).Model(&models.Enrollment{}).Where("course_id = ? AND workflow_state != ?", courseID, "deleted")
	query = tenantScopedByCourse(query, accountID)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Preload("User").Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&enrollments).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Enrollment]{
		Items:      enrollments,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *enrollmentRepo) ListByUserID(ctx context.Context, userID, accountID uint) ([]models.Enrollment, error) {
	var enrollments []models.Enrollment
	q := r.db.WithContext(ctx).Where("user_id = ? AND workflow_state = ?", userID, "active")
	q = tenantScopedByCourse(q, accountID)
	if err := q.Find(&enrollments).Error; err != nil {
		return nil, err
	}
	return enrollments, nil
}

func (r *enrollmentRepo) Update(ctx context.Context, enrollment *models.Enrollment) error {
	return r.db.WithContext(ctx).Save(enrollment).Error
}

func (r *enrollmentRepo) FindByUserAndCourse(ctx context.Context, userID, courseID, accountID uint) (*models.Enrollment, error) {
	var enrollment models.Enrollment
	q := r.db.WithContext(ctx).Where("user_id = ? AND course_id = ? AND workflow_state = ?", userID, courseID, "active")
	q = tenantScopedByCourse(q, accountID)
	if err := q.First(&enrollment).Error; err != nil {
		return nil, err
	}
	return &enrollment, nil
}

func (r *enrollmentRepo) CountByCourseIDs(ctx context.Context, courseIDs []uint, accountID uint) (map[uint]int64, error) {
	if len(courseIDs) == 0 {
		return map[uint]int64{}, nil
	}
	type result struct {
		CourseID uint
		Count    int64
	}
	var results []result
	q := r.db.WithContext(ctx).
		Model(&models.Enrollment{}).
		Select("course_id, count(*) as count").
		Where("course_id IN ? AND workflow_state = ? AND type = ?", courseIDs, "active", "StudentEnrollment")
	q = tenantScopedByCourse(q, accountID)
	err := q.Group("course_id").Find(&results).Error
	if err != nil {
		return nil, err
	}
	counts := make(map[uint]int64, len(results))
	for _, r := range results {
		counts[r.CourseID] = r.Count
	}
	return counts, nil
}

// ListActiveStudentUserIDsByCourse returns user_ids of active
// StudentEnrollment rows for a course. The leaderboard candidate set
// before any opt-out filter or ranking. Order is by enrollment id
// ascending (stable but not load-bearing — the caller ranks).
func (r *enrollmentRepo) ListActiveStudentUserIDsByCourse(ctx context.Context, courseID, accountID uint) ([]uint, error) {
	var ids []uint
	q := r.db.WithContext(ctx).
		Model(&models.Enrollment{}).
		Where("course_id = ? AND workflow_state = ? AND type = ?", courseID, "active", "StudentEnrollment")
	q = tenantScopedByCourse(q, accountID)
	err := q.Order("id ASC").Pluck("user_id", &ids).Error
	return ids, err
}

// ListActiveStudentEnrollmentsByCourse returns full Enrollment rows
// for the candidate set — same filter as the id-only variant, but
// returns pseudonym + workflow + type fields the leaderboard render
// path needs.
func (r *enrollmentRepo) ListActiveStudentEnrollmentsByCourse(ctx context.Context, courseID, accountID uint) ([]models.Enrollment, error) {
	var rows []models.Enrollment
	q := r.db.WithContext(ctx).
		Where("course_id = ? AND workflow_state = ? AND type = ?", courseID, "active", "StudentEnrollment")
	q = tenantScopedByCourse(q, accountID)
	err := q.Order("id ASC").Find(&rows).Error
	return rows, err
}

// UpdatePseudonymForSelf atomically writes (pool_code, name) to the
// self-enrollment row. The UNIQUE partial index
// (course_id, pseudonym_pool_code, pseudonym_name) WHERE pseudonym_name
// IS NOT NULL gates collisions; we translate that sentinel to
// repository.ErrPseudonymTaken so the handler can return 409.
//
// Empty `name` is serialized to SQL NULL (not the empty string) so it
// stays out of the partial index — that's the "not yet assigned" and
// the first_name special-case state.
func (r *enrollmentRepo) UpdatePseudonymForSelf(ctx context.Context, userID, courseID, accountID uint, poolCode, name string) error {
	updates := map[string]interface{}{
		"pseudonym_pool_code": poolCode,
	}
	if name == "" {
		updates["pseudonym_name"] = gorm.Expr("NULL")
	} else {
		updates["pseudonym_name"] = name
	}
	q := r.db.WithContext(ctx).
		Model(&models.Enrollment{}).
		Where("user_id = ? AND course_id = ? AND workflow_state = ?", userID, courseID, "active")
	q = tenantScopedByCourse(q, accountID)
	tx := q.Updates(updates)
	if err := tx.Error; err != nil {
		// Postgres surfaces unique violations as a string match in the
		// driver error. We translate so handlers don't depend on the
		// driver-specific shape.
		if isUniqueViolation(err) {
			return repository.ErrPseudonymTaken
		}
		return err
	}
	if tx.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// isUniqueViolation pattern-matches the Postgres-side unique-violation
// signature. Kept local because we don't import the pq error package
// at the repo layer today; the string match is stable across recent
// Postgres versions and the test suite covers the path explicitly.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate key value") || strings.Contains(msg, "SQLSTATE 23505")
}
