package qti

// This file is the single source of truth for translating between Canvas
// item types and Paper LMS unified `question_type` values. Both parsers
// (parser_classic.go, parser_newquizzes.go) and the exporter
// (exporter.go) consult this table. The mapping_test.go coverage check
// asserts every Paper LMS unified type has at least one inbound mapping
// AND outbound mapping, so adding a new unified type without updating
// these tables breaks the build — by design.
//
// Why three separate tables (classic→unified, NQ→unified, unified→classic)
// instead of one bidirectional map?
//   - Canvas Classic and NQ both have items that map to the same unified
//     type (e.g. both have "multiple_choice"), so the reverse direction
//     isn't a function — there's no answer to "which Canvas type does
//     `multiple_choice` map to?" without picking a target dialect.
//   - The exporter currently only writes Canvas Classic (the universally
//     accepted target). If we later add NQ export we just add a
//     unifiedToNewQuizzes table; nothing else changes.

// Paper LMS unified question types. Kept in sync with the validTypes
// allowlist in internal/service/quiz_service.go (lines 108-126). If you
// add a new unified type, add it both there and here, and update both
// inbound mapping tables below — the mapping_test.go coverage check will
// fail loudly otherwise.
const (
	UnifiedMultipleChoice         = "multiple_choice"
	UnifiedMultipleAnswer         = "multiple_answer"
	UnifiedTrueFalse              = "true_false"
	UnifiedShortAnswer            = "short_answer"
	UnifiedEssay                  = "essay"
	UnifiedMatching               = "matching"
	UnifiedFillInMultipleBlanks   = "fill_in_multiple_blanks"
	UnifiedNumerical              = "numerical_question"
	UnifiedFormula                = "formula"
	UnifiedFileUpload             = "file_upload"
	UnifiedTextOnly               = "text_only"
	UnifiedFillInTheBlank         = "fill_in_the_blank"
	UnifiedMultipleDropdown       = "multiple_dropdown"
	UnifiedOrdering               = "ordering"
	UnifiedCategorization         = "categorization"
	UnifiedHotSpot                = "hot_spot"
)

// AllUnifiedTypes is the canonical list. The mapping coverage test reads
// this to ensure no type is forgotten.
var AllUnifiedTypes = []string{
	UnifiedMultipleChoice,
	UnifiedMultipleAnswer,
	UnifiedTrueFalse,
	UnifiedShortAnswer,
	UnifiedEssay,
	UnifiedMatching,
	UnifiedFillInMultipleBlanks,
	UnifiedNumerical,
	UnifiedFormula,
	UnifiedFileUpload,
	UnifiedTextOnly,
	UnifiedFillInTheBlank,
	UnifiedMultipleDropdown,
	UnifiedOrdering,
	UnifiedCategorization,
	UnifiedHotSpot,
}

// canvasClassicToUnified maps Canvas Classic QTI 2.1 `<fieldentry>` values
// of `question_type` to a Paper LMS unified type. Classic encodes the
// item type in `<qtimetadatafield><fieldlabel>question_type</fieldlabel>`
// rather than via the QTI interaction tag, which is unusual but it's
// how the real exports look.
var canvasClassicToUnified = map[string]string{
	"multiple_choice_question":         UnifiedMultipleChoice,
	"multiple_answers_question":        UnifiedMultipleAnswer,
	"true_false_question":              UnifiedTrueFalse,
	"short_answer_question":            UnifiedShortAnswer,
	"essay_question":                   UnifiedEssay,
	"matching_question":                UnifiedMatching,
	"fill_in_multiple_blanks_question": UnifiedFillInMultipleBlanks,
	"numerical_question":               UnifiedNumerical,
	"calculated_question":              UnifiedFormula,
	"file_upload_question":             UnifiedFileUpload,
	"text_only_question":               UnifiedTextOnly,
	"multiple_dropdowns_question":      UnifiedMultipleDropdown,
}

// newQuizzesInteractionToUnified maps the NQ interaction element name
// to a unified type. Most are 1:1, but a couple are ambiguous:
//
//   - "choiceInteraction" with maxChoices=1 is multiple_choice; with
//     maxChoices>1 is multiple_answer; with exactly 2 options whose
//     labels are "True"/"False" is true_false. The classifier function
//     ClassifyNewQuizzesChoice() handles this disambiguation.
//   - "textEntryInteraction" can be either short_answer (free text) or
//     numerical (when a tolerance is declared in the response
//     declaration). ClassifyNewQuizzesTextEntry() picks.
//   - "extendedTextInteraction" is always essay.
//
// The map below is the "obvious" cases; the classifier helpers cover the
// ambiguous ones and are invoked from parser_newquizzes.go.
var newQuizzesInteractionToUnified = map[string]string{
	"matchInteraction":         UnifiedMatching,
	"inlineChoiceInteraction":  UnifiedMultipleDropdown,
	"extendedTextInteraction":  UnifiedEssay,
	"uploadInteraction":        UnifiedFileUpload,
	"orderInteraction":         UnifiedOrdering,
	"gapMatchInteraction":      UnifiedCategorization,
	"hotspotInteraction":       UnifiedHotSpot,
}

// unifiedToCanvasClassic is the exporter direction. Every unified type
// needs an entry — the mapping coverage test enforces this.
var unifiedToCanvasClassic = map[string]string{
	UnifiedMultipleChoice:       "multiple_choice_question",
	UnifiedMultipleAnswer:       "multiple_answers_question",
	UnifiedTrueFalse:            "true_false_question",
	UnifiedShortAnswer:          "short_answer_question",
	UnifiedEssay:                "essay_question",
	UnifiedMatching:             "matching_question",
	UnifiedFillInMultipleBlanks: "fill_in_multiple_blanks_question",
	UnifiedNumerical:            "numerical_question",
	UnifiedFormula:              "calculated_question",
	UnifiedFileUpload:           "file_upload_question",
	UnifiedTextOnly:             "text_only_question",
	UnifiedMultipleDropdown:     "multiple_dropdowns_question",
	// Canvas Classic does not have native ordering / categorization /
	// hot_spot / fill_in_the_blank types. We export them as their
	// closest Classic equivalent so the bundle remains importable by
	// any Canvas instance — the round-trip will reimport them as the
	// best-fit Classic type, NOT the original NQ type. This is
	// documented in the roundtrip test.
	UnifiedOrdering:       "matching_question",            // ordering ≈ ordered matching
	UnifiedCategorization: "matching_question",            // categorization ≈ many-to-one matching
	UnifiedHotSpot:        "multiple_choice_question",     // hot_spot degrades to single-correct MC
	UnifiedFillInTheBlank: "short_answer_question",        // single-blank == short_answer
}

// MapCanvasClassicType returns the unified type for a Canvas Classic
// `question_type` metadata field. Unknown types return ("", false) and
// the caller should record an "unknown_item_type" warning.
func MapCanvasClassicType(canvasType string) (string, bool) {
	u, ok := canvasClassicToUnified[canvasType]
	return u, ok
}

// MapNewQuizzesInteraction returns the unified type for a NQ interaction
// element name when the mapping is unambiguous. For ambiguous interactions
// (choiceInteraction, textEntryInteraction) the parser uses its own
// classifier helpers — see parser_newquizzes.go.
func MapNewQuizzesInteraction(interaction string) (string, bool) {
	u, ok := newQuizzesInteractionToUnified[interaction]
	return u, ok
}

// MapUnifiedToCanvasClassic returns the Canvas Classic question_type
// value for an exporter pass. Every unified type has an entry; if you
// see "" it means the unified type allow-list grew without updating
// this table (the test will catch it).
func MapUnifiedToCanvasClassic(unified string) string {
	return unifiedToCanvasClassic[unified]
}

// ClassifyNewQuizzesChoice disambiguates a NQ `<choiceInteraction>`:
//   - 2 choices labeled True/False (case-insensitive) → true_false
//   - maxChoices >= 2 (or 0, which means "all") → multiple_answer
//   - otherwise → multiple_choice
//
// labels is the human-readable text of each `<simpleChoice>` in source
// order; maxChoices is the value of the `maxChoices` XML attribute (0
// when unset, meaning "unlimited" per the QTI 2.2 spec).
func ClassifyNewQuizzesChoice(labels []string, maxChoices int) string {
	if len(labels) == 2 {
		a := normalizeForTF(labels[0])
		b := normalizeForTF(labels[1])
		if (a == "true" && b == "false") || (a == "false" && b == "true") {
			return UnifiedTrueFalse
		}
	}
	if maxChoices == 0 || maxChoices > 1 {
		return UnifiedMultipleAnswer
	}
	return UnifiedMultipleChoice
}

// ClassifyNewQuizzesTextEntry disambiguates a NQ `<textEntryInteraction>`.
// hasNumericTolerance is true when the response declaration carries an
// `<equal toleranceMode="absolute">` or similar numeric matcher.
func ClassifyNewQuizzesTextEntry(hasNumericTolerance bool) string {
	if hasNumericTolerance {
		return UnifiedNumerical
	}
	return UnifiedFillInTheBlank
}

// normalizeForTF lowercases + trims a label for true/false comparison.
// Kept private — only the choice classifier needs it.
func normalizeForTF(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		out = append(out, r)
	}
	return string(out)
}
