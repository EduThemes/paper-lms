package service

// Question-bank sharing and quiz integration (quiz-items lane, G060).
// question_banks is Paper's single bank store; a bank is owned by a course
// and, when shared with its account, usable (read / pull / copy / draw /
// link to a question group) by every course in that account. Editing stays
// with the owning course.

import (
	"context"
	"errors"
	mathrand "math/rand"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// questionBankSharing is implemented by the Postgres question-bank repo.
type questionBankSharing interface {
	ListUsableByCourse(ctx context.Context, courseID uint) ([]models.QuestionBank, error)
	UsableInCourse(ctx context.Context, bankID, courseID uint) (bool, error)
}

// ErrBankNotUsable is returned when a bank is neither owned by nor shared
// with the course.
var ErrBankNotUsable = errors.New("question bank not found in this course")

// ErrTargetQuizNotInCourse is returned when the destination quiz is not in
// the requesting course / tenant.
var ErrTargetQuizNotInCourse = errors.New("target quiz not found in this course")

// bankUsableInCourse applies the sharing rule with whichever repo is wired.
func bankUsableInCourse(ctx context.Context, repo repository.QuestionBankRepository, bank *models.QuestionBank, courseID uint) bool {
	if bank == nil || bank.WorkflowState == "deleted" {
		return false
	}
	if bank.CourseID == courseID {
		return true
	}
	if !bank.SharedWithAccount {
		return false
	}
	if sh, ok := repo.(questionBankSharing); ok {
		usable, err := sh.UsableInCourse(ctx, bank.ID, courseID)
		return err == nil && usable
	}
	return false
}

// ListUsableBanks lists the course's own banks and those shared with its
// account (own first).
func (s *QuestionBankService) ListUsableBanks(ctx context.Context, courseID uint) ([]models.QuestionBank, error) {
	if sh, ok := s.bankRepo.(questionBankSharing); ok {
		return sh.ListUsableByCourse(ctx, courseID)
	}
	res, err := s.bankRepo.ListByCourse(ctx, courseID, repository.PaginationParams{Page: 1, PerPage: 1000})
	if err != nil {
		return nil, err
	}
	return res.Items, nil
}

// BankUsableInCourse reports whether the course may use the bank.
func (s *QuestionBankService) BankUsableInCourse(ctx context.Context, bank *models.QuestionBank, courseID uint) bool {
	return bankUsableInCourse(ctx, s.bankRepo, bank, courseID)
}

// UpdateBankSettings applies title / description / sharing changes from the
// owning course.
func (s *QuestionBankService) UpdateBankSettings(ctx context.Context, bank *models.QuestionBank, title, description *string, shared *bool) (*models.QuestionBank, error) {
	if title != nil {
		if *title == "" {
			return nil, errors.New("title is required")
		}
		bank.Title = *title
	}
	if description != nil {
		bank.Description = *description
	}
	if shared != nil {
		bank.SharedWithAccount = *shared
	}
	if err := s.bankRepo.Update(ctx, bank); err != nil {
		return nil, err
	}
	return bank, nil
}

// entryToQuizQuestion copies a bank entry into a quiz question.
func entryToQuizQuestion(entry *models.QuestionBankEntry, quizID uint, position int) *models.QuizQuestion {
	pts := entry.PointsPossible
	text := entry.QuestionText
	if text == "" {
		text = entry.QuestionName
	}
	answers := entry.Answers
	if answers == "" {
		answers = "[]"
	}
	entryID := entry.ID
	return &models.QuizQuestion{
		QuizID:            quizID,
		QuestionType:      NormalizeQuizQuestionType(entry.QuestionType),
		QuestionText:      text,
		PointsPossible:    &pts,
		Answers:           answers,
		Position:          position,
		CorrectComments:   entry.CorrectComments,
		IncorrectComments: entry.IncorrectComments,
		NeutralComments:   entry.NeutralComments,
		BankItemID:        &entryID,
		WorkflowState:     "active",
	}
}

// targetQuiz verifies the destination quiz lives in courseID and the
// caller's tenant.
func (s *QuestionBankService) targetQuiz(ctx context.Context, quizID, courseID, accountID uint) (*models.Quiz, error) {
	quiz, err := s.quizMetaRepo.FindByID(ctx, quizID, accountID)
	if err != nil || quiz == nil || quiz.CourseID != courseID || quiz.WorkflowState == "deleted" {
		return nil, ErrTargetQuizNotInCourse
	}
	return quiz, nil
}

// PullQuestionsToCourseQuiz copies (selected) entries of a bank usable by
// courseID into one of that course's quizzes.
func (s *QuestionBankService) PullQuestionsToCourseQuiz(ctx context.Context, bank *models.QuestionBank, courseID, quizID, accountID uint, questionIDs []uint) (int, error) {
	if !s.BankUsableInCourse(ctx, bank, courseID) {
		return 0, ErrBankNotUsable
	}
	if _, err := s.targetQuiz(ctx, quizID, courseID, accountID); err != nil {
		return 0, err
	}
	entries, err := s.entryRepo.ListByBankID(ctx, bank.ID)
	if err != nil {
		return 0, err
	}
	want := map[uint]bool{}
	for _, id := range questionIDs {
		want[id] = true
	}
	count := 0
	for i := range entries {
		if len(want) > 0 && !want[entries[i].ID] {
			continue
		}
		if err := s.quizRepo.Create(ctx, entryToQuizQuestion(&entries[i], quizID, entries[i].Position)); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// DrawRandomToQuiz copies `count` randomly chosen entries into the quiz
// (a fixed draw; for a per-student draw link the bank to a question group).
func (s *QuestionBankService) DrawRandomToQuiz(ctx context.Context, bank *models.QuestionBank, courseID, quizID, accountID uint, count int) (int, error) {
	if count <= 0 {
		return 0, errors.New("count must be at least 1")
	}
	if !s.BankUsableInCourse(ctx, bank, courseID) {
		return 0, ErrBankNotUsable
	}
	if _, err := s.targetQuiz(ctx, quizID, courseID, accountID); err != nil {
		return 0, err
	}
	entries, err := s.entryRepo.ListByBankID(ctx, bank.ID)
	if err != nil {
		return 0, err
	}
	r := mathrand.New(mathrand.NewSource(time.Now().UnixNano())) // #nosec G404 -- random question draw, not security sensitive
	r.Shuffle(len(entries), func(i, j int) { entries[i], entries[j] = entries[j], entries[i] })
	if count > len(entries) {
		count = len(entries)
	}
	for i := 0; i < count; i++ {
		if err := s.quizRepo.Create(ctx, entryToQuizQuestion(&entries[i], quizID, 1000+i)); err != nil {
			return i, err
		}
	}
	return count, nil
}

// CopyBank copies a usable (own or shared) bank and its entries into
// courseID as a new bank the course owns.
func (s *QuestionBankService) CopyBank(ctx context.Context, bank *models.QuestionBank, courseID, userID uint) (*models.QuestionBank, int, error) {
	if !s.BankUsableInCourse(ctx, bank, courseID) {
		return nil, 0, ErrBankNotUsable
	}
	entries, err := s.entryRepo.ListByBankID(ctx, bank.ID)
	if err != nil {
		return nil, 0, err
	}
	creator := userID
	copyBank := &models.QuestionBank{
		CourseID:        courseID,
		Title:           bank.Title + " (copy)",
		Description:     bank.Description,
		CreatedByUserID: &creator,
		WorkflowState:   "active",
	}
	if err := s.bankRepo.Create(ctx, copyBank); err != nil {
		return nil, 0, err
	}
	n := 0
	for i := range entries {
		e := entries[i]
		e.ID = 0
		e.QuestionBankID = copyBank.ID
		e.LegacyQuizItemBankItemID = nil
		e.CreatedAt, e.UpdatedAt = time.Time{}, time.Time{}
		if err := s.entryRepo.Create(ctx, &e); err != nil {
			return copyBank, n, err
		}
		n++
	}
	return copyBank, n, nil
}
