package postgres_test

// B000/B001 regression: rich-content columns are sanitized by model
// BeforeSave hooks, so write paths that bypass the API handlers (question
// banks, QTI / IMSCC / Commons imports, blueprint syncs) can no longer store
// script-bearing HTML that the client later renders.

import (
	"context"
	"strings"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
)

const xssPayload = `<p>What is 2+2?</p><img src="x" onerror="fetch('/api/v1/users/self')"><script>alert(1)</script>`

func assertSanitized(t *testing.T, what, got string) {
	t.Helper()
	for _, bad := range []string{"onerror", "<script", "alert(1)"} {
		if strings.Contains(got, bad) {
			t.Fatalf("%s stored unsanitized (contains %q): %q", what, bad, got)
		}
	}
	if !strings.Contains(got, "What is 2+2?") {
		t.Fatalf("%s lost its legitimate content: %q", what, got)
	}
}

func TestRichContentHooks_SanitizeNonHandlerWrites(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()

	if err := g.WithContext(ctx).Exec(
		`INSERT INTO accounts (id, name, workflow_state, mfa_policy, default_locale, tenant_mode, max_upload_size_mb)
		 VALUES (1, 'Acct', 'active', 'off', 'en', 'higher_ed', 500)
		 ON CONFLICT (id) DO NOTHING`).Error; err != nil {
		t.Fatalf("seed account: %v", err)
	}

	course := &models.Course{
		AccountID:     1,
		Name:          "Algebra I",
		CourseCode:    "ALG-1",
		WorkflowState: models.CourseAvailable,
		SyllabusBody:  xssPayload,
	}
	if err := postgres.NewCourseRepository(g).Create(ctx, course); err != nil {
		t.Fatalf("create course: %v", err)
	}
	var storedCourse models.Course
	if err := g.First(&storedCourse, course.ID).Error; err != nil {
		t.Fatalf("reload course: %v", err)
	}
	assertSanitized(t, "course syllabus", storedCourse.SyllabusBody)

	t.Run("question bank entry create and update", func(t *testing.T) {
		bank := &models.QuestionBank{CourseID: course.ID, Title: "Bank", WorkflowState: "active"}
		if err := postgres.NewQuestionBankRepository(g).Create(ctx, bank); err != nil {
			t.Fatalf("create bank: %v", err)
		}
		entryRepo := postgres.NewQuestionBankEntryRepository(g)
		entry := &models.QuestionBankEntry{QuestionBankID: bank.ID, QuestionType: "essay", QuestionText: xssPayload}
		if err := entryRepo.Create(ctx, entry); err != nil {
			t.Fatalf("create entry: %v", err)
		}
		got, err := entryRepo.FindByID(ctx, entry.ID)
		if err != nil {
			t.Fatalf("reload entry: %v", err)
		}
		assertSanitized(t, "question bank entry (create)", got.QuestionText)

		got.QuestionText = xssPayload
		if err := entryRepo.Update(ctx, got); err != nil {
			t.Fatalf("update entry: %v", err)
		}
		again, err := entryRepo.FindByID(ctx, entry.ID)
		if err != nil {
			t.Fatalf("reload entry: %v", err)
		}
		assertSanitized(t, "question bank entry (update)", again.QuestionText)
	})

	t.Run("wiki page create and save", func(t *testing.T) {
		pageRepo := postgres.NewPageRepository(g)
		page := &models.WikiPage{CourseID: course.ID, Title: "Imported", URL: "imported", Body: xssPayload, WorkflowState: "active"}
		if err := pageRepo.Create(ctx, page); err != nil {
			t.Fatalf("create page: %v", err)
		}
		got, err := pageRepo.FindByID(ctx, page.ID, 1)
		if err != nil {
			t.Fatalf("reload page: %v", err)
		}
		assertSanitized(t, "page body (create)", got.Body)

		got.Body = xssPayload
		if err := pageRepo.Update(ctx, got); err != nil {
			t.Fatalf("update page: %v", err)
		}
		again, err := pageRepo.FindByID(ctx, page.ID, 1)
		if err != nil {
			t.Fatalf("reload page: %v", err)
		}
		assertSanitized(t, "page body (save)", again.Body)
	})

	t.Run("item bank item comments on create and update", func(t *testing.T) {
		bank := &models.QuestionBank{CourseID: course.ID, Title: "Item bank", WorkflowState: "active"}
		if err := postgres.NewQuestionBankRepository(g).Create(ctx, bank); err != nil {
			t.Fatalf("create bank: %v", err)
		}
		itemRepo := postgres.NewQuizItemBankItemRepository(g)
		item := &models.QuizItemBankItem{
			BankID: bank.ID, QuestionType: "essay_question", QuestionText: xssPayload,
			CorrectComments: xssPayload, IncorrectComments: xssPayload, NeutralComments: xssPayload,
		}
		if err := itemRepo.Create(ctx, item); err != nil {
			t.Fatalf("create item: %v", err)
		}
		got, err := itemRepo.FindByID(ctx, item.ID)
		if err != nil {
			t.Fatalf("reload item: %v", err)
		}
		assertSanitized(t, "item correct comments (create)", got.CorrectComments)
		assertSanitized(t, "item neutral comments (create)", got.NeutralComments)

		// Update writes through Updates(map), which skips the hook.
		got.QuestionText, got.CorrectComments, got.IncorrectComments, got.NeutralComments = xssPayload, xssPayload, xssPayload, xssPayload
		if err := itemRepo.Update(ctx, got); err != nil {
			t.Fatalf("update item: %v", err)
		}
		again, err := itemRepo.FindByID(ctx, item.ID)
		if err != nil {
			t.Fatalf("reload item: %v", err)
		}
		assertSanitized(t, "item question text (update)", again.QuestionText)
		assertSanitized(t, "item correct comments (update)", again.CorrectComments)
		assertSanitized(t, "item incorrect comments (update)", again.IncorrectComments)
		assertSanitized(t, "item neutral comments (update)", again.NeutralComments)
	})

	t.Run("calendar event description from an import", func(t *testing.T) {
		ev := &models.CalendarEvent{
			ContextType: "Course", ContextID: course.ID, Title: "Imported", Description: xssPayload,
			StartAt: course.CreatedAt, CreatedByUserID: 1, WorkflowState: "active",
		}
		if err := g.WithContext(ctx).Create(ev).Error; err != nil {
			t.Fatalf("create event: %v", err)
		}
		var stored models.CalendarEvent
		if err := g.First(&stored, ev.ID).Error; err != nil {
			t.Fatalf("reload event: %v", err)
		}
		assertSanitized(t, "calendar event description", stored.Description)
	})

	t.Run("stimulus content", func(t *testing.T) {
		stim := &models.QuizStimulus{CourseID: course.ID, Title: "Passage", Content: `"` + strings.ReplaceAll(xssPayload, `"`, `\"`) + `"`}
		if err := postgres.NewQuizStimulusRepository(g).Create(ctx, stim); err != nil {
			t.Fatalf("create stimulus: %v", err)
		}
		var stored models.QuizStimulus
		if err := g.First(&stored, stim.ID).Error; err != nil {
			t.Fatalf("reload stimulus: %v", err)
		}
		assertSanitized(t, "stimulus content", stored.Content)
	})
}
