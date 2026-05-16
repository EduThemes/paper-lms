package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

func newAlignmentService(t *testing.T) (*service.QuizOutcomeAlignmentService, *mocks.MockQuizQuestionOutcomeAlignmentRepository, *mocks.MockQuizQuestionRepository, *mocks.MockLearningOutcomeRepository) {
	t.Helper()
	ar := new(mocks.MockQuizQuestionOutcomeAlignmentRepository)
	qr := new(mocks.MockQuizQuestionRepository)
	or := new(mocks.MockLearningOutcomeRepository)
	return service.NewQuizOutcomeAlignmentService(ar, qr, or), ar, qr, or
}

func TestQuizOutcomeAlignmentService_Align(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		svc, ar, qr, or := newAlignmentService(t)
		qr.On("FindByID", ctx, uint(1)).Return(&models.QuizQuestion{ID: 1}, nil)
		or.On("FindByID", ctx, uint(2), uint(0)).Return(&models.LearningOutcome{ID: 2}, nil)
		ar.On("FindByQuestionAndOutcome", ctx, uint(1), uint(2)).Return(nil, errors.New("not found"))
		ar.On("Create", ctx, mock.MatchedBy(func(a *models.QuizQuestionOutcomeAlignment) bool {
			return a.QuizQuestionID == 1 && a.OutcomeID == 2 && a.MasteryThreshold == 0.8
		})).Return(nil)
		a, err := svc.Align(ctx, 1, 2, 0.8)
		assert.NoError(t, err)
		assert.NotNil(t, a)
	})

	t.Run("unique constraint blocks duplicates", func(t *testing.T) {
		svc, ar, qr, or := newAlignmentService(t)
		qr.On("FindByID", ctx, uint(1)).Return(&models.QuizQuestion{ID: 1}, nil)
		or.On("FindByID", ctx, uint(2), uint(0)).Return(&models.LearningOutcome{ID: 2}, nil)
		existing := &models.QuizQuestionOutcomeAlignment{ID: 99, QuizQuestionID: 1, OutcomeID: 2}
		ar.On("FindByQuestionAndOutcome", ctx, uint(1), uint(2)).Return(existing, nil)
		_, err := svc.Align(ctx, 1, 2, 0.7)
		assert.EqualError(t, err, "alignment already exists")
	})

	t.Run("invalid threshold", func(t *testing.T) {
		svc, _, _, _ := newAlignmentService(t)
		_, err := svc.Align(ctx, 1, 2, 1.5)
		assert.EqualError(t, err, "mastery_threshold must be between 0 and 1")
		_, err = svc.Align(ctx, 1, 2, -0.1)
		assert.EqualError(t, err, "mastery_threshold must be between 0 and 1")
	})

	t.Run("missing ids", func(t *testing.T) {
		svc, _, _, _ := newAlignmentService(t)
		_, err := svc.Align(ctx, 0, 2, 0.5)
		assert.EqualError(t, err, "quiz_question_id is required")
		_, err = svc.Align(ctx, 1, 0, 0.5)
		assert.EqualError(t, err, "outcome_id is required")
	})

	t.Run("question not found", func(t *testing.T) {
		svc, _, qr, _ := newAlignmentService(t)
		qr.On("FindByID", ctx, uint(1)).Return(nil, errors.New("nope"))
		_, err := svc.Align(ctx, 1, 2, 0.5)
		assert.EqualError(t, err, "quiz question not found")
	})

	t.Run("outcome not found", func(t *testing.T) {
		svc, _, qr, or := newAlignmentService(t)
		qr.On("FindByID", ctx, uint(1)).Return(&models.QuizQuestion{ID: 1}, nil)
		or.On("FindByID", ctx, uint(2), uint(0)).Return(nil, errors.New("nope"))
		_, err := svc.Align(ctx, 1, 2, 0.5)
		assert.EqualError(t, err, "learning outcome not found")
	})
}

func TestQuizOutcomeAlignmentService_Unalign(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		svc, ar, _, _ := newAlignmentService(t)
		ar.On("DeleteByQuestionAndOutcome", ctx, uint(1), uint(2)).Return(nil)
		assert.NoError(t, svc.Unalign(ctx, 1, 2))
	})

	t.Run("missing ids", func(t *testing.T) {
		svc, _, _, _ := newAlignmentService(t)
		assert.EqualError(t, svc.Unalign(ctx, 0, 2), "quiz_question_id is required")
		assert.EqualError(t, svc.Unalign(ctx, 1, 0), "outcome_id is required")
	})
}

func TestQuizOutcomeAlignmentService_ListByQuestion(t *testing.T) {
	ctx := context.Background()
	svc, ar, _, _ := newAlignmentService(t)
	want := []models.QuizQuestionOutcomeAlignment{{ID: 1}}
	ar.On("ListByQuestionID", ctx, uint(1)).Return(want, nil)
	got, err := svc.ListByQuestion(ctx, 1)
	assert.NoError(t, err)
	assert.Equal(t, want, got)

	_, err = svc.ListByQuestion(ctx, 0)
	assert.EqualError(t, err, "quiz_question_id is required")
}

func TestQuizOutcomeAlignmentService_ListByOutcome(t *testing.T) {
	ctx := context.Background()
	svc, ar, _, _ := newAlignmentService(t)
	want := []models.QuizQuestionOutcomeAlignment{{ID: 1}}
	ar.On("ListByOutcomeID", ctx, uint(2)).Return(want, nil)
	got, err := svc.ListByOutcome(ctx, 2)
	assert.NoError(t, err)
	assert.Equal(t, want, got)

	_, err = svc.ListByOutcome(ctx, 0)
	assert.EqualError(t, err, "outcome_id is required")
}

// Service must tolerate a nil outcome repository (the integration in main.go
// may not always wire one — e.g., in standalone unit tests of the alignment
// surface area). Behavior: skip the outcome existence check.
func TestQuizOutcomeAlignmentService_Align_NilOutcomeRepo(t *testing.T) {
	ctx := context.Background()
	ar := new(mocks.MockQuizQuestionOutcomeAlignmentRepository)
	qr := new(mocks.MockQuizQuestionRepository)
	svc := service.NewQuizOutcomeAlignmentService(ar, qr, nil)

	qr.On("FindByID", ctx, uint(1)).Return(&models.QuizQuestion{ID: 1}, nil)
	ar.On("FindByQuestionAndOutcome", ctx, uint(1), uint(2)).Return(nil, errors.New("not found"))
	ar.On("Create", ctx, mock.AnythingOfType("*models.QuizQuestionOutcomeAlignment")).Return(nil)

	a, err := svc.Align(ctx, 1, 2, 0.7)
	assert.NoError(t, err)
	assert.NotNil(t, a)
}
