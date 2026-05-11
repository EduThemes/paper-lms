package service

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

func TestQuizAssessmentMetaXML_RoundTripsThroughImporter(t *testing.T) {
	tl := 30
	pts := 10.0
	due := time.Date(2026, 6, 1, 23, 59, 0, 0, time.UTC)
	q := models.Quiz{
		ID:                 42,
		Title:              "Round Trip Quiz",
		Description:        "Body",
		QuizType:           "practice_quiz",
		TimeLimit:          &tl,
		AllowedAttempts:    3,
		PointsPossible:     &pts,
		ShuffleAnswers:     true,
		ScoringPolicy:      "keep_latest",
		ShowCorrectAnswers: false,
		HideResults:        "until_after_last_attempt",
		OneQuestionAtATime: true,
		CantGoBack:         true,
		Published:          true,
		DueAt:              &due,
	}
	body, err := quizAssessmentMetaXML(q)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// The exported XML must parse back through the importer's struct so a
	// round-trip is lossless for the fields applyQuizMeta cares about.
	var meta canvasQuizMeta
	if err := xml.Unmarshal(body, &meta); err != nil {
		t.Fatalf("re-import: %v", err)
	}
	if meta.Title != q.Title {
		t.Errorf("title: %q != %q", meta.Title, q.Title)
	}
	if meta.QuizType != q.QuizType {
		t.Errorf("quiz_type: %q", meta.QuizType)
	}
	if meta.AllowedAttempts != "3" {
		t.Errorf("allowed_attempts: %q", meta.AllowedAttempts)
	}
	if meta.ShuffleAnswers != "true" {
		t.Errorf("shuffle_answers: %q", meta.ShuffleAnswers)
	}
	if meta.ScoringPolicy != "keep_latest" {
		t.Errorf("scoring_policy: %q", meta.ScoringPolicy)
	}
	if meta.Available != "true" {
		t.Errorf("available: %q", meta.Available)
	}
	if !strings.Contains(meta.DueAt, "2026-06-01") {
		t.Errorf("due_at not preserved: %q", meta.DueAt)
	}

	// Re-apply through the importer and verify the fields land back on a
	// fresh Quiz row.
	target := &models.Quiz{ScoringPolicy: "keep_highest", ShowCorrectAnswers: true}
	applyQuizMeta(target, &meta)
	if !target.ShuffleAnswers || target.ScoringPolicy != "keep_latest" {
		t.Errorf("import didn't apply: %+v", target)
	}
	if target.HideResults != "until_after_last_attempt" {
		t.Errorf("hide_results: %q", target.HideResults)
	}
	if target.AllowedAttempts != 3 {
		t.Errorf("allowed_attempts: %d", target.AllowedAttempts)
	}
}

func TestExportModuleItemContentType(t *testing.T) {
	cases := map[string]string{
		"Quiz":            "Quizzes::Quiz",
		"DiscussionTopic": "DiscussionTopic",
		"WikiPage":        "WikiPage",
		"ExternalUrl":     "ExternalUrl",
		"Attachment":      "Attachment",
	}
	for in, want := range cases {
		if got := exportModuleItemContentType(in); got != want {
			t.Errorf("exportModuleItemContentType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSanitizeAttachmentName(t *testing.T) {
	cases := map[string]string{
		"foo.pdf":            "foo.pdf",
		"":                   "file",
		"/etc/passwd":        "etc/passwd",
		"../../bad.txt":      "bad.txt",
		"normal name.pdf":    "normal name.pdf",
		"with..dotsbad.txt":  "withdotsbad.txt",
	}
	for in, want := range cases {
		if got := sanitizeAttachmentName(in); got != want {
			t.Errorf("sanitizeAttachmentName(%q) = %q, want %q", in, got, want)
		}
	}
}
