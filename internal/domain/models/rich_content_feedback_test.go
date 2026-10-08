package models_test

import (
	"strings"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// G010: quiz and question-bank feedback is edited with the rich content
// editor and rendered as HTML, so every write path stores it sanitized.
func TestFeedbackColumnsAreSanitizedOnSave(t *testing.T) {
	bad := `<p>Nice <em>work</em></p><img src=x onerror="alert(1)">`
	check := func(name, got string) {
		t.Helper()
		if strings.Contains(got, "onerror") || !strings.Contains(got, "<em>work</em>") {
			t.Errorf("%s not sanitized as rich content: %q", name, got)
		}
	}

	q := &models.QuizQuestion{CorrectComments: bad, IncorrectComments: bad, NeutralComments: bad}
	if err := q.BeforeSave(nil); err != nil {
		t.Fatal(err)
	}
	check("QuizQuestion.CorrectComments", q.CorrectComments)
	check("QuizQuestion.IncorrectComments", q.IncorrectComments)
	check("QuizQuestion.NeutralComments", q.NeutralComments)

	e := &models.QuestionBankEntry{Feedback: bad}
	if err := e.BeforeSave(nil); err != nil {
		t.Fatal(err)
	}
	check("QuestionBankEntry.Feedback", e.Feedback)

	i := &models.QuizItemBankItem{CorrectComments: bad, IncorrectComments: bad, NeutralComments: bad}
	if err := i.BeforeSave(nil); err != nil {
		t.Fatal(err)
	}
	check("QuizItemBankItem.NeutralComments", i.NeutralComments)
}
