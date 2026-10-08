package service

// Quiz auto-grading methods — the autoGrade dispatcher and every per-item-type
// grader (multiple choice, short answer, numerical, matching, fill-in-blanks,
// and the Wave A1 additions: multiple_answer, multiple_dropdown,
// fill_in_the_blank, formula, file_upload, ordering, categorization, hot_spot,
// text_only). Also owns the answerOption struct, the gradedViaAuto audit
// constant, and the numerical-margin helpers shared between gradeNumerical
// and gradeFormula.
//
// Wave 5 split (chore/wave5-split-quiz-blueprint): all functions are still
// methods on *QuizService (or package-level helpers in the service package),
// so the call surface from quiz_attempts_service.go is unchanged.

import (
	"bytes"
	"encoding/json"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// gradedViaAuto is the audit-trail value stamped on quiz_submission_answer
// rows that were scored by the auto-grader. Added in Wave A1 alongside the
// matching / fill_in_multiple_blanks cutover so legacy pending_review rows
// (which pre-date this constant) can be distinguished from newly-graded ones.
const gradedViaAuto = "auto"

// matchOptionsSentinel is the blank_id the Canvas importer stamps on the
// right-hand "match pool" entries of a matching question. Those entries are
// distractor/option rows, not left→right pairs, so the grader skips them.
const matchOptionsSentinel = "_match_options_"

// flexString is a string that also accepts JSON numbers (and null). Canvas
// exports — and the Canvas importer — serialize numeric fields such as the
// numerical `margin` or an exact value as JSON numbers, while Paper's own
// editor writes strings. Unmarshalling a number into a plain string field
// fails the WHOLE answers array, which made those questions ungradable.
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		*f = ""
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = flexString(s)
		return nil
	}
	if b[0] == 't' || b[0] == 'f' {
		*f = flexString(string(b))
		return nil
	}
	// Any other scalar (a JSON number) is kept verbatim.
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		// Objects / arrays: ignore rather than fail the whole question.
		*f = ""
		return nil
	}
	*f = flexString(n.String())
	return nil
}

func (f flexString) String() string { return string(f) }

// hotSpotRegion is one accepted rectangle authored by HotSpotEditor. All
// values are fractions (0..1) of the rendered image.
type hotSpotRegion struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type categorizationItem struct {
	ID string `json:"id"`
}

type categorizationBucket struct {
	ID      string   `json:"id"`
	ItemIDs []string `json:"item_ids"`
}

// answerOption represents one answer choice in a quiz question's Answers JSON field.
// Extended in Wave A1: the optional Margin field is now read by the numerical
// grader, and several new fields are consumed by the 9 new item-type graders.
type answerOption struct {
	ID       string     `json:"id"`
	Text     flexString `json:"text"`
	Comments string     `json:"comments"`
	Weight   float64    `json:"weight"`
	// Margin is the numerical tolerance for numerical_question / formula items.
	// A bare number is absolute tolerance ("0.5"); a trailing "%" is percentage
	// of the correct value (e.g. "5%" of 200 = ±10). Accepts JSON numbers.
	Margin flexString `json:"margin"`
	// Canvas numerical answer shapes (numerical_answer_type exact_answer /
	// range_answer / precision_answer).
	NumericalAnswerType string     `json:"numerical_answer_type"`
	Exact               flexString `json:"exact"`
	Start               flexString `json:"start"`
	End                 flexString `json:"end"`
	Approximate         flexString `json:"approximate"`
	Precision           flexString `json:"precision"`
	// BlankID identifies which blank an option belongs to (multiple_dropdown,
	// fill_in_multiple_blanks in the Canvas array shape).
	BlankID string `json:"blank_id"`
	// Coordinates for legacy absolute hot_spot accepted rectangles.
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"w"`
	Height float64 `json:"h"`
	// Regions is the HotSpotEditor shape: normalized rectangles stored on the
	// single config option.
	Regions []hotSpotRegion `json:"regions"`
	// RightID is the matching-pair right-hand option ID. MatchID is the
	// Canvas spelling of the same thing.
	RightID string `json:"right_id"`
	MatchID string `json:"match_id"`
	// Left is the matching-pair left-hand label (human readable).
	Left string `json:"left"`
	// Categorization editor shape: one config option carrying items and
	// buckets (each bucket lists the item ids that belong in it).
	Items   []categorizationItem   `json:"items"`
	Buckets []categorizationBucket `json:"buckets"`
	// Formula editor shape: fixed expected value + tolerance, optionally
	// with per-student variables.
	AnswerValue flexString        `json:"answer_value"`
	Tolerance   flexString        `json:"tolerance"`
	Variables   []json.RawMessage `json:"variables"`
}

// ---------- Auto-Grading Logic ----------

// autoGrade evaluates a student's answer against the question's correct answer(s).
// Returns (points, correct, gradable). gradable is false for types that require manual review.
//
// Wave A1 cutover: matching, fill_in_multiple_blanks, and numerical_question
// (with margin) are now auto-graded. Pre-Wave-A1 submissions for these types
// that were already routed to pending_review remain untouched — only newly-
// completed submissions exercise the new code paths. The graded_via column
// on quiz_submission_answer (migration 000014) provides the audit trail.
func (s *QuizService) autoGrade(question *models.QuizQuestion, submittedAnswer string) (float64, bool, bool) {
	pointsPossible := 1.0
	if question.PointsPossible != nil {
		pointsPossible = *question.PointsPossible
	}
	return s.autoGradeWithPoints(question, submittedAnswer, pointsPossible)
}

// autoGradeWithPoints is autoGrade with an explicit points value (used when a
// question group overrides the per-question points via points_per_item).
func (s *QuizService) autoGradeWithPoints(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	switch NormalizeQuizQuestionType(question.QuestionType) {
	case "multiple_choice", "true_false":
		return s.gradeMultipleChoice(question, submittedAnswer, pointsPossible)
	case "short_answer":
		return s.gradeShortAnswer(question, submittedAnswer, pointsPossible)
	case "numerical_question":
		return s.gradeNumerical(question, submittedAnswer, pointsPossible)
	case "essay":
		// Essays cannot be auto-graded
		return 0, false, false
	case "matching":
		return s.gradeMatching(question, submittedAnswer, pointsPossible)
	case "fill_in_multiple_blanks":
		return s.gradeFillInMultipleBlanks(question, submittedAnswer, pointsPossible)

	// ---- Wave A1: 9 new item types ----
	case "multiple_answer":
		return s.gradeMultipleAnswer(question, submittedAnswer, pointsPossible)
	case "multiple_dropdown":
		return s.gradeMultipleDropdown(question, submittedAnswer, pointsPossible)
	case "fill_in_the_blank":
		return s.gradeFillInTheBlank(question, submittedAnswer, pointsPossible)
	case "formula":
		return s.gradeFormula(question, submittedAnswer, pointsPossible)
	case "file_upload":
		// File uploads are never auto-graded; an instructor must review.
		return 0, false, false
	case "ordering":
		return s.gradeOrdering(question, submittedAnswer, pointsPossible)
	case "categorization":
		return s.gradeCategorization(question, submittedAnswer, pointsPossible)
	case "hot_spot":
		return s.gradeHotSpot(question, submittedAnswer, pointsPossible)
	case "text_only":
		// Informational items contribute 0 points but are considered
		// "gradable" (i.e. they never block a submission with pending_review).
		return 0, false, true

	default:
		return 0, false, false
	}
}

// NormalizeQuizQuestionType maps legacy / Canvas spellings that older imports
// persisted (e.g. "multiple_answers", "multiple_dropdowns",
// "multiple_choice_question") onto Paper's unified question_type values so
// rows written before the importer fix still grade and render.
func NormalizeQuizQuestionType(t string) string {
	switch t {
	case "multiple_answers", "multiple_answers_question":
		return "multiple_answer"
	case "multiple_dropdowns", "multiple_dropdowns_question":
		return "multiple_dropdown"
	case "multiple_choice_question":
		return "multiple_choice"
	case "true_false_question":
		return "true_false"
	case "short_answer_question":
		return "short_answer"
	case "essay_question":
		return "essay"
	case "matching_question":
		return "matching"
	case "fill_in_multiple_blanks_question":
		return "fill_in_multiple_blanks"
	case "file_upload_question":
		return "file_upload"
	case "text_only_question":
		return "text_only"
	case "calculated_question":
		return "formula"
	}
	return t
}

func roundScore(score float64) float64 {
	return math.Round(score*100) / 100
}

// gradeMultipleChoice checks if the submitted answer ID matches an answer with weight > 0.
func (s *QuizService) gradeMultipleChoice(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var options []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &options); err != nil {
		return 0, false, false
	}

	submittedAnswer = strings.TrimSpace(submittedAnswer)

	for _, opt := range options {
		if opt.ID == submittedAnswer && opt.Weight > 0 {
			// Partial credit: score = pointsPossible * (weight / 100)
			return roundScore(pointsPossible * (opt.Weight / 100.0)), true, true
		}
	}

	return 0, false, true
}

// gradeShortAnswer checks if the submitted text matches any correct answer (case-insensitive).
//
// Every option on a short_answer question is an "acceptable answer" (Canvas
// semantics). The editor's "Add Acceptable Answer" button historically
// appended options with weight 0, which could never match; a weight of
// exactly 0 is therefore treated as full credit. Positive weights below 100
// keep their partial-credit meaning; negative weights never match.
func (s *QuizService) gradeShortAnswer(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var options []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &options); err != nil {
		return 0, false, false
	}

	submittedAnswer = strings.TrimSpace(strings.ToLower(submittedAnswer))
	if submittedAnswer == "" {
		return 0, false, true
	}

	for _, opt := range options {
		weight := opt.Weight
		if weight == 0 {
			weight = 100
		}
		if weight > 0 && strings.TrimSpace(strings.ToLower(opt.Text.String())) == submittedAnswer {
			return roundScore(pointsPossible * (weight / 100.0)), true, true
		}
	}

	return 0, false, true
}

// gradeNumerical checks if the submitted number matches a correct numerical answer.
// Supported answer shapes (all may coexist in one question):
//
//	[{"id":"a1","text":"42","weight":100,"margin":"0.5"}]            // Paper editor
//	[{"numerical_answer_type":"exact_answer","exact":42,"margin":0.5}] // Canvas exact
//	[{"numerical_answer_type":"range_answer","start":1,"end":2}]       // Canvas range
//	[{"numerical_answer_type":"precision_answer","approximate":1.5,"precision":2}]
//
// Comparison is numeric (Canvas parity): "3.50" and "+3.5" both match "3.5".
// Margin is inclusive and may be absolute ("0.5") or a percentage ("5%").
func (s *QuizService) gradeNumerical(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var options []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &options); err != nil {
		return 0, false, false
	}

	submittedAnswer = strings.TrimSpace(submittedAnswer)

	for _, opt := range options {
		if opt.Weight <= 0 {
			continue
		}
		if numericalMatch(submittedAnswer, opt) {
			return roundScore(pointsPossible * (opt.Weight / 100.0)), true, true
		}
	}

	return 0, false, true
}

// parseDecimal parses a user- or author-entered decimal string exactly
// (big.Rat avoids float rounding at the edge of a tolerance band, the same
// reason Canvas uses BigDecimal). A thousands separator is not supported;
// a single leading "+" is.
func parseDecimal(s string) (*big.Rat, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false
	}
	s = strings.TrimPrefix(s, "+")
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return nil, false
	}
	return r, true
}

// numericalMatch returns true if submitted answers the given option, respecting
// the option's optional margin tolerance. Used by both gradeNumerical and
// gradeFormula.
func numericalMatch(submitted string, opt answerOption) bool {
	submitted = strings.TrimSpace(submitted)
	if submitted == "" {
		return false
	}
	user, userIsNum := parseDecimal(submitted)

	switch {
	case opt.NumericalAnswerType == "range_answer" ||
		(opt.NumericalAnswerType == "" && opt.Text == "" && opt.Exact == "" && opt.Start != "" && opt.End != ""):
		lo, okLo := parseDecimal(opt.Start.String())
		hi, okHi := parseDecimal(opt.End.String())
		if !userIsNum || !okLo || !okHi {
			return false
		}
		if lo.Cmp(hi) > 0 {
			lo, hi = hi, lo
		}
		return user.Cmp(lo) >= 0 && user.Cmp(hi) <= 0

	case opt.NumericalAnswerType == "precision_answer":
		if !userIsNum {
			return false
		}
		return precisionMatch(submitted, opt.Approximate.String(), opt.Precision.String())
	}

	// Exact answer (with optional margin). Canvas stores the value in
	// `exact`; Paper's editor stores it in `text`.
	expectedText := strings.TrimSpace(opt.Exact.String())
	if expectedText == "" {
		expectedText = strings.TrimSpace(opt.Text.String())
	}
	if expectedText == "" {
		return false
	}
	expected, expectedIsNum := parseDecimal(expectedText)
	if !userIsNum || !expectedIsNum {
		// Non-numeric key or submission: only an exact textual match counts.
		return !expectedIsNum && expectedText == submitted
	}

	margin := strings.TrimSpace(opt.Margin.String())
	if margin == "" {
		return user.Cmp(expected) == 0
	}
	tolerance, ok := parseMarginRat(margin, expected)
	if !ok {
		return false
	}
	diff := new(big.Rat).Sub(user, expected)
	diff.Abs(diff)
	return diff.Cmp(tolerance) <= 0
}

// parseMarginRat parses a margin string into an absolute tolerance.
// "0.5" → 0.5. "5%" → 5% of |expected|.
func parseMarginRat(margin string, expected *big.Rat) (*big.Rat, bool) {
	margin = strings.TrimSpace(margin)
	if strings.HasSuffix(margin, "%") {
		pct, ok := parseDecimal(strings.TrimSuffix(margin, "%"))
		if !ok {
			return nil, false
		}
		abs := new(big.Rat).Abs(expected)
		tol := new(big.Rat).Mul(abs, pct)
		tol.Quo(tol, big.NewRat(100, 1))
		return tol.Abs(tol), true
	}
	tol, ok := parseDecimal(margin)
	if !ok {
		return nil, false
	}
	return tol.Abs(tol), true
}

// precisionMatch implements Canvas's precision_answer: the submission matches
// when its sign, decimal exponent, and first `precision` significant digits
// (truncated, not rounded) equal the approximate value's.
func precisionMatch(submitted, approximate, precision string) bool {
	p, err := strconv.Atoi(strings.TrimSpace(precision))
	if err != nil || p <= 0 {
		return false
	}
	u, err1 := strconv.ParseFloat(strings.TrimPrefix(strings.TrimSpace(submitted), "+"), 64)
	a, err2 := strconv.ParseFloat(strings.TrimSpace(approximate), 64)
	if err1 != nil || err2 != nil {
		return false
	}
	us, ud, ue := sciParts(u)
	as, ad, ae := sciParts(a)
	if us != as || ue != ae {
		return false
	}
	for len(ud) < p {
		ud += "0"
	}
	for len(ad) < p {
		ad += "0"
	}
	return ud[:p] == ad[:p]
}

// sciParts splits a float into (negative?, significant digits, exponent)
// using the shortest exact decimal representation.
func sciParts(v float64) (bool, string, string) {
	s := strconv.FormatFloat(math.Abs(v), 'e', -1, 64) // e.g. "1.345e+01"
	mant, exp, _ := strings.Cut(s, "e")
	return v < 0, strings.Replace(mant, ".", "", 1), exp
}

// ---------- Wave A1: New auto-graders ----------

// gradeMultipleAnswer scores a multiple_answer (checkbox) question.
// Submission JSON is an array of selected option IDs. Each correct option ID
// selected awards +(points/|correct|); each incorrect ID selected deducts
// (points/|correct|). Final score floors at 0 (no negative points). Correct
// flag is true only when the selected set exactly equals the correct set.
func (s *QuizService) gradeMultipleAnswer(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var options []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &options); err != nil {
		return 0, false, false
	}

	var selected []string
	submittedAnswer = strings.TrimSpace(submittedAnswer)
	if submittedAnswer == "" {
		// Empty array submission is treated as "no selections"; still gradable.
		selected = nil
	} else if err := json.Unmarshal([]byte(submittedAnswer), &selected); err != nil {
		return 0, false, false
	}

	correctIDs := map[string]bool{}
	for _, opt := range options {
		if opt.Weight > 0 {
			correctIDs[opt.ID] = true
		}
	}

	if len(correctIDs) == 0 {
		return 0, false, true
	}

	perOption := pointsPossible / float64(len(correctIDs))

	score := 0.0
	seen := map[string]bool{}
	exact := true
	for _, sel := range selected {
		if seen[sel] {
			continue
		}
		seen[sel] = true
		if correctIDs[sel] {
			score += perOption
		} else {
			// Known-but-incorrect or unknown option ID: treat as wrong selection.
			score -= perOption
			exact = false
		}
	}
	if score < 0 {
		score = 0
	}
	score = roundScore(score)

	// Exact match only when every correct option was selected and nothing else.
	for id := range correctIDs {
		if !seen[id] {
			exact = false
			break
		}
	}
	return score, exact, true
}

// gradeMultipleDropdown scores a multiple_dropdown question. Submission JSON:
// {"blank_id": "option_id"}. Each blank's selected option ID is matched
// against options where opt.BlankID == blank and opt.Weight > 0. Score per
// blank is pointsPossible/|blanks|; partial credit allowed.
func (s *QuizService) gradeMultipleDropdown(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var options []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &options); err != nil {
		return 0, false, false
	}

	var submitted map[string]string
	if strings.TrimSpace(submittedAnswer) == "" {
		submitted = map[string]string{}
	} else if err := json.Unmarshal([]byte(submittedAnswer), &submitted); err != nil {
		return 0, false, false
	}

	// Group correct option IDs by blank.
	correctByBlank := map[string]map[string]bool{}
	for _, opt := range options {
		if opt.BlankID == "" {
			continue
		}
		if _, ok := correctByBlank[opt.BlankID]; !ok {
			correctByBlank[opt.BlankID] = map[string]bool{}
		}
		if opt.Weight > 0 {
			correctByBlank[opt.BlankID][opt.ID] = true
		}
	}

	blankCount := len(correctByBlank)
	if blankCount == 0 {
		return 0, false, true
	}
	perBlank := pointsPossible / float64(blankCount)

	score := 0.0
	correctCount := 0
	for blank, accepted := range correctByBlank {
		if accepted[submitted[blank]] {
			score += perBlank
			correctCount++
		}
	}
	return roundScore(score), correctCount == blankCount, true
}

// gradeFillInTheBlank scores a single-blank fill_in_the_blank. Like
// short_answer, but partial-credit via the per-option Weight field. The
// submission is the user's typed text; it is matched case-insensitively
// (TrimSpace + ToLower) against each option's Text.
func (s *QuizService) gradeFillInTheBlank(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var options []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &options); err != nil {
		return 0, false, false
	}

	submittedAnswer = strings.TrimSpace(strings.ToLower(submittedAnswer))
	if submittedAnswer == "" {
		return 0, false, true
	}
	for _, opt := range options {
		if opt.Weight > 0 && strings.TrimSpace(strings.ToLower(opt.Text.String())) == submittedAnswer {
			return roundScore(pointsPossible * (opt.Weight / 100.0)), true, true
		}
	}
	return 0, false, true
}

// gradeFormula scores a formula item. Re-uses the numerical-tolerance logic.
//
// Two authoring shapes are accepted:
//   - numerical shape: [{text: "42", margin: "0.5", weight: 100}]
//   - FormulaEditor shape: [{formula, variables, tolerance, answer_value}]
//     where answer_value is the expected value (no-variable mode) and
//     tolerance the ± allowed deviation.
//
// A formula with per-student variables but no fixed expected value cannot be
// scored automatically (variable generation is not implemented), so it is
// routed to manual review rather than mis-graded. A blank submission never
// matches.
func (s *QuizService) gradeFormula(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var options []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &options); err != nil {
		return 0, false, false
	}

	submittedAnswer = strings.TrimSpace(submittedAnswer)
	anyKey := false
	for _, opt := range options {
		if opt.Weight <= 0 {
			continue
		}
		candidate := opt
		if strings.TrimSpace(candidate.Text.String()) == "" && strings.TrimSpace(candidate.Exact.String()) == "" {
			candidate.Text = flexString(strings.TrimSpace(opt.AnswerValue.String()))
			if strings.TrimSpace(candidate.Margin.String()) == "" {
				tol := strings.TrimSpace(opt.Tolerance.String())
				if tol != "" && tol != "0" {
					candidate.Margin = flexString(tol)
				}
			}
		}
		if strings.TrimSpace(candidate.Text.String()) == "" && strings.TrimSpace(candidate.Exact.String()) == "" &&
			candidate.NumericalAnswerType == "" {
			continue
		}
		anyKey = true
		if numericalMatch(submittedAnswer, candidate) {
			return roundScore(pointsPossible * (opt.Weight / 100.0)), true, true
		}
	}
	if !anyKey {
		// No expected value is defined (e.g. variable-mode formula):
		// an instructor has to score it.
		return 0, false, false
	}
	return 0, false, true
}

// gradeOrdering scores an ordering item. Submission JSON: ["id1","id2",…]
// representing the user's order. The canonical order is the authored order
// of the options (OrderingEditor stores them in correct order and does not
// set a weight, so weight is ignored except that explicitly negative-weight
// rows are skipped). Score is pointsPossible × (correctPositions / totalPositions).
func (s *QuizService) gradeOrdering(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var options []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &options); err != nil {
		return 0, false, false
	}

	var canonical []string
	for _, opt := range options {
		if opt.ID == "" || opt.Weight < 0 {
			continue
		}
		canonical = append(canonical, opt.ID)
	}

	var submitted []string
	if strings.TrimSpace(submittedAnswer) == "" {
		submitted = nil
	} else if err := json.Unmarshal([]byte(submittedAnswer), &submitted); err != nil {
		return 0, false, false
	}

	total := len(canonical)
	if total == 0 {
		return 0, false, true
	}

	correct := 0
	for i, id := range canonical {
		if i < len(submitted) && submitted[i] == id {
			correct++
		}
	}

	return roundScore(pointsPossible * float64(correct) / float64(total)), correct == total, true
}

// gradeCategorization scores a categorization (bucket-drop) item. Submission
// JSON: {"item_id":"bucket_id"}. Two key shapes are accepted:
//   - CategorizationEditor shape: one config option carrying
//     {items:[{id}], buckets:[{id, item_ids:[...]}]}; an item's correct bucket
//     is the bucket whose item_ids lists it. Items not assigned to any
//     bucket are distractors and do not count.
//   - legacy shape: one option per item with opt.ID = item_id and
//     opt.RightID = bucket_id (Weight > 0). Options without a RightID are
//     ignored — an empty correct bucket must never match an absent answer.
//
// Score per item is pointsPossible/|items|.
func (s *QuizService) gradeCategorization(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var options []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &options); err != nil {
		return 0, false, false
	}

	correctBucket := map[string]string{}
	for _, opt := range options {
		if len(opt.Buckets) > 0 || len(opt.Items) > 0 {
			for _, b := range opt.Buckets {
				if b.ID == "" {
					continue
				}
				for _, itemID := range b.ItemIDs {
					if itemID != "" {
						correctBucket[itemID] = b.ID
					}
				}
			}
			continue
		}
		if opt.Weight > 0 && opt.ID != "" && opt.RightID != "" {
			correctBucket[opt.ID] = opt.RightID
		}
	}

	var submitted map[string]string
	if strings.TrimSpace(submittedAnswer) == "" {
		submitted = map[string]string{}
	} else if err := json.Unmarshal([]byte(submittedAnswer), &submitted); err != nil {
		return 0, false, false
	}

	itemCount := len(correctBucket)
	if itemCount == 0 {
		return 0, false, true
	}
	perItem := pointsPossible / float64(itemCount)

	score := 0.0
	correctPlaced := 0
	for item, bucket := range correctBucket {
		if got, ok := submitted[item]; ok && got == bucket {
			score += perItem
			correctPlaced++
		}
	}
	return roundScore(score), correctPlaced == itemCount, true
}

// gradeHotSpot scores a hot_spot item. Submission JSON: {"x":N,"y":M}.
// Two key shapes:
//   - HotSpotEditor shape: answers[0].regions = [{x,y,w,h}] as fractions of
//     the image (the player reports the click as fractions too).
//   - legacy shape: each option (Weight > 0) is an absolute rectangle via
//     x, y, w, h (top-left origin).
//
// Binary scoring: the click is correct if it falls inside ANY accepted
// rectangle (boundary inclusive). Zero-area rectangles never match.
func (s *QuizService) gradeHotSpot(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var options []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &options); err != nil {
		return 0, false, false
	}

	var click struct {
		X *float64 `json:"x"`
		Y *float64 `json:"y"`
	}
	if strings.TrimSpace(submittedAnswer) == "" {
		return 0, false, true
	}
	if err := json.Unmarshal([]byte(submittedAnswer), &click); err != nil {
		return 0, false, false
	}
	if click.X == nil || click.Y == nil {
		return 0, false, true
	}
	cx, cy := *click.X, *click.Y

	inside := func(x, y, w, h float64) bool {
		if w <= 0 || h <= 0 {
			return false
		}
		return cx >= x && cx <= x+w && cy >= y && cy <= y+h
	}

	for _, opt := range options {
		if opt.Weight <= 0 {
			continue
		}
		if len(opt.Regions) > 0 {
			for _, r := range opt.Regions {
				if inside(r.X, r.Y, r.W, r.H) {
					return roundScore(pointsPossible), true, true
				}
			}
			continue
		}
		if inside(opt.X, opt.Y, opt.Width, opt.Height) {
			return roundScore(pointsPossible), true, true
		}
	}
	return 0, false, true
}

// matchingKey returns the identifier a submission uses to name the left-hand
// side of a matching pair: the human label when present, else the pair id
// (the Canvas importer emits id + prompt text without a separate label).
func matchingKey(opt answerOption) string {
	if opt.Left != "" {
		return opt.Left
	}
	return opt.ID
}

// gradeMatching scores a matching item. Question Answers JSON is an array of
// option objects, each with Left (label) and RightID (the correct right-hand
// option id; Canvas's match_id is accepted as an alias). Entries flagged with
// the "_match_options_" blank_id are the right-hand pool and are skipped.
// Submission JSON: [{left, right_id}, …]. Score = pointsPossible ×
// (correctPairs / totalPairs).
//
// Each left item is scored at most once: duplicate submissions for the same
// left (e.g. one pair per possible right option) only count the first, so a
// shotgun answer cannot collect full credit.
func (s *QuizService) gradeMatching(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var correctPairs []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &correctPairs); err != nil {
		return 0, false, false
	}

	correctByLeft := map[string]string{}
	for _, p := range correctPairs {
		if p.BlankID == matchOptionsSentinel {
			continue
		}
		right := p.RightID
		if right == "" {
			right = p.MatchID
		}
		key := matchingKey(p)
		if key == "" || right == "" {
			continue
		}
		correctByLeft[key] = right
	}

	type submittedPair struct {
		Left    string `json:"left"`
		RightID string `json:"right_id"`
	}
	var submitted []submittedPair
	if strings.TrimSpace(submittedAnswer) == "" {
		submitted = nil
	} else if err := json.Unmarshal([]byte(submittedAnswer), &submitted); err != nil {
		return 0, false, false
	}

	total := len(correctByLeft)
	if total == 0 {
		// No correctly-formed pairs in the question definition — route to
		// manual review rather than silently auto-completing a malformed item.
		return 0, false, false
	}

	correct := 0
	seenLeft := map[string]bool{}
	for _, sp := range submitted {
		if seenLeft[sp.Left] {
			continue
		}
		seenLeft[sp.Left] = true
		if want, ok := correctByLeft[sp.Left]; ok && want == sp.RightID {
			correct++
		}
	}

	return roundScore(pointsPossible * float64(correct) / float64(total)), correct == total, true
}

// gradeFillInMultipleBlanks scores a fill_in_multiple_blanks item.
// Question Answers JSON is either the Paper map shape
// {"blank_id": ["accepted1", "accepted2", …]} or the Canvas array shape
// [{blank_id, text, weight}] (one entry per accepted answer, weight > 0).
// Submission JSON: {"blank_id": "user_text"}. Per-blank match is
// case-insensitive + TrimSpace'd. Score = pointsPossible × (correct/total).
func (s *QuizService) gradeFillInMultipleBlanks(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	accepted, ok := parseBlankAnswers(question.Answers)
	if !ok {
		return 0, false, false
	}

	var submitted map[string]string
	if strings.TrimSpace(submittedAnswer) == "" {
		submitted = map[string]string{}
	} else if err := json.Unmarshal([]byte(submittedAnswer), &submitted); err != nil {
		return 0, false, false
	}

	total := len(accepted)
	if total == 0 {
		// No blanks defined — malformed item, route to manual review.
		return 0, false, false
	}

	correct := 0
	for blank, acceptables := range accepted {
		user := strings.TrimSpace(strings.ToLower(submitted[blank]))
		if user == "" {
			continue
		}
		for _, ans := range acceptables {
			if strings.TrimSpace(strings.ToLower(ans)) == user {
				correct++
				break
			}
		}
	}

	return roundScore(pointsPossible * float64(correct) / float64(total)), correct == total, true
}

// parseBlankAnswers decodes fill_in_multiple_blanks answers in either the map
// or the Canvas array shape. ok=false means the JSON is malformed.
func parseBlankAnswers(raw string) (map[string][]string, bool) {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "[") {
		var options []answerOption
		if err := json.Unmarshal([]byte(trimmed), &options); err != nil {
			return nil, false
		}
		out := map[string][]string{}
		for _, opt := range options {
			if opt.BlankID == "" || opt.Weight <= 0 {
				continue
			}
			out[opt.BlankID] = append(out[opt.BlankID], opt.Text.String())
		}
		return out, true
	}
	var accepted map[string][]string
	if err := json.Unmarshal([]byte(trimmed), &accepted); err != nil {
		return nil, false
	}
	return accepted, true
}
