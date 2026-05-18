package service_test

// Tests for quiz_authoring_service.go — Question (and Question Group) CRUD.
// Split out of quiz_service_test.go in Wave 5
// (chore/wave5-split-quiz-blueprint) alongside the source-file split.

import (
	"context"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
)

// ---------- CreateQuestion Tests ----------

func TestCreateQuestion_Success(t *testing.T) {
	questionRepo := new(mocks.MockQuizQuestionRepository)
	submissionRepo := new(mocks.MockQuizSubmissionRepository)
	answerRepo := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizService(quizRepo, questionRepo, submissionRepo, answerRepo)

	ctx := context.Background()
	points := 5.0
	question := &models.QuizQuestion{
		QuizID:         1,
		QuestionType:   "multiple_choice",
		QuestionText:   "What is 2+2?",
		PointsPossible: &points,
		Answers:        `[{"id":"a1","text":"4","weight":100},{"id":"a2","text":"3","weight":0}]`,
	}

	questionRepo.On("Create", ctx, question).Return(nil)

	err := svc.CreateQuestion(ctx, question)

	assert.NoError(t, err)
	assert.Equal(t, "active", question.WorkflowState)
	assert.Equal(t, 5.0, *question.PointsPossible)
	questionRepo.AssertExpectations(t)
}

func TestCreateQuestion_MissingText(t *testing.T) {
	questionRepo := new(mocks.MockQuizQuestionRepository)
	submissionRepo := new(mocks.MockQuizSubmissionRepository)
	answerRepo := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizService(quizRepo, questionRepo, submissionRepo, answerRepo)

	ctx := context.Background()
	question := &models.QuizQuestion{
		QuizID:       1,
		QuestionType: "multiple_choice",
		QuestionText: "",
	}

	err := svc.CreateQuestion(ctx, question)

	assert.EqualError(t, err, "question_text is required")
	questionRepo.AssertExpectations(t)
}

func TestCreateQuestion_InvalidType(t *testing.T) {
	questionRepo := new(mocks.MockQuizQuestionRepository)
	submissionRepo := new(mocks.MockQuizSubmissionRepository)
	answerRepo := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizService(quizRepo, questionRepo, submissionRepo, answerRepo)

	ctx := context.Background()
	question := &models.QuizQuestion{
		QuizID:       1,
		QuestionType: "unknown",
		QuestionText: "What is 2+2?",
	}

	err := svc.CreateQuestion(ctx, question)

	assert.EqualError(t, err, "invalid question_type")
	questionRepo.AssertExpectations(t)
}

func TestCreateQuestion_DefaultPoints(t *testing.T) {
	questionRepo := new(mocks.MockQuizQuestionRepository)
	submissionRepo := new(mocks.MockQuizSubmissionRepository)
	answerRepo := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizService(quizRepo, questionRepo, submissionRepo, answerRepo)

	ctx := context.Background()
	question := &models.QuizQuestion{
		QuizID:         1,
		QuestionType:   "true_false",
		QuestionText:   "The sky is blue.",
		PointsPossible: nil,
	}

	questionRepo.On("Create", ctx, question).Return(nil)

	err := svc.CreateQuestion(ctx, question)

	assert.NoError(t, err)
	assert.NotNil(t, question.PointsPossible)
	assert.Equal(t, 1.0, *question.PointsPossible)
	assert.Equal(t, "active", question.WorkflowState)
	questionRepo.AssertExpectations(t)
}
