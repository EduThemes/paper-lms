package predicates_test

import (
	"context"
	"testing"

	"github.com/EduThemes/paper-lms/internal/service/gamification/predicates"
)

func TestEarnedBadge_Present(t *testing.T) {
	actor := predicates.ActorSnapshot{EarnedBadges: []uint{1, 2, 3}}
	p := predicates.EarnedBadge{BadgeID: 2}
	got, _ := p.Evaluate(context.Background(), actor)
	if !got {
		t.Fatalf("expected true when badge present")
	}
}

func TestEarnedBadge_Absent(t *testing.T) {
	actor := predicates.ActorSnapshot{EarnedBadges: []uint{1, 2, 3}}
	p := predicates.EarnedBadge{BadgeID: 99}
	got, trace := p.Evaluate(context.Background(), actor)
	if got {
		t.Fatalf("expected false when badge absent")
	}
	if trace.Reason == "" {
		t.Fatalf("expected trace.Reason")
	}
}

func TestEarnedBadge_EmptySlice(t *testing.T) {
	p := predicates.EarnedBadge{BadgeID: 1}
	got, _ := p.Evaluate(context.Background(), predicates.ActorSnapshot{})
	if got {
		t.Fatalf("expected false when slice empty")
	}
}
