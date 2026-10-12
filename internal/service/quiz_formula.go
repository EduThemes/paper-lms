package service

// Formula (Canvas "calculated") questions with per-student variables.
//
// A formula item stores one config option in quiz_questions.answers:
//
//	{
//	  "id": "...", "weight": 100,
//	  "formula": "x * 2 + y",                       // the expression
//	  "variables": [{"name":"x","min":1,"max":10,"precision":0}],
//	  "tolerance": "0.5" | "5%",                    // ± margin (absolute or %)
//	  "answer_precision": 2,                        // decimals the answer is rounded to
//	  "answer_value": "42",                         // fixed key (no-variable mode)
//	  "answer_sets": [{"variables":[{"name":"x","value":3}],"answer":6}]
//	}
//
// Each attempt sees its own variable values: they are derived from a seed
// built from (submission id, question id), so the same attempt always gets
// the same numbers on reload and the grader recomputes exactly what the
// student was shown — nothing per-student needs to be stored. When the
// question carries precomputed answer_sets (a Canvas import), one set is
// picked by the same seed instead of generating fresh values.

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	mathrand "math/rand"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// FormulaVariable is one randomized variable definition.
type FormulaVariable struct {
	Name      string  `json:"name"`
	Min       float64 `json:"min"`
	Max       float64 `json:"max"`
	Precision int     `json:"precision"` // decimal places of generated values
}

// FormulaAnswerSet is a precomputed (variables → answer) combination.
type FormulaAnswerSet struct {
	Variables []FormulaVarValue `json:"variables"`
	Answer    float64           `json:"answer"`
}

// FormulaVarValue is one variable's concrete value in an attempt.
type FormulaVarValue struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// FormulaConfig is the decoded authoring config of a formula item.
type FormulaConfig struct {
	Formula         string             `json:"formula"`
	Variables       []FormulaVariable  `json:"variables"`
	Tolerance       flexString         `json:"tolerance"`
	Margin          flexString         `json:"margin"`
	AnswerPrecision *int               `json:"answer_precision"`
	AnswerValue     flexString         `json:"answer_value"`
	AnswerSets      []FormulaAnswerSet `json:"answer_sets"`
	Weight          *float64           `json:"weight"`
}

// FormulaInstance is what one attempt sees and is graded against.
type FormulaInstance struct {
	Values   []FormulaVarValue `json:"variables"`
	Expected float64           `json:"expected"`
	Margin   string            `json:"margin"`
}

// ParseFormulaConfig decodes the first config entry of a formula item's
// answers. ok=false when there is no usable formula/answer-set config
// (e.g. the legacy numerical shape {text, margin}).
func ParseFormulaConfig(answers string) (FormulaConfig, bool) {
	var list []FormulaConfig
	if err := json.Unmarshal([]byte(strings.TrimSpace(answers)), &list); err != nil || len(list) == 0 {
		return FormulaConfig{}, false
	}
	cfg := list[0]
	if strings.TrimSpace(cfg.Formula) == "" && len(cfg.AnswerSets) == 0 {
		return cfg, false
	}
	return cfg, true
}

// margin returns the configured tolerance ("" = exact). "0" counts as exact.
func (c FormulaConfig) margin() string {
	m := strings.TrimSpace(c.Margin.String())
	if m == "" {
		m = strings.TrimSpace(c.Tolerance.String())
	}
	if m == "0" {
		return ""
	}
	return m
}

// FormulaSeed is the per-attempt seed for a formula item.
func FormulaSeed(submissionID, questionID uint) int64 {
	return QuizShuffleSeed(submissionID, questionID, 0xF0)
}

// Instance computes the variable values and expected answer for a seed.
func (c FormulaConfig) Instance(seed int64) (FormulaInstance, error) {
	if len(c.AnswerSets) > 0 {
		idx := int(uint64(seed) % uint64(len(c.AnswerSets))) // #nosec G115 -- modulo of a non-negative length
		set := c.AnswerSets[idx]
		return FormulaInstance{Values: set.Variables, Expected: set.Answer, Margin: c.margin()}, nil
	}
	r := mathrand.New(mathrand.NewSource(seed)) // #nosec G404 -- reproducible per-attempt values; the seed is keyed (QuizShuffleSeed)
	values := make([]FormulaVarValue, 0, len(c.Variables))
	env := map[string]float64{}
	for _, v := range c.Variables {
		name := strings.TrimSpace(v.Name)
		if name == "" {
			continue
		}
		lo, hi := v.Min, v.Max
		if lo > hi {
			lo, hi = hi, lo
		}
		val := lo + r.Float64()*(hi-lo)
		val = roundTo(val, clampPrecision(v.Precision))
		values = append(values, FormulaVarValue{Name: name, Value: val})
		env[name] = val
	}
	expected, err := EvalFormula(c.Formula, env)
	if err != nil {
		return FormulaInstance{}, err
	}
	if c.AnswerPrecision != nil {
		expected = roundTo(expected, clampPrecision(*c.AnswerPrecision))
	}
	return FormulaInstance{Values: values, Expected: expected, Margin: c.margin()}, nil
}

func clampPrecision(p int) int {
	if p < 0 {
		return 0
	}
	if p > 10 {
		return 10
	}
	return p
}

func roundTo(v float64, places int) float64 {
	pow := math.Pow(10, float64(places))
	return math.Round(v*pow) / pow
}

// FormatFormulaNumber renders a value the way it is shown to students:
// shortest exact decimal, no exponent for ordinary magnitudes.
func FormatFormulaNumber(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

var formulaTokenRe = regexp.MustCompile(`\[([A-Za-z_][A-Za-z0-9_]*)\]`)

// SubstituteFormulaText replaces [name] tokens in the question text with
// the attempt's variable values. Unknown tokens are left untouched.
func SubstituteFormulaText(text string, values []FormulaVarValue) string {
	if len(values) == 0 {
		return text
	}
	byName := make(map[string]float64, len(values))
	for _, v := range values {
		byName[v.Name] = v.Value
	}
	return formulaTokenRe.ReplaceAllStringFunc(text, func(tok string) string {
		name := tok[1 : len(tok)-1]
		if v, ok := byName[name]; ok {
			return FormatFormulaNumber(v)
		}
		return tok
	})
}

// PersonalizeFormulaQuestion returns a copy of q with the attempt's
// variable values substituted into the question text, plus the instance
// (nil when q is not a variable formula item or evaluation failed).
func PersonalizeFormulaQuestion(q models.QuizQuestion, submissionID uint) (models.QuizQuestion, *FormulaInstance) {
	if NormalizeQuizQuestionType(q.QuestionType) != "formula" {
		return q, nil
	}
	cfg, ok := ParseFormulaConfig(q.Answers)
	if !ok {
		return q, nil
	}
	inst, err := cfg.Instance(FormulaSeed(submissionID, q.ID))
	if err != nil {
		return q, nil
	}
	q.QuestionText = SubstituteFormulaText(q.QuestionText, inst.Values)
	return q, &inst
}

// gradeFormulaInstance scores a submitted value against an attempt's
// expected answer.
func gradeFormulaInstance(inst FormulaInstance, submitted string, pointsPossible, weight float64) (float64, bool, bool) {
	opt := answerOption{
		NumericalAnswerType: "exact_answer",
		Exact:               flexString(strconv.FormatFloat(inst.Expected, 'f', -1, 64)),
		Margin:              flexString(inst.Margin),
	}
	if numericalMatch(submitted, opt) {
		if weight <= 0 {
			weight = 100
		}
		return roundScore(pointsPossible * (weight / 100.0)), true, true
	}
	return 0, false, true
}

// gradeAnswerForSubmission grades one answer in the context of an attempt:
// variable formula items are scored against the values that attempt saw;
// every other type goes through the stateless graders.
func (s *QuizService) gradeAnswerForSubmission(submissionID uint, question *models.QuizQuestion, answer string, pointsPossible float64) (float64, bool, bool) {
	if NormalizeQuizQuestionType(question.QuestionType) == "formula" {
		if cfg, ok := ParseFormulaConfig(question.Answers); ok {
			inst, err := cfg.Instance(FormulaSeed(submissionID, question.ID))
			if err != nil {
				// A broken formula cannot be scored automatically.
				return 0, false, false
			}
			weight := 100.0
			if cfg.Weight != nil {
				weight = *cfg.Weight
			}
			return gradeFormulaInstance(inst, answer, pointsPossible, weight)
		}
	}
	return s.autoGradeWithPoints(question, answer, pointsPossible)
}

// FormulaPreview generates n sample instances of a config (authoring aid).
func FormulaPreview(cfg FormulaConfig, n int) ([]FormulaInstance, error) {
	if n <= 0 {
		n = 1
	}
	if n > 20 {
		n = 20
	}
	if strings.TrimSpace(cfg.Formula) == "" && len(cfg.AnswerSets) == 0 {
		return nil, errors.New("formula is required")
	}
	out := make([]FormulaInstance, 0, n)
	for i := 0; i < n; i++ {
		inst, err := cfg.Instance(int64(i+1) * 7919)
		if err != nil {
			return nil, err
		}
		out = append(out, inst)
	}
	return out, nil
}

// ---------- Expression evaluator ----------
//
// A small recursive-descent evaluator for author-written formulas. It
// supports numbers (including 1.5e3), variables, the constants pi and e,
// + - * / % ^ (right-associative power), unary +/-, parentheses and a
// fixed set of math functions. No identifiers outside the variable set
// and that list are accepted, so a formula can never reach anything but
// arithmetic.

// ErrFormula wraps every evaluation failure.
var ErrFormula = errors.New("invalid formula")

type formulaParser struct {
	src  []rune
	pos  int
	vars map[string]float64
}

// EvalFormula evaluates expr with the given variable values.
func EvalFormula(expr string, vars map[string]float64) (float64, error) {
	if len(expr) > 2000 {
		return 0, fmt.Errorf("%w: formula is too long", ErrFormula)
	}
	p := &formulaParser{src: []rune(expr), vars: vars}
	v, err := p.parseExpr(0)
	if err != nil {
		return 0, err
	}
	p.skipSpace()
	if p.pos < len(p.src) {
		return 0, fmt.Errorf("%w: unexpected %q", ErrFormula, string(p.src[p.pos]))
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("%w: result is not a finite number", ErrFormula)
	}
	return v, nil
}

func (p *formulaParser) skipSpace() {
	for p.pos < len(p.src) && unicode.IsSpace(p.src[p.pos]) {
		p.pos++
	}
}

func (p *formulaParser) peek() rune {
	p.skipSpace()
	if p.pos >= len(p.src) {
		return 0
	}
	return p.src[p.pos]
}

// parseExpr handles + and - (lowest precedence). depth bounds recursion.
func (p *formulaParser) parseExpr(depth int) (float64, error) {
	if depth > 100 {
		return 0, fmt.Errorf("%w: nested too deeply", ErrFormula)
	}
	left, err := p.parseTerm(depth)
	if err != nil {
		return 0, err
	}
	for {
		switch p.peek() {
		case '+':
			p.pos++
			right, err := p.parseTerm(depth)
			if err != nil {
				return 0, err
			}
			left += right
		case '-':
			p.pos++
			right, err := p.parseTerm(depth)
			if err != nil {
				return 0, err
			}
			left -= right
		default:
			return left, nil
		}
	}
}

func (p *formulaParser) parseTerm(depth int) (float64, error) {
	left, err := p.parseUnary(depth)
	if err != nil {
		return 0, err
	}
	for {
		switch p.peek() {
		case '*':
			p.pos++
			right, err := p.parseUnary(depth)
			if err != nil {
				return 0, err
			}
			left *= right
		case '/':
			p.pos++
			right, err := p.parseUnary(depth)
			if err != nil {
				return 0, err
			}
			if right == 0 {
				return 0, fmt.Errorf("%w: division by zero", ErrFormula)
			}
			left /= right
		case '%':
			p.pos++
			right, err := p.parseUnary(depth)
			if err != nil {
				return 0, err
			}
			if right == 0 {
				return 0, fmt.Errorf("%w: division by zero", ErrFormula)
			}
			left = math.Mod(left, right)
		default:
			return left, nil
		}
	}
}

func (p *formulaParser) parseUnary(depth int) (float64, error) {
	switch p.peek() {
	case '-':
		p.pos++
		v, err := p.parseUnary(depth + 1)
		return -v, err
	case '+':
		p.pos++
		return p.parseUnary(depth + 1)
	}
	return p.parsePower(depth)
}

// parsePower: base ^ exponent, right-associative; the exponent may carry
// its own unary sign (2^-1).
func (p *formulaParser) parsePower(depth int) (float64, error) {
	base, err := p.parsePrimary(depth)
	if err != nil {
		return 0, err
	}
	if p.peek() == '^' {
		p.pos++
		exp, err := p.parseUnary(depth + 1)
		if err != nil {
			return 0, err
		}
		return math.Pow(base, exp), nil
	}
	return base, nil
}

func (p *formulaParser) parsePrimary(depth int) (float64, error) {
	c := p.peek()
	switch {
	case c == 0:
		return 0, fmt.Errorf("%w: unexpected end of formula", ErrFormula)
	case c == '(':
		p.pos++
		v, err := p.parseExpr(depth + 1)
		if err != nil {
			return 0, err
		}
		if p.peek() != ')' {
			return 0, fmt.Errorf("%w: missing )", ErrFormula)
		}
		p.pos++
		return v, nil
	case unicode.IsDigit(c) || c == '.':
		return p.parseNumber()
	case unicode.IsLetter(c) || c == '_':
		return p.parseIdent(depth)
	}
	return 0, fmt.Errorf("%w: unexpected %q", ErrFormula, string(c))
}

func (p *formulaParser) parseNumber() (float64, error) {
	start := p.pos
	for p.pos < len(p.src) && (unicode.IsDigit(p.src[p.pos]) || p.src[p.pos] == '.') {
		p.pos++
	}
	// Optional exponent: e/E followed by digits (with optional sign).
	if p.pos < len(p.src) && (p.src[p.pos] == 'e' || p.src[p.pos] == 'E') {
		save := p.pos
		p.pos++
		if p.pos < len(p.src) && (p.src[p.pos] == '+' || p.src[p.pos] == '-') {
			p.pos++
		}
		digits := 0
		for p.pos < len(p.src) && unicode.IsDigit(p.src[p.pos]) {
			p.pos++
			digits++
		}
		if digits == 0 {
			// Not an exponent ("2e" is not a number); leave the 'e' for
			// the caller, which then reports it as unexpected.
			p.pos = save
		}
	}
	v, err := strconv.ParseFloat(string(p.src[start:p.pos]), 64)
	if err != nil {
		return 0, fmt.Errorf("%w: bad number %q", ErrFormula, string(p.src[start:p.pos]))
	}
	return v, nil
}

var formulaFuncs1 = map[string]func(float64) float64{
	"abs": math.Abs, "sqrt": math.Sqrt, "sin": math.Sin, "cos": math.Cos, "tan": math.Tan,
	"asin": math.Asin, "acos": math.Acos, "atan": math.Atan, "ln": math.Log, "log": math.Log10,
	"exp": math.Exp, "round": math.Round, "floor": math.Floor, "ceil": math.Ceil,
}

var formulaFuncs2 = map[string]func(float64, float64) float64{
	"min": math.Min, "max": math.Max, "pow": math.Pow,
}

func (p *formulaParser) parseIdent(depth int) (float64, error) {
	start := p.pos
	for p.pos < len(p.src) && (unicode.IsLetter(p.src[p.pos]) || unicode.IsDigit(p.src[p.pos]) || p.src[p.pos] == '_') {
		p.pos++
	}
	name := string(p.src[start:p.pos])
	if p.peek() == '(' {
		p.pos++
		args := []float64{}
		if p.peek() != ')' {
			for {
				v, err := p.parseExpr(depth + 1)
				if err != nil {
					return 0, err
				}
				args = append(args, v)
				if p.peek() == ',' {
					p.pos++
					continue
				}
				break
			}
		}
		if p.peek() != ')' {
			return 0, fmt.Errorf("%w: missing ) after %s(", ErrFormula, name)
		}
		p.pos++
		lname := strings.ToLower(name)
		if f, ok := formulaFuncs1[lname]; ok && len(args) == 1 {
			return f(args[0]), nil
		}
		if f, ok := formulaFuncs2[lname]; ok && len(args) == 2 {
			return f(args[0], args[1]), nil
		}
		return 0, fmt.Errorf("%w: unknown function %s/%d", ErrFormula, name, len(args))
	}
	if v, ok := p.vars[name]; ok {
		return v, nil
	}
	switch strings.ToLower(name) {
	case "pi":
		return math.Pi, nil
	case "e":
		return math.E, nil
	}
	return 0, fmt.Errorf("%w: unknown variable %s", ErrFormula, name)
}
