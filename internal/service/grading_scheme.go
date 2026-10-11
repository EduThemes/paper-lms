package service

// Grading schemes and grading types (G023 / G039).
//
// A scheme is a list of (name, lower bound) pairs, lower bounds as
// fractions of 1, sorted descending. Points-based schemes are entered and
// shown in points out of ScalingFactor (e.g. a 4.0 GPA scale) but are
// stored as fractions too.
//
// Assignment grading types follow Canvas:
//   points        typed number = points (a trailing % = percent of possible)
//   percent       typed number = percent of possible; grade shown as "85%"
//   letter_grade  typed letter = top of that letter's band; grade shown as
//   gpa_scale     the scheme name for score / possible
//   pass_fail     complete/pass = full points, incomplete/fail = 0
//   not_graded    cannot be graded

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// GradingScheme is a resolved scheme ready for conversions.
type GradingScheme struct {
	ID            uint
	Title         string
	Entries       []GradeEntry // descending by Value
	PointsBased   bool
	ScalingFactor float64
}

// DefaultGradingScheme is the standard US scale used when a course has no
// scheme of its own.
func DefaultGradingScheme() *GradingScheme {
	return &GradingScheme{Title: "Default Grading Scheme", Entries: append([]GradeEntry(nil), defaultGradingScale...), ScalingFactor: 1}
}

// SchemeFromStandard builds a scheme from a stored grading standard; nil
// when the data cannot be parsed.
func SchemeFromStandard(gs *models.GradingStandard) *GradingScheme {
	if gs == nil {
		return nil
	}
	entries := parseGradingStandardData(gs.Data)
	if len(entries) == 0 {
		return nil
	}
	sortEntriesDesc(entries)
	sf := gs.ScalingFactor
	if sf <= 0 {
		sf = 1
	}
	return &GradingScheme{ID: gs.ID, Title: gs.Title, Entries: entries, PointsBased: gs.PointsBased, ScalingFactor: sf}
}

func sortEntriesDesc(entries []GradeEntry) {
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Value > entries[j].Value })
}

// ErrInvalidScheme wraps scheme validation failures (handlers map it to 400).
var ErrInvalidScheme = errors.New("invalid grading scheme")

// NormalizeSchemeData validates grading scheme data and returns it in the
// stored form: a JSON array of [name, lower_bound] pairs sorted by lower
// bound descending. It accepts [[name, value], ...] or
// [{"name":..,"value":..}, ...]; when every value is a whole percentage
// (any value above 1) all values are read as percentages. Canvas rules:
// no negative or duplicate cutoffs, a 0 cutoff for the lowest grade, and
// (here) non-empty unique names.
func NormalizeSchemeData(raw json.RawMessage) (string, []GradeEntry, error) {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil || len(arr) == 0 {
		return "", nil, fmt.Errorf("%w: data must be a non-empty list of [name, minimum] pairs", ErrInvalidScheme)
	}
	if len(arr) > 100 {
		return "", nil, fmt.Errorf("%w: at most 100 grades", ErrInvalidScheme)
	}
	entries := make([]GradeEntry, 0, len(arr))
	for _, item := range arr {
		var name string
		var value float64
		var pair []json.RawMessage
		if json.Unmarshal(item, &pair) == nil && len(pair) >= 2 {
			if json.Unmarshal(pair[0], &name) != nil {
				return "", nil, fmt.Errorf("%w: grade names must be text", ErrInvalidScheme)
			}
			if json.Unmarshal(pair[1], &value) != nil {
				return "", nil, fmt.Errorf("%w: grade minimums must be numbers", ErrInvalidScheme)
			}
		} else {
			var obj struct {
				Name  string   `json:"name"`
				Value *float64 `json:"value"`
			}
			if json.Unmarshal(item, &obj) != nil || obj.Value == nil {
				return "", nil, fmt.Errorf("%w: each grade needs a name and a minimum", ErrInvalidScheme)
			}
			name, value = obj.Name, *obj.Value
		}
		name = strings.TrimSpace(name)
		if name == "" || len(name) > 255 {
			return "", nil, fmt.Errorf("%w: grade names must be 1-255 characters", ErrInvalidScheme)
		}
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return "", nil, fmt.Errorf("%w: grade minimums cannot be negative", ErrInvalidScheme)
		}
		entries = append(entries, GradeEntry{Name: name, Value: value})
	}
	percent := false
	for _, e := range entries {
		if e.Value > 1 {
			percent = true
			break
		}
	}
	seenName := map[string]bool{}
	seenValue := map[float64]bool{}
	hasZero := false
	for i := range entries {
		if percent {
			entries[i].Value /= 100
		}
		entries[i].Value = math.Round(entries[i].Value*10000) / 10000
		if entries[i].Value > 1 {
			return "", nil, fmt.Errorf("%w: grade minimums cannot exceed 100%%", ErrInvalidScheme)
		}
		key := strings.ToLower(entries[i].Name)
		if seenName[key] {
			return "", nil, fmt.Errorf("%w: duplicate grade name %q", ErrInvalidScheme, entries[i].Name)
		}
		seenName[key] = true
		if seenValue[entries[i].Value] {
			return "", nil, fmt.Errorf("%w: two grades share the minimum %.2f%%", ErrInvalidScheme, entries[i].Value*100)
		}
		seenValue[entries[i].Value] = true
		if entries[i].Value == 0 {
			hasZero = true
		}
	}
	if !hasZero {
		return "", nil, fmt.Errorf("%w: the lowest grade must start at 0%%", ErrInvalidScheme)
	}
	sortEntriesDesc(entries)
	out := make([][2]interface{}, len(entries))
	for i, e := range entries {
		out[i] = [2]interface{}{e.Name, e.Value}
	}
	b, _ := json.Marshal(out)
	return string(b), entries, nil
}

// ScoreToGrade converts a percentage (0-100, may exceed 100) to the
// scheme's grade name: the highest grade whose cutoff the score reaches,
// else the lowest grade.
func (s *GradingScheme) ScoreToGrade(percent float64) string {
	if s == nil || len(s.Entries) == 0 {
		return scoreToLetterGradeWithScale(percent, nil)
	}
	if percent < 0 {
		percent = 0
	}
	if s.PointsBased && s.ScalingFactor > 0 && s.ScalingFactor != 100 {
		// Points-based schemes compare the score in points rounded to
		// two decimals (what the user sees).
		scaled := math.Round(percent/(100/s.ScalingFactor)*100) / 100
		percent = scaled / s.ScalingFactor * 100
	}
	percent = math.Round(percent*1e6) / 1e6
	for _, e := range s.Entries {
		if percent >= math.Round(e.Value*100*1e6)/1e6 {
			return e.Name
		}
	}
	return s.Entries[len(s.Entries)-1].Name
}

// GradeToPercent converts a grade name to a percentage: 100 for the top
// grade, otherwise just under the next-higher grade's cutoff (the top of
// the named band). ok=false when the name is not in the scheme.
func (s *GradingScheme) GradeToPercent(grade string) (float64, bool) {
	if s == nil {
		return 0, false
	}
	idx := s.indexOf(grade)
	if idx < 0 {
		return 0, false
	}
	if idx == 0 {
		return 100, true
	}
	upper := s.Entries[idx-1].Value
	if s.PointsBased && s.ScalingFactor > 0 {
		return ((upper * s.ScalingFactor) - 0.1) / s.ScalingFactor * 100, true
	}
	step := 1.0
	if math.Abs(s.Entries[idx].Value-upper) < 0.01 {
		step = 0.1
	}
	return upper*100 - step, true
}

var numericGrade = regexp.MustCompile(`^[+-]?\d*\.?\d+$`)

func (s *GradingScheme) indexOf(grade string) int {
	g := strings.TrimSpace(grade)
	if g == "" {
		return -1
	}
	// Canvas accepts a typographic minus for "B−".
	alt := strings.TrimSuffix(g, "−") + "-"
	for i, e := range s.Entries {
		if numericGrade.MatchString(g) {
			// Numeric names ("4", "3.5") match numerically.
			if numericGrade.MatchString(e.Name) {
				a, _ := strconv.ParseFloat(e.Name, 64)
				b, _ := strconv.ParseFloat(g, 64)
				if a == b {
					return i
				}
			}
			continue
		}
		if strings.EqualFold(e.Name, g) || (strings.HasSuffix(g, "−") && strings.EqualFold(e.Name, alt)) {
			return i
		}
	}
	return -1
}

// usesGradingScheme reports whether the grading type converts through a
// scheme.
func usesGradingScheme(gradingType string) bool {
	return gradingType == "letter_grade" || gradingType == "gpa_scale"
}

// ErrNotGradable is returned when grading a not_graded assignment.
var ErrNotGradable = errors.New("this assignment is not graded")

// InterpretGrade turns a typed grade into a score in points for
// assignment (Canvas grade_to_score). An empty grade is not handled here.
func InterpretGrade(a *models.Assignment, scheme *GradingScheme, grade string) (float64, error) {
	grade = strings.TrimSpace(grade)
	gradingType := "points"
	possible := 0.0
	if a != nil {
		if a.GradingType != "" {
			gradingType = a.GradingType
		}
		if a.PointsPossible != nil {
			possible = *a.PointsPossible
		}
	}
	if gradingType == "not_graded" {
		return 0, ErrNotGradable
	}
	invalid := fmt.Errorf("invalid grade value: %s", grade)

	var score float64
	var parsed bool
	lower := strings.ToLower(grade)
	switch {
	case strings.HasSuffix(grade, "%") && numericGrade.MatchString(strings.TrimSuffix(grade, "%")):
		v, _ := strconv.ParseFloat(strings.TrimSuffix(grade, "%"), 64)
		score, parsed = possible*v/100, true
	case numericGrade.MatchString(grade):
		v, _ := strconv.ParseFloat(grade, 64)
		switch {
		case gradingType == "percent":
			score = possible * v / 100
		case usesGradingScheme(gradingType) && scheme != nil && scheme.indexOf(grade) >= 0:
			// A numeric scheme name (GPA "4") is a grade, not points.
			p, _ := scheme.GradeToPercent(grade)
			score = possible * p / 100
		default:
			score = v
		}
		parsed = true
	case lower == "complete" || lower == "pass":
		score, parsed = possible, true
	case lower == "incomplete" || lower == "fail":
		score, parsed = 0, true
	default:
		if usesGradingScheme(gradingType) || gradingType == "points" || gradingType == "percent" {
			sch := scheme
			if sch == nil {
				sch = DefaultGradingScheme()
			}
			if usesGradingScheme(gradingType) {
				if p, ok := sch.GradeToPercent(grade); ok {
					score, parsed = possible*p/100, true
				}
			}
		}
	}
	if !parsed || math.IsNaN(score) || math.IsInf(score, 0) {
		return 0, invalid
	}
	if gradingType == "pass_fail" {
		switch {
		case score == possible:
			return possible, nil
		case score == 0:
			return 0, nil
		default:
			return 0, fmt.Errorf("pass/fail assignments take complete or incomplete")
		}
	}
	return math.Round(score*1e6) / 1e6, nil
}

// FormatGrade renders a score as the assignment's grade string (Canvas
// score_to_grade). given is the grade the grader typed, used for
// point-less letter / pass-fail assignments.
func FormatGrade(a *models.Assignment, scheme *GradingScheme, score float64, given string) string {
	gradingType := "points"
	possible := 0.0
	if a != nil {
		if a.GradingType != "" {
			gradingType = a.GradingType
		}
		if a.PointsPossible != nil {
			possible = *a.PointsPossible
		}
	}
	switch gradingType {
	case "percent":
		p := 0.0
		if possible > 0 {
			p = score / possible * 100
		}
		return formatNumber(math.Round(p*100)/100) + "%"
	case "pass_fail":
		passed := false
		if possible > 0 {
			passed = score > 0
		} else {
			g := strings.ToLower(strings.TrimSpace(given))
			passed = g == "complete" || g == "pass"
		}
		if passed {
			return "complete"
		}
		return "incomplete"
	case "letter_grade", "gpa_scale":
		sch := scheme
		if sch == nil {
			sch = DefaultGradingScheme()
		}
		if possible > 0 {
			return sch.ScoreToGrade(score / possible * 100)
		}
		if strings.TrimSpace(given) != "" {
			return strings.TrimSpace(given)
		}
		return sch.ScoreToGrade(score)
	}
	return formatNumber(score)
}

func formatNumber(v float64) string {
	return strconv.FormatFloat(math.Round(v*1e6)/1e6, 'f', -1, 64)
}
