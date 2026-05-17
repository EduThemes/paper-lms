package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

type QuizRepository interface {
	Create(ctx context.Context, quiz *models.Quiz) error
	// 13.1.D — tenant scope via parent course's account_id.
	FindByID(ctx context.Context, id, accountID uint) (*models.Quiz, error)
	Update(ctx context.Context, quiz *models.Quiz) error
	Delete(ctx context.Context, id uint) error
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.Quiz], error)
}

type QuizQuestionRepository interface {
	Create(ctx context.Context, question *models.QuizQuestion) error
	FindByID(ctx context.Context, id uint) (*models.QuizQuestion, error)
	FindByIDs(ctx context.Context, ids []uint) ([]models.QuizQuestion, error)
	Update(ctx context.Context, question *models.QuizQuestion) error
	Delete(ctx context.Context, id uint) error
	ListByQuizID(ctx context.Context, quizID uint, params PaginationParams) (*PaginatedResult[models.QuizQuestion], error)
	ListByGroupID(ctx context.Context, groupID uint) ([]models.QuizQuestion, error)
}

type QuizSubmissionRepository interface {
	Create(ctx context.Context, submission *models.QuizSubmission) error
	FindByID(ctx context.Context, id uint) (*models.QuizSubmission, error)
	Update(ctx context.Context, submission *models.QuizSubmission) error
	FindByQuizAndUser(ctx context.Context, quizID, userID uint) (*models.QuizSubmission, error)
	// ListByUserAndQuizIDs is the snapshot loader's targeted read for the
	// SubmittedQuiz predicate. Returns the latest attempt per quiz in the
	// supplied set; callers that need attempt history should still use
	// FindByQuizAndUser plus the attempt column.
	ListByUserAndQuizIDs(ctx context.Context, userID uint, quizIDs []uint) ([]models.QuizSubmission, error)
	ListByQuizID(ctx context.Context, quizID uint, params PaginationParams) (*PaginatedResult[models.QuizSubmission], error)
	ListCompletedByQuizID(ctx context.Context, quizID uint) ([]models.QuizSubmission, error)
}

type QuizSubmissionAnswerRepository interface {
	Create(ctx context.Context, answer *models.QuizSubmissionAnswer) error
	BulkCreate(ctx context.Context, answers []models.QuizSubmissionAnswer) error
	FindByID(ctx context.Context, id uint) (*models.QuizSubmissionAnswer, error)
	Update(ctx context.Context, answer *models.QuizSubmissionAnswer) error
	ListBySubmissionID(ctx context.Context, submissionID uint) ([]models.QuizSubmissionAnswer, error)
	FindBySubmissionAndQuestion(ctx context.Context, submissionID, questionID uint) (*models.QuizSubmissionAnswer, error)
	ListBySubmissionIDs(ctx context.Context, submissionIDs []uint) ([]models.QuizSubmissionAnswer, error)
}

type QuizQuestionGroupRepository interface {
	Create(ctx context.Context, group *models.QuizQuestionGroup) error
	FindByID(ctx context.Context, id uint) (*models.QuizQuestionGroup, error)
	Update(ctx context.Context, group *models.QuizQuestionGroup) error
	Delete(ctx context.Context, id uint) error
	ListByQuizID(ctx context.Context, quizID uint) ([]models.QuizQuestionGroup, error)
}

// Wave A2: Quiz Item Banks, Stimuli, Per-Question Outcome Alignment.

type QuizItemBankRepository interface {
	Create(ctx context.Context, bank *models.QuizItemBank) error
	FindByID(ctx context.Context, id uint) (*models.QuizItemBank, error)
	Update(ctx context.Context, bank *models.QuizItemBank) error
	Delete(ctx context.Context, id uint) error
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.QuizItemBank], error)
}

type QuizItemBankItemRepository interface {
	Create(ctx context.Context, item *models.QuizItemBankItem) error
	FindByID(ctx context.Context, id uint) (*models.QuizItemBankItem, error)
	Update(ctx context.Context, item *models.QuizItemBankItem) error
	Delete(ctx context.Context, id uint) error
	ListByBankID(ctx context.Context, bankID uint) ([]models.QuizItemBankItem, error)
}

type QuizStimulusRepository interface {
	Create(ctx context.Context, stimulus *models.QuizStimulus) error
	FindByID(ctx context.Context, id uint) (*models.QuizStimulus, error)
	Update(ctx context.Context, stimulus *models.QuizStimulus) error
	Delete(ctx context.Context, id uint) error
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.QuizStimulus], error)
	ListQuestionsForStimulus(ctx context.Context, stimulusID uint) ([]models.QuizQuestion, error)
	SetQuestionStimulus(ctx context.Context, questionID uint, stimulusID *uint) error
}

type QuizQuestionOutcomeAlignmentRepository interface {
	Create(ctx context.Context, alignment *models.QuizQuestionOutcomeAlignment) error
	Delete(ctx context.Context, id uint) error
	DeleteByQuestionAndOutcome(ctx context.Context, quizQuestionID, outcomeID uint) error
	FindByQuestionAndOutcome(ctx context.Context, quizQuestionID, outcomeID uint) (*models.QuizQuestionOutcomeAlignment, error)
	ListByQuestionID(ctx context.Context, quizQuestionID uint) ([]models.QuizQuestionOutcomeAlignment, error)
	ListByOutcomeID(ctx context.Context, outcomeID uint) ([]models.QuizQuestionOutcomeAlignment, error)
}

// Question Banks (separate aggregate from QuizItemBank — older Canvas
// API surface; kept distinct because the data shapes differ).

type QuestionBankRepository interface {
	Create(ctx context.Context, qb *models.QuestionBank) error
	FindByID(ctx context.Context, id uint) (*models.QuestionBank, error)
	Update(ctx context.Context, qb *models.QuestionBank) error
	Delete(ctx context.Context, id uint) error
	ListByCourse(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.QuestionBank], error)
}

type QuestionBankEntryRepository interface {
	Create(ctx context.Context, entry *models.QuestionBankEntry) error
	FindByID(ctx context.Context, id uint) (*models.QuestionBankEntry, error)
	Update(ctx context.Context, entry *models.QuestionBankEntry) error
	Delete(ctx context.Context, id uint) error
	ListByBankID(ctx context.Context, bankID uint) ([]models.QuestionBankEntry, error)
}
