package predicates_test

import (
	"context"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/service/gamification/predicates"
)

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

func TestViewedContent_HasView(t *testing.T) {
	actor := predicates.ActorSnapshot{
		ViewedContent: map[uint]time.Time{17: time.Now()},
	}
	p := predicates.ViewedContent{ContentID: 17}
	got, _ := p.Evaluate(context.Background(), actor)
	if !got {
		t.Fatalf("expected true when view recorded")
	}
}

func TestViewedContent_MinSecondsAnnotatedButPassthrough(t *testing.T) {
	// Wave 1 records the gap in trace.Reason but returns true on presence.
	actor := predicates.ActorSnapshot{
		ViewedContent: map[uint]time.Time{17: time.Now()},
	}
	p := predicates.ViewedContent{ContentID: 17, MinSecondsViewed: 30}
	got, trace := p.Evaluate(context.Background(), actor)
	if !got {
		t.Fatalf("expected true on presence even with MinSecondsViewed set in Wave 1")
	}
	if trace.Reason == "" {
		t.Fatalf("expected trace.Reason flagging the duration-tracking gap")
	}
}
