package service

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// QuizOutcomeAlignmentService manages per-question outcome alignment.
// NOTE: the quiz grader does not currently consume this data — Wave A1 owns
// the grader. This service exists so instructor UIs and reporting can read
// and write alignments today; grader integration is a follow-up wave.
type QuizOutcomeAlignmentService struct {
	alignmentRepo repository.QuizQuestionOutcomeAlignmentRepository
	questionRepo  repository.QuizQuestionRepository
	outcomeRepo   repository.LearningOutcomeRepository
}

func NewQuizOutcomeAlignmentService(
	alignmentRepo repository.QuizQuestionOutcomeAlignmentRepository,
	questionRepo repository.QuizQuestionRepository,
	outcomeRepo repository.LearningOutcomeRepository,
) *QuizOutcomeAlignmentService {
	return &QuizOutcomeAlignmentService{
		alignmentRepo: alignmentRepo,
		questionRepo:  questionRepo,
		outcomeRepo:   outcomeRepo,
	}
}

// Align creates an alignment between a quiz question and a learning outcome.
// Returns the existing record if the pair is already aligned, so callers can
// rely on Align being idempotent and don't have to swallow unique-key errors.
func (s *QuizOutcomeAlignmentService) Align(ctx context.Context, quizQuestionID, outcomeID uint, masteryThreshold float64) (*models.QuizQuestionOutcomeAlignment, error) {
	if quizQuestionID == 0 {
		return nil, errors.New("quiz_question_id is required")
	}
	if outcomeID == 0 {
		return nil, errors.New("outcome_id is required")
	}
	if masteryThreshold < 0 || masteryThreshold > 1 {
		return nil, errors.New("mastery_threshold must be between 0 and 1")
	}

	if _, err := s.questionRepo.FindByID(ctx, quizQuestionID); err != nil {
		return nil, errors.New("quiz question not found")
	}
	if s.outcomeRepo != nil {
		if _, err := s.outcomeRepo.FindByID(ctx, outcomeID); err != nil {
			return nil, errors.New("learning outcome not found")
		}
	}

	if existing, err := s.alignmentRepo.FindByQuestionAndOutcome(ctx, quizQuestionID, outcomeID); err == nil && existing != nil {
		return existing, errors.New("alignment already exists")
	}

	alignment := &models.QuizQuestionOutcomeAlignment{
		QuizQuestionID:   quizQuestionID,
		OutcomeID:        outcomeID,
		MasteryThreshold: masteryThreshold,
	}
	if err := s.alignmentRepo.Create(ctx, alignment); err != nil {
		return nil, err
	}
	return alignment, nil
}

// Unalign removes an alignment for a (question, outcome) pair. Idempotent.
func (s *QuizOutcomeAlignmentService) Unalign(ctx context.Context, quizQuestionID, outcomeID uint) error {
	if quizQuestionID == 0 {
		return errors.New("quiz_question_id is required")
	}
	if outcomeID == 0 {
		return errors.New("outcome_id is required")
	}
	return s.alignmentRepo.DeleteByQuestionAndOutcome(ctx, quizQuestionID, outcomeID)
}

func (s *QuizOutcomeAlignmentService) ListByQuestion(ctx context.Context, quizQuestionID uint) ([]models.QuizQuestionOutcomeAlignment, error) {
	if quizQuestionID == 0 {
		return nil, errors.New("quiz_question_id is required")
	}
	return s.alignmentRepo.ListByQuestionID(ctx, quizQuestionID)
}

func (s *QuizOutcomeAlignmentService) ListByOutcome(ctx context.Context, outcomeID uint) ([]models.QuizQuestionOutcomeAlignment, error) {
	if outcomeID == 0 {
		return nil, errors.New("outcome_id is required")
	}
	return s.alignmentRepo.ListByOutcomeID(ctx, outcomeID)
}
