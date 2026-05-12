package predicates_test

import (
	"context"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/service/gamification/predicates"
)

func contentViewState(id uint, count int, totalSeconds int64) predicates.ContentViewState {
	now := time.Now()
	return predicates.ContentViewState{
		ObjectID:      id,
		ViewCount:     count,
		TotalSeconds:  totalSeconds,
		FirstViewedAt: now.Add(-time.Hour),
		LastViewedAt:  now,
	}
}

func TestViewedContent_NoView(t *testing.T) {
	p := predicates.ViewedContent{ContentID: 17}
	got, trace := p.Evaluate(context.Background(), predicates.ActorSnapshot{})
	if got {
		t.Fatalf("expected false when no view recorded")
	}
	if trace.Reason == "" {
		t.Fatalf("expected trace.Reason")
	}
}

func TestViewedContent_PresenceCountsAsOneView(t *testing.T) {
	actor := predicates.ActorSnapshot{
		ViewedContent: map[uint]predicates.ContentViewState{
			17: contentViewState(17, 1, 0),
		},
	}
	p := predicates.ViewedContent{ContentID: 17}
	got, _ := p.Evaluate(context.Background(), actor)
	if !got {
		t.Fatalf("expected true on single view with default MinViews=1")
	}
}

func TestViewedContent_MinViewsGate(t *testing.T) {
	actor := predicates.ActorSnapshot{
		ViewedContent: map[uint]predicates.ContentViewState{
			17: contentViewState(17, 2, 0),
		},
	}
	p := predicates.ViewedContent{ContentID: 17, MinViews: 3}
	got, trace := p.Evaluate(context.Background(), actor)
	if got {
		t.Fatalf("expected false at 2 views when MinViews=3")
	}
	if trace.Reason == "" {
		t.Fatalf("expected trace.Reason explaining count gap")
	}
}

func TestViewedContent_MinViewsMet(t *testing.T) {
	actor := predicates.ActorSnapshot{
		ViewedContent: map[uint]predicates.ContentViewState{
			17: contentViewState(17, 5, 0),
		},
	}
	p := predicates.ViewedContent{ContentID: 17, MinViews: 3}
	got, _ := p.Evaluate(context.Background(), actor)
	if !got {
		t.Fatalf("expected true at 5 views with MinViews=3")
	}
}

func TestViewedContent_MinSecondsGate(t *testing.T) {
	actor := predicates.ActorSnapshot{
		ViewedContent: map[uint]predicates.ContentViewState{
			17: contentViewState(17, 1, 20),
		},
	}
	p := predicates.ViewedContent{ContentID: 17, MinSecondsViewed: 60}
	got, trace := p.Evaluate(context.Background(), actor)
	if got {
		t.Fatalf("expected false at 20 cumulative seconds when MinSecondsViewed=60")
	}
	if trace.Reason == "" {
		t.Fatalf("expected trace.Reason explaining duration gap")
	}
}

func TestViewedContent_MinSecondsMet(t *testing.T) {
	actor := predicates.ActorSnapshot{
		ViewedContent: map[uint]predicates.ContentViewState{
			17: contentViewState(17, 1, 90),
		},
	}
	p := predicates.ViewedContent{ContentID: 17, MinSecondsViewed: 60}
	got, _ := p.Evaluate(context.Background(), actor)
	if !got {
		t.Fatalf("expected true at 90 cumulative seconds with MinSecondsViewed=60")
	}
}

func TestViewedContent_Needs(t *testing.T) {
	p := predicates.ViewedContent{ContentID: 17}
	needs := p.Needs()
	if len(needs.ContentIDs) != 1 || needs.ContentIDs[0] != 17 {
		t.Fatalf("expected Needs.ContentIDs=[17], got %v", needs.ContentIDs)
	}
}
