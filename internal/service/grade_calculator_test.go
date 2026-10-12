package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

func calcAsg(id uint, group uint, points float64) models.Assignment {
	p := points
	g := group
	a := models.Assignment{ID: id, Name: "A", PointsPossible: &p, WorkflowState: models.AssignmentPublished, GradingType: "points"}
	if group != 0 {
		a.AssignmentGroupID = &g
	}
	return a
}

func graded(assignmentID uint, score float64) *models.Submission {
	s := score
	return &models.Submission{AssignmentID: assignmentID, Score: &s, WorkflowState: models.SubmissionGraded}
}

func pct(t *testing.T, v *float64) float64 {
	t.Helper()
	require.NotNil(t, v)
	return *v
}

func TestParseDropRules(t *testing.T) {
	r := ParseDropRules(`{"drop_lowest":2,"drop_highest":"1","never_drop":[4,"7"]}`)
	assert.Equal(t, DropRules{DropLowest: 2, DropHighest: 1, NeverDrop: []uint{4, 7}}, r)
	r = ParseDropRules("drop_lowest:1\ndrop_highest:0\nnever_drop:9\n")
	assert.Equal(t, DropRules{DropLowest: 1, NeverDrop: []uint{9}}, r)
	assert.True(t, ParseDropRules("{}").IsZero())
	assert.True(t, ParseDropRules("not json").IsZero())
	assert.Equal(t, `{"drop_lowest":1,"drop_highest":0,"never_drop":[]}`, MarshalDropRules(DropRules{DropLowest: 1}))
}

// G020: drop lowest removes the score that hurts the percentage most,
// which with unequal points is not always the lowest raw score.
func TestCalculate_DropLowestPicksBestSubset(t *testing.T) {
	groups := []models.AssignmentGroup{{ID: 1, Name: "Quizzes", Rules: `{"drop_lowest":1}`}}
	asgs := []models.Assignment{calcAsg(1, 1, 100), calcAsg(2, 1, 10), calcAsg(3, 1, 10)}
	subs := map[uint]*models.Submission{1: graded(1, 60), 2: graded(2, 10), 3: graded(3, 2)}
	res := CalculateCourseGrade(CourseGradeInput{Assignments: asgs, Groups: groups, Submissions: subs})
	// Keep {1,2}: 70/110=63.6%; keep {2,3}: 12/20=60%; keep {1,3}: 62/110.
	// Dropping the 100-point 60 would give 60%, so assignment 3 goes.
	assert.Equal(t, 63.64, pct(t, res.CurrentScore))
	assert.Equal(t, []uint{3}, res.Groups[0].Dropped)

	// With equal points it is simply the lowest score.
	asgs = []models.Assignment{calcAsg(1, 1, 10), calcAsg(2, 1, 10), calcAsg(3, 1, 10)}
	subs = map[uint]*models.Submission{1: graded(1, 4), 2: graded(2, 9), 3: graded(3, 7)}
	res = CalculateCourseGrade(CourseGradeInput{Assignments: asgs, Groups: groups, Submissions: subs})
	assert.Equal(t, 80.0, pct(t, res.CurrentScore))
	assert.Equal(t, []uint{1}, res.Groups[0].Dropped)
}

func TestCalculate_DropHighestAndNeverDrop(t *testing.T) {
	groups := []models.AssignmentGroup{{ID: 1, Rules: `{"drop_highest":1,"drop_lowest":1,"never_drop":[1]}`}}
	asgs := []models.Assignment{calcAsg(1, 1, 10), calcAsg(2, 1, 10), calcAsg(3, 1, 10), calcAsg(4, 1, 10)}
	subs := map[uint]*models.Submission{1: graded(1, 1), 2: graded(2, 10), 3: graded(3, 5), 4: graded(4, 7)}
	res := CalculateCourseGrade(CourseGradeInput{Assignments: asgs, Groups: groups, Submissions: subs})
	// Assignment 1 (the lowest) is never dropped; 3 (lowest droppable)
	// and 2 (highest) are dropped: (1 + 7) / 20.
	assert.Equal(t, 40.0, pct(t, res.CurrentScore))
	assert.ElementsMatch(t, []uint{2, 3}, res.Groups[0].Dropped)
}

func TestCalculate_DropKeepsAtLeastOne(t *testing.T) {
	groups := []models.AssignmentGroup{{ID: 1, Rules: `{"drop_lowest":5}`}}
	asgs := []models.Assignment{calcAsg(1, 1, 10), calcAsg(2, 1, 10)}
	subs := map[uint]*models.Submission{1: graded(1, 3), 2: graded(2, 8)}
	res := CalculateCourseGrade(CourseGradeInput{Assignments: asgs, Groups: groups, Submissions: subs})
	assert.Equal(t, 80.0, pct(t, res.CurrentScore))
}

// Current counts graded work only; final treats ungraded as zero, and the
// final drop applies to the zeros.
func TestCalculate_CurrentVsFinalWithDrops(t *testing.T) {
	groups := []models.AssignmentGroup{{ID: 1, Rules: `{"drop_lowest":1}`}}
	asgs := []models.Assignment{calcAsg(1, 1, 10), calcAsg(2, 1, 10), calcAsg(3, 1, 10)}
	subs := map[uint]*models.Submission{1: graded(1, 6), 2: graded(2, 8)}
	res := CalculateCourseGrade(CourseGradeInput{Assignments: asgs, Groups: groups, Submissions: subs})
	assert.Equal(t, 80.0, pct(t, res.CurrentScore)) // drops the 6
	assert.Equal(t, 70.0, pct(t, res.FinalScore))   // drops the missing 0
	assert.Equal(t, []uint{3}, res.Groups[0].FinalDropped)
}

// G021: excused work leaves both earned and possible; not_graded and
// unpublished work never count; 0-point work is extra credit.
func TestCalculate_ExcusedNotGradedExtraCredit(t *testing.T) {
	asgs := []models.Assignment{calcAsg(1, 0, 10), calcAsg(2, 0, 10), calcAsg(3, 0, 10), calcAsg(4, 0, 0), calcAsg(5, 0, 10)}
	asgs[2].GradingType = "not_graded"
	asgs[4].WorkflowState = models.AssignmentUnpublished
	excused := &models.Submission{AssignmentID: 2, Excused: true}
	subs := map[uint]*models.Submission{1: graded(1, 8), 2: excused, 3: graded(3, 10), 4: graded(4, 1), 5: graded(5, 0)}
	res := CalculateCourseGrade(CourseGradeInput{Assignments: asgs, Submissions: subs})
	assert.Equal(t, 90.0, pct(t, res.CurrentScore)) // (8+1)/10
	assert.Equal(t, 90.0, pct(t, res.FinalScore))
}

// G019: an assignment not assigned to the student does not count.
func TestCalculate_UnassignedDoesNotCount(t *testing.T) {
	asgs := []models.Assignment{calcAsg(1, 0, 10), calcAsg(2, 0, 10)}
	subs := map[uint]*models.Submission{1: graded(1, 10)}
	res := CalculateCourseGrade(CourseGradeInput{Assignments: asgs, Submissions: subs, Unassigned: map[uint]bool{2: true}})
	assert.Equal(t, 100.0, pct(t, res.FinalScore))
}

func TestCalculate_WeightedGroupsScaleUpBelow100(t *testing.T) {
	groups := []models.AssignmentGroup{{ID: 1, GroupWeight: 30}, {ID: 2, GroupWeight: 30}}
	asgs := []models.Assignment{calcAsg(1, 1, 10), calcAsg(2, 2, 10)}
	subs := map[uint]*models.Submission{1: graded(1, 10), 2: graded(2, 5)}
	res := CalculateCourseGrade(CourseGradeInput{Assignments: asgs, Groups: groups, Submissions: subs, ApplyGroupWeights: true})
	assert.Equal(t, 75.0, pct(t, res.CurrentScore))

	// Weights over 100 are not scaled down (extra-credit groups).
	groups = []models.AssignmentGroup{{ID: 1, GroupWeight: 100}, {ID: 2, GroupWeight: 10}}
	subs = map[uint]*models.Submission{1: graded(1, 10), 2: graded(2, 10)}
	res = CalculateCourseGrade(CourseGradeInput{Assignments: asgs, Groups: groups, Submissions: subs, ApplyGroupWeights: true})
	assert.Equal(t, 110.0, pct(t, res.CurrentScore))
}

func TestCalculate_GradingPeriods(t *testing.T) {
	w1, w2 := 40.0, 60.0
	jan := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	periods := []models.GradingPeriod{
		{ID: 10, Title: "Q1", StartDate: jan, EndDate: jan.AddDate(0, 3, 0), Weight: &w1},
		{ID: 11, Title: "Q2", StartDate: jan.AddDate(0, 3, 0), EndDate: jan.AddDate(0, 6, 0), Weight: &w2},
	}
	asgs := []models.Assignment{calcAsg(1, 0, 10), calcAsg(2, 0, 30)}
	subs := map[uint]*models.Submission{1: graded(1, 10), 2: graded(2, 15)}
	in := CourseGradeInput{Assignments: asgs, Submissions: subs, Periods: periods, PeriodOf: map[uint]uint{1: 10, 2: 11}}

	res := CalculateCourseGrade(in)
	require.Len(t, res.Periods, 2)
	assert.Equal(t, 100.0, pct(t, res.Periods[0].CurrentScore))
	assert.Equal(t, 50.0, pct(t, res.Periods[1].CurrentScore))
	// Unweighted set: the total is over every assignment.
	assert.Equal(t, 62.5, pct(t, res.CurrentScore))

	in.WeightedPeriods = true
	res = CalculateCourseGrade(in)
	assert.Equal(t, 70.0, pct(t, res.CurrentScore)) // 100*.4 + 50*.6

	// Only Q1 graded: the current grade scales Q1's weight up to 100.
	in.Submissions = map[uint]*models.Submission{1: graded(1, 8)}
	res = CalculateCourseGrade(in)
	assert.Equal(t, 80.0, pct(t, res.CurrentScore))
	assert.Equal(t, 32.0, pct(t, res.FinalScore)) // 80*.4 + 0*.6
}

// The What-If calculator (web/src/utils/gradeSummary.js computeTotals) is
// a port of this engine; gradeSummary.test.js pins the same fixtures to
// the same numbers, so What-If with unchanged scores reproduces the
// server total.
func TestCalculate_WhatIfTwinFixtures(t *testing.T) {
	// Weighted, weights 70 + 50 = 120 used as is; never_drop in the drop
	// decision; a pending_review partial score is not graded yet.
	groups := []models.AssignmentGroup{
		{ID: 1, Name: "Homework", GroupWeight: 70, Rules: `{"drop_lowest":1,"never_drop":[1]}`},
		{ID: 2, Name: "Tests", GroupWeight: 50},
	}
	asgs := []models.Assignment{calcAsg(1, 1, 10), calcAsg(2, 1, 10), calcAsg(3, 1, 10), calcAsg(4, 2, 10), calcAsg(5, 2, 10)}
	pending := graded(5, 4)
	pending.WorkflowState = models.SubmissionPendingReview
	subs := map[uint]*models.Submission{1: graded(1, 2), 2: graded(2, 3), 3: graded(3, 9), 4: graded(4, 8), 5: pending}
	res := CalculateCourseGrade(CourseGradeInput{Assignments: asgs, Groups: groups, Submissions: subs, ApplyGroupWeights: true})
	assert.Equal(t, 78.5, pct(t, res.CurrentScore))
	assert.Equal(t, 58.5, pct(t, res.FinalScore))
	assert.Equal(t, []uint{2}, res.Groups[0].Dropped)

	// Weights over 100 are not normalised (Canvas): 0.9*80 + 0.6*40 = 96.
	groups = []models.AssignmentGroup{{ID: 1, Name: "A", GroupWeight: 80}, {ID: 2, Name: "B", GroupWeight: 40}}
	asgs = []models.Assignment{calcAsg(1, 1, 10), calcAsg(2, 2, 10)}
	subs = map[uint]*models.Submission{1: graded(1, 9), 2: graded(2, 6)}
	res = CalculateCourseGrade(CourseGradeInput{Assignments: asgs, Groups: groups, Submissions: subs, ApplyGroupWeights: true})
	assert.Equal(t, 96.0, pct(t, res.CurrentScore))

	// The never-drop 0/100 counts in the ratio, so the 10/10 is dropped:
	// (0+50)/200 = 25%.
	groups = []models.AssignmentGroup{{ID: 1, Name: "A", Rules: `{"drop_lowest":1,"never_drop":[1]}`}}
	asgs = []models.Assignment{calcAsg(1, 1, 100), calcAsg(2, 1, 10), calcAsg(3, 1, 100)}
	subs = map[uint]*models.Submission{1: graded(1, 0), 2: graded(2, 10), 3: graded(3, 50)}
	res = CalculateCourseGrade(CourseGradeInput{Assignments: asgs, Groups: groups, Submissions: subs})
	assert.Equal(t, 25.0, pct(t, res.CurrentScore))

	// drop_lowest 1 + drop_highest 2 on three items: drop_highest is
	// ignored, so the best two are kept: (10+8)/20 = 90%.
	groups = []models.AssignmentGroup{{ID: 1, Name: "A", Rules: `{"drop_lowest":1,"drop_highest":2}`}}
	asgs = []models.Assignment{calcAsg(1, 1, 10), calcAsg(2, 1, 10), calcAsg(3, 1, 10)}
	subs = map[uint]*models.Submission{1: graded(1, 10), 2: graded(2, 8), 3: graded(3, 2)}
	res = CalculateCourseGrade(CourseGradeInput{Assignments: asgs, Groups: groups, Submissions: subs})
	assert.Equal(t, 90.0, pct(t, res.CurrentScore))
}
