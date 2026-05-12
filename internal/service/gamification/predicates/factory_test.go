package predicates_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/EduThemes/paper-lms/internal/service/gamification/predicates"
)

// withKind splices a "kind" discriminator into the JSON encoding of a
// predicate struct so the result is a valid input for DecodePredicate.
// Tests use this instead of hand-typing JSON literals so the shape stays
// in sync with the Go field names.
func withKind(t *testing.T, kind string, v any) json.RawMessage {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %s: %v", kind, err)
	}
	if len(body) < 2 || body[0] != '{' {
		t.Fatalf("withKind: expected JSON object for %s, got %s", kind, string(body))
	}
	if len(body) == 2 { // "{}"
		return json.RawMessage(fmt.Sprintf(`{"kind":%q}`, kind))
	}
	return json.RawMessage(fmt.Sprintf(`{"kind":%q,%s`, kind, string(body[1:])))
}

func fptr(f float64) *float64 { return &f }

// equalPredicate compares two predicates for structural equality, dereferencing
// *float64 fields so identity-vs-value differences don't cause false negatives.
func equalPredicate(t *testing.T, want, got predicates.Predicate) {
	t.Helper()
	if w, ok := want.(predicates.SubmittedAssignment); ok {
		g, gok := got.(predicates.SubmittedAssignment)
		if !gok {
			t.Fatalf("want SubmittedAssignment, got %T", got)
		}
		if w.AssignmentID != g.AssignmentID ||
			w.RequireOnTime != g.RequireOnTime ||
			!floatPtrEq(w.MinScore, g.MinScore) ||
			!floatPtrEq(w.MaxScore, g.MaxScore) {
			t.Fatalf("SubmittedAssignment mismatch:\n want %+v\n  got %+v", w, g)
		}
		return
	}
	if w, ok := want.(predicates.SubmittedQuiz); ok {
		g, gok := got.(predicates.SubmittedQuiz)
		if !gok {
			t.Fatalf("want SubmittedQuiz, got %T", got)
		}
		if w.QuizID != g.QuizID ||
			!floatPtrEq(w.MinScore, g.MinScore) ||
			!floatPtrEq(w.MaxScore, g.MaxScore) {
			t.Fatalf("SubmittedQuiz mismatch:\n want %+v\n  got %+v", w, g)
		}
		return
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("predicate mismatch:\n want %+v\n  got %+v", want, got)
	}
}

func floatPtrEq(a, b *float64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// ---------------------------------------------------------------------------
// Atomic round-trip table
// ---------------------------------------------------------------------------

func TestDecodePredicate_AtomicRoundTrips(t *testing.T) {
	cases := []struct {
		name string
		kind string
		in   predicates.Predicate
	}{
		{
			name: "SubmittedAssignment with score bounds",
			kind: "SubmittedAssignment",
			in: predicates.SubmittedAssignment{
				AssignmentID:  42,
				MinScore:      fptr(80),
				MaxScore:      fptr(100),
				RequireOnTime: true,
			},
		},
		{
			name: "SubmittedAssignment minimal",
			kind: "SubmittedAssignment",
			in:   predicates.SubmittedAssignment{AssignmentID: 1},
		},
		{
			name: "SubmittedQuiz with bounds",
			kind: "SubmittedQuiz",
			in: predicates.SubmittedQuiz{
				QuizID:   17,
				MinScore: fptr(70),
				MaxScore: fptr(90),
			},
		},
		{
			name: "ViewedContent",
			kind: "ViewedContent",
			in: predicates.ViewedContent{
				ContentID:        99,
				MinSecondsViewed: 30,
			},
		},
		{
			name: "OutcomeMastery proficient",
			kind: "OutcomeMastery",
			in: predicates.OutcomeMastery{
				OutcomeID:  5,
				MinLevel:   "proficient",
				CalcMethod: "khan_spaced_retrieval",
			},
		},
		{
			name: "OutcomeMastery novice",
			kind: "OutcomeMastery",
			in:   predicates.OutcomeMastery{OutcomeID: 1, MinLevel: "novice"},
		},
		{
			name: "CurrencyThreshold",
			kind: "CurrencyThreshold",
			in:   predicates.CurrencyThreshold{Code: "xp", MinAmount: 500},
		},
		{
			name: "EarnedBadge",
			kind: "EarnedBadge",
			in:   predicates.EarnedBadge{BadgeID: 12},
		},
		{
			name: "ReputationThreshold",
			kind: "ReputationThreshold",
			in:   predicates.ReputationThreshold{MinAmount: 25},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			raw := withKind(t, tc.kind, tc.in)
			got, err := predicates.DecodePredicate(raw)
			if err != nil {
				t.Fatalf("DecodePredicate(%s): %v", tc.kind, err)
			}
			if got.Kind() != tc.kind {
				t.Fatalf("Kind mismatch: want %q got %q", tc.kind, got.Kind())
			}
			equalPredicate(t, tc.in, got)
		})
	}
}

// ---------------------------------------------------------------------------
// ConditionSet round-trip
// ---------------------------------------------------------------------------

// buildConditionSetJSON hand-assembles a ConditionSet JSON blob whose children
// are themselves wrapped with their kind discriminators. We build the JSON
// directly (rather than relying on ConditionSet's default marshal) because the
// type holds children as Predicate interfaces, and Go's default marshal would
// strip the discriminator off each child.
func buildConditionSetJSON(t *testing.T, op predicates.Op, threshold int, childKinds []string, childStructs []any) json.RawMessage {
	t.Helper()
	if len(childKinds) != len(childStructs) {
		t.Fatalf("kinds/structs length mismatch")
	}
	children := make([]json.RawMessage, len(childKinds))
	for i := range childKinds {
		children[i] = withKind(t, childKinds[i], childStructs[i])
	}
	wrapper := map[string]any{
		"kind":      "ConditionSet",
		"op":        string(op),
		"threshold": threshold,
		"children":  children,
	}
	out, err := json.Marshal(wrapper)
	if err != nil {
		t.Fatalf("marshal wrapper: %v", err)
	}
	return out
}

func TestDecodePredicate_ConditionSetAND(t *testing.T) {
	raw := buildConditionSetJSON(t,
		predicates.OpAND, 0,
		[]string{"SubmittedAssignment", "OutcomeMastery"},
		[]any{
			predicates.SubmittedAssignment{AssignmentID: 7, RequireOnTime: true},
			predicates.OutcomeMastery{OutcomeID: 3, MinLevel: "mastered"},
		},
	)
	got, err := predicates.DecodePredicate(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	cs, ok := got.(predicates.ConditionSet)
	if !ok {
		t.Fatalf("expected ConditionSet, got %T", got)
	}
	if cs.Op != predicates.OpAND {
		t.Fatalf("Op = %q, want AND", cs.Op)
	}
	if len(cs.Children) != 2 {
		t.Fatalf("Children len = %d, want 2", len(cs.Children))
	}
	if cs.Children[0].Kind() != "SubmittedAssignment" {
		t.Fatalf("child[0].Kind = %q", cs.Children[0].Kind())
	}
	if cs.Children[1].Kind() != "OutcomeMastery" {
		t.Fatalf("child[1].Kind = %q", cs.Children[1].Kind())
	}
}

func TestDecodePredicate_ConditionSetNOfM(t *testing.T) {
	raw := buildConditionSetJSON(t,
		predicates.OpNOfM, 2,
		[]string{"SubmittedAssignment", "SubmittedAssignment", "SubmittedAssignment"},
		[]any{
			predicates.SubmittedAssignment{AssignmentID: 1},
			predicates.SubmittedAssignment{AssignmentID: 2},
			predicates.SubmittedAssignment{AssignmentID: 3},
		},
	)
	got, err := predicates.DecodePredicate(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	cs, ok := got.(predicates.ConditionSet)
	if !ok {
		t.Fatalf("expected ConditionSet, got %T", got)
	}
	if cs.Op != predicates.OpNOfM {
		t.Fatalf("Op = %q, want N_OF_M", cs.Op)
	}
	if cs.Threshold != 2 {
		t.Fatalf("Threshold = %d, want 2", cs.Threshold)
	}
	if len(cs.Children) != 3 {
		t.Fatalf("Children len = %d, want 3", len(cs.Children))
	}
}

func TestDecodePredicate_DeeplyNested(t *testing.T) {
	// AND( OR( AND(SubA, SubA), OutcomeMastery ), CurrencyThreshold )
	innerAND := buildConditionSetJSON(t,
		predicates.OpAND, 0,
		[]string{"SubmittedAssignment", "SubmittedAssignment"},
		[]any{
			predicates.SubmittedAssignment{AssignmentID: 1},
			predicates.SubmittedAssignment{AssignmentID: 2},
		},
	)
	outcomeMastery := withKind(t, "OutcomeMastery",
		predicates.OutcomeMastery{OutcomeID: 9, MinLevel: "proficient"})
	or := map[string]any{
		"kind":     "ConditionSet",
		"op":       string(predicates.OpOR),
		"children": []json.RawMessage{innerAND, outcomeMastery},
	}
	orJSON, err := json.Marshal(or)
	if err != nil {
		t.Fatalf("marshal or: %v", err)
	}
	currency := withKind(t, "CurrencyThreshold",
		predicates.CurrencyThreshold{Code: "xp", MinAmount: 100})
	outer := map[string]any{
		"kind":     "ConditionSet",
		"op":       string(predicates.OpAND),
		"children": []json.RawMessage{json.RawMessage(orJSON), currency},
	}
	raw, err := json.Marshal(outer)
	if err != nil {
		t.Fatalf("marshal outer: %v", err)
	}

	got, err := predicates.DecodePredicate(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	cs, ok := got.(predicates.ConditionSet)
	if !ok {
		t.Fatalf("expected ConditionSet, got %T", got)
	}
	if cs.Op != predicates.OpAND || len(cs.Children) != 2 {
		t.Fatalf("outer shape wrong: op=%s nchildren=%d", cs.Op, len(cs.Children))
	}
	orCS, ok := cs.Children[0].(predicates.ConditionSet)
	if !ok || orCS.Op != predicates.OpOR || len(orCS.Children) != 2 {
		t.Fatalf("middle OR shape wrong: %#v", cs.Children[0])
	}
	innerCS, ok := orCS.Children[0].(predicates.ConditionSet)
	if !ok || innerCS.Op != predicates.OpAND || len(innerCS.Children) != 2 {
		t.Fatalf("inner AND shape wrong: %#v", orCS.Children[0])
	}
	if _, ok := innerCS.Children[0].(predicates.SubmittedAssignment); !ok {
		t.Fatalf("inner[0] type: %T", innerCS.Children[0])
	}
	if _, ok := orCS.Children[1].(predicates.OutcomeMastery); !ok {
		t.Fatalf("or[1] type: %T", orCS.Children[1])
	}
	if _, ok := cs.Children[1].(predicates.CurrencyThreshold); !ok {
		t.Fatalf("outer[1] type: %T", cs.Children[1])
	}
}

// TestDecodePredicate_ConditionSet_StructMarshalRoundTrip is the
// regression for a real bug found in code review: an earlier draft of
// the factory looked up "Op"/"Threshold"/"Children" (capitalized) but
// ConditionSet's struct tags are snake_case, so a production
// `json.Marshal(ConditionSet{...})` would produce keys the factory
// couldn't read. This test serializes ConditionSet via the standard
// library — the same path the rule_authoring tool will use — and
// confirms the round-trip succeeds.
func TestDecodePredicate_ConditionSet_StructMarshalRoundTrip(t *testing.T) {
	// Build a struct, hand-splice the "kind" discriminator on each child
	// so the recursive decoder can dispatch.
	childA := withKind(t, "SubmittedAssignment", predicates.SubmittedAssignment{
		AssignmentID:  42,
		RequireOnTime: true,
	})
	childB := withKind(t, "OutcomeMastery", predicates.OutcomeMastery{
		OutcomeID: 7,
		MinLevel:  "proficient",
	})

	// Splice "kind":"ConditionSet" into the JSON object Go's default
	// marshaller produces from a ConditionSet shell. We can't marshal
	// ConditionSet directly because its Children are an interface slice
	// that would strip the per-child "kind" tags.
	shell := struct {
		Op        predicates.Op     `json:"op"`
		Threshold int               `json:"threshold,omitempty"`
		Children  []json.RawMessage `json:"children"`
	}{
		Op:       predicates.OpAND,
		Children: []json.RawMessage{childA, childB},
	}
	body, err := json.Marshal(shell)
	if err != nil {
		t.Fatalf("marshal shell: %v", err)
	}
	raw := json.RawMessage(fmt.Sprintf(`{"kind":"ConditionSet",%s`, string(body[1:])))

	got, err := predicates.DecodePredicate(raw)
	if err != nil {
		t.Fatalf("decode struct-marshalled ConditionSet: %v", err)
	}
	cs, ok := got.(predicates.ConditionSet)
	if !ok {
		t.Fatalf("expected ConditionSet, got %T", got)
	}
	if cs.Op != predicates.OpAND || len(cs.Children) != 2 {
		t.Fatalf("shape wrong: op=%s nchildren=%d", cs.Op, len(cs.Children))
	}
	if _, ok := cs.Children[0].(predicates.SubmittedAssignment); !ok {
		t.Fatalf("child[0] type: %T", cs.Children[0])
	}
	if _, ok := cs.Children[1].(predicates.OutcomeMastery); !ok {
		t.Fatalf("child[1] type: %T", cs.Children[1])
	}
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestDecodePredicate_UnknownKind(t *testing.T) {
	raw := json.RawMessage(`{"kind":"NoSuchPredicate","Foo":1}`)
	_, err := predicates.DecodePredicate(raw)
	if err == nil {
		t.Fatalf("expected error for unknown kind")
	}
}

func TestDecodePredicate_MissingKind(t *testing.T) {
	raw := json.RawMessage(`{"AssignmentID":1}`)
	_, err := predicates.DecodePredicate(raw)
	if err == nil {
		t.Fatalf("expected error when kind missing")
	}
}

func TestDecodePredicate_MalformedJSON(t *testing.T) {
	raw := json.RawMessage(`{not json`)
	_, err := predicates.DecodePredicate(raw)
	if err == nil {
		t.Fatalf("expected error for malformed JSON")
	}
}

func TestDecodePredicate_EmptyInput(t *testing.T) {
	_, err := predicates.DecodePredicate(nil)
	if err == nil {
		t.Fatalf("expected error for empty JSON")
	}
}

func TestDecodePredicate_ConditionSetBogusOp(t *testing.T) {
	raw := json.RawMessage(`{"kind":"ConditionSet","Op":"BOGUS","Children":[]}`)
	_, err := predicates.DecodePredicate(raw)
	if err == nil {
		t.Fatalf("expected error for bogus Op")
	}
}

func TestDecodePredicate_ConditionSetNOfMZeroThreshold(t *testing.T) {
	raw := json.RawMessage(`{"kind":"ConditionSet","Op":"N_OF_M","Threshold":0,"Children":[]}`)
	_, err := predicates.DecodePredicate(raw)
	if err == nil {
		t.Fatalf("expected error for N_OF_M with Threshold=0")
	}
}

func TestDecodePredicate_OutcomeMasteryInvalidMinLevel(t *testing.T) {
	raw := withKind(t, "OutcomeMastery",
		predicates.OutcomeMastery{OutcomeID: 1, MinLevel: "godlike"})
	_, err := predicates.DecodePredicate(raw)
	if err == nil {
		t.Fatalf("expected error for invalid MinLevel")
	}
}

func TestDecodePredicate_SubmittedAssignmentZeroID(t *testing.T) {
	raw := withKind(t, "SubmittedAssignment",
		predicates.SubmittedAssignment{AssignmentID: 0})
	_, err := predicates.DecodePredicate(raw)
	if err == nil {
		t.Fatalf("expected error for AssignmentID=0")
	}
}

func TestDecodePredicate_RequiredIDValidation(t *testing.T) {
	cases := []struct {
		name string
		kind string
		in   any
	}{
		{"SubmittedQuiz zero", "SubmittedQuiz", predicates.SubmittedQuiz{QuizID: 0}},
		{"ViewedContent zero", "ViewedContent", predicates.ViewedContent{ContentID: 0}},
		{"OutcomeMastery zero", "OutcomeMastery", predicates.OutcomeMastery{OutcomeID: 0, MinLevel: "novice"}},
		{"EarnedBadge zero", "EarnedBadge", predicates.EarnedBadge{BadgeID: 0}},
		{"CurrencyThreshold empty code", "CurrencyThreshold", predicates.CurrencyThreshold{Code: "", MinAmount: 10}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			raw := withKind(t, tc.kind, tc.in)
			_, err := predicates.DecodePredicate(raw)
			if err == nil {
				t.Fatalf("expected validation error for %s", tc.name)
			}
		})
	}
}

func TestDecodePredicate_ConditionSetChildError(t *testing.T) {
	// A ConditionSet whose child has an invalid kind should bubble up.
	badChild := json.RawMessage(`{"kind":"NoSuchPredicate"}`)
	outer := map[string]any{
		"kind":     "ConditionSet",
		"Op":       string(predicates.OpAND),
		"Children": []json.RawMessage{badChild},
	}
	raw, err := json.Marshal(outer)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	_, err = predicates.DecodePredicate(raw)
	if err == nil {
		t.Fatalf("expected error when child kind is unknown")
	}
}
