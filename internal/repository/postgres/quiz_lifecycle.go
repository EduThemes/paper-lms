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
