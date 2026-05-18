package service

// Quiz authoring methods — Question and QuestionGroup CRUD.
//
// This file is part of the Wave 5 god-file split (chore/wave5-split-quiz-blueprint).
// The methods here all hang off *QuizService so the public surface is unchanged
// for the API handlers in internal/api/v1/handlers/quiz_*; only the source-file
// organization moved. Tests for these methods live in quiz_authoring_test.go.

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// ---------- Quiz Question Methods ----------

func (s *QuizService) CreateQuestion(ctx context.Context, question *models.QuizQuestion) error {
	if question.QuestionText == "" {
		return errors.New("question_text is required")
	}
	if question.QuestionType == "" {
		return errors.New("question_type is required")
	}

	validTypes := map[string]bool{
		"multiple_choice":         true,
		"true_false":              true,
		"short_answer":            true,
		"essay":                   true,
		"matching":                true,
		"fill_in_multiple_blanks": true,
		"numerical_question":      true,
		// Wave A1: 9 new item types added to the auto-grader.
		"multiple_answer":   true,
		"multiple_dropdown": true,
		"fill_in_the_blank": true,
		"formula":           true,
		"file_upload":       true,
		"ordering":          true,
		"categorization":    true,
		"hot_spot":          true,
		"text_only":         true,
	}
	if !validTypes[question.QuestionType] {
		return errors.New("invalid question_type")
	}

	if question.WorkflowState == "" {
		question.WorkflowState = "active"
	}

	if question.PointsPossible == nil {
		defaultPoints := 1.0
		question.PointsPossible = &defaultPoints
	}

	return s.questionRepo.Create(ctx, question)
}

func (s *QuizService) GetQuestion(ctx context.Context, id uint) (*models.QuizQuestion, error) {
	return s.questionRepo.FindByID(ctx, id)
}

func (s *QuizService) UpdateQuestion(ctx context.Context, question *models.QuizQuestion) error {
	return s.questionRepo.Update(ctx, question)
}

func (s *QuizService) DeleteQuestion(ctx context.Context, id uint) error {
	return s.questionRepo.Delete(ctx, id)
}

func (s *QuizService) ListQuestions(ctx context.Context, quizID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.QuizQuestion], error) {
	return s.questionRepo.ListByQuizID(ctx, quizID, params)
}

// ---------- Quiz Question Group Methods ----------

func (s *QuizService) CreateQuestionGroup(ctx context.Context, group *models.QuizQuestionGroup) error {
	if s.groupRepo == nil {
		return errors.New("question group repository not configured")
	}
	if group.PickCount < 1 {
		group.PickCount = 1
	}
	return s.groupRepo.Create(ctx, group)
}

func (s *QuizService) GetQuestionGroup(ctx context.Context, id uint) (*models.QuizQuestionGroup, error) {
	if s.groupRepo == nil {
		return nil, errors.New("question group repository not configured")
	}
	return s.groupRepo.FindByID(ctx, id)
}

func (s *QuizService) UpdateQuestionGroup(ctx context.Context, group *models.QuizQuestionGroup) error {
	if s.groupRepo == nil {
		return errors.New("question group repository not configured")
	}
	return s.groupRepo.Update(ctx, group)
}

func (s *QuizService) DeleteQuestionGroup(ctx context.Context, id uint) error {
	if s.groupRepo == nil {
		return errors.New("question group repository not configured")
	}
	return s.groupRepo.Delete(ctx, id)
}

func (s *QuizService) ListQuestionGroups(ctx context.Context, quizID uint) ([]models.QuizQuestionGroup, error) {
	if s.groupRepo == nil {
		return nil, errors.New("question group repository not configured")
	}
	return s.groupRepo.ListByQuizID(ctx, quizID)
}
