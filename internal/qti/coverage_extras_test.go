package qti

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// TestImportQTIFileStandaloneClassic exercises the standalone (single
// XML file, no .imscc wrapper) Classic import path.
func TestImportQTIFileStandaloneClassic(t *testing.T) {
	tmp, err := os.CreateTemp("", "standalone-classic-*.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())

	// One-item Classic file (no manifest, no zip).
	xmlDoc := `<?xml version="1.0"?>
<questestinterop>
  <assessment ident="A" title="Solo">
    <qtimetadata/>
    <section ident="s">
      ` + classicItemFromTmpl("q1", "multiple_choice_question", "1", `
        <presentation>
          <material><mattext>?</mattext></material>
          <response_lid ident="response1" rcardinality="Single">
            <render_choice>
              <response_label ident="a"><material><mattext>A</mattext></material></response_label>
            </render_choice>
          </response_lid>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" varname="SCORE"/></outcomes>
          <respcondition continue="No">
            <conditionvar><varequal respident="response1">a</varequal></conditionvar>
            <setvar varname="SCORE" action="Set">100</setvar>
          </respcondition>
        </resprocessing>`) + `
    </section>
  </assessment>
</questestinterop>`
	tmp.WriteString(xmlDoc)
	tmp.Close()

	imp := NewImporter()
	result, err := imp.ImportQTIFile(context.Background(), tmp.Name(), 0)
	if err != nil {
		t.Fatalf("ImportQTIFile: %v", err)
	}
	if result.Dialect != DialectClassic {
		t.Errorf("want Classic dialect, got %s", result.Dialect)
	}
	if len(result.Quizzes) != 1 || len(result.Quizzes[0].Questions) != 1 {
		t.Errorf("want 1 quiz / 1 question, got %d / %v", len(result.Quizzes), result.Quizzes)
	}
}

// TestImportQTIFileStandaloneNewQuizzes covers the single-NQ-item path.
func TestImportQTIFileStandaloneNewQuizzes(t *testing.T) {
	tmp, err := os.CreateTemp("", "standalone-nq-*.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())

	tmp.WriteString(`<?xml version="1.0"?>
<assessmentItem identifier="solo">
  <responseDeclaration identifier="RESPONSE" cardinality="single" baseType="identifier">
    <correctResponse><value>a</value></correctResponse>
  </responseDeclaration>
  <itemBody>
    <choiceInteraction responseIdentifier="RESPONSE" maxChoices="1">
      <simpleChoice identifier="a">A</simpleChoice>
      <simpleChoice identifier="b">B</simpleChoice>
    </choiceInteraction>
  </itemBody>
</assessmentItem>`)
	tmp.Close()

	imp := NewImporter()
	result, err := imp.ImportQTIFile(context.Background(), tmp.Name(), 0)
	if err != nil {
		t.Fatalf("ImportQTIFile: %v", err)
	}
	if result.Dialect != DialectNewQuizzes {
		t.Errorf("want NQ dialect, got %s", result.Dialect)
	}
	if len(result.Quizzes) != 1 || len(result.Quizzes[0].Questions) != 1 {
		t.Errorf("standalone quiz/question count off: %v", result.Quizzes)
	}
}

// TestImportQTIFileUnknownDialect verifies a non-QTI XML file produces
// an error rather than a confusing partial result.
func TestImportQTIFileUnknownDialect(t *testing.T) {
	tmp, _ := os.CreateTemp("", "bogus-*.xml")
	defer os.Remove(tmp.Name())
	tmp.WriteString(`<?xml version="1.0"?><randomroot>nope</randomroot>`)
	tmp.Close()

	imp := NewImporter()
	_, err := imp.ImportQTIFile(context.Background(), tmp.Name(), 0)
	if err == nil {
		t.Error("expected error for unknown dialect")
	}
}

// TestExporterWithBanks verifies the exporter's bank XML path produces
// a parseable bundle.
func TestExporterWithBanks(t *testing.T) {
	points := 1.0
	quiz := &models.Quiz{ID: 1, Title: "Quiz With Bank", QuizType: "assignment"}
	questions := []models.QuizQuestion{{
		ID: 1, Position: 0,
		QuestionType:   UnifiedMultipleChoice,
		QuestionText:   "Q?",
		PointsPossible: &points,
		Answers: marshalAnswers(t, []map[string]interface{}{
			{"id": "a", "text": "A", "weight": 100.0},
			{"id": "b", "text": "B", "weight": 0.0},
		}),
	}}
	banks := []ItemBankImport{{
		Identifier: "bk1",
		Title:      "Round-trip Bank",
		Items: []BankItemImport{{
			Identifier:     "bi1",
			Position:       0,
			QuestionType:   UnifiedShortAnswer,
			QuestionText:   "Capital of France?",
			PointsPossible: &points,
			Answers:        marshalAnswers(t, []map[string]interface{}{{"id": "a1", "text": "Paris", "weight": 100.0}}),
		}},
	}}
	exporter := NewExporter()
	zipBytes, err := exporter.ExportQuiz(quiz, questions, banks)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	path := writeTempZip(t, zipBytes)
	imp := NewImporter()
	result, err := imp.ImportIMSCC(context.Background(), path, 1)
	if err != nil {
		t.Fatalf("reimport: %v", err)
	}
	if len(result.ItemBanks) != 1 {
		t.Errorf("want 1 bank after round-trip, got %d", len(result.ItemBanks))
	}
}

// TestExporterFeedbackRoundtrip verifies the three feedback kinds make
// it through the export/import cycle.
func TestExporterFeedbackRoundtrip(t *testing.T) {
	points := 1.0
	quiz := &models.Quiz{ID: 1, Title: "FB", QuizType: "assignment"}
	questions := []models.QuizQuestion{{
		ID: 1, Position: 0,
		QuestionType:      UnifiedMultipleChoice,
		QuestionText:      "Q?",
		PointsPossible:    &points,
		CorrectComments:   "Nice work",
		IncorrectComments: "Try again",
		NeutralComments:   "Note this",
		Answers: marshalAnswers(t, []map[string]interface{}{
			{"id": "a", "text": "A", "weight": 100.0},
			{"id": "b", "text": "B", "weight": 0.0},
		}),
	}}
	exporter := NewExporter()
	zipBytes, err := exporter.ExportQuiz(quiz, questions, nil)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	path := writeTempZip(t, zipBytes)
	imp := NewImporter()
	result, err := imp.ImportIMSCC(context.Background(), path, 1)
	if err != nil {
		t.Fatalf("reimport: %v", err)
	}
	q := result.Quizzes[0].Questions[0]
	if !strings.Contains(q.CorrectComments, "Nice") {
		t.Errorf("correct comment lost: %q", q.CorrectComments)
	}
	if !strings.Contains(q.IncorrectComments, "Try again") {
		t.Errorf("incorrect comment lost: %q", q.IncorrectComments)
	}
	if !strings.Contains(q.NeutralComments, "Note this") {
		t.Errorf("neutral comment lost: %q", q.NeutralComments)
	}
}

// TestParseRectCoordsBad covers the failure path.
func TestParseRectCoordsBad(t *testing.T) {
	if _, _, _, _, ok := parseRectCoords("not coords"); ok {
		t.Error("expected ok=false for bogus coords")
	}
	if _, _, _, _, ok := parseRectCoords("1,2,3,nope"); ok {
		t.Error("expected ok=false for non-numeric coord")
	}
}

// TestMapNewQuizzesInteraction round-trips the static map for sanity.
// (Exercised here because the static map is the easiest path to drive
// the unused public API to non-zero coverage; the production parser
// uses ClassifyNewQuizzesChoice for the ambiguous cases.)
func TestMapNewQuizzesInteractionStatic(t *testing.T) {
	cases := map[string]string{
		"matchInteraction":        UnifiedMatching,
		"orderInteraction":        UnifiedOrdering,
		"gapMatchInteraction":     UnifiedCategorization,
		"hotspotInteraction":      UnifiedHotSpot,
		"extendedTextInteraction": UnifiedEssay,
		"uploadInteraction":       UnifiedFileUpload,
		"inlineChoiceInteraction": UnifiedMultipleDropdown,
	}
	for inter, want := range cases {
		got, ok := MapNewQuizzesInteraction(inter)
		if !ok || got != want {
			t.Errorf("%s: want %s, got %s (ok=%v)", inter, want, got, ok)
		}
	}
	if _, ok := MapNewQuizzesInteraction("imaginaryInteraction"); ok {
		t.Error("unknown interaction should not be ok")
	}
}
