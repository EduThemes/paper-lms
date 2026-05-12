package predicates_test

import (
	"context"
	"testing"

	"github.com/EduThemes/paper-lms/internal/service/gamification/predicates"
)

func TestReputationThreshold_Above(t *testing.T) {
	actor := predicates.ActorSnapshot{
		CurrencyByCode: map[string]uint{predicates.ReputationCode: 4},
		WalletBalances: map[uint]int64{4: 25},
	}
	p := predicates.ReputationThreshold{MinAmount: 20}
	got, trace := p.Evaluate(context.Background(), actor)
	if !got {
		t.Fatalf("expected true at rep 25 with MinAmount 20")
	}
	if trace.Kind != "ReputationThreshold" {
		t.Fatalf("expected trace.Kind ReputationThreshold, got %q", trace.Kind)
	}
}

func TestReputationThreshold_Below(t *testing.T) {
	actor := predicates.ActorSnapshot{
		CurrencyByCode: map[string]uint{predicates.ReputationCode: 4},
		WalletBalances: map[uint]int64{4: 5},
	}
	p := predicates.ReputationThreshold{MinAmount: 20}
	got, _ := p.Evaluate(context.Background(), actor)
	if got {
		t.Fatalf("expected false at rep 5 with MinAmount 20")
	}
}

func TestReputationThreshold_CodeUnresolved(t *testing.T) {
	p := predicates.ReputationThreshold{MinAmount: 1}
	got, trace := p.Evaluate(context.Background(), predicates.ActorSnapshot{})
	if got {
		t.Fatalf("expected false when reputation code not in snapshot")
	}
	if trace.Kind != "ReputationThreshold" {
		t.Fatalf("expected trace.Kind ReputationThreshold even on miss, got %q", trace.Kind)
	}
}
