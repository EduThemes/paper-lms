package service

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type QuestionBankService struct {
	bankRepo     repository.QuestionBankRepository
	entryRepo    repository.QuestionBankEntryRepository
	quizRepo     repository.QuizQuestionRepository
	quizMetaRepo repository.QuizRepository
}

func NewQuestionBankService(
	bankRepo repository.QuestionBankRepository,
	entryRepo repository.QuestionBankEntryRepository,
	quizRepo repository.QuizQuestionRepository,
	quizMetaRepo repository.QuizRepository,
) *QuestionBankService {
	return &QuestionBankService{
		bankRepo:     bankRepo,
		entryRepo:    entryRepo,
		quizRepo:     quizRepo,
		quizMetaRepo: quizMetaRepo,
	}
}

func (s *QuestionBankService) CreateBank(ctx context.Context, bank *models.QuestionBank) error {
	if bank.Title == "" {
		return errors.New("title is required")
	}
	bank.WorkflowState = "active"
	return s.bankRepo.Create(ctx, bank)
}

func (s *QuestionBankService) GetBank(ctx context.Context, id uint) (*models.QuestionBank, error) {
	return s.bankRepo.FindByID(ctx, id)
}

func (s *QuestionBankService) UpdateBank(ctx context.Context, id uint, title string) (*models.QuestionBank, error) {
	bank, err := s.bankRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("question bank not found")
	}
	bank.Title = title
	if err := s.bankRepo.Update(ctx, bank); err != nil {
		return nil, err
	}
	return bank, nil
}

func (s *QuestionBankService) DeleteBank(ctx context.Context, id uint) error {
	return s.bankRepo.Delete(ctx, id)
}

func (s *QuestionBankService) ListBanks(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.QuestionBank], error) {
	return s.bankRepo.ListByCourse(ctx, courseID, params)
}

func (s *QuestionBankService) AddQuestion(ctx context.Context, entry *models.QuestionBankEntry) error {
	if entry.QuestionText == "" {
		return errors.New("question_text is required")
	}
	return s.entryRepo.Create(ctx, entry)
}

func (s *QuestionBankService) UpdateQuestion(ctx context.Context, id uint, entry *models.QuestionBankEntry) (*models.QuestionBankEntry, error) {
	existing, err := s.entryRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("question not found")
	}
	existing.QuestionName = entry.QuestionName
	existing.QuestionType = entry.QuestionType
	existing.QuestionText = entry.QuestionText
	existing.PointsPossible = entry.PointsPossible
	existing.Answers = entry.Answers
	existing.Feedback = entry.Feedback
	existing.Position = entry.Position
	if err := s.entryRepo.Update(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func (s *QuestionBankService) DeleteQuestion(ctx context.Context, id uint) error {
	return s.entryRepo.Delete(ctx, id)
}

// GetEntry loads a single bank entry by id. Callers MUST verify the
// returned entry's QuestionBankID matches the bank named in the request
// path before acting on it — the entry repo is not tenant-scoped, so the
// parent-bank tie is what enforces tenant/course isolation.
func (s *QuestionBankService) GetEntry(ctx context.Context, id uint) (*models.QuestionBankEntry, error) {
	return s.entryRepo.FindByID(ctx, id)
}

func (s *QuestionBankService) ListQuestions(ctx context.Context, bankID uint) ([]models.QuestionBankEntry, error) {
	return s.entryRepo.ListByBankID(ctx, bankID)
}

// PullQuestionsToQuiz copies questions from a question bank into a quiz as QuizQuestions.
func (s *QuestionBankService) PullQuestionsToQuiz(ctx context.Context, bankID, quizID, accountID uint, questionIDs []uint) (int, error) {
	// Tenant/parent tie: the destination quiz must live in the same
	// course as the bank (and the caller's tenant). Without this a teacher
	// could copy bank questions INTO another tenant's quiz by id.
	bank, err := s.bankRepo.FindByID(ctx, bankID)
	if err != nil {
		return 0, err
	}
	quiz, err := s.quizMetaRepo.FindByID(ctx, quizID, accountID)
	if err != nil || quiz == nil || quiz.CourseID != bank.CourseID {
		return 0, errors.New("target quiz not found in this course")
	}

	entries, err := s.entryRepo.ListByBankID(ctx, bankID)
	if err != nil {
		return 0, err
	}

	// If questionIDs is provided, filter to only those
	wantIDs := make(map[uint]bool)
	if len(questionIDs) > 0 {
		for _, id := range questionIDs {
			wantIDs[id] = true
		}
	}

	count := 0
	for _, entry := range entries {
		if len(wantIDs) > 0 && !wantIDs[entry.ID] {
			continue
		}

		pts := entry.PointsPossible
		qq := &models.QuizQuestion{
			QuizID:         quizID,
			QuestionType:   entry.QuestionType,
			QuestionText:   entry.QuestionText,
			PointsPossible: &pts,
			Answers:        entry.Answers,
			Position:       entry.Position,
		}
		if err := s.quizRepo.Create(ctx, qq); err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}
