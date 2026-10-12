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
	quizRepo      repository.QuizRepository
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
		if _, err := s.outcomeRepo.FindByID(ctx, outcomeID, 0); err != nil {
			return nil, errors.New("learning outcome not found")
		}
	}
	return s.createAlignment(ctx, quizQuestionID, outcomeID, masteryThreshold)
}

// createAlignment writes the (question, outcome) row once both ids are
// validated by the caller.
func (s *QuizOutcomeAlignmentService) createAlignment(ctx context.Context, quizQuestionID, outcomeID uint, masteryThreshold float64) (*models.QuizQuestionOutcomeAlignment, error) {
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

// ErrAlignmentScope is returned by the course-scoped methods when the quiz,
// question or outcome is not in the course (or not in the caller's tenant).
// Handlers map it to a 404.
var ErrAlignmentScope = errors.New("not found")

// SetQuizRepo wires the quiz repository the course-scoped methods use to tie
// a question to its quiz's course.
func (s *QuizOutcomeAlignmentService) SetQuizRepo(r repository.QuizRepository) *QuizOutcomeAlignmentService {
	s.quizRepo = r
	return s
}

// RequireQuestionInCourse checks that quizID is a quiz of courseID in the
// caller's tenant and that questionID belongs to that quiz.
func (s *QuizOutcomeAlignmentService) RequireQuestionInCourse(ctx context.Context, courseID, quizID, questionID, accountID uint) error {
	if s.quizRepo == nil || courseID == 0 || quizID == 0 || questionID == 0 || accountID == 0 {
		return ErrAlignmentScope
	}
	quiz, err := s.quizRepo.FindByID(ctx, quizID, accountID)
	if err != nil || quiz == nil || quiz.CourseID != courseID {
		return ErrAlignmentScope
	}
	q, err := s.questionRepo.FindByID(ctx, questionID)
	if err != nil || q == nil || q.QuizID != quizID {
		return ErrAlignmentScope
	}
	return nil
}

// requireOutcomeUsableInCourse: the outcome must be the course's own, or an
// account-level outcome of the caller's tenant (the repository's account
// scope covers both branches).
func (s *QuizOutcomeAlignmentService) requireOutcomeUsableInCourse(ctx context.Context, courseID, outcomeID, accountID uint) error {
	if s.outcomeRepo == nil || outcomeID == 0 || accountID == 0 {
		return ErrAlignmentScope
	}
	o, err := s.outcomeRepo.FindByID(ctx, outcomeID, accountID)
	if err != nil || o == nil {
		return ErrAlignmentScope
	}
	if o.ContextType == "Course" && o.ContextID != courseID {
		return ErrAlignmentScope
	}
	return nil
}

// AlignInCourse aligns a question of courseID with an outcome usable in that
// course. Both ids are checked against the course and the caller's tenant
// before anything is written.
func (s *QuizOutcomeAlignmentService) AlignInCourse(ctx context.Context, courseID, quizID, questionID, outcomeID, accountID uint, masteryThreshold float64) (*models.QuizQuestionOutcomeAlignment, error) {
	if err := s.RequireQuestionInCourse(ctx, courseID, quizID, questionID, accountID); err != nil {
		return nil, err
	}
	if outcomeID == 0 {
		return nil, errors.New("outcome_id is required")
	}
	if masteryThreshold < 0 || masteryThreshold > 1 {
		return nil, errors.New("mastery_threshold must be between 0 and 1")
	}
	if err := s.requireOutcomeUsableInCourse(ctx, courseID, outcomeID, accountID); err != nil {
		return nil, err
	}
	return s.createAlignment(ctx, questionID, outcomeID, masteryThreshold)
}

// ListByOutcomeInCourse lists the outcome's alignments to questions of
// quizzes in courseID only.
func (s *QuizOutcomeAlignmentService) ListByOutcomeInCourse(ctx context.Context, courseID, outcomeID, accountID uint) ([]models.QuizQuestionOutcomeAlignment, error) {
	if err := s.requireOutcomeUsableInCourse(ctx, courseID, outcomeID, accountID); err != nil {
		return nil, err
	}
	if s.quizRepo == nil {
		return nil, ErrAlignmentScope
	}
	all, err := s.alignmentRepo.ListByOutcomeID(ctx, outcomeID)
	if err != nil {
		return nil, err
	}
	quizInCourse := map[uint]bool{}
	out := make([]models.QuizQuestionOutcomeAlignment, 0, len(all))
	for _, a := range all {
		q, err := s.questionRepo.FindByID(ctx, a.QuizQuestionID)
		if err != nil || q == nil {
			continue
		}
		ok, seen := quizInCourse[q.QuizID]
		if !seen {
			quiz, err := s.quizRepo.FindByID(ctx, q.QuizID, accountID)
			ok = err == nil && quiz != nil && quiz.CourseID == courseID
			quizInCourse[q.QuizID] = ok
		}
		if ok {
			out = append(out, a)
		}
	}
	return out, nil
}
