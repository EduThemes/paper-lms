package service_test

// G060 — one question-bank system, on real Postgres:
//   - migration 000091 copies quiz_item_banks/items into question_banks /
//     question_bank_entries and remaps quiz_questions.bank_item_id;
//   - the /quiz_item_banks repositories read and write question_banks;
//   - a bank shared with the account is usable (list, group draw, pull,
//     copy) by other courses in the account, never by another tenant.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
)

var pageAll = repository.PaginationParams{Page: 1, PerPage: 100}

func TestQuestionBankConsolidationDB_MigrationCopiesItemBanks(t *testing.T) {
	f := newQuizDBFixture(t)
	g := f.g

	// Legacy item bank + item + a quiz question that came from it.
	if err := g.Exec(`INSERT INTO quiz_item_banks (id, course_id, title, description, created_by_user_id) VALUES (900, ?, 'Legacy bank', 'old', ?)`, f.courseID, f.userID).Error; err != nil {
		t.Fatal(err)
	}
	if err := g.Exec(`INSERT INTO quiz_item_bank_items (id, bank_id, position, question_type, question_text, points_possible, answers, correct_comments)
		VALUES (901, 900, 1, 'multiple_choice', 'Legacy Q', 3, '[{"id":"a","text":"A","weight":100}]', 'Well done')`).Error; err != nil {
		t.Fatal(err)
	}
	quiz := f.quiz(t, true)
	legacyItem := uint(901)
	q := &models.QuizQuestion{QuizID: quiz.ID, QuestionType: "multiple_choice", QuestionText: "copy", Answers: "[]", WorkflowState: "active", BankItemID: &legacyItem}
	if err := g.Create(q).Error; err != nil {
		t.Fatal(err)
	}

	sqlText, err := os.ReadFile(filepath.Join("..", "db", "migrations", "000091_question_bank_consolidation.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Exec(string(sqlText)).Error; err != nil {
		t.Fatalf("apply 000091: %v", err)
	}

	var bank models.QuestionBank
	if err := g.Where("legacy_quiz_item_bank_id = ?", 900).First(&bank).Error; err != nil {
		t.Fatalf("bank not copied: %v", err)
	}
	if bank.CourseID != f.courseID || bank.Title != "Legacy bank" || bank.Description != "old" || bank.AccountID == nil || *bank.AccountID != 1 {
		t.Fatalf("copied bank = %+v", bank)
	}
	var entry models.QuestionBankEntry
	if err := g.Where("legacy_quiz_item_bank_item_id = ?", 901).First(&entry).Error; err != nil {
		t.Fatalf("item not copied: %v", err)
	}
	if entry.QuestionBankID != bank.ID || entry.PointsPossible != 3 || entry.CorrectComments != "Well done" {
		t.Fatalf("copied entry = %+v", entry)
	}
	var reloaded models.QuizQuestion
	g.First(&reloaded, q.ID)
	if reloaded.BankItemID == nil || *reloaded.BankItemID != entry.ID {
		t.Fatalf("bank_item_id not remapped: %v", reloaded.BankItemID)
	}

	// Idempotent: re-running copies nothing twice.
	if err := g.Exec(string(sqlText)).Error; err != nil {
		t.Fatal(err)
	}
	var n int64
	g.Model(&models.QuestionBank{}).Where("legacy_quiz_item_bank_id = ?", 900).Count(&n)
	if n != 1 {
		t.Fatalf("bank copied %d times", n)
	}

	// The item-bank API repositories now serve the same rows.
	ctx := context.Background()
	itemBanks, err := postgres.NewQuizItemBankRepository(g).ListByCourseID(ctx, f.courseID, pageAll)
	if err != nil || len(itemBanks.Items) != 1 || itemBanks.Items[0].ID != bank.ID {
		t.Fatalf("item-bank listing: %+v %v", itemBanks, err)
	}
	items, err := postgres.NewQuizItemBankItemRepository(g).ListByBankID(ctx, bank.ID)
	if err != nil || len(items) != 1 || items[0].ID != entry.ID || items[0].CorrectComments != "Well done" {
		t.Fatalf("item listing: %+v %v", items, err)
	}
}

func TestQuestionBankConsolidationDB_ItemBankAPIWritesQuestionBanks(t *testing.T) {
	f := newQuizDBFixture(t)
	ctx := context.Background()
	repo := postgres.NewQuizItemBankRepository(f.g)
	itemRepo := postgres.NewQuizItemBankItemRepository(f.g)
	ib := &models.QuizItemBank{CourseID: f.courseID, Title: "From item API", CreatedByUserID: f.userID}
	if err := repo.Create(ctx, ib); err != nil {
		t.Fatal(err)
	}
	pts := 2.0
	item := &models.QuizItemBankItem{BankID: ib.ID, QuestionType: "short_answer", QuestionText: "Capital?", PointsPossible: &pts, Answers: `[{"id":"s","text":"Paris","weight":100}]`}
	if err := itemRepo.Create(ctx, item); err != nil {
		t.Fatal(err)
	}

	bankSvc := service.NewQuestionBankService(postgres.NewQuestionBankRepository(f.g), postgres.NewQuestionBankEntryRepository(f.g), postgres.NewQuizQuestionRepository(f.g), postgres.NewQuizRepository(f.g))
	banks, err := bankSvc.ListUsableBanks(ctx, f.courseID)
	if err != nil || len(banks) != 1 || banks[0].ID != ib.ID || banks[0].AccountID == nil {
		t.Fatalf("question-bank view: %+v %v", banks, err)
	}
	entries, _ := bankSvc.ListQuestions(ctx, ib.ID)
	if len(entries) != 1 || entries[0].PointsPossible != 2 {
		t.Fatalf("entries: %+v", entries)
	}

	// Deleting through the item API soft-deletes the bank everywhere.
	if err := repo.Delete(ctx, ib.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindByID(ctx, ib.ID); err == nil {
		t.Fatal("deleted bank still found")
	}
	if banks, _ := bankSvc.ListUsableBanks(ctx, f.courseID); len(banks) != 0 {
		t.Fatalf("deleted bank still listed: %+v", banks)
	}
}

func TestQuestionBankConsolidationDB_AccountSharing(t *testing.T) {
	f := newQuizDBFixture(t)
	ctx := context.Background()
	// A sibling course in the same account (f.other is another tenant).
	sibling := &models.Course{AccountID: 1, Name: "Course A2", CourseCode: "A-2", WorkflowState: "available"}
	if err := f.g.Create(sibling).Error; err != nil {
		t.Fatal(err)
	}
	bankRepo := postgres.NewQuestionBankRepository(f.g)
	bankSvc := service.NewQuestionBankService(bankRepo, postgres.NewQuestionBankEntryRepository(f.g), postgres.NewQuizQuestionRepository(f.g), postgres.NewQuizRepository(f.g))
	bank := f.bank(t, f.courseID, 3)

	usable := func(courseID uint) bool {
		banks, err := bankSvc.ListUsableBanks(ctx, courseID)
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range banks {
			if b.ID == bank.ID {
				return true
			}
		}
		return false
	}
	if usable(sibling.ID) {
		t.Fatal("an unshared bank leaked to a sibling course")
	}

	shared := true
	if _, err := bankSvc.UpdateBankSettings(ctx, bank, nil, nil, &shared); err != nil {
		t.Fatal(err)
	}
	reloaded, _ := bankSvc.GetBank(ctx, bank.ID)
	if !usable(sibling.ID) || !bankSvc.BankUsableInCourse(ctx, reloaded, sibling.ID) {
		t.Fatal("shared bank not usable by a course in the same account")
	}
	if usable(f.other) || bankSvc.BankUsableInCourse(ctx, reloaded, f.other) {
		t.Fatal("shared bank crossed tenants")
	}

	// A question group in the sibling course draws from the shared bank.
	siblingQuiz := &models.Quiz{CourseID: sibling.ID, Title: "Sibling quiz", QuizType: "assignment", AllowedAttempts: -1, Published: true, WorkflowState: "published"}
	if err := f.g.Create(siblingQuiz).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ValidateQuestionBankForCourse(ctx, bank.ID, sibling.ID); err != nil {
		t.Fatalf("shared bank rejected for a group: %v", err)
	}
	if err := f.svc.ValidateQuestionBankForCourse(ctx, bank.ID, f.other); err == nil {
		t.Fatal("shared bank accepted in another tenant")
	}
	bid := bank.ID
	if err := f.svc.CreateQuestionGroup(ctx, &models.QuizQuestionGroup{QuizID: siblingQuiz.ID, Name: "Shared pool", PickCount: 2, QuestionBankID: &bid}); err != nil {
		t.Fatal(err)
	}
	sub, err := f.svc.StartSubmission(ctx, siblingQuiz.ID, f.userID, nil)
	if err != nil {
		t.Fatal(err)
	}
	qs, _ := f.svc.GetSubmissionQuestions(ctx, sub.ID)
	if len(qs) != 2 {
		t.Fatalf("drew %d questions from the shared bank, want 2", len(qs))
	}

	// Pull, random draw and copy into the sibling course.
	if n, err := bankSvc.PullQuestionsToCourseQuiz(ctx, reloaded, sibling.ID, siblingQuiz.ID, 1, nil); err != nil || n != 3 {
		t.Fatalf("pull: %d %v", n, err)
	}
	if n, err := bankSvc.DrawRandomToQuiz(ctx, reloaded, sibling.ID, siblingQuiz.ID, 1, 2); err != nil || n != 2 {
		t.Fatalf("random draw: %d %v", n, err)
	}
	copied, n, err := bankSvc.CopyBank(ctx, reloaded, sibling.ID, f.userID)
	if err != nil || n != 3 || copied.CourseID != sibling.ID || copied.SharedWithAccount {
		t.Fatalf("copy: %+v %d %v", copied, n, err)
	}
	// A quiz in another course is never a pull target.
	if _, err := bankSvc.PullQuestionsToCourseQuiz(ctx, reloaded, sibling.ID, f.quiz(t, true).ID, 1, nil); err == nil {
		t.Fatal("pulled into a quiz outside the requesting course")
	}
}
