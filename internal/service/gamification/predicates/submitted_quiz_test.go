package predicates_test

import (
	"context"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/service/gamification/predicates"
)

func snapshotWithQuiz(quizID uint, attempt predicates.QuizState) predicates.ActorSnapshot {
	return predicates.ActorSnapshot{
		UserID:   42,
		TenantID: 1,
		Now:      time.Now(),
		QuizAttempts: map[uint]predicates.QuizState{
			quizID: attempt,
		},
	}
}

func TestSubmittedQuiz_NoSubmission(t *testing.T) {
	p := predicates.SubmittedQuiz{QuizID: 9}
	got, trace := p.Evaluate(context.Background(), predicates.ActorSnapshot{})
	if got {
		t.Fatalf("expected false when no attempt present")
	}
	if trace.Reason == "" {
		t.Fatalf("expected non-empty trace.Reason")
	}
}

func TestSubmittedQuiz_AnySubmission(t *testing.T) {
	now := time.Now()
	actor := snapshotWithQuiz(9, predicates.QuizState{
		QuizID:      9,
		SubmittedAt: ptrTime(now),
	})
	p := predicates.SubmittedQuiz{QuizID: 9}
	got, _ := p.Evaluate(context.Background(), actor)
	if !got {
		t.Fatalf("expected true for any attempt when no score bound set")
	}
}

func TestSubmittedQuiz_ScoreInRange(t *testing.T) {
	now := time.Now()
	actor := snapshotWithQuiz(9, predicates.QuizState{
		QuizID:      9,
		SubmittedAt: ptrTime(now),
		Score:       ptrFloat(85),
	})
	p := predicates.SubmittedQuiz{
		QuizID:   9,
		MinScore: ptrFloat(80),
		MaxScore: ptrFloat(100),
	}
	got, _ := p.Evaluate(context.Background(), actor)
	if !got {
		t.Fatalf("expected true for score 85 in [80,100]")
	}
}

func TestSubmittedQuiz_ScoreBelowMin(t *testing.T) {
	now := time.Now()
	actor := snapshotWithQuiz(9, predicates.QuizState{
		QuizID:      9,
		SubmittedAt: ptrTime(now),
		Score:       ptrFloat(70),
	})
	p := predicates.SubmittedQuiz{
		QuizID:   9,
		MinScore: ptrFloat(80),
	}
	got, trace := p.Evaluate(context.Background(), actor)
	if got {
		t.Fatalf("expected false for score 70 with MinScore 80")
	}
	if trace.Reason == "" {
		t.Fatalf("expected trace.Reason explaining the gap")
	}
}

func TestSubmittedQuiz_ScoreAboveMax(t *testing.T) {
	now := time.Now()
	actor := snapshotWithQuiz(9, predicates.QuizState{
		QuizID:      9,
		SubmittedAt: ptrTime(now),
		Score:       ptrFloat(105),
	})
	p := predicates.SubmittedQuiz{
		QuizID:   9,
		MaxScore: ptrFloat(100),
	}
	got, _ := p.Evaluate(context.Background(), actor)
	if got {
		t.Fatalf("expected false for score 105 with MaxScore 100")
	}
}

func TestSubmittedQuiz_MinScoreButUngraded(t *testing.T) {
	now := time.Now()
	actor := snapshotWithQuiz(9, predicates.QuizState{
		QuizID:      9,
		SubmittedAt: ptrTime(now),
		Score:       nil,
	})
	p := predicates.SubmittedQuiz{
		QuizID:   9,
		MinScore: ptrFloat(60),
	}
	got, trace := p.Evaluate(context.Background(), actor)
	if got {
		t.Fatalf("expected false when ungraded with MinScore set")
	}
	if trace.Reason == "" {
		t.Fatalf("expected trace.Reason explaining ungraded state")
	}
}
