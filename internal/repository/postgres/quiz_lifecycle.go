package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ---- quiz_extensions ----

type quizExtensionRepo struct{ db *gorm.DB }

func NewQuizExtensionRepository(db *gorm.DB) *quizExtensionRepo {
	return &quizExtensionRepo{db: db}
}

func (r *quizExtensionRepo) Upsert(ctx context.Context, ext *models.QuizExtension) error {
	now := time.Now()
	if ext.CreatedAt.IsZero() {
		ext.CreatedAt = now
	}
	ext.UpdatedAt = now
	// Raw upsert so false / 0 values are written (GORM omits zero values
	// on Create and would let the DB defaults hide an explicit reset).
	return r.db.WithContext(ctx).Raw(`
		INSERT INTO quiz_extensions (quiz_id, user_id, extra_attempts, extra_time, manually_unlocked, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (quiz_id, user_id) DO UPDATE SET
			extra_attempts = EXCLUDED.extra_attempts,
			extra_time = EXCLUDED.extra_time,
			manually_unlocked = EXCLUDED.manually_unlocked,
			updated_at = EXCLUDED.updated_at
		RETURNING id, created_at`,
		ext.QuizID, ext.UserID, ext.ExtraAttempts, ext.ExtraTime, ext.ManuallyUnlocked, ext.CreatedAt, ext.UpdatedAt,
	).Row().Scan(&ext.ID, &ext.CreatedAt)
}

func (r *quizExtensionRepo) FindByQuizAndUser(ctx context.Context, quizID, userID uint) (*models.QuizExtension, error) {
	var ext models.QuizExtension
	err := r.db.WithContext(ctx).Where("quiz_id = ? AND user_id = ?", quizID, userID).First(&ext).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ext, nil
}

func (r *quizExtensionRepo) ListByQuizID(ctx context.Context, quizID uint) ([]models.QuizExtension, error) {
	var out []models.QuizExtension
	err := r.db.WithContext(ctx).Where("quiz_id = ?", quizID).Order("user_id ASC").Find(&out).Error
	return out, err
}

// ---- quiz_submission_events ----

type quizSubmissionEventRepo struct{ db *gorm.DB }

func NewQuizSubmissionEventRepository(db *gorm.DB) *quizSubmissionEventRepo {
	return &quizSubmissionEventRepo{db: db}
}

func (r *quizSubmissionEventRepo) BulkCreate(ctx context.Context, events []models.QuizSubmissionEvent) error {
	if len(events) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Omit(clause.Associations).Create(&events).Error
}

func (r *quizSubmissionEventRepo) ListBySubmissionID(ctx context.Context, submissionID uint) ([]models.QuizSubmissionEvent, error) {
	var out []models.QuizSubmissionEvent
	err := r.db.WithContext(ctx).
		Where("quiz_submission_id = ?", submissionID).
		Order("created_at ASC, id ASC").
		Find(&out).Error
	return out, err
}

// ---- quiz_question_regrades ----

type quizQuestionRegradeRepo struct{ db *gorm.DB }

func NewQuizQuestionRegradeRepository(db *gorm.DB) *quizQuestionRegradeRepo {
	return &quizQuestionRegradeRepo{db: db}
}

func (r *quizQuestionRegradeRepo) Create(ctx context.Context, regrade *models.QuizQuestionRegrade) error {
	return r.db.WithContext(ctx).Create(regrade).Error
}

func (r *quizQuestionRegradeRepo) ListByQuizID(ctx context.Context, quizID uint) ([]models.QuizQuestionRegrade, error) {
	var out []models.QuizQuestionRegrade
	err := r.db.WithContext(ctx).Where("quiz_id = ?", quizID).Order("created_at DESC, id DESC").Find(&out).Error
	return out, err
}

// ---- overdue attempts (quiz_submissions) ----

// ListOverdueUntaken returns in-progress attempts whose end_at passed
// before cutoff, oldest first.
func (r *quizSubmissionRepo) ListOverdueUntaken(ctx context.Context, cutoff time.Time, limit int) ([]models.QuizSubmission, error) {
	if limit <= 0 {
		limit = 200
	}
	var out []models.QuizSubmission
	err := r.db.WithContext(ctx).
		Where("workflow_state = ? AND end_at IS NOT NULL AND end_at < ?", models.QuizSubmissionUntaken, cutoff).
		Order("end_at ASC, id ASC").
		Limit(limit).
		Find(&out).Error
	return out, err
}

// ---- graded quizzes without a gradebook assignment (quizzes) ----

// ListGradedWithoutAssignment returns live graded quizzes with id >
// afterID that have no gradebook assignment of their own: assignment_id
// is NULL, or it points at an assignment outside the quiz's course (a
// link carried over by a copy or import). Oldest first.
func (r *quizRepo) ListGradedWithoutAssignment(ctx context.Context, afterID uint, limit int) ([]models.Quiz, error) {
	if limit <= 0 {
		limit = 100
	}
	var out []models.Quiz
	err := r.db.WithContext(ctx).
		Where("id > ? AND workflow_state <> ?", afterID, "deleted").
		Where(`(assignment_id IS NULL OR NOT EXISTS (
			SELECT 1 FROM assignments a WHERE a.id = quizzes.assignment_id AND a.course_id = quizzes.course_id))`).
		Where("(quiz_type IN (?, ?) OR quiz_type = '' OR quiz_type IS NULL)", models.QuizTypeAssignment, models.QuizTypeGradedSurvey).
		Order("id ASC").
		Limit(limit).
		Find(&out).Error
	return out, err
}

// SetEndAtIfInProgress moves an attempt's deadline only while it is still
// in progress, touching nothing else on the row. Returns false when the
// attempt was turned in meanwhile (a full-row Save here would reopen it).
func (r *quizSubmissionRepo) SetEndAtIfInProgress(ctx context.Context, id uint, endAt time.Time) (bool, error) {
	res := r.db.WithContext(ctx).
		Model(&models.QuizSubmission{}).
		Where("id = ? AND workflow_state = ?", id, models.QuizSubmissionUntaken).
		Updates(map[string]interface{}{"end_at": endAt, "updated_at": time.Now()})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// MarkResultsSeen stamps the first time the student viewed an attempt's
// results (one_time_results). Only that column is written, and only once.
func (r *quizSubmissionRepo) MarkResultsSeen(ctx context.Context, id uint, at time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.QuizSubmission{}).
		Where("id = ? AND results_seen_at IS NULL", id).
		UpdateColumn("results_seen_at", at).Error
}
