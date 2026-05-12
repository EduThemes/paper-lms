package service

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// Wave A2 service interfaces. The QuizService itself is intentionally not
// re-declared here so Wave A1 owns its own interface surface.

// QuizItemBankServiceInterface defines the operations on course-scoped item
// banks and reusable bank items.
type QuizItemBankServiceInterface interface {
	CreateBank(ctx context.Context, bank *models.QuizItemBank) error
	GetBank(ctx context.Context, courseID, id uint) (*models.QuizItemBank, error)
	ListBanks(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.QuizItemBank], error)
	UpdateBank(ctx context.Context, courseID uint, bank *models.QuizItemBank) error
	DeleteBank(ctx context.Context, courseID, id uint) error

	CreateBankItem(ctx context.Context, item *models.QuizItemBankItem) error
	GetBankItem(ctx context.Context, id uint) (*models.QuizItemBankItem, error)
	ListBankItems(ctx context.Context, bankID uint) ([]models.QuizItemBankItem, error)
	UpdateBankItem(ctx context.Context, item *models.QuizItemBankItem) error
	DeleteBankItem(ctx context.Context, id uint) error

	AddBankItemToQuiz(ctx context.Context, bankItemID, quizID uint, position int) (*models.QuizQuestion, error)
	RandomDrawFromBank(ctx context.Context, bankID uint, count int) ([]models.QuizQuestion, error)
}

// QuizStimulusServiceInterface defines the operations on shared stimulus
// passages and the question -> stimulus relationship.
type QuizStimulusServiceInterface interface {
	CreateStimulus(ctx context.Context, s *models.QuizStimulus) error
	GetStimulus(ctx context.Context, courseID, id uint) (*models.QuizStimulus, error)
	ListStimuli(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.QuizStimulus], error)
	UpdateStimulus(ctx context.Context, courseID uint, s *models.QuizStimulus) error
	DeleteStimulus(ctx context.Context, courseID, id uint) error

	LinkQuestionToStimulus(ctx context.Context, questionID, stimulusID uint) error
	UnlinkQuestionFromStimulus(ctx context.Context, questionID uint) error
	ListQuestionsForStimulus(ctx context.Context, stimulusID uint) ([]models.QuizQuestion, error)
}

// QuizOutcomeAlignmentServiceInterface defines per-question outcome alignment.
// The grader does NOT consume this data layer yet (Wave A1 owns the grader);
// it is exposed for instructor UIs and downstream reporting.
type QuizOutcomeAlignmentServiceInterface interface {
	Align(ctx context.Context, quizQuestionID, outcomeID uint, masteryThreshold float64) (*models.QuizQuestionOutcomeAlignment, error)
	Unalign(ctx context.Context, quizQuestionID, outcomeID uint) error
	ListByQuestion(ctx context.Context, quizQuestionID uint) ([]models.QuizQuestionOutcomeAlignment, error)
	ListByOutcome(ctx context.Context, outcomeID uint) ([]models.QuizQuestionOutcomeAlignment, error)
}
