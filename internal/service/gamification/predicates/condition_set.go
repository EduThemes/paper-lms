package predicates

import (
	"context"
	"fmt"
)

// Op is the composition operator for a ConditionSet.
type Op string

const (
	OpAND    Op = "AND"
	OpOR     Op = "OR"
	OpNOfM   Op = "N_OF_M"
)

// ConditionSet composes child predicates with AND, OR, or N_OF_M
// semantics. N_OF_M requires `Threshold` true children; AND short-circuits
// on the first false child; OR short-circuits on the first true child.
//
// The set itself implements Predicate, so condition sets nest arbitrarily.
// This is how the 24-predicate vocabulary becomes the recursive AND/OR/
// N_OF_M tree that the rules engine evaluates against an ActorSnapshot.
type ConditionSet struct {
	Op        Op
	Threshold int // only meaningful for OpNOfM
	Children  []Predicate
}

func (cs ConditionSet) Kind() string { return "ConditionSet" }

// Evaluate walks the children in order and applies the composition op.
// Short-circuit semantics are critical for performance: a 20-child AND
// stops at the first false; a 20-child OR stops at the first true. The
// caller-visible trace still records every visited child so debuggers can
// see exactly how far evaluation reached.
func (cs ConditionSet) Evaluate(ctx context.Context, actor ActorSnapshot) (bool, Trace) {
	trace := Trace{
		Kind:   cs.Kind(),
		Params: map[string]any{"op": string(cs.Op)},
	}
	if cs.Op == OpNOfM {
		trace.Params["threshold"] = cs.Threshold
	}

	switch cs.Op {
	case OpAND:
		for _, child := range cs.Children {
			ok, t := child.Evaluate(ctx, actor)
			trace.Children = append(trace.Children, t)
			if !ok {
				trace.Result = false
				trace.Reason = fmt.Sprintf("AND short-circuited on %s", t.Kind)
				return false, trace
			}
		}
		trace.Result = true
		return true, trace

	case OpOR:
		for _, child := range cs.Children {
			ok, t := child.Evaluate(ctx, actor)
			trace.Children = append(trace.Children, t)
			if ok {
				trace.Result = true
				trace.Reason = fmt.Sprintf("OR satisfied by %s", t.Kind)
				return true, trace
			}
		}
		trace.Result = false
		return false, trace

	case OpNOfM:
		hits := 0
		needed := cs.Threshold
		remaining := len(cs.Children)
		for _, child := range cs.Children {
			ok, t := child.Evaluate(ctx, actor)
			trace.Children = append(trace.Children, t)
			remaining--
			if ok {
				hits++
				if hits >= needed {
					trace.Result = true
					trace.Reason = fmt.Sprintf("N_OF_M reached %d/%d", hits, needed)
					return true, trace
				}
			}
			// Cannot possibly hit the threshold even if all remaining are true.
			if hits+remaining < needed {
				trace.Result = false
				trace.Reason = fmt.Sprintf("N_OF_M cannot reach %d (have %d, %d remaining)", needed, hits, remaining)
				return false, trace
			}
		}
		// Walked every child without short-circuit; threshold not met.
		trace.Result = false
		trace.Reason = fmt.Sprintf("N_OF_M ended at %d/%d", hits, needed)
		return false, trace

	default:
		trace.Result = false
		trace.Reason = fmt.Sprintf("unknown op %q", cs.Op)
		return false, trace
	}
}
