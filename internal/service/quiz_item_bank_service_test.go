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

func newBankService(t *testing.T) (*service.QuizItemBankService, *mocks.MockQuizItemBankRepository, *mocks.MockQuizItemBankItemRepository, *mocks.MockQuizQuestionRepository) {
	t.Helper()
	br := new(mocks.MockQuizItemBankRepository)
	ir := new(mocks.MockQuizItemBankItemRepository)
	qr := new(mocks.MockQuizQuestionRepository)
	return service.NewQuizItemBankService(br, ir, qr), br, ir, qr
}

// ---------- Bank CRUD ----------

func TestQuizItemBankService_CreateBank(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		svc, br, _, _ := newBankService(t)
		bank := &models.QuizItemBank{CourseID: 1, Title: "Algebra", CreatedByUserID: 7}
		br.On("Create", ctx, bank).Return(nil)
		err := svc.CreateBank(ctx, bank)
		assert.NoError(t, err)
		br.AssertExpectations(t)
	})

	t.Run("missing title", func(t *testing.T) {
		svc, _, _, _ := newBankService(t)
		err := svc.CreateBank(ctx, &models.QuizItemBank{CourseID: 1, CreatedByUserID: 7})
		assert.EqualError(t, err, "title is required")
	})

	t.Run("missing course id", func(t *testing.T) {
		svc, _, _, _ := newBankService(t)
		err := svc.CreateBank(ctx, &models.QuizItemBank{Title: "x", CreatedByUserID: 7})
		assert.EqualError(t, err, "course_id is required")
	})

	t.Run("missing creator", func(t *testing.T) {
		svc, _, _, _ := newBankService(t)
		err := svc.CreateBank(ctx, &models.QuizItemBank{Title: "x", CourseID: 1})
		assert.EqualError(t, err, "created_by_user_id is required")
	})

	t.Run("nil bank", func(t *testing.T) {
		svc, _, _, _ := newBankService(t)
		err := svc.CreateBank(ctx, nil)
		assert.EqualError(t, err, "bank is required")
	})
}

func TestQuizItemBankService_GetBank(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		svc, br, _, _ := newBankService(t)
		want := &models.QuizItemBank{ID: 9, CourseID: 1, Title: "B"}
		br.On("FindByID", ctx, uint(9)).Return(want, nil)
		got, err := svc.GetBank(ctx, 1, 9)
		assert.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("not found", func(t *testing.T) {
		svc, br, _, _ := newBankService(t)
		br.On("FindByID", ctx, uint(9)).Return(nil, errors.New("nope"))
		_, err := svc.GetBank(ctx, 1, 9)
		assert.EqualError(t, err, "item bank not found")
	})

	t.Run("cross-course rejection", func(t *testing.T) {
		svc, br, _, _ := newBankService(t)
		br.On("FindByID", ctx, uint(9)).Return(&models.QuizItemBank{ID: 9, CourseID: 2}, nil)
		_, err := svc.GetBank(ctx, 1, 9)
		assert.EqualError(t, err, "item bank does not belong to course")
	})
}

func TestQuizItemBankService_ListBanks(t *testing.T) {
	ctx := context.Background()
	svc, br, _, _ := newBankService(t)
	params := repository.PaginationParams{Page: 1, PerPage: 20}
	want := &repository.PaginatedResult[models.QuizItemBank]{Items: []models.QuizItemBank{{ID: 1}}, TotalCount: 1, Page: 1, PerPage: 20}
	br.On("ListByCourseID", ctx, uint(5), params).Return(want, nil)
	got, err := svc.ListBanks(ctx, 5, params)
	assert.NoError(t, err)
	assert.Equal(t, want, got)

	_, err = svc.ListBanks(ctx, 0, params)
	assert.EqualError(t, err, "course_id is required")
}

func TestQuizItemBankService_UpdateBank(t *testing.T) {
	ctx := context.Background()

	t.Run("immutable fields are preserved", func(t *testing.T) {
		svc, br, _, _ := newBankService(t)
		br.On("FindByID", ctx, uint(3)).Return(&models.QuizItemBank{ID: 3, CourseID: 1, CreatedByUserID: 42}, nil)
		br.On("Update", ctx, mock.MatchedBy(func(b *models.QuizItemBank) bool {
			return b.ID == 3 && b.CourseID == 1 && b.CreatedByUserID == 42 && b.Title == "Updated"
		})).Return(nil)
		bank := &models.QuizItemBank{ID: 3, Title: "Updated", CourseID: 999, CreatedByUserID: 999}
		err := svc.UpdateBank(ctx, 1, bank)
		assert.NoError(t, err)
		assert.Equal(t, uint(1), bank.CourseID)
		assert.Equal(t, uint(42), bank.CreatedByUserID)
	})

	t.Run("not found", func(t *testing.T) {
		svc, br, _, _ := newBankService(t)
		br.On("FindByID", ctx, uint(3)).Return(nil, errors.New("nope"))
		err := svc.UpdateBank(ctx, 1, &models.QuizItemBank{ID: 3})
		assert.EqualError(t, err, "item bank not found")
	})

	t.Run("cross-course rejection", func(t *testing.T) {
		svc, br, _, _ := newBankService(t)
		br.On("FindByID", ctx, uint(3)).Return(&models.QuizItemBank{ID: 3, CourseID: 2}, nil)
		err := svc.UpdateBank(ctx, 1, &models.QuizItemBank{ID: 3})
		assert.EqualError(t, err, "item bank does not belong to course")
	})

	t.Run("missing id", func(t *testing.T) {
		svc, _, _, _ := newBankService(t)
		err := svc.UpdateBank(ctx, 1, &models.QuizItemBank{})
		assert.EqualError(t, err, "bank id is required")
	})
}

func TestQuizItemBankService_DeleteBank(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		svc, br, _, _ := newBankService(t)
		br.On("FindByID", ctx, uint(3)).Return(&models.QuizItemBank{ID: 3, CourseID: 1}, nil)
		br.On("Delete", ctx, uint(3)).Return(nil)
		assert.NoError(t, svc.DeleteBank(ctx, 1, 3))
	})

	t.Run("cross-course rejection", func(t *testing.T) {
		svc, br, _, _ := newBankService(t)
		br.On("FindByID", ctx, uint(3)).Return(&models.QuizItemBank{ID: 3, CourseID: 2}, nil)
		err := svc.DeleteBank(ctx, 1, 3)
		assert.EqualError(t, err, "item bank does not belong to course")
	})

	t.Run("not found", func(t *testing.T) {
		svc, br, _, _ := newBankService(t)
		br.On("FindByID", ctx, uint(3)).Return(nil, errors.New("nope"))
		assert.EqualError(t, svc.DeleteBank(ctx, 1, 3), "item bank not found")
	})
}

// ---------- Bank Item CRUD ----------

func TestQuizItemBankService_CreateBankItem(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path applies defaults", func(t *testing.T) {
		svc, br, ir, _ := newBankService(t)
		br.On("FindByID", ctx, uint(2)).Return(&models.QuizItemBank{ID: 2}, nil)
		ir.On("Create", ctx, mock.MatchedBy(func(it *models.QuizItemBankItem) bool {
			return it.BankID == 2 && it.PointsPossible != nil && *it.PointsPossible == 1.0 && it.Answers == "[]"
		})).Return(nil)
		item := &models.QuizItemBankItem{BankID: 2, QuestionType: "essay", QuestionText: "Why?"}
		err := svc.CreateBankItem(ctx, item)
		assert.NoError(t, err)
	})

	t.Run("missing bank id", func(t *testing.T) {
		svc, _, _, _ := newBankService(t)
		err := svc.CreateBankItem(ctx, &models.QuizItemBankItem{QuestionType: "essay", QuestionText: "x"})
		assert.EqualError(t, err, "bank_id is required")
	})

	t.Run("missing type", func(t *testing.T) {
		svc, _, _, _ := newBankService(t)
		err := svc.CreateBankItem(ctx, &models.QuizItemBankItem{BankID: 1, QuestionText: "x"})
		assert.EqualError(t, err, "question_type is required")
	})

	t.Run("missing text", func(t *testing.T) {
		svc, _, _, _ := newBankService(t)
		err := svc.CreateBankItem(ctx, &models.QuizItemBankItem{BankID: 1, QuestionType: "essay"})
		assert.EqualError(t, err, "question_text is required")
	})

	t.Run("bank not found", func(t *testing.T) {
		svc, br, _, _ := newBankService(t)
		br.On("FindByID", ctx, uint(9)).Return(nil, errors.New("nope"))
		err := svc.CreateBankItem(ctx, &models.QuizItemBankItem{BankID: 9, QuestionType: "essay", QuestionText: "x"})
		assert.EqualError(t, err, "item bank not found")
	})

	t.Run("nil item", func(t *testing.T) {
		svc, _, _, _ := newBankService(t)
		assert.EqualError(t, svc.CreateBankItem(ctx, nil), "item is required")
	})
}

func TestQuizItemBankService_GetBankItem(t *testing.T) {
	ctx := context.Background()
	svc, _, ir, _ := newBankService(t)
	want := &models.QuizItemBankItem{ID: 5, BankID: 1}
	ir.On("FindByID", ctx, uint(5)).Return(want, nil)
	got, err := svc.GetBankItem(ctx, 5)
	assert.NoError(t, err)
	assert.Equal(t, want, got)

	ir.On("FindByID", ctx, uint(6)).Return(nil, errors.New("nope"))
	_, err = svc.GetBankItem(ctx, 6)
	assert.EqualError(t, err, "bank item not found")
}

func TestQuizItemBankService_ListBankItems(t *testing.T) {
	ctx := context.Background()
	svc, _, ir, _ := newBankService(t)
	want := []models.QuizItemBankItem{{ID: 1}, {ID: 2}}
	ir.On("ListByBankID", ctx, uint(1)).Return(want, nil)
	got, err := svc.ListBankItems(ctx, 1)
	assert.NoError(t, err)
	assert.Equal(t, want, got)

	_, err = svc.ListBankItems(ctx, 0)
	assert.EqualError(t, err, "bank_id is required")
}

func TestQuizItemBankService_UpdateBankItem(t *testing.T) {
	ctx := context.Background()

	t.Run("bank id is immutable", func(t *testing.T) {
		svc, _, ir, _ := newBankService(t)
		ir.On("FindByID", ctx, uint(5)).Return(&models.QuizItemBankItem{ID: 5, BankID: 1}, nil)
		ir.On("Update", ctx, mock.MatchedBy(func(it *models.QuizItemBankItem) bool {
			return it.BankID == 1 && it.QuestionText == "New"
		})).Return(nil)
		err := svc.UpdateBankItem(ctx, &models.QuizItemBankItem{ID: 5, BankID: 999, QuestionText: "New"})
		assert.NoError(t, err)
	})

	t.Run("not found", func(t *testing.T) {
		svc, _, ir, _ := newBankService(t)
		ir.On("FindByID", ctx, uint(5)).Return(nil, errors.New("nope"))
		err := svc.UpdateBankItem(ctx, &models.QuizItemBankItem{ID: 5})
		assert.EqualError(t, err, "bank item not found")
	})

	t.Run("missing id", func(t *testing.T) {
		svc, _, _, _ := newBankService(t)
		err := svc.UpdateBankItem(ctx, &models.QuizItemBankItem{})
		assert.EqualError(t, err, "item id is required")
	})
}

func TestQuizItemBankService_DeleteBankItem(t *testing.T) {
	ctx := context.Background()
	svc, _, ir, _ := newBankService(t)

	ir.On("FindByID", ctx, uint(1)).Return(&models.QuizItemBankItem{ID: 1}, nil)
	ir.On("Delete", ctx, uint(1)).Return(nil)
	assert.NoError(t, svc.DeleteBankItem(ctx, 1))

	ir.On("FindByID", ctx, uint(2)).Return(nil, errors.New("nope"))
	assert.EqualError(t, svc.DeleteBankItem(ctx, 2), "bank item not found")
}

// ---------- Quiz integration ----------

func TestQuizItemBankService_AddBankItemToQuiz(t *testing.T) {
	ctx := context.Background()

	t.Run("copies shape and sets bank_item_id", func(t *testing.T) {
		svc, _, ir, qr := newBankService(t)
		pts := 4.0
		src := &models.QuizItemBankItem{
			ID: 10, BankID: 1,
			QuestionType: "multiple_choice", QuestionText: "Q?",
			PointsPossible:    &pts,
			Answers:           `[{"id":"a","weight":100}]`,
			CorrectComments:   "yay",
			IncorrectComments: "boo",
			NeutralComments:   "ok",
		}
		ir.On("FindByID", ctx, uint(10)).Return(src, nil)
		qr.On("Create", ctx, mock.MatchedBy(func(q *models.QuizQuestion) bool {
			return q.QuizID == 7 &&
				q.Position == 3 &&
				q.QuestionType == "multiple_choice" &&
				q.QuestionText == "Q?" &&
				q.Answers == src.Answers &&
				q.CorrectComments == "yay" &&
				q.IncorrectComments == "boo" &&
				q.NeutralComments == "ok" &&
				q.BankItemID != nil && *q.BankItemID == 10 &&
				q.WorkflowState == "active"
		})).Return(nil)
		q, err := svc.AddBankItemToQuiz(ctx, 10, 7, 3)
		assert.NoError(t, err)
		assert.NotNil(t, q.BankItemID)
		assert.Equal(t, uint(10), *q.BankItemID)
	})

	t.Run("missing quiz id", func(t *testing.T) {
		svc, _, _, _ := newBankService(t)
		_, err := svc.AddBankItemToQuiz(ctx, 10, 0, 0)
		assert.EqualError(t, err, "quiz_id is required")
	})

	t.Run("bank item not found", func(t *testing.T) {
		svc, _, ir, _ := newBankService(t)
		ir.On("FindByID", ctx, uint(10)).Return(nil, errors.New("nope"))
		_, err := svc.AddBankItemToQuiz(ctx, 10, 7, 0)
		assert.EqualError(t, err, "bank item not found")
	})
}

func TestQuizItemBankService_RandomDrawFromBank(t *testing.T) {
	ctx := context.Background()
	pool := []models.QuizItemBankItem{
		{ID: 1, BankID: 9, QuestionType: "essay", QuestionText: "Q1"},
		{ID: 2, BankID: 9, QuestionType: "essay", QuestionText: "Q2"},
		{ID: 3, BankID: 9, QuestionType: "essay", QuestionText: "Q3"},
		{ID: 4, BankID: 9, QuestionType: "essay", QuestionText: "Q4"},
	}

	t.Run("returns requested count and no repeats", func(t *testing.T) {
		svc, _, ir, _ := newBankService(t)
		ir.On("ListByBankID", ctx, uint(9)).Return(pool, nil)
		got, err := svc.RandomDrawFromBank(ctx, 9, 3)
		assert.NoError(t, err)
		assert.Len(t, got, 3)
		seen := map[uint]bool{}
		for _, q := range got {
			assert.NotNil(t, q.BankItemID, "every drawn question must remember its bank item")
			seen[*q.BankItemID] = true
		}
		assert.Len(t, seen, 3, "no repeats in the draw")
	})

	t.Run("count larger than pool returns full pool", func(t *testing.T) {
		svc, _, ir, _ := newBankService(t)
		ir.On("ListByBankID", ctx, uint(9)).Return(pool, nil)
		got, err := svc.RandomDrawFromBank(ctx, 9, 99)
		assert.NoError(t, err)
		assert.Len(t, got, len(pool))
	})

	t.Run("empty pool returns empty result", func(t *testing.T) {
		svc, _, ir, _ := newBankService(t)
		ir.On("ListByBankID", ctx, uint(9)).Return([]models.QuizItemBankItem{}, nil)
		got, err := svc.RandomDrawFromBank(ctx, 9, 3)
		assert.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("zero count is rejected", func(t *testing.T) {
		svc, _, _, _ := newBankService(t)
		_, err := svc.RandomDrawFromBank(ctx, 9, 0)
		assert.EqualError(t, err, "count must be positive")
	})

	t.Run("missing bank id", func(t *testing.T) {
		svc, _, _, _ := newBankService(t)
		_, err := svc.RandomDrawFromBank(ctx, 0, 3)
		assert.EqualError(t, err, "bank_id is required")
	})
}
