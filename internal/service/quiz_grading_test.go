package service_test

// Tests for quiz_grading_service.go — the autoGrade dispatcher and the
// per-item-type graders. Includes the two CompleteSubmission paths that
// drive auto-grading (Success + EssayPendingReview) plus the AutoGrade_*
// integration tests that drive CompleteSubmission with each item type.
//
// The 9-item-type Wave A1 graders are exercised by the dedicated
// quiz_service_grading_new_types_test.go; this file holds the original
// MC / TF / short-answer / numerical / essay coverage that pre-dated
// Wave A1, plus the two top-level CompleteSubmission success paths.
//
// Split out of quiz_service_test.go in Wave 5
// (chore/wave5-split-quiz-blueprint).

import (
	"context"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCompleteSubmission_Success(t *testing.T) {
	questionRepo := new(mocks.MockQuizQuestionRepository)
	submissionRepo := new(mocks.MockQuizSubmissionRepository)
	answerRepo := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizService(quizRepo, questionRepo, submissionRepo, answerRepo)

	ctx := context.Background()

	startedAt := time.Now().Add(-10 * time.Minute)
	submission := &models.QuizSubmission{
		ID:            1,
		QuizID:        1,
		UserID:        10,
		Attempt:       1,
		StartedAt:     &startedAt,
		WorkflowState: "untaken",
	}

	points := 2.0
	question := &models.QuizQuestion{
		ID:             100,
		QuizID:         1,
		QuestionType:   "multiple_choice",
		QuestionText:   "What is 2+2?",
		PointsPossible: &points,
		Answers:        `[{"id":"a1","text":"4","weight":100},{"id":"a2","text":"3","weight":0}]`,
	}

	answers := []models.QuizSubmissionAnswer{
		{
			ID:               1,
			QuizSubmissionID: 1,
			QuestionID:       100,
			Answer:           "a1",
		},
	}

	submissionRepo.On("FindByID", ctx, uint(1)).Return(submission, nil)
	answerRepo.On("ListBySubmissionID", ctx, uint(1)).Return(answers, nil)
	questionRepo.On("FindByID", ctx, uint(100)).Return(question, nil)
	answerRepo.On("Update", ctx, mock.AnythingOfType("*models.QuizSubmissionAnswer")).Return(nil)
	submissionRepo.On("Update", ctx, submission).Return(nil)

	result, err := svc.CompleteSubmission(ctx, 1, 10)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "complete", result.WorkflowState)
	assert.NotNil(t, result.FinishedAt)
	assert.NotNil(t, result.Score)
	assert.Equal(t, 2.0, *result.Score)
	assert.True(t, result.TimeSpent > 0)
	submissionRepo.AssertExpectations(t)
	answerRepo.AssertExpectations(t)
	questionRepo.AssertExpectations(t)
}

func TestCompleteSubmission_EssayPendingReview(t *testing.T) {
	questionRepo := new(mocks.MockQuizQuestionRepository)
	submissionRepo := new(mocks.MockQuizSubmissionRepository)
	answerRepo := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizService(quizRepo, questionRepo, submissionRepo, answerRepo)

	ctx := context.Background()

	startedAt := time.Now().Add(-5 * time.Minute)
	submission := &models.QuizSubmission{
		ID:            2,
		QuizID:        1,
		UserID:        10,
		Attempt:       1,
		StartedAt:     &startedAt,
		WorkflowState: "untaken",
	}

	essayPoints := 5.0
	essayQuestion := &models.QuizQuestion{
		ID:             200,
		QuizID:         1,
		QuestionType:   "essay",
		QuestionText:   "Explain the theory of relativity.",
		PointsPossible: &essayPoints,
		Answers:        `[]`,
	}

	answers := []models.QuizSubmissionAnswer{
		{
			ID:               10,
			QuizSubmissionID: 2,
			QuestionID:       200,
			Answer:           "Einstein proposed that...",
		},
	}

	submissionRepo.On("FindByID", ctx, uint(2)).Return(submission, nil)
	answerRepo.On("ListBySubmissionID", ctx, uint(2)).Return(answers, nil)
	questionRepo.On("FindByID", ctx, uint(200)).Return(essayQuestion, nil)
	answerRepo.On("Update", ctx, mock.AnythingOfType("*models.QuizSubmissionAnswer")).Return(nil)
	submissionRepo.On("Update", ctx, submission).Return(nil)

	result, err := svc.CompleteSubmission(ctx, 2, 10)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "pending_review", result.WorkflowState)
	assert.NotNil(t, result.FinishedAt)
	assert.NotNil(t, result.Score)
	assert.Equal(t, 0.0, *result.Score)
	submissionRepo.AssertExpectations(t)
	answerRepo.AssertExpectations(t)
	questionRepo.AssertExpectations(t)
}

// ---------- Auto-Grading Tests ----------

func TestAutoGrade_MultipleChoiceCorrect(t *testing.T) {
	questionRepo := new(mocks.MockQuizQuestionRepository)
	submissionRepo := new(mocks.MockQuizSubmissionRepository)
	answerRepo := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizService(quizRepo, questionRepo, submissionRepo, answerRepo)

	ctx := context.Background()

	startedAt := time.Now().Add(-5 * time.Minute)
	points := 10.0
	submission := &models.QuizSubmission{
		ID:            1,
		QuizID:        1,
		UserID:        10,
		Attempt:       1,
		StartedAt:     &startedAt,
		WorkflowState: "untaken",
	}

	question := &models.QuizQuestion{
		ID:             100,
		QuizID:         1,
		QuestionType:   "multiple_choice",
		QuestionText:   "What is 2+2?",
		PointsPossible: &points,
		Answers:        `[{"id":"a1","text":"4","weight":100},{"id":"a2","text":"3","weight":0}]`,
	}

	answers := []models.QuizSubmissionAnswer{
		{
			ID:               1,
			QuizSubmissionID: 1,
			QuestionID:       100,
			Answer:           "a1",
		},
	}

	submissionRepo.On("FindByID", ctx, uint(1)).Return(submission, nil)
	answerRepo.On("ListBySubmissionID", ctx, uint(1)).Return(answers, nil)
	questionRepo.On("FindByID", ctx, uint(100)).Return(question, nil)
	answerRepo.On("Update", ctx, mock.AnythingOfType("*models.QuizSubmissionAnswer")).Return(nil)
	submissionRepo.On("Update", ctx, submission).Return(nil)

	result, err := svc.CompleteSubmission(ctx, 1, 10)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 10.0, *result.Score)
	assert.Equal(t, "complete", result.WorkflowState)
	submissionRepo.AssertExpectations(t)
	answerRepo.AssertExpectations(t)
	questionRepo.AssertExpectations(t)
}

func TestAutoGrade_MultipleChoiceIncorrect(t *testing.T) {
	questionRepo := new(mocks.MockQuizQuestionRepository)
	submissionRepo := new(mocks.MockQuizSubmissionRepository)
	answerRepo := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizService(quizRepo, questionRepo, submissionRepo, answerRepo)

	ctx := context.Background()

	startedAt := time.Now().Add(-5 * time.Minute)
	points := 10.0
	submission := &models.QuizSubmission{
		ID:            1,
		QuizID:        1,
		UserID:        10,
		Attempt:       1,
		StartedAt:     &startedAt,
		WorkflowState: "untaken",
	}

	question := &models.QuizQuestion{
		ID:             100,
		QuizID:         1,
		QuestionType:   "multiple_choice",
		QuestionText:   "What is 2+2?",
		PointsPossible: &points,
		Answers:        `[{"id":"a1","text":"4","weight":100},{"id":"a2","text":"3","weight":0}]`,
	}

	answers := []models.QuizSubmissionAnswer{
		{
			ID:               1,
			QuizSubmissionID: 1,
			QuestionID:       100,
			Answer:           "a2",
		},
	}

	submissionRepo.On("FindByID", ctx, uint(1)).Return(submission, nil)
	answerRepo.On("ListBySubmissionID", ctx, uint(1)).Return(answers, nil)
	questionRepo.On("FindByID", ctx, uint(100)).Return(question, nil)
	answerRepo.On("Update", ctx, mock.AnythingOfType("*models.QuizSubmissionAnswer")).Return(nil)
	submissionRepo.On("Update", ctx, submission).Return(nil)

	result, err := svc.CompleteSubmission(ctx, 1, 10)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0.0, *result.Score)
	assert.Equal(t, "complete", result.WorkflowState)
	submissionRepo.AssertExpectations(t)
	answerRepo.AssertExpectations(t)
	questionRepo.AssertExpectations(t)
}

func TestAutoGrade_TrueFalse(t *testing.T) {
	questionRepo := new(mocks.MockQuizQuestionRepository)
	submissionRepo := new(mocks.MockQuizSubmissionRepository)
	answerRepo := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizService(quizRepo, questionRepo, submissionRepo, answerRepo)

	ctx := context.Background()

	startedAt := time.Now().Add(-5 * time.Minute)
	points := 1.0
	submission := &models.QuizSubmission{
		ID:            1,
		QuizID:        1,
		UserID:        10,
		Attempt:       1,
		StartedAt:     &startedAt,
		WorkflowState: "untaken",
	}

	question := &models.QuizQuestion{
		ID:             101,
		QuizID:         1,
		QuestionType:   "true_false",
		QuestionText:   "The sky is blue.",
		PointsPossible: &points,
		Answers:        `[{"id":"t1","text":"True","weight":100},{"id":"t2","text":"False","weight":0}]`,
	}

	answers := []models.QuizSubmissionAnswer{
		{
			ID:               1,
			QuizSubmissionID: 1,
			QuestionID:       101,
			Answer:           "t1",
		},
	}

	submissionRepo.On("FindByID", ctx, uint(1)).Return(submission, nil)
	answerRepo.On("ListBySubmissionID", ctx, uint(1)).Return(answers, nil)
	questionRepo.On("FindByID", ctx, uint(101)).Return(question, nil)
	answerRepo.On("Update", ctx, mock.AnythingOfType("*models.QuizSubmissionAnswer")).Return(nil)
	submissionRepo.On("Update", ctx, submission).Return(nil)

	result, err := svc.CompleteSubmission(ctx, 1, 10)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1.0, *result.Score)
	assert.Equal(t, "complete", result.WorkflowState)
	submissionRepo.AssertExpectations(t)
	answerRepo.AssertExpectations(t)
	questionRepo.AssertExpectations(t)
}

func TestAutoGrade_ShortAnswerCaseInsensitive(t *testing.T) {
	questionRepo := new(mocks.MockQuizQuestionRepository)
	submissionRepo := new(mocks.MockQuizSubmissionRepository)
	answerRepo := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizService(quizRepo, questionRepo, submissionRepo, answerRepo)

	ctx := context.Background()

	startedAt := time.Now().Add(-5 * time.Minute)
	points := 2.0
	submission := &models.QuizSubmission{
		ID:            1,
		QuizID:        1,
		UserID:        10,
		Attempt:       1,
		StartedAt:     &startedAt,
		WorkflowState: "untaken",
	}

	question := &models.QuizQuestion{
		ID:             102,
		QuizID:         1,
		QuestionType:   "short_answer",
		QuestionText:   "Spell out the number 4.",
		PointsPossible: &points,
		Answers:        `[{"id":"s1","text":"four","weight":100}]`,
	}

	answers := []models.QuizSubmissionAnswer{
		{
			ID:               1,
			QuizSubmissionID: 1,
			QuestionID:       102,
			Answer:           "FOUR",
		},
	}

	submissionRepo.On("FindByID", ctx, uint(1)).Return(submission, nil)
	answerRepo.On("ListBySubmissionID", ctx, uint(1)).Return(answers, nil)
	questionRepo.On("FindByID", ctx, uint(102)).Return(question, nil)
	answerRepo.On("Update", ctx, mock.AnythingOfType("*models.QuizSubmissionAnswer")).Return(nil)
	submissionRepo.On("Update", ctx, submission).Return(nil)

	result, err := svc.CompleteSubmission(ctx, 1, 10)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 2.0, *result.Score)
	assert.Equal(t, "complete", result.WorkflowState)
	submissionRepo.AssertExpectations(t)
	answerRepo.AssertExpectations(t)
	questionRepo.AssertExpectations(t)
}

func TestAutoGrade_Numerical(t *testing.T) {
	questionRepo := new(mocks.MockQuizQuestionRepository)
	submissionRepo := new(mocks.MockQuizSubmissionRepository)
	answerRepo := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizService(quizRepo, questionRepo, submissionRepo, answerRepo)

	ctx := context.Background()

	startedAt := time.Now().Add(-5 * time.Minute)
	points := 3.0
	submission := &models.QuizSubmission{
		ID:            1,
		QuizID:        1,
		UserID:        10,
		Attempt:       1,
		StartedAt:     &startedAt,
		WorkflowState: "untaken",
	}

	question := &models.QuizQuestion{
		ID:             103,
		QuizID:         1,
		QuestionType:   "numerical_question",
		QuestionText:   "What is the answer to life?",
		PointsPossible: &points,
		Answers:        `[{"id":"n1","text":"42","weight":100}]`,
	}

	answers := []models.QuizSubmissionAnswer{
		{
			ID:               1,
			QuizSubmissionID: 1,
			QuestionID:       103,
			Answer:           "42",
		},
	}

	submissionRepo.On("FindByID", ctx, uint(1)).Return(submission, nil)
	answerRepo.On("ListBySubmissionID", ctx, uint(1)).Return(answers, nil)
	questionRepo.On("FindByID", ctx, uint(103)).Return(question, nil)
	answerRepo.On("Update", ctx, mock.AnythingOfType("*models.QuizSubmissionAnswer")).Return(nil)
	submissionRepo.On("Update", ctx, submission).Return(nil)

	result, err := svc.CompleteSubmission(ctx, 1, 10)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 3.0, *result.Score)
	assert.Equal(t, "complete", result.WorkflowState)
	submissionRepo.AssertExpectations(t)
	answerRepo.AssertExpectations(t)
	questionRepo.AssertExpectations(t)
}

func TestAutoGrade_EssayNotGradable(t *testing.T) {
	questionRepo := new(mocks.MockQuizQuestionRepository)
	submissionRepo := new(mocks.MockQuizSubmissionRepository)
	answerRepo := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizService(quizRepo, questionRepo, submissionRepo, answerRepo)

	ctx := context.Background()

	startedAt := time.Now().Add(-5 * time.Minute)
	mcPoints := 2.0
	essayPoints := 5.0
	submission := &models.QuizSubmission{
		ID:            1,
		QuizID:        1,
		UserID:        10,
		Attempt:       1,
		StartedAt:     &startedAt,
		WorkflowState: "untaken",
	}

	mcQuestion := &models.QuizQuestion{
		ID:             100,
		QuizID:         1,
		QuestionType:   "multiple_choice",
		QuestionText:   "What is 2+2?",
		PointsPossible: &mcPoints,
		Answers:        `[{"id":"a1","text":"4","weight":100},{"id":"a2","text":"3","weight":0}]`,
	}

	essayQuestion := &models.QuizQuestion{
		ID:             200,
		QuizID:         1,
		QuestionType:   "essay",
		QuestionText:   "Describe your favorite book.",
		PointsPossible: &essayPoints,
		Answers:        `[]`,
	}

	answers := []models.QuizSubmissionAnswer{
		{
			ID:               1,
			QuizSubmissionID: 1,
			QuestionID:       100,
			Answer:           "a1",
		},
		{
			ID:               2,
			QuizSubmissionID: 1,
			QuestionID:       200,
			Answer:           "My favorite book is...",
		},
	}

	submissionRepo.On("FindByID", ctx, uint(1)).Return(submission, nil)
	answerRepo.On("ListBySubmissionID", ctx, uint(1)).Return(answers, nil)
	questionRepo.On("FindByID", ctx, uint(100)).Return(mcQuestion, nil)
	questionRepo.On("FindByID", ctx, uint(200)).Return(essayQuestion, nil)
	answerRepo.On("Update", ctx, mock.AnythingOfType("*models.QuizSubmissionAnswer")).Return(nil)
	submissionRepo.On("Update", ctx, submission).Return(nil)

	result, err := svc.CompleteSubmission(ctx, 1, 10)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	// Only the multiple choice score counts (essay is 0 until manually graded)
	assert.Equal(t, 2.0, *result.Score)
	// Should be pending_review because essay needs manual grading
	assert.Equal(t, "pending_review", result.WorkflowState)
	assert.NotNil(t, result.FinishedAt)
	submissionRepo.AssertExpectations(t)
	answerRepo.AssertExpectations(t)
	questionRepo.AssertExpectations(t)
}
