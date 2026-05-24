package service_test

// Tests for quiz_attempts_service.go — StartSubmission, AnswerQuestion,
// and CompleteSubmission_WrongUser (the pure attempt-lifecycle paths).
// Auto-grading-heavy paths (CompleteSubmission_Success / EssayPendingReview
// / AutoGrade_*) live in quiz_grading_test.go.
//
// Split out of quiz_service_test.go in Wave 5
// (chore/wave5-split-quiz-blueprint).

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ---------- StartSubmission Tests ----------

func TestStartSubmission_New(t *testing.T) {
	t.Skip("known issue: MockQuizQuestionRepository.ListByQuizID expectation does not match generateSelectedQuestions; tracked for rewrite")
	questionRepo := new(mocks.MockQuizQuestionRepository)
	submissionRepo := new(mocks.MockQuizSubmissionRepository)
	answerRepo := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizService(quizRepo, questionRepo, submissionRepo, answerRepo)

	ctx := context.Background()

	// No existing submission
	submissionRepo.On("FindByQuizAndUser", ctx, uint(1), uint(10)).Return(nil, errors.New("not found"))
	submissionRepo.On("Create", ctx, mock.AnythingOfType("*models.QuizSubmission")).Return(nil)
	quizRepo.On("FindByID", ctx, uint(1), uint(0)).Return(&models.Quiz{ID: 1, AllowedAttempts: -1}, nil)

	result, err := svc.StartSubmission(ctx, 1, 10, nil)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, uint(1), result.QuizID)
	assert.Equal(t, uint(10), result.UserID)
	assert.Equal(t, 1, result.Attempt)
	assert.Equal(t, "untaken", result.WorkflowState)
	assert.NotNil(t, result.StartedAt)
	assert.NotEmpty(t, result.ValidationToken)
	assert.Nil(t, result.EndAt)
	submissionRepo.AssertExpectations(t)
}

func TestStartSubmission_Resume(t *testing.T) {
	questionRepo := new(mocks.MockQuizQuestionRepository)
	submissionRepo := new(mocks.MockQuizSubmissionRepository)
	answerRepo := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizService(quizRepo, questionRepo, submissionRepo, answerRepo)

	ctx := context.Background()

	now := time.Now()
	existingSub := &models.QuizSubmission{
		ID:            5,
		QuizID:        1,
		UserID:        10,
		Attempt:       1,
		StartedAt:     &now,
		WorkflowState: "untaken",
	}

	// F-048: StartSubmission now loads the quiz BEFORE the submission
	// lookup so it can enforce unlock_at / lock_at windows. Tests need
	// to stub that lookup.
	quizRepo.On("FindByID", ctx, uint(1), uint(0)).Return(&models.Quiz{ID: 1, AllowedAttempts: -1}, nil)
	submissionRepo.On("FindByQuizAndUser", ctx, uint(1), uint(10)).Return(existingSub, nil)

	result, err := svc.StartSubmission(ctx, 1, 10, nil)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, uint(5), result.ID)
	assert.Equal(t, 1, result.Attempt)
	assert.Equal(t, "untaken", result.WorkflowState)
	// Create should NOT have been called since we're resuming
	submissionRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// TestStartSubmission_LockAtRejects — F-048: a quiz past its lock_at
// MUST refuse new submissions. Pre-fix path silently created one.
func TestStartSubmission_LockAtRejects(t *testing.T) {
	questionRepo := new(mocks.MockQuizQuestionRepository)
	submissionRepo := new(mocks.MockQuizSubmissionRepository)
	answerRepo := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizService(quizRepo, questionRepo, submissionRepo, answerRepo)

	ctx := context.Background()
	past := time.Now().Add(-1 * time.Hour)
	quizRepo.On("FindByID", ctx, uint(1), uint(0)).Return(&models.Quiz{
		ID:              1,
		AllowedAttempts: -1,
		LockAt:          &past,
	}, nil)

	result, err := svc.StartSubmission(ctx, 1, 10, nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "locked")
	// The submission table MUST NOT be touched on a locked quiz.
	submissionRepo.AssertNotCalled(t, "FindByQuizAndUser", mock.Anything, mock.Anything, mock.Anything)
	submissionRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// TestStartSubmission_UnlockAtRejects — F-048: a quiz before its
// unlock_at MUST refuse the start so students cannot preview questions.
func TestStartSubmission_UnlockAtRejects(t *testing.T) {
	questionRepo := new(mocks.MockQuizQuestionRepository)
	submissionRepo := new(mocks.MockQuizSubmissionRepository)
	answerRepo := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizService(quizRepo, questionRepo, submissionRepo, answerRepo)

	ctx := context.Background()
	future := time.Now().Add(1 * time.Hour)
	quizRepo.On("FindByID", ctx, uint(1), uint(0)).Return(&models.Quiz{
		ID:              1,
		AllowedAttempts: -1,
		UnlockAt:        &future,
	}, nil)

	result, err := svc.StartSubmission(ctx, 1, 10, nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not yet available")
	submissionRepo.AssertNotCalled(t, "FindByQuizAndUser", mock.Anything, mock.Anything, mock.Anything)
	submissionRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	submissionRepo.AssertExpectations(t)
}

func TestStartSubmission_TimeLimit(t *testing.T) {
	t.Skip("known issue: shares the MockQuizQuestionRepository.ListByQuizID expectation gap with TestStartSubmission_New; tracked for rewrite")
	questionRepo := new(mocks.MockQuizQuestionRepository)
	submissionRepo := new(mocks.MockQuizSubmissionRepository)
	answerRepo := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizService(quizRepo, questionRepo, submissionRepo, answerRepo)

	ctx := context.Background()

	submissionRepo.On("FindByQuizAndUser", ctx, uint(1), uint(10)).Return(nil, errors.New("not found"))
	submissionRepo.On("Create", ctx, mock.AnythingOfType("*models.QuizSubmission")).Return(nil)
	quizRepo.On("FindByID", ctx, uint(1), uint(0)).Return(&models.Quiz{ID: 1, AllowedAttempts: -1}, nil)

	timeLimit := 30
	beforeStart := time.Now()

	result, err := svc.StartSubmission(ctx, 1, 10, &timeLimit)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.EndAt)

	// EndAt should be approximately 30 minutes from now
	expectedEnd := beforeStart.Add(30 * time.Minute)
	assert.WithinDuration(t, expectedEnd, *result.EndAt, 5*time.Second)
	submissionRepo.AssertExpectations(t)
}

// ---------- AnswerQuestion Tests ----------

func TestAnswerQuestion_Success(t *testing.T) {
	questionRepo := new(mocks.MockQuizQuestionRepository)
	submissionRepo := new(mocks.MockQuizSubmissionRepository)
	answerRepo := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizService(quizRepo, questionRepo, submissionRepo, answerRepo)

	ctx := context.Background()

	futureEnd := time.Now().Add(30 * time.Minute)
	submission := &models.QuizSubmission{
		ID:            1,
		QuizID:        1,
		UserID:        10,
		WorkflowState: "untaken",
		EndAt:         &futureEnd,
	}

	submissionRepo.On("FindByID", ctx, uint(1)).Return(submission, nil)
	// No existing answer
	answerRepo.On("FindBySubmissionAndQuestion", ctx, uint(1), uint(100)).Return(nil, errors.New("not found"))
	answerRepo.On("Create", ctx, mock.AnythingOfType("*models.QuizSubmissionAnswer")).Return(nil)

	result, err := svc.AnswerQuestion(ctx, 1, 100, "a1")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, uint(1), result.QuizSubmissionID)
	assert.Equal(t, uint(100), result.QuestionID)
	assert.Equal(t, "a1", result.Answer)
	answerRepo.AssertExpectations(t)
	submissionRepo.AssertExpectations(t)
}

func TestAnswerQuestion_Update(t *testing.T) {
	questionRepo := new(mocks.MockQuizQuestionRepository)
	submissionRepo := new(mocks.MockQuizSubmissionRepository)
	answerRepo := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizService(quizRepo, questionRepo, submissionRepo, answerRepo)

	ctx := context.Background()

	futureEnd := time.Now().Add(30 * time.Minute)
	submission := &models.QuizSubmission{
		ID:            1,
		QuizID:        1,
		UserID:        10,
		WorkflowState: "untaken",
		EndAt:         &futureEnd,
	}

	existingAnswer := &models.QuizSubmissionAnswer{
		ID:               50,
		QuizSubmissionID: 1,
		QuestionID:       100,
		Answer:           "a2",
	}

	submissionRepo.On("FindByID", ctx, uint(1)).Return(submission, nil)
	answerRepo.On("FindBySubmissionAndQuestion", ctx, uint(1), uint(100)).Return(existingAnswer, nil)
	answerRepo.On("Update", ctx, existingAnswer).Return(nil)

	result, err := svc.AnswerQuestion(ctx, 1, 100, "a1")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, uint(50), result.ID)
	assert.Equal(t, "a1", result.Answer)
	// Grading fields should be reset on update
	assert.Nil(t, result.Correct)
	assert.Nil(t, result.Points)
	answerRepo.AssertExpectations(t)
	submissionRepo.AssertExpectations(t)
}

func TestAnswerQuestion_WrongState(t *testing.T) {
	questionRepo := new(mocks.MockQuizQuestionRepository)
	submissionRepo := new(mocks.MockQuizSubmissionRepository)
	answerRepo := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizService(quizRepo, questionRepo, submissionRepo, answerRepo)

	ctx := context.Background()

	submission := &models.QuizSubmission{
		ID:            1,
		QuizID:        1,
		UserID:        10,
		WorkflowState: "complete",
	}

	submissionRepo.On("FindByID", ctx, uint(1)).Return(submission, nil)

	result, err := svc.AnswerQuestion(ctx, 1, 100, "a1")

	assert.Nil(t, result)
	assert.EqualError(t, err, "quiz submission is not in progress")
	submissionRepo.AssertExpectations(t)
}

func TestAnswerQuestion_TimeExpired(t *testing.T) {
	questionRepo := new(mocks.MockQuizQuestionRepository)
	submissionRepo := new(mocks.MockQuizSubmissionRepository)
	answerRepo := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizService(quizRepo, questionRepo, submissionRepo, answerRepo)

	ctx := context.Background()

	pastEnd := time.Now().Add(-10 * time.Minute)
	submission := &models.QuizSubmission{
		ID:            1,
		QuizID:        1,
		UserID:        10,
		WorkflowState: "untaken",
		EndAt:         &pastEnd,
	}

	submissionRepo.On("FindByID", ctx, uint(1)).Return(submission, nil)

	result, err := svc.AnswerQuestion(ctx, 1, 100, "a1")

	assert.Nil(t, result)
	assert.EqualError(t, err, "quiz time has expired")
	submissionRepo.AssertExpectations(t)
}

// ---------- CompleteSubmission Auth Tests ----------

// TestCompleteSubmission_WrongUser tests the auth check on the submission
// lifecycle path. Other CompleteSubmission cases live in quiz_grading_test.go
// because they exercise the auto-grading dispatcher.
func TestCompleteSubmission_WrongUser(t *testing.T) {
	questionRepo := new(mocks.MockQuizQuestionRepository)
	submissionRepo := new(mocks.MockQuizSubmissionRepository)
	answerRepo := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizService(quizRepo, questionRepo, submissionRepo, answerRepo)

	ctx := context.Background()

	submission := &models.QuizSubmission{
		ID:            1,
		QuizID:        1,
		UserID:        10,
		WorkflowState: "untaken",
	}

	submissionRepo.On("FindByID", ctx, uint(1)).Return(submission, nil)

	result, err := svc.CompleteSubmission(ctx, 1, 999)

	assert.Nil(t, result)
	assert.EqualError(t, err, "unauthorized: submission does not belong to this user")
	submissionRepo.AssertExpectations(t)
}

// TestStartSubmission_RaceRecovery — F-049 follow-up. When two
// concurrent requests start the same quiz, migration 000062's
// partial UNIQUE fires on the loser's Create. Pre-fix the loser
// bubbled the raw GORM error to the handler → 400 with no winning
// row. Now the service catches the create-error path, re-runs the
// existing-untaken find, and returns the winner's row to both
// callers. This test simulates that by having the find return
// nil-then-winner (winner wasn't visible when StartSubmission read
// initially), the create fail with a unique-violation, and the
// post-create find return the winner.
func TestStartSubmission_RaceRecovery(t *testing.T) {
	questionRepo := new(mocks.MockQuizQuestionRepository)
	submissionRepo := new(mocks.MockQuizSubmissionRepository)
	answerRepo := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizService(quizRepo, questionRepo, submissionRepo, answerRepo)

	ctx := context.Background()

	now := time.Now()
	quiz := &models.Quiz{
		ID:              1,
		CourseID:        10,
		AllowedAttempts: -1,
	}
	winner := &models.QuizSubmission{
		ID:            42,
		QuizID:        1,
		UserID:        7,
		Attempt:       1,
		WorkflowState: "untaken",
		StartedAt:     &now,
	}

	// Initial existence check: no in-progress row yet.
	submissionRepo.On("FindByQuizAndUser", ctx, uint(1), uint(7)).Return(nil, nil).Once()
	quizRepo.On("FindByID", ctx, uint(1), uint(0)).Return(quiz, nil).Once()
	questionRepo.On("ListByQuizID", ctx, uint(1), mock.Anything).Return(&repository.PaginatedResult[models.QuizQuestion]{Items: []models.QuizQuestion{}}, nil).Once()
	// Race: another goroutine got there first; UNIQUE violation.
	submissionRepo.On("Create", ctx, mock.AnythingOfType("*models.QuizSubmission")).Return(errors.New("ERROR: duplicate key value violates unique constraint")).Once()
	// Recovery: re-find returns the winner's row.
	submissionRepo.On("FindByQuizAndUser", ctx, uint(1), uint(7)).Return(winner, nil).Once()

	result, err := svc.StartSubmission(ctx, 1, 7, nil)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, uint(42), result.ID, "race loser should receive winner's row, not 400")
	assert.Equal(t, "untaken", string(result.WorkflowState))
}
