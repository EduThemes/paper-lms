package qti

import (
	"context"
	"path/filepath"
	"testing"
)

// TestImportClassicIMSCC zips the classic_qti21 fixture directory into a
// .imscc bundle, hands it to Importer.ImportIMSCC, and asserts:
//   - dialect is Classic
//   - all 12 distinct item types are imported
//   - the bank-1 fixture is imported with 3 items
//   - the section-with-sourcebank_ref produces a bank reference question
//   - zero errors
func TestImportClassicIMSCC(t *testing.T) {
	dir := filepath.Join("testdata", "fixtures", "classic_qti21")
	zipBytes := makeIMSCCFromDir(t, dir)
	path := writeTempZip(t, zipBytes)

	imp := NewImporter()
	result, err := imp.ImportIMSCC(context.Background(), path, 42)
	if err != nil {
		t.Fatalf("ImportIMSCC: %v", err)
	}

	if result.Dialect != DialectClassic {
		t.Errorf("want dialect Classic, got %s", result.Dialect)
	}
	if len(result.Errors) > 0 {
		t.Errorf("unexpected errors: %+v", result.Errors)
	}
	if len(result.Quizzes) != 1 {
		t.Fatalf("want 1 quiz, got %d", len(result.Quizzes))
	}
	quiz := result.Quizzes[0]
	if quiz.Title != "Classic Sample Quiz" {
		t.Errorf("want title 'Classic Sample Quiz', got %q", quiz.Title)
	}

	// 12 typed questions + 1 bank-reference question = 13.
	if len(quiz.Questions) != 13 {
		t.Errorf("want 13 questions, got %d", len(quiz.Questions))
		for i, q := range quiz.Questions {
			t.Logf("Q%d: type=%s text=%q", i, q.QuestionType, truncate(q.QuestionText, 40))
		}
	}

	// Each of the 12 Canvas Classic types should appear exactly once.
	wantTypes := map[string]int{
		UnifiedMultipleChoice:       1,
		UnifiedMultipleAnswer:       1,
		UnifiedTrueFalse:            1,
		UnifiedShortAnswer:          1,
		UnifiedFillInMultipleBlanks: 1,
		UnifiedMultipleDropdown:     1,
		UnifiedMatching:             1,
		UnifiedNumerical:            1,
		UnifiedFormula:              1,
		UnifiedEssay:                1,
		UnifiedFileUpload:           1,
		UnifiedTextOnly:             1,
	}
	gotTypes := map[string]int{}
	bankRefs := 0
	for _, q := range quiz.Questions {
		if q.BankItemIdentifier != "" {
			bankRefs++
			continue
		}
		gotTypes[q.QuestionType]++
	}
	for typ, want := range wantTypes {
		if gotTypes[typ] != want {
			t.Errorf("type %s: want %d, got %d", typ, want, gotTypes[typ])
		}
	}
	if bankRefs != 1 {
		t.Errorf("want 1 bank reference, got %d", bankRefs)
	}

	// Bank assertions.
	if len(result.ItemBanks) != 1 {
		t.Fatalf("want 1 bank, got %d", len(result.ItemBanks))
	}
	bank := result.ItemBanks[0]
	if bank.Title != "Sample Bank" {
		t.Errorf("bank title: %q", bank.Title)
	}
	if len(bank.Items) != 3 {
		t.Errorf("want 3 bank items, got %d", len(bank.Items))
	}
}

// TestImportNewQuizzesIMSCC handles the NQ fixture. Assertions:
//   - dialect = NewQuizzes
//   - all 12 question files are parsed (one of each item type plus
//     duplicates for choiceInteraction sub-types — see fixture)
//   - the stimulus passage shows up in result.Stimuli
//   - questions in the stimulus section have StimulusIdentifier set
//   - zero errors
func TestImportNewQuizzesIMSCC(t *testing.T) {
	dir := filepath.Join("testdata", "fixtures", "newquizzes_qti22")
	zipBytes := makeIMSCCFromDir(t, dir)
	path := writeTempZip(t, zipBytes)

	imp := NewImporter()
	result, err := imp.ImportIMSCC(context.Background(), path, 7)
	if err != nil {
		t.Fatalf("ImportIMSCC: %v", err)
	}

	if result.Dialect != DialectNewQuizzes {
		t.Errorf("want dialect NewQuizzes, got %s", result.Dialect)
	}
	if len(result.Errors) > 0 {
		t.Errorf("unexpected errors: %+v", result.Errors)
	}
	if len(result.Quizzes) != 1 {
		t.Fatalf("want 1 quiz, got %d", len(result.Quizzes))
	}
	quiz := result.Quizzes[0]
	// 12 item files in the assessment.
	if len(quiz.Questions) != 12 {
		t.Errorf("want 12 questions, got %d", len(quiz.Questions))
		for i, q := range quiz.Questions {
			t.Logf("Q%d: type=%s stim=%q text=%q", i, q.QuestionType, q.StimulusIdentifier, truncate(q.QuestionText, 40))
		}
	}

	// Check we have at least one of each distinct NQ type.
	wantTypes := []string{
		UnifiedMultipleChoice, UnifiedMultipleAnswer, UnifiedTrueFalse,
		UnifiedFillInTheBlank, UnifiedNumerical, UnifiedEssay,
		UnifiedFileUpload, UnifiedMatching, UnifiedMultipleDropdown,
		UnifiedOrdering, UnifiedCategorization, UnifiedHotSpot,
	}
	got := map[string]bool{}
	for _, q := range quiz.Questions {
		got[q.QuestionType] = true
	}
	for _, typ := range wantTypes {
		if !got[typ] {
			t.Errorf("missing question type: %s", typ)
		}
	}

	// Stimulus: order/gap/hot questions sit in the stimulus section.
	if len(result.Stimuli) != 1 {
		t.Errorf("want 1 stimulus, got %d", len(result.Stimuli))
	}
	stimulusLinked := 0
	for _, q := range quiz.Questions {
		if q.StimulusIdentifier != "" {
			stimulusLinked++
		}
	}
	if stimulusLinked != 3 {
		t.Errorf("want 3 questions linked to stimulus, got %d", stimulusLinked)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
