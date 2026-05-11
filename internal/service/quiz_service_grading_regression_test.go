package service_test

// Wave 0 regression capture for the existing quiz auto-grader.
//
// PURPOSE
// -------
// Phase 5 adds 9 new quiz item types and a Canvas QTI importer. Before any
// of that lands, this file pins down what `(*QuizService).autoGrade` does
// TODAY for the 7 currently-supported question types. Every case here is
// asserting current behavior — including quirks and apparent bugs — so we
// have a tripwire if the unified quiz engine extension changes scoring for
// existing quizzes.
//
// METHODOLOGY
// -----------
// `autoGrade` is unexported, so each case drives a single-question, single-
// answer submission through the public `CompleteSubmission` entry point and
// asserts on `*submission.Score` plus `workflow_state` (`complete` when the
// type is auto-gradable, `pending_review` when the grader flags it for
// manual review).
//
// Cases tagged `// REGRESSION: current behavior — see Wave A planning
// notes` are quirks worth flagging in Wave A planning.

import (
	"context"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// gradingCase describes one regression scenario.
type gradingCase struct {
	name          string
	questionType  string
	points        float64
	answersJSON   string
	submitted     string
	wantScore     float64
	wantWorkflow  string // "complete" or "pending_review"
	note          string // optional human-readable comment about the quirk
}

func runGradingCase(t *testing.T, tc gradingCase) {
	t.Helper()

	questionRepo := new(mocks.MockQuizQuestionRepository)
	submissionRepo := new(mocks.MockQuizSubmissionRepository)
	answerRepo := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizService(quizRepo, questionRepo, submissionRepo, answerRepo)

	ctx := context.Background()

	startedAt := time.Now().Add(-1 * time.Minute)
	submission := &models.QuizSubmission{
		ID:            1,
		QuizID:        1,
		UserID:        10,
		Attempt:       1,
		StartedAt:     &startedAt,
		WorkflowState: "untaken",
	}

	pts := tc.points
	question := &models.QuizQuestion{
		ID:             100,
		QuizID:         1,
		QuestionType:   tc.questionType,
		QuestionText:   "regression: " + tc.name,
		PointsPossible: &pts,
		Answers:        tc.answersJSON,
	}

	answers := []models.QuizSubmissionAnswer{
		{
			ID:               1,
			QuizSubmissionID: 1,
			QuestionID:       100,
			Answer:           tc.submitted,
		},
	}

	submissionRepo.On("FindByID", ctx, uint(1)).Return(submission, nil)
	answerRepo.On("ListBySubmissionID", ctx, uint(1)).Return(answers, nil)
	questionRepo.On("FindByID", ctx, uint(100)).Return(question, nil)
	answerRepo.On("Update", ctx, mock.AnythingOfType("*models.QuizSubmissionAnswer")).Return(nil)
	submissionRepo.On("Update", ctx, submission).Return(nil)

	result, err := svc.CompleteSubmission(ctx, 1, 10)

	assert.NoError(t, err, tc.name)
	if assert.NotNil(t, result, tc.name) {
		if assert.NotNil(t, result.Score, tc.name) {
			assert.InDelta(t, tc.wantScore, *result.Score, 0.001, "score mismatch for %s (%s)", tc.name, tc.note)
		}
		assert.Equal(t, tc.wantWorkflow, result.WorkflowState, "workflow_state mismatch for %s (%s)", tc.name, tc.note)
	}

	submissionRepo.AssertExpectations(t)
	answerRepo.AssertExpectations(t)
	questionRepo.AssertExpectations(t)
}

// TestGradingRegression_MultipleChoice locks in multiple_choice scoring.
func TestGradingRegression_MultipleChoice(t *testing.T) {
	cases := []gradingCase{
		{
			name:         "happy_path_full_credit",
			questionType: "multiple_choice",
			points:       2.0,
			answersJSON:  `[{"id":"a1","text":"4","weight":100},{"id":"a2","text":"3","weight":0}]`,
			submitted:    "a1",
			wantScore:    2.0,
			wantWorkflow: "complete",
		},
		{
			name:         "wrong_choice_zero",
			questionType: "multiple_choice",
			points:       2.0,
			answersJSON:  `[{"id":"a1","text":"4","weight":100},{"id":"a2","text":"3","weight":0}]`,
			submitted:    "a2",
			wantScore:    0.0,
			wantWorkflow: "complete",
		},
		{
			name:         "empty_answer_zero",
			questionType: "multiple_choice",
			points:       2.0,
			answersJSON:  `[{"id":"a1","text":"4","weight":100}]`,
			submitted:    "",
			wantScore:    0.0,
			wantWorkflow: "complete",
		},
		{
			name:         "whitespace_only_zero",
			questionType: "multiple_choice",
			points:       2.0,
			answersJSON:  `[{"id":"a1","text":"4","weight":100}]`,
			submitted:    "   ",
			wantScore:    0.0,
			wantWorkflow: "complete",
		},
		{
			name:         "answer_id_with_surrounding_whitespace_trimmed",
			questionType: "multiple_choice",
			points:       2.0,
			answersJSON:  `[{"id":"a1","text":"4","weight":100}]`,
			submitted:    "  a1  ",
			wantScore:    2.0,
			wantWorkflow: "complete",
			note:         "submittedAnswer is TrimSpace'd before comparison to opt.ID",
		},
		{
			// REGRESSION: current behavior — see Wave A planning notes
			// MC answer ID comparison is case-sensitive (only the submission is
			// trimmed, not lower-cased). "A1" submitted against id "a1" scores 0.
			name:         "answer_id_match_is_case_sensitive",
			questionType: "multiple_choice",
			points:       2.0,
			answersJSON:  `[{"id":"a1","text":"4","weight":100}]`,
			submitted:    "A1",
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "QUIRK: multiple_choice ID match is case-sensitive",
		},
		{
			// REGRESSION: current behavior — see Wave A planning notes
			// Partial-credit MC works: weight 50 on a 4pt question yields 2.
			name:         "partial_credit_via_weight",
			questionType: "multiple_choice",
			points:       4.0,
			answersJSON:  `[{"id":"a1","text":"close","weight":50},{"id":"a2","text":"correct","weight":100}]`,
			submitted:    "a1",
			wantScore:    2.0,
			wantWorkflow: "complete",
			note:         "Partial credit supported via per-option weight field",
		},
		{
			// REGRESSION: current behavior — see Wave A planning notes
			// Score is rounded to 2 decimal places via math.Round(score*100)/100.
			name:         "partial_credit_rounds_to_two_decimals",
			questionType: "multiple_choice",
			points:       1.0,
			answersJSON:  `[{"id":"a1","text":"x","weight":33}]`,
			submitted:    "a1",
			wantScore:    0.33,
			wantWorkflow: "complete",
			note:         "Score rounded to 2 decimal places",
		},
		{
			// REGRESSION: current behavior — see Wave A planning notes
			// Malformed Answers JSON makes the grader return gradable=false, so
			// the question is treated as needing manual review (NOT a panic, NOT
			// auto-zero with workflow_state=complete).
			name:         "malformed_answers_json_triggers_pending_review",
			questionType: "multiple_choice",
			points:       2.0,
			answersJSON:  `not valid json`,
			submitted:    "a1",
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "QUIRK: malformed Answers JSON flags submission for manual review",
		},
		{
			// REGRESSION: current behavior — see Wave A planning notes
			// Option with weight 0 never scores — even if its id is submitted.
			name:         "zero_weight_option_scores_zero",
			questionType: "multiple_choice",
			points:       2.0,
			answersJSON:  `[{"id":"a1","text":"distractor","weight":0}]`,
			submitted:    "a1",
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "Weight must be > 0 to score (not >= 0)",
		},
		{
			// REGRESSION: current behavior — see Wave A planning notes
			// Negative weights are filtered out (weight > 0 check).
			name:         "negative_weight_option_scores_zero",
			questionType: "multiple_choice",
			points:       2.0,
			answersJSON:  `[{"id":"a1","text":"penalty","weight":-50}]`,
			submitted:    "a1",
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "Negative weights cannot deduct points",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) { runGradingCase(t, tc) })
	}
}

// TestGradingRegression_TrueFalse — true_false shares the multiple_choice
// code path; verify the dispatch.
func TestGradingRegression_TrueFalse(t *testing.T) {
	cases := []gradingCase{
		{
			name:         "true_correct",
			questionType: "true_false",
			points:       1.0,
			answersJSON:  `[{"id":"t1","text":"True","weight":100},{"id":"t2","text":"False","weight":0}]`,
			submitted:    "t1",
			wantScore:    1.0,
			wantWorkflow: "complete",
		},
		{
			name:         "false_chosen_when_true_correct",
			questionType: "true_false",
			points:       1.0,
			answersJSON:  `[{"id":"t1","text":"True","weight":100},{"id":"t2","text":"False","weight":0}]`,
			submitted:    "t2",
			wantScore:    0.0,
			wantWorkflow: "complete",
		},
		{
			name:         "empty_answer_zero",
			questionType: "true_false",
			points:       1.0,
			answersJSON:  `[{"id":"t1","text":"True","weight":100}]`,
			submitted:    "",
			wantScore:    0.0,
			wantWorkflow: "complete",
		},
		{
			name:         "malformed_json_pending_review",
			questionType: "true_false",
			points:       1.0,
			answersJSON:  `{bogus}`,
			submitted:    "t1",
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "QUIRK: malformed JSON flags pending_review (same as MC)",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) { runGradingCase(t, tc) })
	}
}

// TestGradingRegression_ShortAnswer — text match, case-insensitive, trimmed.
func TestGradingRegression_ShortAnswer(t *testing.T) {
	cases := []gradingCase{
		{
			name:         "exact_match_full_credit",
			questionType: "short_answer",
			points:       2.0,
			answersJSON:  `[{"id":"s1","text":"four","weight":100}]`,
			submitted:    "four",
			wantScore:    2.0,
			wantWorkflow: "complete",
		},
		{
			// REGRESSION: current behavior — see Wave A planning notes
			// short_answer comparison is case-insensitive on BOTH sides
			// (option text and submitted answer are ToLower'd + TrimSpace'd).
			name:         "case_insensitive_match",
			questionType: "short_answer",
			points:       2.0,
			answersJSON:  `[{"id":"s1","text":"Four","weight":100}]`,
			submitted:    "FOUR",
			wantScore:    2.0,
			wantWorkflow: "complete",
			note:         "QUIRK: short_answer is case-insensitive (unlike MC IDs)",
		},
		{
			name:         "leading_trailing_whitespace_trimmed",
			questionType: "short_answer",
			points:       2.0,
			answersJSON:  `[{"id":"s1","text":"four","weight":100}]`,
			submitted:    "   four   ",
			wantScore:    2.0,
			wantWorkflow: "complete",
		},
		{
			// REGRESSION: current behavior — see Wave A planning notes
			// Internal whitespace is NOT collapsed — "four square" != "four  square".
			name:         "internal_whitespace_not_normalized",
			questionType: "short_answer",
			points:       2.0,
			answersJSON:  `[{"id":"s1","text":"four square","weight":100}]`,
			submitted:    "four  square",
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "QUIRK: internal whitespace is preserved (only outer Trim)",
		},
		{
			name:         "wrong_answer_zero",
			questionType: "short_answer",
			points:       2.0,
			answersJSON:  `[{"id":"s1","text":"four","weight":100}]`,
			submitted:    "five",
			wantScore:    0.0,
			wantWorkflow: "complete",
		},
		{
			name:         "empty_answer_zero",
			questionType: "short_answer",
			points:       2.0,
			answersJSON:  `[{"id":"s1","text":"four","weight":100}]`,
			submitted:    "",
			wantScore:    0.0,
			wantWorkflow: "complete",
		},
		{
			name:         "multiple_accepted_answers_first_match_wins",
			questionType: "short_answer",
			points:       2.0,
			answersJSON:  `[{"id":"s1","text":"four","weight":100},{"id":"s2","text":"IV","weight":100},{"id":"s3","text":"4","weight":100}]`,
			submitted:    "IV",
			wantScore:    2.0,
			wantWorkflow: "complete",
		},
		{
			// REGRESSION: current behavior — see Wave A planning notes
			// Partial-credit short_answer works via weight field, like MC.
			name:         "partial_credit_via_weight",
			questionType: "short_answer",
			points:       4.0,
			answersJSON:  `[{"id":"s1","text":"misspelled","weight":50},{"id":"s2","text":"correct","weight":100}]`,
			submitted:    "misspelled",
			wantScore:    2.0,
			wantWorkflow: "complete",
			note:         "Partial credit supported for short_answer",
		},
		{
			name:         "malformed_json_pending_review",
			questionType: "short_answer",
			points:       2.0,
			answersJSON:  `nope`,
			submitted:    "four",
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "QUIRK: malformed JSON flags pending_review",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) { runGradingCase(t, tc) })
	}
}

// TestGradingRegression_Numerical — exact string match on the .text field
// (despite a code comment mentioning a "margin" tolerance, none is applied).
func TestGradingRegression_Numerical(t *testing.T) {
	cases := []gradingCase{
		{
			name:         "exact_integer_match",
			questionType: "numerical_question",
			points:       3.0,
			answersJSON:  `[{"id":"n1","text":"42","weight":100}]`,
			submitted:    "42",
			wantScore:    3.0,
			wantWorkflow: "complete",
		},
		{
			name:         "wrong_integer_zero",
			questionType: "numerical_question",
			points:       3.0,
			answersJSON:  `[{"id":"n1","text":"42","weight":100}]`,
			submitted:    "41",
			wantScore:    0.0,
			wantWorkflow: "complete",
		},
		{
			// REGRESSION: current behavior — see Wave A planning notes
			// Numerical grader does an exact STRING match. "42" != "42.0".
			name:         "string_match_no_numeric_normalization_42_vs_42_0",
			questionType: "numerical_question",
			points:       3.0,
			answersJSON:  `[{"id":"n1","text":"42","weight":100}]`,
			submitted:    "42.0",
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "QUIRK: numerical grader is string-equality; 42 != 42.0",
		},
		{
			// CHANGED IN WAVE A1: Was previously routed to pending_review (bug). Now auto-grades per-pair. See PATCH.md.
			// Bug 1B fixed: when `margin` is set, the numerical grader parses
			// both sides as float64 and accepts the answer if |user-correct|
			// is within tolerance. 42.1 vs 42 ±0.5 → accepted, full credit.
			name:         "no_tolerance_applied_off_by_point_one",
			questionType: "numerical_question",
			points:       3.0,
			answersJSON:  `[{"id":"n1","text":"42","margin":"0.5","weight":100}]`,
			submitted:    "42.1",
			wantScore:    3.0,
			wantWorkflow: "complete",
			note:         "FIXED IN WAVE A1: margin tolerance is now respected",
		},
		{
			// Legacy preservation: when `margin` is empty/missing the grader
			// falls back to string equality, matching pre-Wave-A1 behavior so
			// existing quizzes are not silently re-scored.
			name:         "no_margin_falls_back_to_string_equality",
			questionType: "numerical_question",
			points:       3.0,
			answersJSON:  `[{"id":"n1","text":"42","weight":100}]`,
			submitted:    "42.0",
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "Legacy behavior preserved: no margin → exact string match",
		},
		{
			// Percent-margin semantics: "5%" of 200 = ±10, so 205 is accepted.
			name:         "percent_margin_accepts_within_band",
			questionType: "numerical_question",
			points:       2.0,
			answersJSON:  `[{"id":"n1","text":"200","margin":"5%","weight":100}]`,
			submitted:    "205",
			wantScore:    2.0,
			wantWorkflow: "complete",
			note:         "Percent margin: 5%% of 200 = ±10",
		},
		{
			name:         "negative_number_correct",
			questionType: "numerical_question",
			points:       2.0,
			answersJSON:  `[{"id":"n1","text":"-7","weight":100}]`,
			submitted:    "-7",
			wantScore:    2.0,
			wantWorkflow: "complete",
		},
		{
			name:         "negative_number_wrong_sign",
			questionType: "numerical_question",
			points:       2.0,
			answersJSON:  `[{"id":"n1","text":"-7","weight":100}]`,
			submitted:    "7",
			wantScore:    0.0,
			wantWorkflow: "complete",
		},
		{
			name:         "float_exact_match",
			questionType: "numerical_question",
			points:       2.0,
			answersJSON:  `[{"id":"n1","text":"3.14","weight":100}]`,
			submitted:    "3.14",
			wantScore:    2.0,
			wantWorkflow: "complete",
		},
		{
			name:         "whitespace_trimmed_before_match",
			questionType: "numerical_question",
			points:       2.0,
			answersJSON:  `[{"id":"n1","text":"42","weight":100}]`,
			submitted:    "  42  ",
			wantScore:    2.0,
			wantWorkflow: "complete",
		},
		{
			name:         "empty_answer_zero",
			questionType: "numerical_question",
			points:       2.0,
			answersJSON:  `[{"id":"n1","text":"42","weight":100}]`,
			submitted:    "",
			wantScore:    0.0,
			wantWorkflow: "complete",
		},
		{
			name:         "malformed_json_pending_review",
			questionType: "numerical_question",
			points:       2.0,
			answersJSON:  `nope`,
			submitted:    "42",
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "QUIRK: malformed JSON flags pending_review",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) { runGradingCase(t, tc) })
	}
}

// TestGradingRegression_Essay — never auto-graded; always pending_review,
// always 0 points until an instructor grades.
func TestGradingRegression_Essay(t *testing.T) {
	cases := []gradingCase{
		{
			name:         "essay_long_answer_flagged_for_review",
			questionType: "essay",
			points:       5.0,
			answersJSON:  `[]`,
			submitted:    "An essay response goes here.",
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "Essays are never auto-graded",
		},
		{
			name:         "essay_empty_response_still_pending_review",
			questionType: "essay",
			points:       5.0,
			answersJSON:  `[]`,
			submitted:    "",
			wantScore:    0.0,
			wantWorkflow: "pending_review",
		},
		{
			name:         "essay_malformed_answers_still_pending_review",
			questionType: "essay",
			points:       5.0,
			answersJSON:  `not json`,
			submitted:    "response",
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "Essay grader never parses Answers JSON, so malformed JSON is irrelevant",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) { runGradingCase(t, tc) })
	}
}

// TestGradingRegression_Matching — matching is currently NOT auto-graded.
// REGRESSION: current behavior — see Wave A planning notes.
// Per-pair scoring is not implemented; the entire item is flagged for manual
// review and scores 0 until graded. Once we land the unified engine, this
// is one of the highest-value upgrades.
func TestGradingRegression_Matching(t *testing.T) {
	cases := []gradingCase{
		{
			// CHANGED IN WAVE A1: Was previously routed to pending_review (bug). Now auto-grades per-pair. See PATCH.md.
			// Bug 2B fixed. Question JSON: [{left, right_id}]. Submission JSON:
			// [{left, right_id}]. Score = points × (correct pairs / total pairs).
			name:         "matching_correct_pairs_auto_graded",
			questionType: "matching",
			points:       4.0,
			answersJSON:  `[{"left":"apple","right_id":"r1","weight":100},{"left":"banana","right_id":"r2","weight":100}]`,
			submitted:    `[{"left":"apple","right_id":"r1"},{"left":"banana","right_id":"r2"}]`,
			wantScore:    4.0,
			wantWorkflow: "complete",
			note:         "FIXED IN WAVE A1: matching now scores per-pair",
		},
		{
			// Legacy preservation: pre-Wave-A1 matching items used the old
			// answers JSON shape ({"id","text":"left=right"}) which is NOT
			// parseable by the new per-pair grader. Those items still route
			// to pending_review so historical submissions are preserved
			// (until they're re-authored with the new shape).
			name:         "matching_legacy_shape_still_pending_review",
			questionType: "matching",
			points:       4.0,
			answersJSON:  `[{"id":"m1","text":"left=right","weight":100}]`,
			submitted:    `{"m1":"right"}`,
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "Legacy quiz JSON shape preserved as pending_review",
		},
		{
			name:         "matching_empty_answer_pending_review",
			questionType: "matching",
			points:       4.0,
			answersJSON:  `[]`,
			submitted:    "",
			wantScore:    0.0,
			wantWorkflow: "pending_review",
		},
		{
			name:         "matching_partial_pairs_pending_review",
			questionType: "matching",
			points:       4.0,
			answersJSON:  `[{"id":"m1","text":"a=1","weight":100},{"id":"m2","text":"b=2","weight":100}]`,
			submitted:    `{"m1":"1","m2":"wrong"}`,
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "QUIRK: no per-pair scoring; whole matching item routes to manual review",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) { runGradingCase(t, tc) })
	}
}

// TestGradingRegression_FillInMultipleBlanks — like matching, currently NOT
// auto-graded. REGRESSION: current behavior — see Wave A planning notes.
func TestGradingRegression_FillInMultipleBlanks(t *testing.T) {
	cases := []gradingCase{
		{
			// CHANGED IN WAVE A1: Was previously routed to pending_review (bug). Now auto-grades per-pair. See PATCH.md.
			// Bug 2B fixed. Question JSON: {blank_id: [accepted_answers]}.
			// Submission JSON: {blank_id: user_answer}. Match is
			// case-insensitive + TrimSpace'd per blank.
			name:         "fimb_correct_blanks_auto_graded",
			questionType: "fill_in_multiple_blanks",
			points:       6.0,
			answersJSON:  `{"color":["red","crimson"],"animal":["cat"]}`,
			submitted:    `{"color":"RED","animal":"cat"}`,
			wantScore:    6.0,
			wantWorkflow: "complete",
			note:         "FIXED IN WAVE A1: FIMB now scores per-blank",
		},
		{
			// Legacy preservation: pre-Wave-A1 FIMB items used the old
			// answers JSON shape ([{id,text:"color=red"}…]) which is NOT
			// parseable by the new map-based grader, so they remain in
			// pending_review until re-authored.
			name:         "fimb_legacy_shape_still_pending_review",
			questionType: "fill_in_multiple_blanks",
			points:       6.0,
			answersJSON:  `[{"id":"b1","text":"color=red","weight":100},{"id":"b2","text":"animal=cat","weight":100}]`,
			submitted:    `{"color":"red","animal":"cat"}`,
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "Legacy quiz JSON shape preserved as pending_review",
		},
		{
			name:         "fimb_empty_answer_pending_review",
			questionType: "fill_in_multiple_blanks",
			points:       6.0,
			answersJSON:  `[]`,
			submitted:    "",
			wantScore:    0.0,
			wantWorkflow: "pending_review",
		},
		{
			name:         "fimb_partial_blanks_pending_review",
			questionType: "fill_in_multiple_blanks",
			points:       6.0,
			answersJSON:  `[{"id":"b1","text":"color=red","weight":100},{"id":"b2","text":"animal=cat","weight":100}]`,
			submitted:    `{"color":"red","animal":"dog"}`,
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "QUIRK: no per-blank scoring; whole FIMB item routes to manual review",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) { runGradingCase(t, tc) })
	}
}

// TestGradingRegression_DefaultPointsPossible — if PointsPossible is nil on
// the question, autoGrade falls back to 1.0. Lock this in.
func TestGradingRegression_DefaultPointsPossible(t *testing.T) {
	questionRepo := new(mocks.MockQuizQuestionRepository)
	submissionRepo := new(mocks.MockQuizSubmissionRepository)
	answerRepo := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizService(quizRepo, questionRepo, submissionRepo, answerRepo)

	ctx := context.Background()

	startedAt := time.Now().Add(-1 * time.Minute)
	submission := &models.QuizSubmission{
		ID:            1,
		QuizID:        1,
		UserID:        10,
		Attempt:       1,
		StartedAt:     &startedAt,
		WorkflowState: "untaken",
	}

	question := &models.QuizQuestion{
		ID:             100,
		QuizID:         1,
		QuestionType:   "multiple_choice",
		QuestionText:   "Default points",
		PointsPossible: nil, // <-- key under test
		Answers:        `[{"id":"a1","text":"x","weight":100}]`,
	}

	answers := []models.QuizSubmissionAnswer{
		{ID: 1, QuizSubmissionID: 1, QuestionID: 100, Answer: "a1"},
	}

	submissionRepo.On("FindByID", ctx, uint(1)).Return(submission, nil)
	answerRepo.On("ListBySubmissionID", ctx, uint(1)).Return(answers, nil)
	questionRepo.On("FindByID", ctx, uint(100)).Return(question, nil)
	answerRepo.On("Update", ctx, mock.AnythingOfType("*models.QuizSubmissionAnswer")).Return(nil)
	submissionRepo.On("Update", ctx, submission).Return(nil)

	result, err := svc.CompleteSubmission(ctx, 1, 10)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Score)
	assert.InDelta(t, 1.0, *result.Score, 0.001, "nil PointsPossible should default to 1.0 inside autoGrade")
	assert.Equal(t, "complete", result.WorkflowState)
}

// TestGradingRegression_UnknownQuestionType — an unrecognized type returns
// gradable=false (no panic, no auto-score). REGRESSION: current behavior —
// see Wave A planning notes. After Wave A adds the 9 new types, this test
// will need its "unknown" sentinel updated to a still-unrecognized value.
func TestGradingRegression_UnknownQuestionType(t *testing.T) {
	questionRepo := new(mocks.MockQuizQuestionRepository)
	submissionRepo := new(mocks.MockQuizSubmissionRepository)
	answerRepo := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizService(quizRepo, questionRepo, submissionRepo, answerRepo)

	ctx := context.Background()

	startedAt := time.Now().Add(-1 * time.Minute)
	submission := &models.QuizSubmission{
		ID:            1,
		QuizID:        1,
		UserID:        10,
		Attempt:       1,
		StartedAt:     &startedAt,
		WorkflowState: "untaken",
	}

	pts := 3.0
	question := &models.QuizQuestion{
		ID:             100,
		QuizID:         1,
		QuestionType:   "definitely_not_a_real_type",
		QuestionText:   "Sentinel for unknown-type fallback path",
		PointsPossible: &pts,
		Answers:        `[]`,
	}

	answers := []models.QuizSubmissionAnswer{
		{ID: 1, QuizSubmissionID: 1, QuestionID: 100, Answer: "anything"},
	}

	submissionRepo.On("FindByID", ctx, uint(1)).Return(submission, nil)
	answerRepo.On("ListBySubmissionID", ctx, uint(1)).Return(answers, nil)
	questionRepo.On("FindByID", ctx, uint(100)).Return(question, nil)
	answerRepo.On("Update", ctx, mock.AnythingOfType("*models.QuizSubmissionAnswer")).Return(nil)
	submissionRepo.On("Update", ctx, submission).Return(nil)

	result, err := svc.CompleteSubmission(ctx, 1, 10)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Score)
	assert.InDelta(t, 0.0, *result.Score, 0.001)
	assert.Equal(t, "pending_review", result.WorkflowState, "unknown types should route to manual review")
}
