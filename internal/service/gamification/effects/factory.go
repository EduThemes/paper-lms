package effects

import (
	"encoding/json"
	"fmt"
)

// DecodeEffect parses a single JSON effect spec into the matching Effect
// implementation. The discriminator is the top-level "kind" field.
//
// Wave 1 supports only AwardCurrency. Future effects (AwardBadge,
// ReleaseContent, BranchPath, UnlockCapability, Notify,
// AdvanceRankOrLevel, EnrollInGroup) extend the switch below.
func DecodeEffect(raw json.RawMessage) (Effect, error) {
	var head struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, fmt.Errorf("decode effect kind: %w", err)
	}

	switch head.Kind {
	case "AwardCurrency":
		var ac AwardCurrency
		if err := json.Unmarshal(raw, &ac); err != nil {
			return nil, fmt.Errorf("decode AwardCurrency: %w", err)
		}
		if ac.Code == "" {
			return nil, fmt.Errorf("AwardCurrency.Code must be non-empty")
		}
		if ac.Amount <= 0 {
			return nil, fmt.Errorf("AwardCurrency.Amount must be > 0, got %d", ac.Amount)
		}
		return ac, nil
	default:
		return nil, fmt.Errorf("unknown effect kind: %q", head.Kind)
	}
}

// DecodeEffects parses a JSON array of effect specs in order. Effects run
// in the order they appear in the array (per Sprint C's stop-on-first-error
// semantics — preserve order in the returned slice). Returns an error if
// any individual decode fails, wrapped with the offending index.
func DecodeEffects(raw json.RawMessage) ([]Effect, error) {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, fmt.Errorf("decode effects array: %w", err)
	}

	out := make([]Effect, 0, len(arr))
	for i, item := range arr {
		eff, err := DecodeEffect(item)
		if err != nil {
			return nil, fmt.Errorf("effect %d: %w", i, err)
		}
		out = append(out, eff)
	}
	return out, nil
}
