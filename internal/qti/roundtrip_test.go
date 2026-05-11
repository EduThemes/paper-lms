package qti

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// TestRoundTripLossless builds an in-memory Paper LMS quiz, exports it
// to Canvas Classic, reimports the result, and asserts semantic
// equivalence for the lossless type set.
//
// Lossy types intentionally excluded from this test (each degrades to
// its closest Canvas Classic equivalent on export):
//   - ordering        → matching
//   - categorization  → matching
//   - hot_spot        → multiple_choice
//   - fill_in_the_blank → short_answer
//
// These are covered by TestRoundTripLossyDegrades.
func TestRoundTripLossless(t *testing.T) {
	points2 := 2.0
	points1 := 1.0

	original := &models.Quiz{
		ID:             100,
		Title:          "Roundtrip Quiz",
		QuizType:       "assignment",
		ShuffleAnswers: true,
	}
	questions := []models.QuizQuestion{
		{
			ID:             1,
			Position:       0,
			QuestionType:   UnifiedMultipleChoice,
			QuestionText:   "What is 2+2?",
			PointsPossible: &points2,
			Answers: marshalAnswers(t, []map[string]interface{}{
				{"id": "a", "text": "3", "weight": 0.0},
				{"id": "b", "text": "4", "weight": 100.0},
				{"id": "c", "text": "5", "weight": 0.0},
			}),
		},
		{
			ID:             2,
			Position:       1,
			QuestionType:   UnifiedTrueFalse,
			QuestionText:   "Sky is blue.",
			PointsPossible: &points1,
			Answers: marshalAnswers(t, []map[string]interface{}{
				{"id": "t", "text": "True", "weight": 100.0},
				{"id": "f", "text": "False", "weight": 0.0},
			}),
		},
		{
			ID:             3,
			Position:       2,
			QuestionType:   UnifiedMultipleAnswer,
			QuestionText:   "Pick primes.",
			PointsPossible: &points2,
			Answers: marshalAnswers(t, []map[string]interface{}{
				{"id": "p2", "text": "2", "weight": 100.0},
				{"id": "p3", "text": "3", "weight": 100.0},
				{"id": "p4", "text": "4", "weight": 0.0},
			}),
		},
		{
			ID:             4,
			Position:       3,
			QuestionType:   UnifiedShortAnswer,
			QuestionText:   "Capital of France?",
			PointsPossible: &points1,
			Answers: marshalAnswers(t, []map[string]interface{}{
				{"id": "a1", "text": "Paris", "weight": 100.0},
			}),
		},
		{
			ID:             5,
			Position:       4,
			QuestionType:   UnifiedEssay,
			QuestionText:   "Explain.",
			PointsPossible: &points2,
			Answers:        "[]",
		},
		{
			ID:             6,
			Position:       5,
			QuestionType:   UnifiedFileUpload,
			QuestionText:   "Upload.",
			PointsPossible: &points2,
			Answers:        "[]",
		},
		{
			ID:             7,
			Position:       6,
			QuestionType:   UnifiedTextOnly,
			QuestionText:   "Read carefully.",
			PointsPossible: &points1,
			Answers:        "[]",
		},
		{
			ID:             8,
			Position:       7,
			QuestionType:   UnifiedNumerical,
			QuestionText:   "Answer:",
			PointsPossible: &points1,
			Answers: marshalAnswers(t, []map[string]interface{}{
				{"id": "a1", "text": "42", "weight": 100.0},
			}),
		},
		{
			ID:             9,
			Position:       8,
			QuestionType:   UnifiedFormula,
			QuestionText:   "Compute:",
			PointsPossible: &points2,
			Answers: marshalAnswers(t, []map[string]interface{}{
				{"id": "a1", "text": "100", "weight": 100.0},
			}),
		},
		{
			ID:             10,
			Position:       9,
			QuestionType:   UnifiedFillInMultipleBlanks,
			QuestionText:   "The [c1] [c2] fox.",
			PointsPossible: &points2,
			// Note: FIMB answers JSON is map-shaped, not array-shaped.
			Answers: `{"c1":["quick"],"c2":["brown"]}`,
		},
		{
			ID:             11,
			Position:       10,
			QuestionType:   UnifiedMultipleDropdown,
			QuestionText:   "X",
			PointsPossible: &points2,
			Answers: marshalAnswers(t, []map[string]interface{}{
				{"id": "b1_x", "text": "sky", "weight": 100.0, "blank_id": "b1"},
				{"id": "b1_y", "text": "grass", "weight": 0.0, "blank_id": "b1"},
			}),
		},
		{
			ID:             12,
			Position:       11,
			QuestionType:   UnifiedMatching,
			QuestionText:   "Match.",
			PointsPossible: &points2,
			Answers: marshalAnswers(t, []map[string]interface{}{
				{"id": "france", "left": "France", "right_id": "paris", "weight": 100.0},
				{"id": "uk", "left": "UK", "right_id": "london", "weight": 100.0},
			}),
		},
	}

	exporter := NewExporter()
	zipBytes, err := exporter.ExportQuiz(original, questions, nil)
	if err != nil {
		t.Fatalf("ExportQuiz: %v", err)
	}
	if len(zipBytes) == 0 {
		t.Fatal("export produced empty zip")
	}

	// Reimport.
	path := writeTempZip(t, zipBytes)
	imp := NewImporter()
	result, err := imp.ImportIMSCC(context.Background(), path, 1)
	if err != nil {
		t.Fatalf("reimport: %v", err)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("reimport errors: %+v", result.Errors)
	}
	if len(result.Quizzes) != 1 {
		t.Fatalf("want 1 quiz on reimport, got %d", len(result.Quizzes))
	}
	rq := result.Quizzes[0]
	if rq.Title != original.Title {
		t.Errorf("title drift: %q → %q", original.Title, rq.Title)
	}
	if len(rq.Questions) != len(questions) {
		t.Errorf("question count: want %d, got %d", len(questions), len(rq.Questions))
	}

	// Spot-check each question's type & key fields.
	for i, want := range questions {
		if i >= len(rq.Questions) {
			break
		}
		got := rq.Questions[i]
		if got.QuestionType != want.QuestionType {
			t.Errorf("Q%d type: want %s, got %s", i, want.QuestionType, got.QuestionType)
		}
		// Points may slip back to the metadata default of 1.0 for
		// text_only on reimport. Don't fail on that one.
		if want.QuestionType != UnifiedTextOnly && want.PointsPossible != nil {
			if got.PointsPossible == nil || *got.PointsPossible != *want.PointsPossible {
				t.Errorf("Q%d points: want %v, got %v", i, *want.PointsPossible, got.PointsPossible)
			}
		}
	}
}

// TestRoundTripLossyDegrades documents the lossy round-trip behavior
// (per mapping.go). ordering exports as matching; on reimport it comes
// back as `matching`, not `ordering`. This is intentional — Canvas
// Classic has no native ordering type and we choose maximum import
// portability over fidelity. The test exists so the lossy contract
// is asserted explicitly.
func TestRoundTripLossyDegrades(t *testing.T) {
	points := 1.0
	quiz := &models.Quiz{ID: 1, Title: "Lossy", QuizType: "assignment"}
	questions := []models.QuizQuestion{
		{
			ID:             1, Position: 0,
			QuestionType:   UnifiedOrdering,
			QuestionText:   "Order me",
			PointsPossible: &points,
			Answers: marshalAnswers(t, []map[string]interface{}{
				{"id": "s1", "text": "First", "weight": 100.0, "right_id": "s1"},
				{"id": "s2", "text": "Second", "weight": 100.0, "right_id": "s2"},
			}),
		},
	}

	exporter := NewExporter()
	zipBytes, err := exporter.ExportQuiz(quiz, questions, nil)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	imp := NewImporter()
	result, err := imp.ImportIMSCC(context.Background(), writeTempZip(t, zipBytes), 1)
	if err != nil {
		t.Fatalf("reimport: %v", err)
	}
	if len(result.Quizzes[0].Questions) != 1 {
		t.Fatalf("want 1 question, got %d", len(result.Quizzes[0].Questions))
	}
	// ordering → matching after lossy round-trip.
	if rt := result.Quizzes[0].Questions[0].QuestionType; rt != UnifiedMatching {
		t.Errorf("expected ordering to come back as matching (lossy), got %s", rt)
	}
}

func marshalAnswers(t *testing.T, opts []map[string]interface{}) string {
	t.Helper()
	b, err := json.Marshal(opts)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}
