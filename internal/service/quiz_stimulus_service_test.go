package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

func newStimulusService(t *testing.T) (*service.QuizStimulusService, *mocks.MockQuizStimulusRepository, *mocks.MockQuizQuestionRepository) {
	t.Helper()
	sr := new(mocks.MockQuizStimulusRepository)
	qr := new(mocks.MockQuizQuestionRepository)
	return service.NewQuizStimulusService(sr, qr), sr, qr
}

func TestQuizStimulusService_CreateStimulus(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path defaults empty content to {}", func(t *testing.T) {
		svc, sr, _ := newStimulusService(t)
		sr.On("Create", ctx, mock.MatchedBy(func(s *models.QuizStimulus) bool {
			return s.CourseID == 1 && s.Title == "Passage" && s.Content == "{}"
		})).Return(nil)
		err := svc.CreateStimulus(ctx, &models.QuizStimulus{CourseID: 1, Title: "Passage"})
		assert.NoError(t, err)
	})

	t.Run("missing title", func(t *testing.T) {
		svc, _, _ := newStimulusService(t)
		err := svc.CreateStimulus(ctx, &models.QuizStimulus{CourseID: 1})
		assert.EqualError(t, err, "title is required")
	})

	t.Run("missing course id", func(t *testing.T) {
		svc, _, _ := newStimulusService(t)
		err := svc.CreateStimulus(ctx, &models.QuizStimulus{Title: "x"})
		assert.EqualError(t, err, "course_id is required")
	})

	t.Run("nil", func(t *testing.T) {
		svc, _, _ := newStimulusService(t)
		err := svc.CreateStimulus(ctx, nil)
		assert.EqualError(t, err, "stimulus is required")
	})
}

func TestQuizStimulusService_GetStimulus(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		svc, sr, _ := newStimulusService(t)
		sr.On("FindByID", ctx, uint(1)).Return(&models.QuizStimulus{ID: 1, CourseID: 5}, nil)
		got, err := svc.GetStimulus(ctx, 5, 1)
		assert.NoError(t, err)
		assert.Equal(t, uint(1), got.ID)
	})

	t.Run("cross-course rejection", func(t *testing.T) {
		svc, sr, _ := newStimulusService(t)
		sr.On("FindByID", ctx, uint(1)).Return(&models.QuizStimulus{ID: 1, CourseID: 5}, nil)
		_, err := svc.GetStimulus(ctx, 6, 1)
		assert.EqualError(t, err, "stimulus does not belong to course")
	})

	t.Run("not found", func(t *testing.T) {
		svc, sr, _ := newStimulusService(t)
		sr.On("FindByID", ctx, uint(1)).Return(nil, errors.New("nope"))
		_, err := svc.GetStimulus(ctx, 5, 1)
		assert.EqualError(t, err, "stimulus not found")
	})
}

func TestQuizStimulusService_ListStimuli(t *testing.T) {
	ctx := context.Background()
	svc, sr, _ := newStimulusService(t)
	params := repository.PaginationParams{Page: 1, PerPage: 10}
	want := &repository.PaginatedResult[models.QuizStimulus]{Items: []models.QuizStimulus{{ID: 1}}}
	sr.On("ListByCourseID", ctx, uint(2), params).Return(want, nil)
	got, err := svc.ListStimuli(ctx, 2, params)
	assert.NoError(t, err)
	assert.Equal(t, want, got)

	_, err = svc.ListStimuli(ctx, 0, params)
	assert.EqualError(t, err, "course_id is required")
}

func TestQuizStimulusService_UpdateStimulus(t *testing.T) {
	ctx := context.Background()

	t.Run("course id is immutable", func(t *testing.T) {
		svc, sr, _ := newStimulusService(t)
		sr.On("FindByID", ctx, uint(1)).Return(&models.QuizStimulus{ID: 1, CourseID: 5}, nil)
		sr.On("Update", ctx, mock.MatchedBy(func(s *models.QuizStimulus) bool {
			return s.CourseID == 5 && s.Title == "T"
		})).Return(nil)
		err := svc.UpdateStimulus(ctx, 5, &models.QuizStimulus{ID: 1, CourseID: 999, Title: "T"})
		assert.NoError(t, err)
	})

	t.Run("cross-course rejection", func(t *testing.T) {
		svc, sr, _ := newStimulusService(t)
		sr.On("FindByID", ctx, uint(1)).Return(&models.QuizStimulus{ID: 1, CourseID: 5}, nil)
		err := svc.UpdateStimulus(ctx, 6, &models.QuizStimulus{ID: 1})
		assert.EqualError(t, err, "stimulus does not belong to course")
	})

	t.Run("missing id", func(t *testing.T) {
		svc, _, _ := newStimulusService(t)
		err := svc.UpdateStimulus(ctx, 1, &models.QuizStimulus{})
		assert.EqualError(t, err, "stimulus id is required")
	})

	t.Run("not found", func(t *testing.T) {
		svc, sr, _ := newStimulusService(t)
		sr.On("FindByID", ctx, uint(1)).Return(nil, errors.New("nope"))
		err := svc.UpdateStimulus(ctx, 5, &models.QuizStimulus{ID: 1})
		assert.EqualError(t, err, "stimulus not found")
	})
}

func TestQuizStimulusService_DeleteStimulus(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		svc, sr, _ := newStimulusService(t)
		sr.On("FindByID", ctx, uint(1)).Return(&models.QuizStimulus{ID: 1, CourseID: 5}, nil)
		sr.On("Delete", ctx, uint(1)).Return(nil)
		assert.NoError(t, svc.DeleteStimulus(ctx, 5, 1))
	})

	t.Run("cross-course rejection", func(t *testing.T) {
		svc, sr, _ := newStimulusService(t)
		sr.On("FindByID", ctx, uint(1)).Return(&models.QuizStimulus{ID: 1, CourseID: 5}, nil)
		err := svc.DeleteStimulus(ctx, 6, 1)
		assert.EqualError(t, err, "stimulus does not belong to course")
	})

	t.Run("not found", func(t *testing.T) {
		svc, sr, _ := newStimulusService(t)
		sr.On("FindByID", ctx, uint(1)).Return(nil, errors.New("nope"))
		assert.EqualError(t, svc.DeleteStimulus(ctx, 5, 1), "stimulus not found")
	})
}

func TestQuizStimulusService_LinkQuestionToStimulus(t *testing.T) {
	ctx := context.Background()

	t.Run("sets the stimulus FK", func(t *testing.T) {
		svc, sr, qr := newStimulusService(t)
		qr.On("FindByID", ctx, uint(7)).Return(&models.QuizQuestion{ID: 7}, nil)
		sr.On("FindByID", ctx, uint(3)).Return(&models.QuizStimulus{ID: 3}, nil)
		sr.On("SetQuestionStimulus", ctx, uint(7), mock.MatchedBy(func(sid *uint) bool {
			return sid != nil && *sid == 3
		})).Return(nil)
		assert.NoError(t, svc.LinkQuestionToStimulus(ctx, 7, 3))
	})

	t.Run("missing question id", func(t *testing.T) {
		svc, _, _ := newStimulusService(t)
		assert.EqualError(t, svc.LinkQuestionToStimulus(ctx, 0, 3), "question_id is required")
	})

	t.Run("missing stimulus id", func(t *testing.T) {
		svc, _, _ := newStimulusService(t)
		assert.EqualError(t, svc.LinkQuestionToStimulus(ctx, 7, 0), "stimulus_id is required")
	})

	t.Run("question not found", func(t *testing.T) {
		svc, _, qr := newStimulusService(t)
		qr.On("FindByID", ctx, uint(7)).Return(nil, errors.New("nope"))
		assert.EqualError(t, svc.LinkQuestionToStimulus(ctx, 7, 3), "quiz question not found")
	})

	t.Run("stimulus not found", func(t *testing.T) {
		svc, sr, qr := newStimulusService(t)
		qr.On("FindByID", ctx, uint(7)).Return(&models.QuizQuestion{ID: 7}, nil)
		sr.On("FindByID", ctx, uint(3)).Return(nil, errors.New("nope"))
		assert.EqualError(t, svc.LinkQuestionToStimulus(ctx, 7, 3), "stimulus not found")
	})
}

func TestQuizStimulusService_UnlinkQuestionFromStimulus(t *testing.T) {
	ctx := context.Background()

	t.Run("clears the stimulus FK", func(t *testing.T) {
		svc, sr, qr := newStimulusService(t)
		qr.On("FindByID", ctx, uint(7)).Return(&models.QuizQuestion{ID: 7}, nil)
		sr.On("SetQuestionStimulus", ctx, uint(7), (*uint)(nil)).Return(nil)
		assert.NoError(t, svc.UnlinkQuestionFromStimulus(ctx, 7))
	})

	t.Run("missing id", func(t *testing.T) {
		svc, _, _ := newStimulusService(t)
		assert.EqualError(t, svc.UnlinkQuestionFromStimulus(ctx, 0), "question_id is required")
	})

	t.Run("question not found", func(t *testing.T) {
		svc, _, qr := newStimulusService(t)
		qr.On("FindByID", ctx, uint(7)).Return(nil, errors.New("nope"))
		assert.EqualError(t, svc.UnlinkQuestionFromStimulus(ctx, 7), "quiz question not found")
	})
}

func TestQuizStimulusService_ListQuestionsForStimulus(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		svc, sr, _ := newStimulusService(t)
		want := []models.QuizQuestion{{ID: 1}, {ID: 2}}
		sr.On("FindByID", ctx, uint(3)).Return(&models.QuizStimulus{ID: 3}, nil)
		sr.On("ListQuestionsForStimulus", ctx, uint(3)).Return(want, nil)
		got, err := svc.ListQuestionsForStimulus(ctx, 3)
		assert.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("missing id", func(t *testing.T) {
		svc, _, _ := newStimulusService(t)
		_, err := svc.ListQuestionsForStimulus(ctx, 0)
		assert.EqualError(t, err, "stimulus_id is required")
	})

	t.Run("not found", func(t *testing.T) {
		svc, sr, _ := newStimulusService(t)
		sr.On("FindByID", ctx, uint(3)).Return(nil, errors.New("nope"))
		_, err := svc.ListQuestionsForStimulus(ctx, 3)
		assert.EqualError(t, err, "stimulus not found")
	})
}
