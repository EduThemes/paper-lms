package service

import (
	"context"
	"errors"
	mathrand "math/rand"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// QuizItemBankService manages course-scoped reusable question banks.
type QuizItemBankService struct {
	bankRepo     repository.QuizItemBankRepository
	itemRepo     repository.QuizItemBankItemRepository
	questionRepo repository.QuizQuestionRepository
	now          func() time.Time
	rand         *mathrand.Rand
}

func NewQuizItemBankService(
	bankRepo repository.QuizItemBankRepository,
	itemRepo repository.QuizItemBankItemRepository,
	questionRepo repository.QuizQuestionRepository,
) *QuizItemBankService {
	return &QuizItemBankService{
		bankRepo:     bankRepo,
		itemRepo:     itemRepo,
		questionRepo: questionRepo,
		now:          time.Now,
		rand:         mathrand.New(mathrand.NewSource(time.Now().UnixNano())),
	}
}

// ---------- Bank CRUD ----------

func (s *QuizItemBankService) CreateBank(ctx context.Context, bank *models.QuizItemBank) error {
	if bank == nil {
		return errors.New("bank is required")
	}
	if bank.Title == "" {
		return errors.New("title is required")
	}
	if bank.CourseID == 0 {
		return errors.New("course_id is required")
	}
	if bank.CreatedByUserID == 0 {
		return errors.New("created_by_user_id is required")
	}
	return s.bankRepo.Create(ctx, bank)
}

func (s *QuizItemBankService) GetBank(ctx context.Context, courseID, id uint) (*models.QuizItemBank, error) {
	bank, err := s.bankRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("item bank not found")
	}
	if courseID != 0 && bank.CourseID != courseID {
		return nil, errors.New("item bank does not belong to course")
	}
	return bank, nil
}

func (s *QuizItemBankService) ListBanks(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.QuizItemBank], error) {
	if courseID == 0 {
		return nil, errors.New("course_id is required")
	}
	return s.bankRepo.ListByCourseID(ctx, courseID, params)
}

func (s *QuizItemBankService) UpdateBank(ctx context.Context, courseID uint, bank *models.QuizItemBank) error {
	if bank == nil || bank.ID == 0 {
		return errors.New("bank id is required")
	}
	existing, err := s.bankRepo.FindByID(ctx, bank.ID)
	if err != nil {
		return errors.New("item bank not found")
	}
	if courseID != 0 && existing.CourseID != courseID {
		return errors.New("item bank does not belong to course")
	}
	// Preserve immutable fields.
	bank.CourseID = existing.CourseID
	bank.CreatedByUserID = existing.CreatedByUserID
	bank.CreatedAt = existing.CreatedAt
	return s.bankRepo.Update(ctx, bank)
}

func (s *QuizItemBankService) DeleteBank(ctx context.Context, courseID, id uint) error {
	existing, err := s.bankRepo.FindByID(ctx, id)
	if err != nil {
		return errors.New("item bank not found")
	}
	if courseID != 0 && existing.CourseID != courseID {
		return errors.New("item bank does not belong to course")
	}
	return s.bankRepo.Delete(ctx, id)
}

// ---------- Bank Item CRUD ----------

func (s *QuizItemBankService) CreateBankItem(ctx context.Context, item *models.QuizItemBankItem) error {
	if item == nil {
		return errors.New("item is required")
	}
	if item.BankID == 0 {
		return errors.New("bank_id is required")
	}
	if item.QuestionType == "" {
		return errors.New("question_type is required")
	}
	if item.QuestionText == "" {
		return errors.New("question_text is required")
	}
	if _, err := s.bankRepo.FindByID(ctx, item.BankID); err != nil {
		return errors.New("item bank not found")
	}
	if item.PointsPossible == nil {
		def := 1.0
		item.PointsPossible = &def
	}
	if item.Answers == "" {
		item.Answers = "[]"
	}
	return s.itemRepo.Create(ctx, item)
}

func (s *QuizItemBankService) GetBankItem(ctx context.Context, id uint) (*models.QuizItemBankItem, error) {
	item, err := s.itemRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("bank item not found")
	}
	return item, nil
}

func (s *QuizItemBankService) ListBankItems(ctx context.Context, bankID uint) ([]models.QuizItemBankItem, error) {
	if bankID == 0 {
		return nil, errors.New("bank_id is required")
	}
	return s.itemRepo.ListByBankID(ctx, bankID)
}

func (s *QuizItemBankService) UpdateBankItem(ctx context.Context, item *models.QuizItemBankItem) error {
	if item == nil || item.ID == 0 {
		return errors.New("item id is required")
	}
	existing, err := s.itemRepo.FindByID(ctx, item.ID)
	if err != nil {
		return errors.New("bank item not found")
	}
	// BankID is immutable.
	item.BankID = existing.BankID
	item.CreatedAt = existing.CreatedAt
	return s.itemRepo.Update(ctx, item)
}

func (s *QuizItemBankService) DeleteBankItem(ctx context.Context, id uint) error {
	if _, err := s.itemRepo.FindByID(ctx, id); err != nil {
		return errors.New("bank item not found")
	}
	return s.itemRepo.Delete(ctx, id)
}

// ---------- Quiz integration ----------

// AddBankItemToQuiz copies the bank item shape into a new QuizQuestion with
// BankItemID set, so the question retains its provenance.
func (s *QuizItemBankService) AddBankItemToQuiz(ctx context.Context, bankItemID, quizID uint, position int) (*models.QuizQuestion, error) {
	if quizID == 0 {
		return nil, errors.New("quiz_id is required")
	}
	item, err := s.itemRepo.FindByID(ctx, bankItemID)
	if err != nil {
		return nil, errors.New("bank item not found")
	}

	bankItemIDCopy := item.ID
	q := &models.QuizQuestion{
		QuizID:            quizID,
		Position:          position,
		QuestionType:      item.QuestionType,
		QuestionText:      item.QuestionText,
		PointsPossible:    item.PointsPossible,
		Answers:           item.Answers,
		CorrectComments:   item.CorrectComments,
		IncorrectComments: item.IncorrectComments,
		NeutralComments:   item.NeutralComments,
		WorkflowState:     "active",
		BankItemID:        &bankItemIDCopy,
	}
	if err := s.questionRepo.Create(ctx, q); err != nil {
		return nil, err
	}
	return q, nil
}

// RandomDrawFromBank returns `count` distinct QuizQuestion-shaped items copied
// from the bank. They are NOT persisted; the caller decides where they go.
// If count > pool size, every item is returned (in random order) exactly once.
func (s *QuizItemBankService) RandomDrawFromBank(ctx context.Context, bankID uint, count int) ([]models.QuizQuestion, error) {
	if bankID == 0 {
		return nil, errors.New("bank_id is required")
	}
	if count <= 0 {
		return nil, errors.New("count must be positive")
	}
	pool, err := s.itemRepo.ListByBankID(ctx, bankID)
	if err != nil {
		return nil, err
	}
	if len(pool) == 0 {
		return []models.QuizQuestion{}, nil
	}

	// Shuffle a copy so we never mutate the caller's slice.
	shuffled := make([]models.QuizItemBankItem, len(pool))
	copy(shuffled, pool)
	s.rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	if count > len(shuffled) {
		count = len(shuffled)
	}

	out := make([]models.QuizQuestion, 0, count)
	for i := 0; i < count; i++ {
		it := shuffled[i]
		bankItemIDCopy := it.ID
		out = append(out, models.QuizQuestion{
			Position:          i,
			QuestionType:      it.QuestionType,
			QuestionText:      it.QuestionText,
			PointsPossible:    it.PointsPossible,
			Answers:           it.Answers,
			CorrectComments:   it.CorrectComments,
			IncorrectComments: it.IncorrectComments,
			NeutralComments:   it.NeutralComments,
			WorkflowState:     "active",
			BankItemID:        &bankItemIDCopy,
		})
	}
	return out, nil
}
