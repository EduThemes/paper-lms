package service

import (
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// A point-less pass/fail grade is labelled by the typed grade; without one
// (a machine score) any positive score passes, as in Canvas score_to_grade.
func TestFormatGrade_PointlessPassFail(t *testing.T) {
	zero := 0.0
	a := &models.Assignment{GradingType: "pass_fail", PointsPossible: &zero}
	cases := []struct {
		score float64
		given string
		want  string
	}{
		{0, "complete", "complete"},
		{0, "incomplete", "incomplete"},
		{0, "", "incomplete"},
		{1, "", "complete"},
		{1, "incomplete", "incomplete"},
	}
	for _, c := range cases {
		if got := FormatGrade(a, nil, c.score, c.given); got != c.want {
			t.Errorf("FormatGrade(pf0, %v, %q) = %q, want %q", c.score, c.given, got, c.want)
		}
	}
}
