package service

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// G150: every calculation method matches Canvas rollup_score.rb. Scores are
// given newest first: 4 (newest), 2, 3 (oldest).
func TestCalculateRollupScore_AllMethodsMatchCanvas(t *testing.T) {
	rs := resultsNewestFirst(4, 2, 3)
	cases := []struct {
		method string
		n      int
		want   float64
		ok     bool
	}{
		{"latest", 0, 4, true},
		{"highest", 0, 4, true},
		{"average", 0, 3, true},
		// Canvas decaying_average_set / weighted_average: 65% newest +
		// 35% mean of the rest = 0.65*4 + 0.35*2.5.
		{"decaying_average", 65, 3.48, true},
		{"weighted_average", 65, 3.48, true},
		// Canvas standard_decaying_average folds oldest→newest:
		// (3*0.35 + 2*0.65) = 2.35; 2.35*0.35 + 4*0.65 = 3.4225.
		{"standard_decaying_average", 65, 3.42, true},
		// n_mastery (mastery 3): mastering scores are 4 and 3; n=2 ok.
		{"n_mastery", 2, 3.5, true},
		{"n_mastery", 3, 0, false},
	}
	for _, tc := range cases {
		got, ok := calculateRollupScore(rs, tc.method, tc.n, 3)
		assert.Equal(t, tc.ok, ok, tc.method)
		if tc.ok {
			assert.InDelta(t, tc.want, got, 0.001, "%s n=%d", tc.method, tc.n)
		}
	}
}

func TestNormalizeOutcomeCalculation(t *testing.T) {
	o := &models.LearningOutcome{CalculationMethod: "n_mastery"}
	require.NoError(t, NormalizeOutcomeCalculation(o))
	assert.Equal(t, 5, o.CalculationInt, "Canvas default n")

	o = &models.LearningOutcome{CalculationMethod: "standard_decaying_average"}
	require.NoError(t, NormalizeOutcomeCalculation(o))
	assert.Equal(t, 65, o.CalculationInt)

	for _, bad := range []*models.LearningOutcome{
		{CalculationMethod: "median"},
		{CalculationMethod: "n_mastery", CalculationInt: 65},
		{CalculationMethod: "decaying_average", CalculationInt: 100},
		{CalculationMethod: "latest", MasteryPoints: -1},
	} {
		err := NormalizeOutcomeCalculation(bad)
		assert.True(t, errors.Is(err, ErrInvalidOutcomeCalculation), "%+v", bad)
	}
	require.NoError(t, NormalizeOutcomeCalculation(&models.LearningOutcome{CalculationMethod: "average", CalculationInt: 65}))
}

func TestDefaultOutcomeRatings(t *testing.T) {
	o := &models.LearningOutcome{}
	defaultOutcomeRatings(o)
	defaultPointsPossibleFromRatings(o)
	assert.Equal(t, 5.0, o.PointsPossible)

	kept := &models.LearningOutcome{RatingsData: `[{"points":4}]`}
	defaultOutcomeRatings(kept)
	assert.Equal(t, `[{"points":4}]`, kept.RatingsData)
}
