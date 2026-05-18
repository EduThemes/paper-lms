package service

// Quiz statistics methods — read-only fan-out used by the statistics
// handler (internal/api/v1/handlers/quiz_statistics.go) to assemble
// item-level reports and submission analytics.
//
// Wave 5 split (chore/wave5-split-quiz-blueprint): methods stay on
// *QuizService; only the source organization moved.

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// GetQuiz returns a quiz by ID.
func (s *QuizService) GetQuiz(ctx context.Context, quizID uint) (*models.Quiz, error) {
	return s.quizRepo.FindByID(ctx, quizID, 0)
}

// ListAllQuestions returns all active questions for a quiz (no pagination).
func (s *QuizService) ListAllQuestions(ctx context.Context, quizID uint) ([]models.QuizQuestion, error) {
	result, err := s.questionRepo.ListByQuizID(ctx, quizID, repository.PaginationParams{Page: 1, PerPage: 10000})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// ListAllCompletedSubmissions returns all completed/pending_review submissions for a quiz.
func (s *QuizService) ListAllCompletedSubmissions(ctx context.Context, quizID uint) ([]models.QuizSubmission, error) {
	return s.submissionRepo.ListCompletedByQuizID(ctx, quizID)
}

// ListAnswersBySubmissionIDs returns all answers for the given submission IDs.
func (s *QuizService) ListAnswersBySubmissionIDs(ctx context.Context, submissionIDs []uint) ([]models.QuizSubmissionAnswer, error) {
	return s.answerRepo.ListBySubmissionIDs(ctx, submissionIDs)
}
