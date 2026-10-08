package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

func fptr(f float64) *float64 { return &f }

// resultsNewestFirst builds results whose timestamps descend in slice order,
// so index 0 is the newest.
func resultsNewestFirst(scores ...float64) []models.LearningOutcomeResult {
	base := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	out := make([]models.LearningOutcomeResult, len(scores))
	for i, s := range scores {
		at := base.Add(-time.Duration(i) * time.Hour)
		out[i] = models.LearningOutcomeResult{ID: uint(i + 1), Score: fptr(s), AssessedAt: &at}
	}
	return out
}

// B194: n_mastery keeps only mastering scores (Canvas rollup_score.rb
// n_mastery_set) instead of averaging the newest n.
func TestCalculateRollupScore_NMasteryMatchesCanvas(t *testing.T) {
	score, ok := calculateRollupScore(resultsNewestFirst(1, 4, 4), "n_mastery", 2, 3)
	require.True(t, ok)
	assert.Equal(t, 4.0, score)

	// Only one score reaches mastery -> no score (Canvas returns nil).
	_, ok = calculateRollupScore(resultsNewestFirst(1, 4, 2), "n_mastery", 2, 3)
	assert.False(t, ok)
}

// B194: "average" is offered in the UI; it must average every score instead
// of falling through to "latest".
func TestCalculateRollupScore_Average(t *testing.T) {
	score, ok := calculateRollupScore(resultsNewestFirst(1, 2, 3), "average", 0, 3)
	require.True(t, ok)
	assert.Equal(t, 2.0, score)
}

func TestCalculateRollupScore_OrderIndependent(t *testing.T) {
	rs := resultsNewestFirst(4, 1)
	// Reverse the slice: the newest result is now last.
	rs[0], rs[1] = rs[1], rs[0]
	score, ok := calculateRollupScore(rs, "latest", 0, 3)
	require.True(t, ok)
	assert.Equal(t, 4.0, score)

	// decaying_average 65: 0.65*newest + 0.35*mean(older)
	score, ok = calculateRollupScore(rs, "decaying_average", 65, 3)
	require.True(t, ok)
	assert.Equal(t, 2.95, score)
}

// B090: a 10/100 assignment score aligned to a 3-of-5 outcome scales to 0.5
// on the outcome scale and is not mastery.
func TestScaledOutcomeScore_UsesOutcomeScale(t *testing.T) {
	outcome := &models.LearningOutcome{PointsPossible: 5, MasteryPoints: 3}
	r := &models.LearningOutcomeResult{Score: fptr(10), Possible: fptr(100)}
	assert.InDelta(t, 0.5, ScaledOutcomeScore(r, outcome), 1e-9)

	r = &models.LearningOutcomeResult{Score: fptr(90), Possible: fptr(100)}
	assert.InDelta(t, 4.5, ScaledOutcomeScore(r, outcome), 1e-9)

	// No possible: the score is already on the outcome scale.
	r = &models.LearningOutcomeResult{Score: fptr(2)}
	assert.Equal(t, 2.0, ScaledOutcomeScore(r, outcome))
}

func TestDefaultPointsPossibleFromRatings(t *testing.T) {
	o := &models.LearningOutcome{RatingsData: `[{"description":"Exceeds","points":5},{"description":"Meets","points":3}]`}
	defaultPointsPossibleFromRatings(o)
	assert.Equal(t, 5.0, o.PointsPossible)

	o = &models.LearningOutcome{PointsPossible: 4, RatingsData: `[{"points":10}]`}
	defaultPointsPossibleFromRatings(o)
	assert.Equal(t, 4.0, o.PointsPossible, "explicit points_possible wins")
}

// --- CreateResult (B090) with minimal fakes ---------------------------------

type rollupFakeOutcomeRepo struct {
	repository.LearningOutcomeRepository
	byID     map[uint]*models.LearningOutcome
	byCourse []models.LearningOutcome
}

func (f *rollupFakeOutcomeRepo) FindByID(ctx context.Context, id, accountID uint) (*models.LearningOutcome, error) {
	if o, ok := f.byID[id]; ok {
		return o, nil
	}
	return nil, errors.New("not found")
}

func (f *rollupFakeOutcomeRepo) ListByContext(ctx context.Context, contextType string, contextID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LearningOutcome], error) {
	return &repository.PaginatedResult[models.LearningOutcome]{Items: f.byCourse, TotalCount: int64(len(f.byCourse))}, nil
}

type rollupFakeResultRepo struct {
	repository.LearningOutcomeResultRepository
	saved     []*models.LearningOutcomeResult
	byOutcome map[uint][]models.LearningOutcomeResult
}

func (f *rollupFakeResultRepo) Upsert(ctx context.Context, r *models.LearningOutcomeResult) (*bool, error) {
	f.saved = append(f.saved, r)
	return nil, nil
}

func (f *rollupFakeResultRepo) ListByOutcomeID(ctx context.Context, outcomeID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LearningOutcomeResult], error) {
	items := f.byOutcome[outcomeID]
	return &repository.PaginatedResult[models.LearningOutcomeResult]{Items: items, TotalCount: int64(len(items))}, nil
}

func TestCreateResult_ScalesRawAssignmentPointsBeforeMastery(t *testing.T) {
	outcomes := &rollupFakeOutcomeRepo{byID: map[uint]*models.LearningOutcome{
		7: {ID: 7, PointsPossible: 5, MasteryPoints: 3},
	}}
	results := &rollupFakeResultRepo{}
	svc := NewLearningOutcomeService(nil, outcomes, results)

	low := &models.LearningOutcomeResult{UserID: 1, LearningOutcomeID: 7, Score: fptr(10), Possible: fptr(100)}
	require.NoError(t, svc.CreateResult(context.Background(), low))
	require.NotNil(t, low.Mastery)
	assert.False(t, *low.Mastery, "10/100 must not master a 3-of-5 outcome")

	high := &models.LearningOutcomeResult{UserID: 1, LearningOutcomeID: 7, Score: fptr(80), Possible: fptr(100)}
	require.NoError(t, svc.CreateResult(context.Background(), high))
	require.NotNil(t, high.Mastery)
	assert.True(t, *high.Mastery, "80/100 = 4 of 5 masters a 3-of-5 outcome")
}

// B090: the course rollup must scale too, not compare raw points.
func TestGetMasteryGradebook_RollupScaledToOutcome(t *testing.T) {
	outcome := models.LearningOutcome{ID: 7, PointsPossible: 5, MasteryPoints: 3, CalculationMethod: "latest"}
	outcomes := &rollupFakeOutcomeRepo{byCourse: []models.LearningOutcome{outcome}}
	results := &rollupFakeResultRepo{byOutcome: map[uint][]models.LearningOutcomeResult{
		7: {{UserID: 1, LearningOutcomeID: 7, Score: fptr(10), Possible: fptr(100)}},
	}}
	svc := NewLearningOutcomeService(nil, outcomes, results)

	rollups, _, err := svc.GetMasteryGradebook(context.Background(), 1, 1, repository.PaginationParams{Page: 1, PerPage: 10})
	require.NoError(t, err)
	require.Len(t, rollups, 1)
	require.NotNil(t, rollups[0].Scores[0].Score)
	assert.InDelta(t, 0.5, *rollups[0].Scores[0].Score, 1e-9)
	assert.False(t, *rollups[0].Scores[0].Mastery)
}

// B195: paging through the rollups must be deterministic and cover every
// student exactly once.
func TestGetMasteryGradebook_StablePagination(t *testing.T) {
	outcome := models.LearningOutcome{ID: 7, PointsPossible: 5, MasteryPoints: 3}
	var rs []models.LearningOutcomeResult
	for uid := uint(1); uid <= 25; uid++ {
		rs = append(rs, models.LearningOutcomeResult{UserID: uid, LearningOutcomeID: 7, Score: fptr(4)})
	}
	svc := NewLearningOutcomeService(nil,
		&rollupFakeOutcomeRepo{byCourse: []models.LearningOutcome{outcome}},
		&rollupFakeResultRepo{byOutcome: map[uint][]models.LearningOutcomeResult{7: rs}})

	seen := map[uint]int{}
	for page := 1; page <= 3; page++ {
		var first []uint
		for attempt := 0; attempt < 5; attempt++ {
			rollups, _, err := svc.GetMasteryGradebook(context.Background(), 1, 1, repository.PaginationParams{Page: page, PerPage: 10})
			require.NoError(t, err)
			var ids []uint
			for _, r := range rollups {
				ids = append(ids, r.StudentID)
			}
			if attempt == 0 {
				first = ids
				for _, id := range ids {
					seen[id]++
				}
			} else {
				assert.Equal(t, first, ids, fmt.Sprintf("page %d must be stable", page))
			}
		}
	}
	assert.Len(t, seen, 25)
	for id, n := range seen {
		assert.Equal(t, 1, n, "student %d appeared on more than one page", id)
	}
}

// B091: a 12/100 raw score must not rate "Exceeds Mastery" on a 0-4 scale.
func TestMasteryCellScore_RatesOnScaleRange(t *testing.T) {
	scale := &models.OutcomeProficiency{Ratings: models.DefaultProficiencyRatings()}
	svc := &MasteryGradebookService{}
	outcome := models.LearningOutcome{ID: 7, PointsPossible: 5, MasteryPoints: 3, CalculationMethod: "latest"}

	score, possible, rating := svc.masteryCellScore(
		[]models.LearningOutcomeResult{{Score: fptr(12), Possible: fptr(100)}}, outcome, scale)
	require.NotNil(t, score)
	require.NotNil(t, rating)
	var maxPts float64
	for _, r := range scale.Ratings {
		if r.Points > maxPts {
			maxPts = r.Points
		}
	}
	require.NotNil(t, possible)
	assert.Equal(t, maxPts, *possible)
	assert.InDelta(t, 0.12*maxPts, *score, 0.01)
	assert.False(t, rating.Mastery)
	assert.NotEqual(t, maxPts, rating.Points, "12%% must not land on the top rating")

	score, _, rating = svc.masteryCellScore(
		[]models.LearningOutcomeResult{{Score: fptr(100), Possible: fptr(100)}}, outcome, scale)
	require.NotNil(t, score)
	assert.Equal(t, maxPts, rating.Points)
}
