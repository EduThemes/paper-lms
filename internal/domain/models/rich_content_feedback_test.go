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

	// Item-bank items are stored as question-bank entries; the entry hook
	// must cover their per-outcome comments too.
	e2 := &models.QuestionBankEntry{CorrectComments: bad, IncorrectComments: bad, NeutralComments: bad}
	if err := e2.BeforeSave(nil); err != nil {
		t.Fatal(err)
	}
	check("QuestionBankEntry.CorrectComments", e2.CorrectComments)
	check("QuestionBankEntry.IncorrectComments", e2.IncorrectComments)
	check("QuestionBankEntry.NeutralComments", e2.NeutralComments)
}

// Calendar event descriptions are rich content; IMSCC imports create
// events without the handler, so the model sanitizes them.
func TestCalendarEventDescriptionSanitizedOnSave(t *testing.T) {
	ev := &models.CalendarEvent{Description: `<p>Field <strong>trip</strong></p><script>alert(1)</script><a href="javascript:alert(1)">x</a>`}
	if err := ev.BeforeSave(nil); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ev.Description, "<script") || strings.Contains(ev.Description, "javascript:") {
		t.Errorf("description not sanitized: %q", ev.Description)
	}
	if !strings.Contains(ev.Description, "<strong>trip</strong>") {
		t.Errorf("formatting lost: %q", ev.Description)
	}
}

// Stimulus passages are rendered as HTML for every quiz taker.
func TestStimulusContentSanitizedOnSave(t *testing.T) {
	cases := []struct {
		name, in      string
		mustNot, must []string
	}{
		{"json string", `"<p>Read <em>this</em></p><img src=x onerror=alert(1)>"`, []string{"onerror"}, []string{"<em>this</em>", `"`}},
		{"html node", `{"type":"doc","content":[{"type":"html","html":"<b>ok</b><script>bad()</script>"}]}`, []string{"<script", "bad()"}, []string{"<b>ok</b>", `"type":"doc"`}},
		{"raw html", `<p>hi</p><iframe src="javascript:alert(1)"></iframe>`, []string{"javascript:"}, []string{"<p>hi</p>"}},
	}
	for _, tc := range cases {
		s := &models.QuizStimulus{Content: tc.in}
		if err := s.BeforeSave(nil); err != nil {
			t.Fatal(err)
		}
		for _, bad := range tc.mustNot {
			if strings.Contains(s.Content, bad) {
				t.Errorf("%s: %q survived in %q", tc.name, bad, s.Content)
			}
		}
		for _, good := range tc.must {
			if !strings.Contains(s.Content, good) {
				t.Errorf("%s: %q missing from %q", tc.name, good, s.Content)
			}
		}
	}

	// A TipTap doc with only text nodes is left byte-for-byte alone (text
	// is plain text, not HTML), as is the empty doc.
	doc := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"A & B <c>"}]}]}`
	if got := models.SanitizeStimulusContent(doc); got != doc {
		t.Errorf("text-only doc changed: %q", got)
	}
	if got := models.SanitizeStimulusContent("{}"); got != "{}" {
		t.Errorf("empty doc changed: %q", got)
	}
}
