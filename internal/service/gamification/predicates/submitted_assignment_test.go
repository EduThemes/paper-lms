package predicates_test

import (
	"context"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/service/gamification/predicates"
)

func snapshotWithSubmission(assignmentID uint, sub predicates.SubmissionState) predicates.ActorSnapshot {
	return predicates.ActorSnapshot{
		UserID:   42,
		TenantID: 1,
		Now:      time.Now(),
		Submissions: map[uint]predicates.SubmissionState{
			assignmentID: sub,
		},
	}
}

func ptrFloat(f float64) *float64 { return &f }
func ptrTime(t time.Time) *time.Time { return &t }

func TestSubmittedAssignment_NoSubmission(t *testing.T) {
	p := predicates.SubmittedAssignment{AssignmentID: 7}
	got, trace := p.Evaluate(context.Background(), predicates.ActorSnapshot{})
	if got {
		t.Fatalf("expected false when no submission present")
	}
	if trace.Reason == "" {
		t.Fatalf("expected a non-empty trace.Reason explaining the failure")
	}
}

func TestSubmittedAssignment_AnySubmission(t *testing.T) {
	now := time.Now()
	actor := snapshotWithSubmission(7, predicates.SubmissionState{
		AssignmentID: 7,
		SubmittedAt:  ptrTime(now),
		OnTime:       true,
	})
	p := predicates.SubmittedAssignment{AssignmentID: 7}
	got, _ := p.Evaluate(context.Background(), actor)
	if !got {
		t.Fatalf("expected true for any submission when no score bound set")
	}
}

func TestSubmittedAssignment_ScoreInRange(t *testing.T) {
	now := time.Now()
	actor := snapshotWithSubmission(7, predicates.SubmissionState{
		AssignmentID: 7,
		SubmittedAt:  ptrTime(now),
		Score:        ptrFloat(85),
		OnTime:       true,
	})
	p := predicates.SubmittedAssignment{
		AssignmentID: 7,
		MinScore:     ptrFloat(80),
		MaxScore:     ptrFloat(100),
	}
	got, _ := p.Evaluate(context.Background(), actor)
	if !got {
		t.Fatalf("expected true for score 85 in [80,100]")
	}
}

func TestSubmittedAssignment_ScoreBelowMin(t *testing.T) {
	now := time.Now()
	actor := snapshotWithSubmission(7, predicates.SubmissionState{
		AssignmentID: 7,
		SubmittedAt:  ptrTime(now),
		Score:        ptrFloat(70),
	})
	p := predicates.SubmittedAssignment{
		AssignmentID: 7,
		MinScore:     ptrFloat(80),
	}
	got, trace := p.Evaluate(context.Background(), actor)
	if got {
		t.Fatalf("expected false for score 70 with MinScore 80")
	}
	if trace.Reason == "" {
		t.Fatalf("expected trace.Reason explaining the score gap")
	}
}

func TestSubmittedAssignment_ScoreAboveMax(t *testing.T) {
	now := time.Now()
	actor := snapshotWithSubmission(7, predicates.SubmissionState{
		AssignmentID: 7,
		SubmittedAt:  ptrTime(now),
		Score:        ptrFloat(105),
	})
	p := predicates.SubmittedAssignment{
		AssignmentID: 7,
		MaxScore:     ptrFloat(100),
	}
	got, _ := p.Evaluate(context.Background(), actor)
	if got {
		t.Fatalf("expected false for score 105 with MaxScore 100")
	}
}

func TestSubmittedAssignment_RequireOnTime_LateRejected(t *testing.T) {
	now := time.Now()
	actor := snapshotWithSubmission(7, predicates.SubmissionState{
		AssignmentID: 7,
		SubmittedAt:  ptrTime(now),
		OnTime:       false,
	})
	p := predicates.SubmittedAssignment{
		AssignmentID:  7,
		RequireOnTime: true,
	}
	got, trace := p.Evaluate(context.Background(), actor)
	if got {
		t.Fatalf("expected false for late submission with RequireOnTime")
	}
	if trace.Reason == "" {
		t.Fatalf("expected trace.Reason explaining late rejection")
	}
}

func TestSubmittedAssignment_MinScoreButUngraded(t *testing.T) {
	now := time.Now()
	actor := snapshotWithSubmission(7, predicates.SubmissionState{
		AssignmentID: 7,
		SubmittedAt:  ptrTime(now),
		Score:        nil, // ungraded
	})
	p := predicates.SubmittedAssignment{
		AssignmentID: 7,
		MinScore:     ptrFloat(60),
	}
	got, trace := p.Evaluate(context.Background(), actor)
	if got {
		t.Fatalf("expected false when score bound is set but submission ungraded")
	}
	if trace.Reason == "" {
		t.Fatalf("expected trace.Reason explaining the ungraded state")
	}
}
