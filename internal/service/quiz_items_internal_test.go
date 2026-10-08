package service

// Unit tests for the quiz-items lane helpers: the formula evaluator and
// per-attempt variables (G051), shuffle (G049), IP filter (G055) and survey
// semantics (G058).

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

func TestEvalFormula(t *testing.T) {
	vars := map[string]float64{"x": 3, "y": 4, "rate_2": 0.5}
	cases := []struct {
		expr string
		want float64
	}{
		{"x * 2 + y", 10},
		{"sqrt(x^2 + y^2)", 5},
		{"-x + +y", 1},
		{"2^3^2", 512}, // right-associative
		{"2^-1", 0.5},
		{"(x + y) / 2", 3.5},
		{"10 % 4", 2},
		{"rate_2 * 100", 50},
		{"max(x, y) - min(x, y)", 1},
		{"round(pi * 100) / 100", 3.14},
		{"ln(e)", 1},
		{"log(1000)", 3},
		{"1.5e2 + 1", 151},
		{"abs(-7)", 7},
		{"pow(2, 10)", 1024},
	}
	for _, tc := range cases {
		got, err := EvalFormula(tc.expr, vars)
		if err != nil {
			t.Fatalf("%s: unexpected error %v", tc.expr, err)
		}
		if math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("%s = %v, want %v", tc.expr, got, tc.want)
		}
	}
}

func TestEvalFormulaRejectsBadInput(t *testing.T) {
	for _, expr := range []string{
		"", "x +", "unknown + 1", "1 / 0", "foo(1)", "(1 + 2", "1 2", "sqrt(-1)", "2e", "os.Exit(1)",
		strings.Repeat("(", 200) + "1" + strings.Repeat(")", 200),
	} {
		if _, err := EvalFormula(expr, map[string]float64{"x": 1}); err == nil {
			t.Errorf("%q: expected an error", expr)
		}
	}
}

func formulaQuestion(answers string) *models.QuizQuestion {
	pts := 2.0
	return &models.QuizQuestion{ID: 77, QuizID: 1, QuestionType: "formula", QuestionText: "What is [x] times 2 plus [y]?", PointsPossible: &pts, Answers: answers}
}

const variableFormula = `[{"id":"f","weight":100,"formula":"x*2 + y","variables":[{"name":"x","min":1,"max":50,"precision":0},{"name":"y","min":0,"max":1,"precision":2}],"tolerance":"0.01","answer_precision":2}]`

func TestFormulaInstanceIsStablePerAttemptAndVariesAcrossAttempts(t *testing.T) {
	cfg, ok := ParseFormulaConfig(variableFormula)
	if !ok {
		t.Fatal("expected a formula config")
	}
	a1, err := cfg.Instance(FormulaSeed(10, 77))
	if err != nil {
		t.Fatal(err)
	}
	again, _ := cfg.Instance(FormulaSeed(10, 77))
	if a1.Expected != again.Expected || a1.Values[0].Value != again.Values[0].Value {
		t.Fatal("the same attempt must always see the same values")
	}
	distinct := map[float64]bool{}
	for sub := uint(1); sub <= 30; sub++ {
		inst, _ := cfg.Instance(FormulaSeed(sub, 77))
		x := inst.Values[0].Value
		if x < 1 || x > 50 || x != math.Trunc(x) {
			t.Fatalf("x=%v outside [1,50] or not an integer", x)
		}
		y := inst.Values[1].Value
		if y < 0 || y > 1 || math.Abs(y*100-math.Round(y*100)) > 1e-9 {
			t.Fatalf("y=%v outside [0,1] or more than 2 decimals", y)
		}
		if math.Abs(inst.Expected-(x*2+y)) > 1e-9 {
			t.Fatalf("expected %v for x=%v y=%v, got %v", x*2+y, x, y, inst.Expected)
		}
		distinct[x] = true
	}
	if len(distinct) < 5 {
		t.Fatalf("variables should vary across attempts, got %d distinct values", len(distinct))
	}
}

func TestPersonalizeFormulaQuestionSubstitutesText(t *testing.T) {
	q := formulaQuestion(variableFormula)
	got, inst := PersonalizeFormulaQuestion(*q, 5)
	if inst == nil {
		t.Fatal("expected an instance")
	}
	if strings.Contains(got.QuestionText, "[x]") || strings.Contains(got.QuestionText, "[y]") {
		t.Fatalf("tokens were not substituted: %q", got.QuestionText)
	}
	if !strings.Contains(got.QuestionText, FormatFormulaNumber(inst.Values[0].Value)) {
		t.Fatalf("x value missing from %q", got.QuestionText)
	}
	if q.QuestionText != "What is [x] times 2 plus [y]?" {
		t.Fatal("the original question must not be mutated")
	}
}

func TestGradeAnswerForSubmissionFormula(t *testing.T) {
	s := &QuizService{}
	q := formulaQuestion(variableFormula)
	cfg, _ := ParseFormulaConfig(variableFormula)
	inst, _ := cfg.Instance(FormulaSeed(9, q.ID))
	right := FormatFormulaNumber(inst.Expected)

	pts, correct, gradable := s.gradeAnswerForSubmission(9, q, right, 2)
	if !gradable || !correct || pts != 2 {
		t.Fatalf("own answer: got (%v,%v,%v)", pts, correct, gradable)
	}
	// Within the 0.01 tolerance.
	pts, correct, _ = s.gradeAnswerForSubmission(9, q, FormatFormulaNumber(inst.Expected+0.005), 2)
	if !correct || pts != 2 {
		t.Fatal("answer within tolerance should be correct")
	}
	pts, correct, gradable = s.gradeAnswerForSubmission(9, q, FormatFormulaNumber(inst.Expected+1), 2)
	if correct || pts != 0 || !gradable {
		t.Fatal("wrong answer should score 0 and still be auto-graded")
	}
	// Another attempt's correct value is (almost surely) wrong here.
	other, _ := cfg.Instance(FormulaSeed(10, q.ID))
	if math.Abs(other.Expected-inst.Expected) > 0.01 {
		if _, correct, _ := s.gradeAnswerForSubmission(9, q, FormatFormulaNumber(other.Expected), 2); correct {
			t.Fatal("a neighbour's answer must not be accepted")
		}
	}
}

func TestFormulaAnswerSetsAndPercentTolerance(t *testing.T) {
	answers := `[{"id":"f","weight":100,"formula":"x+1","tolerance":"10%","answer_sets":[{"variables":[{"name":"x","value":9}],"answer":10},{"variables":[{"name":"x","value":19}],"answer":20}]}]`
	cfg, ok := ParseFormulaConfig(answers)
	if !ok {
		t.Fatal("expected config")
	}
	inst, err := cfg.Instance(FormulaSeed(3, 4))
	if err != nil {
		t.Fatal(err)
	}
	if inst.Expected != 10 && inst.Expected != 20 {
		t.Fatalf("expected an answer-set value, got %v", inst.Expected)
	}
	s := &QuizService{}
	q := formulaQuestion(answers)
	q.ID = 4
	// 10% of the expected value is accepted.
	near := FormatFormulaNumber(inst.Expected * 1.09)
	if _, correct, _ := s.gradeAnswerForSubmission(3, q, near, 1); !correct {
		t.Fatalf("value within 10%% (%s vs %v) should be accepted", near, inst.Expected)
	}
}

func TestFormulaFixedValueStillUsesStatelessGrader(t *testing.T) {
	s := &QuizService{}
	q := formulaQuestion(`[{"id":"f","weight":100,"formula":"","variables":[],"tolerance":0.5,"answer_value":"42"}]`)
	if _, correct, gradable := s.gradeAnswerForSubmission(1, q, "42.4", 2); !correct || !gradable {
		t.Fatal("fixed expected value within tolerance should be correct")
	}
}

func TestFormulaPreview(t *testing.T) {
	cfg, _ := ParseFormulaConfig(variableFormula)
	out, err := FormulaPreview(cfg, 3)
	if err != nil || len(out) != 3 {
		t.Fatalf("preview: %v %d", err, len(out))
	}
	bad, _ := ParseFormulaConfig(`[{"formula":"x / 0","variables":[{"name":"x","min":1,"max":2}]}]`)
	if _, err := FormulaPreview(bad, 1); err == nil {
		t.Fatal("division by zero must surface as an error")
	}
}

func TestShuffleQuizAnswers(t *testing.T) {
	opts := `[{"id":"a","text":"A"},{"id":"b","text":"B"},{"id":"c","text":"C"},{"id":"d","text":"D"},{"id":"e","text":"E"}]`
	order := func(js string) string {
		var list []struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal([]byte(js), &list)
		ids := make([]string, len(list))
		for i, o := range list {
			ids[i] = o.ID
		}
		return strings.Join(ids, "")
	}
	if ShuffleQuizAnswers("multiple_choice", opts, 1) != ShuffleQuizAnswers("multiple_choice", opts, 1) {
		t.Fatal("shuffle must be stable for a seed")
	}
	seen := map[string]bool{}
	for seed := int64(1); seed < 40; seed++ {
		got := order(ShuffleQuizAnswers("multiple_choice", opts, seed))
		if len(got) != 5 {
			t.Fatalf("options lost: %q", got)
		}
		seen[got] = true
	}
	if len(seen) < 5 {
		t.Fatalf("expected varied orders, got %d", len(seen))
	}
	tf := `[{"id":"true","text":"True"},{"id":"false","text":"False"}]`
	for _, typ := range []string{"true_false", "matching", "fill_in_multiple_blanks", "essay"} {
		if ShuffleQuizAnswers(typ, tf, 3) != tf {
			t.Errorf("%s must not be shuffled", typ)
		}
	}
	if ShuffleQuizAnswers("multiple_answers", opts, 2) == opts && ShuffleQuizAnswers("multiple_answers", opts, 3) == opts {
		t.Error("legacy multiple_answers spelling should be shuffled too")
	}
}

func TestIPFilter(t *testing.T) {
	norm, err := NormalizeIPFilter(" 10.0.0.0/8, 192.168.1.10\n172.16.0.0/255.255.0.0 ,2001:db8::/32 ")
	if err != nil {
		t.Fatal(err)
	}
	if norm != "10.0.0.0/8,192.168.1.10,172.16.0.0/255.255.0.0,2001:db8::/32" {
		t.Fatalf("normalized = %q", norm)
	}
	allowed := []string{"10.1.2.3", "192.168.1.10", "172.16.200.1", "2001:db8::1"}
	denied := []string{"192.168.1.11", "11.0.0.1", "172.17.0.1", "2001:db9::1", "not-an-ip", ""}
	for _, ip := range allowed {
		if !QuizIPAllowed(norm, ip) {
			t.Errorf("%s should be allowed", ip)
		}
	}
	for _, ip := range denied {
		if QuizIPAllowed(norm, ip) {
			t.Errorf("%s should be denied", ip)
		}
	}
	if !QuizIPAllowed("", "1.2.3.4") {
		t.Error("an empty filter allows everyone")
	}
	for _, bad := range []string{"300.1.1.1", "10.0.0.0/33", "abc", "10.0.0.0/255.0.255.0"} {
		if _, err := NormalizeIPFilter(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
	if empty, err := NormalizeIPFilter("  ,  "); err != nil || empty != "" {
		t.Errorf("blank filter should normalize to empty, got %q %v", empty, err)
	}
}

func TestApplySurveySemantics(t *testing.T) {
	pp := 5.0
	yes := true
	one := 1.0
	answers := []models.QuizSubmissionAnswer{{ID: 1, Correct: &yes, Points: &one}, {ID: 2}}

	graded := &models.Quiz{QuizType: "graded_survey", PointsPossible: &pp}
	total, review := 1.0, true
	applySurveySemantics(graded, answers, &total, &review)
	if total != 5 || review {
		t.Fatalf("graded survey: total=%v review=%v", total, review)
	}
	for _, a := range answers {
		if a.Correct != nil || a.Points == nil || *a.Points != 0 || a.GradedVia == nil {
			t.Fatalf("survey answers carry no correctness: %+v", a)
		}
	}

	total, review = 3, true
	applySurveySemantics(&models.Quiz{QuizType: "survey", PointsPossible: &pp}, answers, &total, &review)
	if total != 0 || review {
		t.Fatalf("ungraded survey: total=%v review=%v", total, review)
	}

	total, review = 3, true
	applySurveySemantics(&models.Quiz{QuizType: "assignment"}, answers, &total, &review)
	if total != 3 || !review {
		t.Fatal("non-survey quizzes are untouched")
	}
}

func TestCantGoBackActive(t *testing.T) {
	if CantGoBackActive(&models.Quiz{CantGoBack: true}) {
		t.Fatal("cant_go_back needs one_question_at_a_time")
	}
	if !CantGoBackActive(&models.Quiz{CantGoBack: true, OneQuestionAtATime: true}) {
		t.Fatal("expected active")
	}
	got := ReadQuestionSet(&models.QuizSubmission{ReadQuestions: "[3,5]"})
	if !got[3] || !got[5] || got[4] {
		t.Fatalf("read set = %v", got)
	}
}
