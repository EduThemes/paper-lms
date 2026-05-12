package predicates_test

import (
	"context"
	"testing"

	"github.com/EduThemes/paper-lms/internal/service/gamification/predicates"
)

// stubPredicate is a test double that records whether Evaluate was called.
// It lets us assert ConditionSet's short-circuit behavior — a predicate
// past a short-circuit point should NOT see Evaluate.
type stubPredicate struct {
	result bool
	called *bool
}

func (s stubPredicate) Kind() string { return "Stub" }
func (s stubPredicate) Evaluate(_ context.Context, _ predicates.ActorSnapshot) (bool, predicates.Trace) {
	if s.called != nil {
		*s.called = true
	}
	return s.result, predicates.Trace{Kind: "Stub", Result: s.result}
}

func newCalled() *bool {
	b := false
	return &b
}

func TestConditionSet_AND_AllTrue(t *testing.T) {
	cs := predicates.ConditionSet{
		Op: predicates.OpAND,
		Children: []predicates.Predicate{
			stubPredicate{result: true},
			stubPredicate{result: true},
			stubPredicate{result: true},
		},
	}
	got, trace := cs.Evaluate(context.Background(), predicates.ActorSnapshot{})
	if !got {
		t.Fatalf("AND of three trues should be true, got false")
	}
	if len(trace.Children) != 3 {
		t.Fatalf("expected all 3 children visited, got %d", len(trace.Children))
	}
}

func TestConditionSet_AND_ShortCircuitsOnFalse(t *testing.T) {
	thirdCalled := newCalled()
	cs := predicates.ConditionSet{
		Op: predicates.OpAND,
		Children: []predicates.Predicate{
			stubPredicate{result: true},
			stubPredicate{result: false},
			stubPredicate{result: true, called: thirdCalled},
		},
	}
	got, trace := cs.Evaluate(context.Background(), predicates.ActorSnapshot{})
	if got {
		t.Fatalf("AND with a false child should be false")
	}
	if *thirdCalled {
		t.Fatalf("AND must short-circuit: third predicate should not run after a false")
	}
	if len(trace.Children) != 2 {
		t.Fatalf("expected 2 child traces (stopped after false), got %d", len(trace.Children))
	}
}

func TestConditionSet_OR_AllFalse(t *testing.T) {
	cs := predicates.ConditionSet{
		Op: predicates.OpOR,
		Children: []predicates.Predicate{
			stubPredicate{result: false},
			stubPredicate{result: false},
		},
	}
	got, _ := cs.Evaluate(context.Background(), predicates.ActorSnapshot{})
	if got {
		t.Fatalf("OR of all-false should be false")
	}
}

func TestConditionSet_OR_ShortCircuitsOnTrue(t *testing.T) {
	thirdCalled := newCalled()
	cs := predicates.ConditionSet{
		Op: predicates.OpOR,
		Children: []predicates.Predicate{
			stubPredicate{result: false},
			stubPredicate{result: true},
			stubPredicate{result: true, called: thirdCalled},
		},
	}
	got, _ := cs.Evaluate(context.Background(), predicates.ActorSnapshot{})
	if !got {
		t.Fatalf("OR with a true child should be true")
	}
	if *thirdCalled {
		t.Fatalf("OR must short-circuit: third predicate should not run after a true")
	}
}

func TestConditionSet_NOfM_ReachedThreshold(t *testing.T) {
	fifthCalled := newCalled()
	cs := predicates.ConditionSet{
		Op:        predicates.OpNOfM,
		Threshold: 2,
		Children: []predicates.Predicate{
			stubPredicate{result: true},
			stubPredicate{result: false},
			stubPredicate{result: true},
			stubPredicate{result: false},
			stubPredicate{result: true, called: fifthCalled},
		},
	}
	got, trace := cs.Evaluate(context.Background(), predicates.ActorSnapshot{})
	if !got {
		t.Fatalf("N_OF_M=2 should be satisfied after 2 trues")
	}
	if *fifthCalled {
		t.Fatalf("N_OF_M must short-circuit once threshold reached")
	}
	if len(trace.Children) != 3 {
		t.Fatalf("expected 3 child traces (stopped on 2nd true at index 2), got %d", len(trace.Children))
	}
}

func TestConditionSet_NOfM_ShortCircuitsWhenUnreachable(t *testing.T) {
	// Threshold 3 across 4 children, where the first 2 are false. After
	// the 2nd false the remaining 2 children could at best give 2 trues,
	// short of the threshold of 3 → must short-circuit.
	fourthCalled := newCalled()
	cs := predicates.ConditionSet{
		Op:        predicates.OpNOfM,
		Threshold: 3,
		Children: []predicates.Predicate{
			stubPredicate{result: false},
			stubPredicate{result: false},
			stubPredicate{result: true},
			stubPredicate{result: true, called: fourthCalled},
		},
	}
	got, _ := cs.Evaluate(context.Background(), predicates.ActorSnapshot{})
	if got {
		t.Fatalf("N_OF_M=3 with only 2 possible trues should be false")
	}
	if *fourthCalled {
		t.Fatalf("N_OF_M must short-circuit when remaining children cannot reach threshold")
	}
}

func TestConditionSet_NOfM_NotMet(t *testing.T) {
	cs := predicates.ConditionSet{
		Op:        predicates.OpNOfM,
		Threshold: 3,
		Children: []predicates.Predicate{
			stubPredicate{result: true},
			stubPredicate{result: true},
			stubPredicate{result: false},
			stubPredicate{result: false},
			stubPredicate{result: true},
		},
	}
	// 3 trues out of 5 — exactly the threshold; should short-circuit true
	// at the third true (index 4).
	got, trace := cs.Evaluate(context.Background(), predicates.ActorSnapshot{})
	if !got {
		t.Fatalf("N_OF_M=3 with 3 trues should be true")
	}
	if len(trace.Children) != 5 {
		t.Fatalf("expected all 5 children visited before threshold reached, got %d", len(trace.Children))
	}
}

func TestConditionSet_NestedAND_OR(t *testing.T) {
	// (A AND B) OR (C AND D) — classic distributive nest.
	cs := predicates.ConditionSet{
		Op: predicates.OpOR,
		Children: []predicates.Predicate{
			predicates.ConditionSet{Op: predicates.OpAND, Children: []predicates.Predicate{
				stubPredicate{result: true},
				stubPredicate{result: false},
			}},
			predicates.ConditionSet{Op: predicates.OpAND, Children: []predicates.Predicate{
				stubPredicate{result: true},
				stubPredicate{result: true},
			}},
		},
	}
	got, _ := cs.Evaluate(context.Background(), predicates.ActorSnapshot{})
	if !got {
		t.Fatalf("(T AND F) OR (T AND T) should be true")
	}
}

func TestConditionSet_UnknownOp(t *testing.T) {
	cs := predicates.ConditionSet{Op: "XOR"}
	got, trace := cs.Evaluate(context.Background(), predicates.ActorSnapshot{})
	if got {
		t.Fatalf("unknown op should evaluate false, not true")
	}
	if trace.Reason == "" {
		t.Fatalf("unknown op should annotate the trace with a reason")
	}
}
