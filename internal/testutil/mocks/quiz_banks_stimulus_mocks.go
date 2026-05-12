package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockQuizItemBankRepository mocks repository.QuizItemBankRepository.
type MockQuizItemBankRepository struct {
	mock.Mock
}

func (m *MockQuizItemBankRepository) Create(ctx context.Context, bank *models.QuizItemBank) error {
	return m.Called(ctx, bank).Error(0)
}

func (m *MockQuizItemBankRepository) FindByID(ctx context.Context, id uint) (*models.QuizItemBank, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.QuizItemBank), args.Error(1)
}

func (m *MockQuizItemBankRepository) Update(ctx context.Context, bank *models.QuizItemBank) error {
	return m.Called(ctx, bank).Error(0)
}

func (m *MockQuizItemBankRepository) Delete(ctx context.Context, id uint) error {
	return m.Called(ctx, id).Error(0)
}

func (m *MockQuizItemBankRepository) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.QuizItemBank], error) {
	args := m.Called(ctx, courseID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.QuizItemBank]), args.Error(1)
}

// MockQuizItemBankItemRepository mocks repository.QuizItemBankItemRepository.
type MockQuizItemBankItemRepository struct {
	mock.Mock
}

func (m *MockQuizItemBankItemRepository) Create(ctx context.Context, item *models.QuizItemBankItem) error {
	return m.Called(ctx, item).Error(0)
}

func (m *MockQuizItemBankItemRepository) FindByID(ctx context.Context, id uint) (*models.QuizItemBankItem, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.QuizItemBankItem), args.Error(1)
}

func (m *MockQuizItemBankItemRepository) Update(ctx context.Context, item *models.QuizItemBankItem) error {
	return m.Called(ctx, item).Error(0)
}

func (m *MockQuizItemBankItemRepository) Delete(ctx context.Context, id uint) error {
	return m.Called(ctx, id).Error(0)
}

func (m *MockQuizItemBankItemRepository) ListByBankID(ctx context.Context, bankID uint) ([]models.QuizItemBankItem, error) {
	args := m.Called(ctx, bankID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.QuizItemBankItem), args.Error(1)
}

// MockQuizStimulusRepository mocks repository.QuizStimulusRepository.
type MockQuizStimulusRepository struct {
	mock.Mock
}

func (m *MockQuizStimulusRepository) Create(ctx context.Context, stim *models.QuizStimulus) error {
	return m.Called(ctx, stim).Error(0)
}

func (m *MockQuizStimulusRepository) FindByID(ctx context.Context, id uint) (*models.QuizStimulus, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.QuizStimulus), args.Error(1)
}

func (m *MockQuizStimulusRepository) Update(ctx context.Context, stim *models.QuizStimulus) error {
	return m.Called(ctx, stim).Error(0)
}

func (m *MockQuizStimulusRepository) Delete(ctx context.Context, id uint) error {
	return m.Called(ctx, id).Error(0)
}

func (m *MockQuizStimulusRepository) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.QuizStimulus], error) {
	args := m.Called(ctx, courseID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.QuizStimulus]), args.Error(1)
}

func (m *MockQuizStimulusRepository) ListQuestionsForStimulus(ctx context.Context, stimulusID uint) ([]models.QuizQuestion, error) {
	args := m.Called(ctx, stimulusID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.QuizQuestion), args.Error(1)
}

func (m *MockQuizStimulusRepository) SetQuestionStimulus(ctx context.Context, questionID uint, stimulusID *uint) error {
	return m.Called(ctx, questionID, stimulusID).Error(0)
}

// MockQuizQuestionOutcomeAlignmentRepository mocks repository.QuizQuestionOutcomeAlignmentRepository.
type MockQuizQuestionOutcomeAlignmentRepository struct {
	mock.Mock
}

func (m *MockQuizQuestionOutcomeAlignmentRepository) Create(ctx context.Context, a *models.QuizQuestionOutcomeAlignment) error {
	return m.Called(ctx, a).Error(0)
}

func (m *MockQuizQuestionOutcomeAlignmentRepository) Delete(ctx context.Context, id uint) error {
	return m.Called(ctx, id).Error(0)
}

func (m *MockQuizQuestionOutcomeAlignmentRepository) DeleteByQuestionAndOutcome(ctx context.Context, quizQuestionID, outcomeID uint) error {
	return m.Called(ctx, quizQuestionID, outcomeID).Error(0)
}

func (m *MockQuizQuestionOutcomeAlignmentRepository) FindByQuestionAndOutcome(ctx context.Context, quizQuestionID, outcomeID uint) (*models.QuizQuestionOutcomeAlignment, error) {
	args := m.Called(ctx, quizQuestionID, outcomeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.QuizQuestionOutcomeAlignment), args.Error(1)
}

func (m *MockQuizQuestionOutcomeAlignmentRepository) ListByQuestionID(ctx context.Context, quizQuestionID uint) ([]models.QuizQuestionOutcomeAlignment, error) {
	args := m.Called(ctx, quizQuestionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.QuizQuestionOutcomeAlignment), args.Error(1)
}

func (m *MockQuizQuestionOutcomeAlignmentRepository) ListByOutcomeID(ctx context.Context, outcomeID uint) ([]models.QuizQuestionOutcomeAlignment, error) {
	args := m.Called(ctx, outcomeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.QuizQuestionOutcomeAlignment), args.Error(1)
}

// MockLearningOutcomeRepository mocks repository.LearningOutcomeRepository.
type MockLearningOutcomeRepository struct {
	mock.Mock
}

func (m *MockLearningOutcomeRepository) Create(ctx context.Context, outcome *models.LearningOutcome) error {
	return m.Called(ctx, outcome).Error(0)
}

func (m *MockLearningOutcomeRepository) FindByID(ctx context.Context, id uint) (*models.LearningOutcome, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.LearningOutcome), args.Error(1)
}

func (m *MockLearningOutcomeRepository) Update(ctx context.Context, outcome *models.LearningOutcome) error {
	return m.Called(ctx, outcome).Error(0)
}

func (m *MockLearningOutcomeRepository) Delete(ctx context.Context, id uint) error {
	return m.Called(ctx, id).Error(0)
}

func (m *MockLearningOutcomeRepository) ListByGroupID(ctx context.Context, groupID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LearningOutcome], error) {
	args := m.Called(ctx, groupID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.LearningOutcome]), args.Error(1)
}

func (m *MockLearningOutcomeRepository) ListByContext(ctx context.Context, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LearningOutcome], error) {
	args := m.Called(ctx, contextType, contextID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.LearningOutcome]), args.Error(1)
}
