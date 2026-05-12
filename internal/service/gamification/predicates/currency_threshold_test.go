package predicates_test

import (
	"context"
	"testing"

	"github.com/EduThemes/paper-lms/internal/service/gamification/predicates"
)

func TestCurrencyThreshold_UnresolvedCode(t *testing.T) {
	actor := predicates.ActorSnapshot{}
	p := predicates.CurrencyThreshold{Code: "xp", MinAmount: 100}
	got, trace := p.Evaluate(context.Background(), actor)
	if got {
		t.Fatalf("expected false when code unresolved")
	}
	if trace.Reason == "" {
		t.Fatalf("expected trace.Reason")
	}
}

func TestCurrencyThreshold_BalanceAtThreshold(t *testing.T) {
	actor := predicates.ActorSnapshot{
		CurrencyByCode: map[string]uint{"xp": 7},
		WalletBalances: map[uint]int64{7: 100},
	}
	p := predicates.CurrencyThreshold{Code: "xp", MinAmount: 100}
	got, _ := p.Evaluate(context.Background(), actor)
	if !got {
		t.Fatalf("expected true at threshold")
	}
}

func TestCurrencyThreshold_BalanceAbove(t *testing.T) {
	actor := predicates.ActorSnapshot{
		CurrencyByCode: map[string]uint{"xp": 7},
		WalletBalances: map[uint]int64{7: 1000},
	}
	p := predicates.CurrencyThreshold{Code: "xp", MinAmount: 100}
	got, _ := p.Evaluate(context.Background(), actor)
	if !got {
		t.Fatalf("expected true above threshold")
	}
}

func TestCurrencyThreshold_BalanceBelow(t *testing.T) {
	actor := predicates.ActorSnapshot{
		CurrencyByCode: map[string]uint{"xp": 7},
		WalletBalances: map[uint]int64{7: 50},
	}
	p := predicates.CurrencyThreshold{Code: "xp", MinAmount: 100}
	got, trace := p.Evaluate(context.Background(), actor)
	if got {
		t.Fatalf("expected false below threshold")
	}
	if trace.Reason == "" {
		t.Fatalf("expected trace.Reason explaining the gap")
	}
}

func TestCurrencyThreshold_ZeroBalanceWhenMissing(t *testing.T) {
	// Code is resolved but WalletBalances has no row for the id — treat as 0.
	actor := predicates.ActorSnapshot{
		CurrencyByCode: map[string]uint{"xp": 7},
		WalletBalances: map[uint]int64{},
	}
	p := predicates.CurrencyThreshold{Code: "xp", MinAmount: 1}
	got, _ := p.Evaluate(context.Background(), actor)
	if got {
		t.Fatalf("expected false when balance row missing")
	}
}
