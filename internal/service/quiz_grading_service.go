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
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// gradedViaAuto is the audit-trail value stamped on quiz_submission_answer
// rows that were scored by the auto-grader. Added in Wave A1 alongside the
// matching / fill_in_multiple_blanks cutover so legacy pending_review rows
// (which pre-date this constant) can be distinguished from newly-graded ones.
const gradedViaAuto = "auto"

// answerOption represents one answer choice in a quiz question's Answers JSON field.
// Extended in Wave A1: the optional Margin field is now read by the numerical
// grader, and several new fields are consumed by the 9 new item-type graders.
type answerOption struct {
	ID       string  `json:"id"`
	Text     string  `json:"text"`
	Comments string  `json:"comments"`
	Weight   float64 `json:"weight"`
	// Margin is the numerical tolerance for numerical_question / formula items.
	// Empty string preserves legacy string-equality behavior. A bare number is
	// absolute tolerance ("0.5"); a trailing "%" is percentage of the correct
	// value (e.g. "5%" of 200 = ±10).
	Margin string `json:"margin"`
	// BlankID identifies which blank an option belongs to (multiple_dropdown).
	BlankID string `json:"blank_id"`
	// Coordinates for hot_spot accepted rectangles.
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"w"`
	Height float64 `json:"h"`
	// RightID is the matching-pair right-hand option ID.
	RightID string `json:"right_id"`
	// Left is the matching-pair left-hand label (human readable).
	Left string `json:"left"`
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
	qType := question.QuestionType
	pointsPossible := 1.0
	if question.PointsPossible != nil {
		pointsPossible = *question.PointsPossible
	}

	switch qType {
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
		// Wave A1 (bug 2B fix): per-pair scoring. Was previously routed to
		// pending_review; legacy already-pending rows stay there because this
		// path only runs on newly-completed submissions.
		return s.gradeMatching(question, submittedAnswer, pointsPossible)
	case "fill_in_multiple_blanks":
		// Wave A1 (bug 2B fix): per-blank case-insensitive scoring.
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
			score := pointsPossible * (opt.Weight / 100.0)
			score = math.Round(score*100) / 100
			return score, true, true
		}
	}

	return 0, false, true
}

// gradeShortAnswer checks if the submitted text matches any correct answer (case-insensitive).
func (s *QuizService) gradeShortAnswer(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var options []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &options); err != nil {
		return 0, false, false
	}

	submittedAnswer = strings.TrimSpace(strings.ToLower(submittedAnswer))

	for _, opt := range options {
		if opt.Weight > 0 && strings.TrimSpace(strings.ToLower(opt.Text)) == submittedAnswer {
			score := pointsPossible * (opt.Weight / 100.0)
			score = math.Round(score*100) / 100
			return score, true, true
		}
	}

	return 0, false, true
}

// gradeNumerical checks if the submitted number matches a correct numerical answer.
// The Answers JSON for numerical questions has the format:
// [{"id":"a1","text":"42","weight":100,"margin":"0.5"}]
//
// Wave A1 (bug 1B fix): the `margin` field is now honored. Two modes:
//   - Empty margin: legacy string-equality match on opt.Text (backwards-compat
//     with quizzes authored before margin support existed).
//   - Non-empty margin: numeric parse + tolerance band. Margin is absolute
//     ("0.5" → ±0.5) or percentage of the correct value ("5%" → ±5% of opt.Text).
//
// Percent semantics: "5%" applied to a correct answer of 200 yields a band of
// ±10 (200 * 0.05). Sign-agnostic — `math.Abs` is used on the correct value.
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
			score := pointsPossible * (opt.Weight / 100.0)
			score = math.Round(score*100) / 100
			return score, true, true
		}
	}

	return 0, false, true
}

// numericalMatch returns true if submitted answers the given option, respecting
// the option's optional margin tolerance. Used by both gradeNumerical and
// gradeFormula (Wave A1).
func numericalMatch(submitted string, opt answerOption) bool {
	optText := strings.TrimSpace(opt.Text)
	margin := strings.TrimSpace(opt.Margin)

	// Legacy path: no margin → exact string equality. Preserves backwards
	// compat for quizzes authored before margin support existed.
	if margin == "" {
		return optText == submitted
	}

	userVal, err := strconv.ParseFloat(submitted, 64)
	if err != nil {
		return false
	}
	correctVal, err := strconv.ParseFloat(optText, 64)
	if err != nil {
		return false
	}

	tolerance, ok := parseMargin(margin, correctVal)
	if !ok {
		return false
	}
	return math.Abs(userVal-correctVal) <= tolerance
}

// parseMargin parses a margin string into an absolute tolerance value.
// "0.5" → 0.5. "5%" → 5% of |correctVal|. Returns (tolerance, ok).
func parseMargin(margin string, correctVal float64) (float64, bool) {
	margin = strings.TrimSpace(margin)
	if strings.HasSuffix(margin, "%") {
		pct, err := strconv.ParseFloat(strings.TrimSuffix(margin, "%"), 64)
		if err != nil {
			return 0, false
		}
		return math.Abs(correctVal) * (pct / 100.0), true
	}
	tol, err := strconv.ParseFloat(margin, 64)
	if err != nil {
		return 0, false
	}
	return math.Abs(tol), true
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
	knownIDs := map[string]bool{}
	for _, opt := range options {
		knownIDs[opt.ID] = true
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
		} else if knownIDs[sel] {
			score -= perOption
			exact = false
		} else {
			// Unknown option ID: treat as wrong selection.
			score -= perOption
			exact = false
		}
	}
	if score < 0 {
		score = 0
	}
	score = math.Round(score*100) / 100

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
	score = math.Round(score*100) / 100
	return score, correctCount == blankCount, true
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
	for _, opt := range options {
		if opt.Weight > 0 && strings.TrimSpace(strings.ToLower(opt.Text)) == submittedAnswer {
			score := pointsPossible * (opt.Weight / 100.0)
			score = math.Round(score*100) / 100
			return score, true, true
		}
	}
	return 0, false, true
}

// gradeFormula scores a formula item. Re-uses the numerical-tolerance logic.
// Submission is the user-computed value; each option carries the expected
// value (Text) and optional Margin (absolute or percent).
func (s *QuizService) gradeFormula(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
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
			score := pointsPossible * (opt.Weight / 100.0)
			score = math.Round(score*100) / 100
			return score, true, true
		}
	}
	return 0, false, true
}

// gradeOrdering scores an ordering item. Submission JSON: ["id1","id2",…]
// representing the user's order. Option list defines the canonical order
// (by Position-like array index of options where Weight > 0). Score is
// pointsPossible × (correctPositions / totalPositions).
func (s *QuizService) gradeOrdering(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var options []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &options); err != nil {
		return 0, false, false
	}

	// Canonical order = order of options as authored (Weight > 0 only).
	var canonical []string
	for _, opt := range options {
		if opt.Weight > 0 {
			canonical = append(canonical, opt.ID)
		}
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

	score := pointsPossible * float64(correct) / float64(total)
	score = math.Round(score*100) / 100
	return score, correct == total, true
}

// gradeCategorization scores a categorization (bucket-drop) item. Submission
// JSON: {"item_id":"bucket_id"}. The question's Answers JSON encodes the
// correct bucket for each item via opt.ID = item_id and opt.RightID = bucket_id
// (only options with Weight > 0). Score per item is pointsPossible/|items|.
func (s *QuizService) gradeCategorization(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var options []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &options); err != nil {
		return 0, false, false
	}

	correctBucket := map[string]string{}
	for _, opt := range options {
		if opt.Weight > 0 && opt.ID != "" {
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
		if submitted[item] == bucket {
			score += perItem
			correctPlaced++
		}
	}
	score = math.Round(score*100) / 100
	return score, correctPlaced == itemCount, true
}

// gradeHotSpot scores a hot_spot item. Submission JSON: {"x":N,"y":M}.
// Each option (Weight > 0) defines an axis-aligned rectangle via X, Y, Width,
// Height (top-left origin). Binary scoring: the click is correct if it falls
// inside ANY of the accepted rectangles (boundary inclusive).
func (s *QuizService) gradeHotSpot(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var options []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &options); err != nil {
		return 0, false, false
	}

	var click struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}
	if strings.TrimSpace(submittedAnswer) == "" {
		return 0, false, true
	}
	if err := json.Unmarshal([]byte(submittedAnswer), &click); err != nil {
		return 0, false, false
	}

	for _, opt := range options {
		if opt.Weight <= 0 {
			continue
		}
		if click.X >= opt.X && click.X <= opt.X+opt.Width &&
			click.Y >= opt.Y && click.Y <= opt.Y+opt.Height {
			score := math.Round(pointsPossible*100) / 100
			return score, true, true
		}
	}
	return 0, false, true
}

// gradeMatching scores a matching item. Question Answers JSON is an array of
// option objects, each with Left (label) and RightID (the correct right-hand
// option id). Submission JSON is the same shape: [{left, right_id}, …].
// Score = pointsPossible × (correctPairs / totalPairs).
//
// Wave A1 NOTE on backwards compatibility: prior to this commit, matching was
// hard-routed to pending_review (see internal/service/quiz_service_grading_regression_test.go).
// Existing submissions already in pending_review are NOT retroactively
// re-graded; only newly-completed submissions hit this code path. The
// graded_via column added in migration 000014 distinguishes them.
func (s *QuizService) gradeMatching(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var correctPairs []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &correctPairs); err != nil {
		return 0, false, false
	}

	correctByLeft := map[string]string{}
	for _, p := range correctPairs {
		if p.Left != "" {
			correctByLeft[p.Left] = p.RightID
		}
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
	for _, sp := range submitted {
		if want, ok := correctByLeft[sp.Left]; ok && want == sp.RightID {
			correct++
		}
	}

	score := pointsPossible * float64(correct) / float64(total)
	score = math.Round(score*100) / 100
	return score, correct == total, true
}

// gradeFillInMultipleBlanks scores a fill_in_multiple_blanks item.
// Question Answers JSON: {"blank_id": ["accepted1", "accepted2", …]}.
// Submission JSON: {"blank_id": "user_text"}. Per-blank match is
// case-insensitive + TrimSpace'd. Score = pointsPossible × (correct/total).
//
// Wave A1 NOTE: same backwards-compat caveat as gradeMatching — legacy
// pending_review submissions are not retroactively scored.
func (s *QuizService) gradeFillInMultipleBlanks(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var accepted map[string][]string
	if err := json.Unmarshal([]byte(question.Answers), &accepted); err != nil {
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

	score := pointsPossible * float64(correct) / float64(total)
	score = math.Round(score*100) / 100
	return score, correct == total, true
}
