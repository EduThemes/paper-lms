package predicates

import (
	"encoding/json"
	"fmt"

	"github.com/EduThemes/paper-lms/internal/service/gamification/mastery"
)

// DecodePredicate parses a JSONB blob from gamification_rules.condition_set
// and returns the corresponding Predicate. The discriminator is the top-
// level "kind" field; recursion into ConditionSet children walks the same
// decoder.
//
// Returns an error on unknown kinds, malformed JSON, or missing required
// fields. Rule authoring tools should validate at create-time; this
// decoder is the eval-time backstop.
//
// The JSON shape follows the snake_case `json:"..."` tags declared on
// each predicate struct (and on ConditionSet). The synthetic "kind"
// field carries the discriminator and is ignored by the per-predicate
// unmarshal pass.
//
// Example JSON for an atomic predicate:
//
//	{"kind":"SubmittedAssignment","assignment_id":42,"require_on_time":true}
//
// Example JSON for a ConditionSet:
//
//	{
//	  "kind":"ConditionSet",
//	  "op":"AND",
//	  "children":[
//	    {"kind":"SubmittedAssignment","assignment_id":42},
//	    {"kind":"OutcomeMastery","outcome_id":7,"min_level":"proficient"}
//	  ]
//	}
func DecodePredicate(raw json.RawMessage) (Predicate, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty predicate JSON")
	}

	// Peek at the discriminator.
	var head struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, fmt.Errorf("predicate JSON: %w", err)
	}
	if head.Kind == "" {
		return nil, fmt.Errorf("predicate JSON missing required \"kind\" field")
	}

	switch head.Kind {
	case "SubmittedAssignment":
		var p SubmittedAssignment
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("SubmittedAssignment: %w", err)
		}
		if p.AssignmentID == 0 {
			return nil, fmt.Errorf("SubmittedAssignment requires AssignmentID > 0")
		}
		return p, nil

	case "SubmittedQuiz":
		var p SubmittedQuiz
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("SubmittedQuiz: %w", err)
		}
		if p.QuizID == 0 {
			return nil, fmt.Errorf("SubmittedQuiz requires QuizID > 0")
		}
		return p, nil

	case "ViewedContent":
		var p ViewedContent
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("ViewedContent: %w", err)
		}
		if p.ContentID == 0 {
			return nil, fmt.Errorf("ViewedContent requires ContentID > 0")
		}
		return p, nil

	case "OutcomeMastery":
		var p OutcomeMastery
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("OutcomeMastery: %w", err)
		}
		if p.OutcomeID == 0 {
			return nil, fmt.Errorf("OutcomeMastery requires OutcomeID > 0")
		}
		if mastery.LevelOrdinal(p.MinLevel) < 0 {
			return nil, fmt.Errorf("OutcomeMastery: invalid MinLevel %q (must be novice|familiar|proficient|mastered)", p.MinLevel)
		}
		return p, nil

	case "CurrencyThreshold":
		var p CurrencyThreshold
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("CurrencyThreshold: %w", err)
		}
		if p.Code == "" {
			return nil, fmt.Errorf("CurrencyThreshold requires non-empty Code")
		}
		return p, nil

	case "EarnedBadge":
		var p EarnedBadge
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("EarnedBadge: %w", err)
		}
		if p.BadgeID == 0 {
			return nil, fmt.Errorf("EarnedBadge requires BadgeID > 0")
		}
		return p, nil

	case "ReputationThreshold":
		var p ReputationThreshold
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("ReputationThreshold: %w", err)
		}
		// MinAmount of 0 is technically meaningful (gate on "any rep"),
		// so no positive-value requirement here.
		return p, nil

	case "ConditionSet":
		// Two-pass: decode op/threshold and raw children, then recurse.
		// Tags must match ConditionSet's snake_case struct tags so a
		// json.Marshal(ConditionSet{...}) → DecodePredicate round-trip
		// works in production rule-authoring flows.
		var shell struct {
			Op        Op                `json:"op"`
			Threshold int               `json:"threshold"`
			Children  []json.RawMessage `json:"children"`
		}
		if err := json.Unmarshal(raw, &shell); err != nil {
			return nil, fmt.Errorf("ConditionSet: %w", err)
		}
		switch shell.Op {
		case OpAND, OpOR, OpNOfM:
			// valid
		default:
			return nil, fmt.Errorf("ConditionSet: invalid Op %q (must be AND|OR|N_OF_M)", shell.Op)
		}
		if shell.Op == OpNOfM && shell.Threshold <= 0 {
			return nil, fmt.Errorf("ConditionSet: Op=N_OF_M requires Threshold > 0 (got %d)", shell.Threshold)
		}

		decoded := make([]Predicate, 0, len(shell.Children))
		for i, childRaw := range shell.Children {
			child, err := DecodePredicate(childRaw)
			if err != nil {
				return nil, fmt.Errorf("ConditionSet child[%d]: %w", i, err)
			}
			decoded = append(decoded, child)
		}
		return ConditionSet{
			Op:        shell.Op,
			Threshold: shell.Threshold,
			Children:  decoded,
		}, nil

	default:
		return nil, fmt.Errorf("unknown predicate kind: %q", head.Kind)
	}
}
