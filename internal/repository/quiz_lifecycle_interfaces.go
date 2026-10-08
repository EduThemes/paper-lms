package repository

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// QuizExtensionRepository stores per-student quiz moderation settings.
type QuizExtensionRepository interface {
	// Upsert creates or replaces the (quiz_id, user_id) row.
	Upsert(ctx context.Context, ext *models.QuizExtension) error
	FindByQuizAndUser(ctx context.Context, quizID, userID uint) (*models.QuizExtension, error)
	ListByQuizID(ctx context.Context, quizID uint) ([]models.QuizExtension, error)
}

// QuizSubmissionEventRepository stores the attempt audit log.
type QuizSubmissionEventRepository interface {
	BulkCreate(ctx context.Context, events []models.QuizSubmissionEvent) error
	ListBySubmissionID(ctx context.Context, submissionID uint) ([]models.QuizSubmissionEvent, error)
}

// QuizQuestionRegradeRepository stores regrade runs.
type QuizQuestionRegradeRepository interface {
	Create(ctx context.Context, regrade *models.QuizQuestionRegrade) error
	ListByQuizID(ctx context.Context, quizID uint) ([]models.QuizQuestionRegrade, error)
}

// QuizOverdueSubmissionFinder lists in-progress attempts whose deadline
// passed before cutoff (the server-side auto-submit job's work queue).
type QuizOverdueSubmissionFinder interface {
	ListOverdueUntaken(ctx context.Context, cutoff time.Time, limit int) ([]models.QuizSubmission, error)
}
