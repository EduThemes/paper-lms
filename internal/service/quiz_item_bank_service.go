package service

import (
	"context"
	"errors"
	mathrand "math/rand"
	"sync"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// QuizItemBankService manages course-scoped reusable question banks.
type QuizItemBankService struct {
	bankRepo     repository.QuizItemBankRepository
	itemRepo     repository.QuizItemBankItemRepository
	questionRepo repository.QuizQuestionRepository
	quizRepo     repository.QuizRepository
	now          func() time.Time
	rand         *mathrand.Rand
	// randMu serializes s.rand: a *rand.Rand is not safe for concurrent
	// use, and random_draw requests from different teachers run in
	// parallel.
	randMu sync.Mutex
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
		rand:         cryptoSeededRand(),
	}
}

// SetQuizRepo wires the quiz repository used to verify that a destination
// quiz lives in the bank's course (and the caller's tenant) before items
// are copied into it. Without it, quiz-integration calls fail closed.
func (s *QuizItemBankService) SetQuizRepo(repo repository.QuizRepository) {
	s.quizRepo = repo
}

// ErrItemBankQuizNotInCourse is returned when the destination quiz is not
// in the item bank's course / caller's tenant.
var ErrItemBankQuizNotInCourse = errors.New("target quiz not found in this course")

// VerifyQuizInCourse checks quizID names a live quiz in courseID within the
// caller's tenant (accountID).
func (s *QuizItemBankService) VerifyQuizInCourse(ctx context.Context, quizID, courseID, accountID uint) error {
	if s.quizRepo == nil || quizID == 0 {
		return ErrItemBankQuizNotInCourse
	}
	quiz, err := s.quizRepo.FindByID(ctx, quizID, accountID)
	if err != nil || quiz == nil || quiz.CourseID != courseID || quiz.WorkflowState == "deleted" {
		return ErrItemBankQuizNotInCourse
	}
	return nil
}

// GetBankItemInBank loads an item and verifies it belongs to bankID.
func (s *QuizItemBankService) GetBankItemInBank(ctx context.Context, bankID, itemID uint) (*models.QuizItemBankItem, error) {
	item, err := s.itemRepo.FindByID(ctx, itemID)
	if err != nil || item == nil || item.BankID != bankID {
		return nil, errors.New("bank item not found")
	}
	return item, nil
}

// DrawItemsIntoQuiz randomly draws `count` items from the bank and persists
// them as questions of quizID (B133: the draw used to be returned but never
// saved). Returns the created questions.
func (s *QuizItemBankService) DrawItemsIntoQuiz(ctx context.Context, bankID, quizID uint, count, startPosition int) ([]models.QuizQuestion, error) {
	if quizID == 0 {
		return nil, errors.New("quiz_id is required")
	}
	drawn, err := s.RandomDrawFromBank(ctx, bankID, count)
	if err != nil {
		return nil, err
	}
	out := make([]models.QuizQuestion, 0, len(drawn))
	for i := range drawn {
		q := drawn[i]
		q.QuizID = quizID
		q.Position = startPosition + i
		if err := s.questionRepo.Create(ctx, &q); err != nil {
			return out, err
		}
		out = append(out, q)
	}
	return out, nil
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
	s.randMu.Lock()
	s.rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	s.randMu.Unlock()

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
