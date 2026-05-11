package service_test

// Wave A1 grader tests for the 9 new question types and the 3 longstanding
// bug fixes (matching, fill_in_multiple_blanks, numerical margin).
//
// Style notes
// -----------
// - Single-question, single-answer harness driven through CompleteSubmission,
//   matching the Wave 0 regression suite. We assert on `*submission.Score`
//   and `WorkflowState` ("complete" vs "pending_review").
// - Each case has a one-line `note` explaining the scenario.
// - Coverage target: ≥90% line coverage on every new grader function. We hit
//   this by exercising happy / wrong / partial / malformed / empty / edge.

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

// newTypesCase is a single grading scenario for Wave A1 item types.
type newTypesCase struct {
	name         string
	questionType string
	points       float64
	answersJSON  string
	submitted    string
	wantScore    float64
	wantWorkflow string
	wantGradedVia *string // optional: assert the audit-trail stamp
	note         string
}

func strPtr(s string) *string { return &s }

func runNewTypesCase(t *testing.T, tc newTypesCase) {
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
		QuestionText:   "wave A1: " + tc.name,
		PointsPossible: &pts,
		Answers:        tc.answersJSON,
	}

	// Capture the updated answer so we can assert on GradedVia.
	var updatedAnswer *models.QuizSubmissionAnswer
	answers := []models.QuizSubmissionAnswer{
		{ID: 1, QuizSubmissionID: 1, QuestionID: 100, Answer: tc.submitted},
	}

	submissionRepo.On("FindByID", ctx, uint(1)).Return(submission, nil)
	answerRepo.On("ListBySubmissionID", ctx, uint(1)).Return(answers, nil)
	questionRepo.On("FindByID", ctx, uint(100)).Return(question, nil)
	answerRepo.On("Update", ctx, mock.AnythingOfType("*models.QuizSubmissionAnswer")).
		Run(func(args mock.Arguments) {
			updatedAnswer = args.Get(1).(*models.QuizSubmissionAnswer)
		}).
		Return(nil)
	submissionRepo.On("Update", ctx, submission).Return(nil)

	result, err := svc.CompleteSubmission(ctx, 1, 10)

	assert.NoError(t, err, tc.name)
	if assert.NotNil(t, result, tc.name) {
		if assert.NotNil(t, result.Score, tc.name) {
			assert.InDelta(t, tc.wantScore, *result.Score, 0.001, "score mismatch for %s (%s)", tc.name, tc.note)
		}
		assert.Equal(t, tc.wantWorkflow, result.WorkflowState, "workflow_state mismatch for %s (%s)", tc.name, tc.note)
	}

	if tc.wantGradedVia != nil {
		if assert.NotNil(t, updatedAnswer, "expected an Update on the answer for %s", tc.name) &&
			assert.NotNil(t, updatedAnswer.GradedVia, "GradedVia should be stamped for %s", tc.name) {
			assert.Equal(t, *tc.wantGradedVia, *updatedAnswer.GradedVia, "GradedVia mismatch for %s", tc.name)
		}
	}

	submissionRepo.AssertExpectations(t)
	answerRepo.AssertExpectations(t)
	questionRepo.AssertExpectations(t)
}

// TestMultipleAnswer — partial credit with negative scoring, floored at 0.
func TestMultipleAnswer(t *testing.T) {
	cases := []newTypesCase{
		{
			name:         "all_correct_full_credit",
			questionType: "multiple_answer",
			points:       4.0,
			answersJSON:  `[{"id":"a1","weight":100},{"id":"a2","weight":100},{"id":"a3","weight":0}]`,
			submitted:    `["a1","a2"]`,
			wantScore:    4.0,
			wantWorkflow: "complete",
			wantGradedVia: strPtr("auto"),
			note:         "both correct ids selected → full credit",
		},
		{
			name:         "partial_one_of_two",
			questionType: "multiple_answer",
			points:       4.0,
			answersJSON:  `[{"id":"a1","weight":100},{"id":"a2","weight":100}]`,
			submitted:    `["a1"]`,
			wantScore:    2.0,
			wantWorkflow: "complete",
			note:         "1 correct, 0 wrong → 1*(4/2) = 2",
		},
		{
			name:         "incorrect_selection_deducts",
			questionType: "multiple_answer",
			points:       4.0,
			answersJSON:  `[{"id":"a1","weight":100},{"id":"a2","weight":100},{"id":"d1","weight":0}]`,
			submitted:    `["a1","d1"]`,
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "1 right (+2) − 1 wrong (−2) = 0",
		},
		{
			name:         "all_wrong_floors_at_zero",
			questionType: "multiple_answer",
			points:       4.0,
			answersJSON:  `[{"id":"a1","weight":100},{"id":"a2","weight":100},{"id":"d1","weight":0},{"id":"d2","weight":0}]`,
			submitted:    `["d1","d2"]`,
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "−4 floored at 0",
		},
		{
			name:         "empty_array_zero",
			questionType: "multiple_answer",
			points:       4.0,
			answersJSON:  `[{"id":"a1","weight":100},{"id":"a2","weight":100}]`,
			submitted:    `[]`,
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "no selections → 0",
		},
		{
			name:         "empty_submission_string_zero",
			questionType: "multiple_answer",
			points:       4.0,
			answersJSON:  `[{"id":"a1","weight":100}]`,
			submitted:    "",
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "empty submission treated as no selections",
		},
		{
			name:         "duplicate_selection_counted_once",
			questionType: "multiple_answer",
			points:       2.0,
			answersJSON:  `[{"id":"a1","weight":100}]`,
			submitted:    `["a1","a1","a1"]`,
			wantScore:    2.0,
			wantWorkflow: "complete",
			note:         "duplicates dedup'd before scoring",
		},
		{
			name:         "unknown_id_treated_as_wrong",
			questionType: "multiple_answer",
			points:       4.0,
			answersJSON:  `[{"id":"a1","weight":100},{"id":"a2","weight":100}]`,
			submitted:    `["a1","ghost"]`,
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "+2 -2 = 0; selecting an unknown id deducts",
		},
		{
			name:         "malformed_submission_pending_review",
			questionType: "multiple_answer",
			points:       4.0,
			answersJSON:  `[{"id":"a1","weight":100}]`,
			submitted:    `not json`,
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "malformed submitted JSON → pending_review",
		},
		{
			name:         "malformed_answers_pending_review",
			questionType: "multiple_answer",
			points:       4.0,
			answersJSON:  `wat`,
			submitted:    `["a1"]`,
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "malformed answers JSON → pending_review",
		},
		{
			name:         "no_correct_options_zero_complete",
			questionType: "multiple_answer",
			points:       4.0,
			answersJSON:  `[{"id":"a1","weight":0}]`,
			submitted:    `["a1"]`,
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "no positive-weight options → 0 (gradable, no panic)",
		},
		{
			name:         "negative_weight_option_does_not_score",
			questionType: "multiple_answer",
			points:       2.0,
			answersJSON:  `[{"id":"a1","weight":-50},{"id":"a2","weight":100}]`,
			submitted:    `["a1"]`,
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "negative-weight option is filtered out (treated as distractor)",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) { runNewTypesCase(t, tc) })
	}
}

// TestMultipleDropdown — per-blank scoring.
func TestMultipleDropdown(t *testing.T) {
	cases := []newTypesCase{
		{
			name:         "all_blanks_correct",
			questionType: "multiple_dropdown",
			points:       6.0,
			answersJSON:  `[{"id":"o1","blank_id":"color","weight":100},{"id":"o2","blank_id":"color","weight":0},{"id":"o3","blank_id":"animal","weight":100},{"id":"o4","blank_id":"animal","weight":0}]`,
			submitted:    `{"color":"o1","animal":"o3"}`,
			wantScore:    6.0,
			wantWorkflow: "complete",
			note:         "both blanks correct",
		},
		{
			name:         "partial_one_blank_correct",
			questionType: "multiple_dropdown",
			points:       6.0,
			answersJSON:  `[{"id":"o1","blank_id":"color","weight":100},{"id":"o3","blank_id":"animal","weight":100},{"id":"o4","blank_id":"animal","weight":0}]`,
			submitted:    `{"color":"o1","animal":"o4"}`,
			wantScore:    3.0,
			wantWorkflow: "complete",
			note:         "1 of 2 blanks correct",
		},
		{
			name:         "missing_blank_zero",
			questionType: "multiple_dropdown",
			points:       6.0,
			answersJSON:  `[{"id":"o1","blank_id":"color","weight":100},{"id":"o3","blank_id":"animal","weight":100}]`,
			submitted:    `{"color":"o1"}`,
			wantScore:    3.0,
			wantWorkflow: "complete",
			note:         "missing blank counted as wrong",
		},
		{
			name:         "empty_submission_zero",
			questionType: "multiple_dropdown",
			points:       6.0,
			answersJSON:  `[{"id":"o1","blank_id":"color","weight":100}]`,
			submitted:    "",
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "empty submission = nothing matches",
		},
		{
			name:         "malformed_submission_pending_review",
			questionType: "multiple_dropdown",
			points:       6.0,
			answersJSON:  `[{"id":"o1","blank_id":"color","weight":100}]`,
			submitted:    `not json`,
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "malformed submitted JSON → pending_review",
		},
		{
			name:         "malformed_answers_pending_review",
			questionType: "multiple_dropdown",
			points:       6.0,
			answersJSON:  `not json`,
			submitted:    `{"color":"o1"}`,
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "malformed answers JSON → pending_review",
		},
		{
			name:         "no_blanks_defined_complete_zero",
			questionType: "multiple_dropdown",
			points:       6.0,
			answersJSON:  `[]`,
			submitted:    `{"color":"o1"}`,
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "no blank-tagged options → 0 but still gradable",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) { runNewTypesCase(t, tc) })
	}
}

// TestFillInTheBlank — single blank, case-insensitive trimmed.
func TestFillInTheBlank(t *testing.T) {
	cases := []newTypesCase{
		{
			name:         "exact_match_full_credit",
			questionType: "fill_in_the_blank",
			points:       2.0,
			answersJSON:  `[{"id":"a1","text":"Paris","weight":100}]`,
			submitted:    "Paris",
			wantScore:    2.0,
			wantWorkflow: "complete",
			note:         "happy path",
		},
		{
			name:         "case_insensitive_match",
			questionType: "fill_in_the_blank",
			points:       2.0,
			answersJSON:  `[{"id":"a1","text":"Paris","weight":100}]`,
			submitted:    "PARIS",
			wantScore:    2.0,
			wantWorkflow: "complete",
			note:         "case-insensitive",
		},
		{
			name:         "whitespace_trimmed",
			questionType: "fill_in_the_blank",
			points:       2.0,
			answersJSON:  `[{"id":"a1","text":"Paris","weight":100}]`,
			submitted:    "   paris  ",
			wantScore:    2.0,
			wantWorkflow: "complete",
			note:         "outer whitespace trimmed",
		},
		{
			name:         "multiple_accepted_answers",
			questionType: "fill_in_the_blank",
			points:       2.0,
			answersJSON:  `[{"id":"a1","text":"Paris","weight":100},{"id":"a2","text":"paris, france","weight":100}]`,
			submitted:    "Paris, France",
			wantScore:    2.0,
			wantWorkflow: "complete",
			note:         "second accepted answer matches",
		},
		{
			name:         "partial_credit_via_weight",
			questionType: "fill_in_the_blank",
			points:       4.0,
			answersJSON:  `[{"id":"a1","text":"Paris","weight":100},{"id":"a2","text":"Parris","weight":50}]`,
			submitted:    "parris",
			wantScore:    2.0,
			wantWorkflow: "complete",
			note:         "misspelled half-credit alias",
		},
		{
			name:         "wrong_answer_zero",
			questionType: "fill_in_the_blank",
			points:       2.0,
			answersJSON:  `[{"id":"a1","text":"Paris","weight":100}]`,
			submitted:    "London",
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "wrong answer",
		},
		{
			name:         "empty_answer_zero",
			questionType: "fill_in_the_blank",
			points:       2.0,
			answersJSON:  `[{"id":"a1","text":"Paris","weight":100}]`,
			submitted:    "",
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "empty submission",
		},
		{
			name:         "malformed_answers_pending_review",
			questionType: "fill_in_the_blank",
			points:       2.0,
			answersJSON:  `not json`,
			submitted:    "Paris",
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "malformed answers JSON",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) { runNewTypesCase(t, tc) })
	}
}

// TestFormula — re-uses numerical tolerance band.
func TestFormula(t *testing.T) {
	cases := []newTypesCase{
		{
			name:         "absolute_margin_within",
			questionType: "formula",
			points:       3.0,
			answersJSON:  `[{"id":"f1","text":"42","margin":"0.5","weight":100}]`,
			submitted:    "42.3",
			wantScore:    3.0,
			wantWorkflow: "complete",
			note:         "within ±0.5",
		},
		{
			name:         "absolute_margin_outside",
			questionType: "formula",
			points:       3.0,
			answersJSON:  `[{"id":"f1","text":"42","margin":"0.5","weight":100}]`,
			submitted:    "43",
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "outside ±0.5",
		},
		{
			name:         "percent_margin",
			questionType: "formula",
			points:       3.0,
			answersJSON:  `[{"id":"f1","text":"200","margin":"5%","weight":100}]`,
			submitted:    "210",
			wantScore:    3.0,
			wantWorkflow: "complete",
			note:         "5%% of 200 = ±10; 210 accepted",
		},
		{
			name:         "percent_margin_outside",
			questionType: "formula",
			points:       3.0,
			answersJSON:  `[{"id":"f1","text":"200","margin":"5%","weight":100}]`,
			submitted:    "211",
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "211 outside ±10",
		},
		{
			name:         "no_margin_falls_back_string_eq",
			questionType: "formula",
			points:       3.0,
			answersJSON:  `[{"id":"f1","text":"42","weight":100}]`,
			submitted:    "42",
			wantScore:    3.0,
			wantWorkflow: "complete",
			note:         "no margin → exact string match works",
		},
		{
			name:         "submitted_not_a_number",
			questionType: "formula",
			points:       3.0,
			answersJSON:  `[{"id":"f1","text":"42","margin":"0.5","weight":100}]`,
			submitted:    "forty-two",
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "non-numeric submission with margin → no match, no panic",
		},
		{
			name:         "malformed_answers_pending_review",
			questionType: "formula",
			points:       3.0,
			answersJSON:  `nope`,
			submitted:    "42",
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "malformed JSON",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) { runNewTypesCase(t, tc) })
	}
}

// TestFileUpload — never auto-graded; route to pending_review.
func TestFileUpload(t *testing.T) {
	cases := []newTypesCase{
		{
			name:         "file_upload_pending_review",
			questionType: "file_upload",
			points:       10.0,
			answersJSON:  `[]`,
			submitted:    `{"file_id":42}`,
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "file uploads always require manual review",
		},
		{
			name:         "file_upload_empty_still_pending_review",
			questionType: "file_upload",
			points:       10.0,
			answersJSON:  `[]`,
			submitted:    "",
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "even empty file_upload pends",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) { runNewTypesCase(t, tc) })
	}
}

// TestOrdering — score = points × (correct positions / total positions).
func TestOrdering(t *testing.T) {
	cases := []newTypesCase{
		{
			name:         "perfect_order_full_credit",
			questionType: "ordering",
			points:       4.0,
			answersJSON:  `[{"id":"a","weight":100},{"id":"b","weight":100},{"id":"c","weight":100},{"id":"d","weight":100}]`,
			submitted:    `["a","b","c","d"]`,
			wantScore:    4.0,
			wantWorkflow: "complete",
			note:         "all positions match",
		},
		{
			name:         "partial_two_of_four",
			questionType: "ordering",
			points:       4.0,
			answersJSON:  `[{"id":"a","weight":100},{"id":"b","weight":100},{"id":"c","weight":100},{"id":"d","weight":100}]`,
			submitted:    `["a","b","d","c"]`,
			wantScore:    2.0,
			wantWorkflow: "complete",
			note:         "2 of 4 positions correct",
		},
		{
			name:         "reversed_zero",
			questionType: "ordering",
			points:       4.0,
			answersJSON:  `[{"id":"a","weight":100},{"id":"b","weight":100},{"id":"c","weight":100},{"id":"d","weight":100}]`,
			submitted:    `["d","c","b","a"]`,
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "fully reversed: no positions match",
		},
		{
			name:         "short_submission_only_scores_matched",
			questionType: "ordering",
			points:       4.0,
			answersJSON:  `[{"id":"a","weight":100},{"id":"b","weight":100},{"id":"c","weight":100},{"id":"d","weight":100}]`,
			submitted:    `["a","b"]`,
			wantScore:    2.0,
			wantWorkflow: "complete",
			note:         "short submission: missing positions count as wrong",
		},
		{
			name:         "empty_submission_zero",
			questionType: "ordering",
			points:       4.0,
			answersJSON:  `[{"id":"a","weight":100},{"id":"b","weight":100}]`,
			submitted:    "",
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "empty submission",
		},
		{
			name:         "malformed_submission_pending_review",
			questionType: "ordering",
			points:       4.0,
			answersJSON:  `[{"id":"a","weight":100}]`,
			submitted:    `oops`,
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "malformed submitted JSON",
		},
		{
			name:         "malformed_answers_pending_review",
			questionType: "ordering",
			points:       4.0,
			answersJSON:  `nope`,
			submitted:    `["a"]`,
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "malformed answers JSON",
		},
		{
			name:         "no_positions_complete_zero",
			questionType: "ordering",
			points:       4.0,
			answersJSON:  `[]`,
			submitted:    `["a"]`,
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "no positions defined → 0",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) { runNewTypesCase(t, tc) })
	}
}

// TestCategorization — per-item bucket placement.
func TestCategorization(t *testing.T) {
	cases := []newTypesCase{
		{
			name:         "all_correct_full_credit",
			questionType: "categorization",
			points:       6.0,
			answersJSON:  `[{"id":"apple","right_id":"fruit","weight":100},{"id":"carrot","right_id":"veg","weight":100},{"id":"bagel","right_id":"bread","weight":100}]`,
			submitted:    `{"apple":"fruit","carrot":"veg","bagel":"bread"}`,
			wantScore:    6.0,
			wantWorkflow: "complete",
			note:         "all 3 placed correctly",
		},
		{
			name:         "partial_one_of_three",
			questionType: "categorization",
			points:       6.0,
			answersJSON:  `[{"id":"apple","right_id":"fruit","weight":100},{"id":"carrot","right_id":"veg","weight":100},{"id":"bagel","right_id":"bread","weight":100}]`,
			submitted:    `{"apple":"fruit","carrot":"fruit","bagel":"veg"}`,
			wantScore:    2.0,
			wantWorkflow: "complete",
			note:         "1 of 3 placed correctly",
		},
		{
			name:         "empty_submission_zero",
			questionType: "categorization",
			points:       6.0,
			answersJSON:  `[{"id":"apple","right_id":"fruit","weight":100}]`,
			submitted:    "",
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "empty submission",
		},
		{
			name:         "malformed_submission_pending_review",
			questionType: "categorization",
			points:       6.0,
			answersJSON:  `[{"id":"apple","right_id":"fruit","weight":100}]`,
			submitted:    `oops`,
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "malformed submitted JSON",
		},
		{
			name:         "malformed_answers_pending_review",
			questionType: "categorization",
			points:       6.0,
			answersJSON:  `oops`,
			submitted:    `{"apple":"fruit"}`,
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "malformed answers JSON",
		},
		{
			name:         "no_items_complete_zero",
			questionType: "categorization",
			points:       6.0,
			answersJSON:  `[]`,
			submitted:    `{"apple":"fruit"}`,
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "no items defined → 0",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) { runNewTypesCase(t, tc) })
	}
}

// TestHotSpot — point-in-rectangle, boundary inclusive.
func TestHotSpot(t *testing.T) {
	cases := []newTypesCase{
		{
			name:         "inside_rectangle_correct",
			questionType: "hot_spot",
			points:       2.0,
			answersJSON:  `[{"id":"h1","x":10,"y":10,"w":100,"h":50,"weight":100}]`,
			submitted:    `{"x":50,"y":30}`,
			wantScore:    2.0,
			wantWorkflow: "complete",
			note:         "click is inside the rect",
		},
		{
			name:         "outside_rectangle_zero",
			questionType: "hot_spot",
			points:       2.0,
			answersJSON:  `[{"id":"h1","x":10,"y":10,"w":100,"h":50,"weight":100}]`,
			submitted:    `{"x":200,"y":200}`,
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "click is outside",
		},
		{
			name:         "exact_top_left_boundary_correct",
			questionType: "hot_spot",
			points:       2.0,
			answersJSON:  `[{"id":"h1","x":10,"y":10,"w":100,"h":50,"weight":100}]`,
			submitted:    `{"x":10,"y":10}`,
			wantScore:    2.0,
			wantWorkflow: "complete",
			note:         "boundary inclusive at top-left",
		},
		{
			name:         "exact_bottom_right_boundary_correct",
			questionType: "hot_spot",
			points:       2.0,
			answersJSON:  `[{"id":"h1","x":10,"y":10,"w":100,"h":50,"weight":100}]`,
			submitted:    `{"x":110,"y":60}`,
			wantScore:    2.0,
			wantWorkflow: "complete",
			note:         "boundary inclusive at bottom-right (x+w, y+h)",
		},
		{
			name:         "multiple_rects_match_one",
			questionType: "hot_spot",
			points:       2.0,
			answersJSON:  `[{"id":"h1","x":0,"y":0,"w":10,"h":10,"weight":100},{"id":"h2","x":100,"y":100,"w":10,"h":10,"weight":100}]`,
			submitted:    `{"x":105,"y":105}`,
			wantScore:    2.0,
			wantWorkflow: "complete",
			note:         "second accepted rect matches",
		},
		{
			name:         "weight_zero_rect_does_not_score",
			questionType: "hot_spot",
			points:       2.0,
			answersJSON:  `[{"id":"h1","x":0,"y":0,"w":10,"h":10,"weight":0}]`,
			submitted:    `{"x":5,"y":5}`,
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "decoy rect with weight 0 doesn't credit",
		},
		{
			name:         "empty_submission_zero",
			questionType: "hot_spot",
			points:       2.0,
			answersJSON:  `[{"id":"h1","x":0,"y":0,"w":10,"h":10,"weight":100}]`,
			submitted:    "",
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "empty submission → 0",
		},
		{
			name:         "malformed_submission_pending_review",
			questionType: "hot_spot",
			points:       2.0,
			answersJSON:  `[{"id":"h1","x":0,"y":0,"w":10,"h":10,"weight":100}]`,
			submitted:    `nope`,
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "malformed submission",
		},
		{
			name:         "malformed_answers_pending_review",
			questionType: "hot_spot",
			points:       2.0,
			answersJSON:  `nope`,
			submitted:    `{"x":1,"y":1}`,
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "malformed answers",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) { runNewTypesCase(t, tc) })
	}
}

// TestTextOnly — always 0 points, never blocks submission.
func TestTextOnly(t *testing.T) {
	cases := []newTypesCase{
		{
			name:         "text_only_zero_and_complete",
			questionType: "text_only",
			points:       0.0,
			answersJSON:  `[]`,
			submitted:    "",
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "informational; never blocks",
		},
		{
			name:         "text_only_with_points_still_zero",
			questionType: "text_only",
			points:       5.0,
			answersJSON:  `[]`,
			submitted:    "",
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "even if points configured, text_only awards 0 and completes",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) { runNewTypesCase(t, tc) })
	}
}

// TestMatchingFix — bug 2B fix: per-pair scoring.
func TestMatchingFix(t *testing.T) {
	cases := []newTypesCase{
		{
			name:         "all_pairs_correct_full_credit",
			questionType: "matching",
			points:       4.0,
			answersJSON:  `[{"left":"apple","right_id":"r1","weight":100},{"left":"banana","right_id":"r2","weight":100}]`,
			submitted:    `[{"left":"apple","right_id":"r1"},{"left":"banana","right_id":"r2"}]`,
			wantScore:    4.0,
			wantWorkflow: "complete",
			wantGradedVia: strPtr("auto"),
			note:         "both pairs correct → full credit, graded_via=auto",
		},
		{
			name:         "half_pairs_correct",
			questionType: "matching",
			points:       4.0,
			answersJSON:  `[{"left":"apple","right_id":"r1","weight":100},{"left":"banana","right_id":"r2","weight":100}]`,
			submitted:    `[{"left":"apple","right_id":"r1"},{"left":"banana","right_id":"wrong"}]`,
			wantScore:    2.0,
			wantWorkflow: "complete",
			note:         "1 of 2 pairs correct",
		},
		{
			name:         "all_wrong_zero",
			questionType: "matching",
			points:       4.0,
			answersJSON:  `[{"left":"a","right_id":"r1","weight":100},{"left":"b","right_id":"r2","weight":100}]`,
			submitted:    `[{"left":"a","right_id":"wrong"},{"left":"b","right_id":"alsowrong"}]`,
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "0 of 2 → 0",
		},
		{
			name:         "missing_pair_counted_as_wrong",
			questionType: "matching",
			points:       4.0,
			answersJSON:  `[{"left":"a","right_id":"r1","weight":100},{"left":"b","right_id":"r2","weight":100}]`,
			submitted:    `[{"left":"a","right_id":"r1"}]`,
			wantScore:    2.0,
			wantWorkflow: "complete",
			note:         "missing pair: only matched ones credit",
		},
		{
			name:         "empty_submission_zero",
			questionType: "matching",
			points:       4.0,
			answersJSON:  `[{"left":"a","right_id":"r1","weight":100}]`,
			submitted:    "",
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "empty submission",
		},
		{
			name:         "malformed_submission_pending_review",
			questionType: "matching",
			points:       4.0,
			answersJSON:  `[{"left":"a","right_id":"r1","weight":100}]`,
			submitted:    `not json`,
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "malformed submission",
		},
		{
			name:         "malformed_answers_pending_review",
			questionType: "matching",
			points:       4.0,
			answersJSON:  `nope`,
			submitted:    `[]`,
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "malformed answers",
		},
		{
			name:         "legacy_shape_routes_to_pending_review",
			questionType: "matching",
			points:       4.0,
			answersJSON:  `[{"id":"m1","text":"a=1","weight":100}]`,
			submitted:    `{"m1":"1"}`,
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "legacy quiz JSON (no left field) → pending_review",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) { runNewTypesCase(t, tc) })
	}
}

// TestFillInMultipleBlanksFix — bug 2B fix: per-blank scoring.
func TestFillInMultipleBlanksFix(t *testing.T) {
	cases := []newTypesCase{
		{
			name:         "all_blanks_correct",
			questionType: "fill_in_multiple_blanks",
			points:       6.0,
			answersJSON:  `{"color":["red","crimson"],"animal":["cat","feline"]}`,
			submitted:    `{"color":"red","animal":"cat"}`,
			wantScore:    6.0,
			wantWorkflow: "complete",
			wantGradedVia: strPtr("auto"),
			note:         "both blanks correct → full credit, graded_via=auto",
		},
		{
			name:         "case_insensitive_match",
			questionType: "fill_in_multiple_blanks",
			points:       6.0,
			answersJSON:  `{"color":["red"],"animal":["cat"]}`,
			submitted:    `{"color":"RED","animal":"  cAt "}`,
			wantScore:    6.0,
			wantWorkflow: "complete",
			note:         "case-insensitive + trimmed",
		},
		{
			name:         "alternate_accepted_answer_matches",
			questionType: "fill_in_multiple_blanks",
			points:       2.0,
			answersJSON:  `{"color":["red","crimson","scarlet"]}`,
			submitted:    `{"color":"Scarlet"}`,
			wantScore:    2.0,
			wantWorkflow: "complete",
			note:         "third synonym matches",
		},
		{
			name:         "one_blank_wrong",
			questionType: "fill_in_multiple_blanks",
			points:       6.0,
			answersJSON:  `{"color":["red"],"animal":["cat"]}`,
			submitted:    `{"color":"red","animal":"dog"}`,
			wantScore:    3.0,
			wantWorkflow: "complete",
			note:         "half credit",
		},
		{
			name:         "missing_blank_zero",
			questionType: "fill_in_multiple_blanks",
			points:       6.0,
			answersJSON:  `{"color":["red"],"animal":["cat"]}`,
			submitted:    `{"color":"red"}`,
			wantScore:    3.0,
			wantWorkflow: "complete",
			note:         "missing animal blank scores 0 for that blank",
		},
		{
			name:         "empty_submission_zero",
			questionType: "fill_in_multiple_blanks",
			points:       6.0,
			answersJSON:  `{"color":["red"]}`,
			submitted:    "",
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "empty submission",
		},
		{
			name:         "malformed_submission_pending_review",
			questionType: "fill_in_multiple_blanks",
			points:       6.0,
			answersJSON:  `{"color":["red"]}`,
			submitted:    `oops`,
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "malformed submission",
		},
		{
			name:         "legacy_shape_routes_to_pending_review",
			questionType: "fill_in_multiple_blanks",
			points:       6.0,
			answersJSON:  `[{"id":"b1","text":"color=red","weight":100}]`,
			submitted:    `{"color":"red"}`,
			wantScore:    0.0,
			wantWorkflow: "pending_review",
			note:         "legacy quiz JSON shape → pending_review",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) { runNewTypesCase(t, tc) })
	}
}

// TestNumericalMarginFix — bug 1B fix.
func TestNumericalMarginFix(t *testing.T) {
	cases := []newTypesCase{
		{
			name:         "absolute_margin_within_band",
			questionType: "numerical_question",
			points:       3.0,
			answersJSON:  `[{"id":"n1","text":"42","margin":"0.5","weight":100}]`,
			submitted:    "42.4",
			wantScore:    3.0,
			wantWorkflow: "complete",
			wantGradedVia: strPtr("auto"),
			note:         "0.4 ≤ 0.5",
		},
		{
			name:         "absolute_margin_outside_band",
			questionType: "numerical_question",
			points:       3.0,
			answersJSON:  `[{"id":"n1","text":"42","margin":"0.5","weight":100}]`,
			submitted:    "42.6",
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "0.6 > 0.5",
		},
		{
			name:         "boundary_exact_margin_accepted",
			questionType: "numerical_question",
			points:       3.0,
			answersJSON:  `[{"id":"n1","text":"42","margin":"0.5","weight":100}]`,
			submitted:    "42.5",
			wantScore:    3.0,
			wantWorkflow: "complete",
			note:         "|user-correct| == margin → accepted (≤, not <)",
		},
		{
			name:         "percent_margin_within",
			questionType: "numerical_question",
			points:       3.0,
			answersJSON:  `[{"id":"n1","text":"200","margin":"5%","weight":100}]`,
			submitted:    "195",
			wantScore:    3.0,
			wantWorkflow: "complete",
			note:         "5%% of 200 = ±10; 195 ok",
		},
		{
			name:         "percent_margin_outside",
			questionType: "numerical_question",
			points:       3.0,
			answersJSON:  `[{"id":"n1","text":"200","margin":"5%","weight":100}]`,
			submitted:    "180",
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "180 outside ±10",
		},
		{
			name:         "negative_correct_with_margin",
			questionType: "numerical_question",
			points:       2.0,
			answersJSON:  `[{"id":"n1","text":"-7","margin":"0.5","weight":100}]`,
			submitted:    "-7.2",
			wantScore:    2.0,
			wantWorkflow: "complete",
			note:         "negative correct value uses abs() for percent base",
		},
		{
			name:         "margin_is_garbage_no_match",
			questionType: "numerical_question",
			points:       2.0,
			answersJSON:  `[{"id":"n1","text":"42","margin":"abc","weight":100}]`,
			submitted:    "42",
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "unparseable margin → no match (and no panic)",
		},
		{
			name:         "no_margin_string_eq_fallback",
			questionType: "numerical_question",
			points:       2.0,
			answersJSON:  `[{"id":"n1","text":"42","weight":100}]`,
			submitted:    "42",
			wantScore:    2.0,
			wantWorkflow: "complete",
			note:         "no margin → legacy string equality (backwards compat)",
		},
		{
			name:         "no_margin_42_vs_42_0_still_misses",
			questionType: "numerical_question",
			points:       2.0,
			answersJSON:  `[{"id":"n1","text":"42","weight":100}]`,
			submitted:    "42.0",
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "legacy quirk preserved: no margin → string equality",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) { runNewTypesCase(t, tc) })
	}
}
