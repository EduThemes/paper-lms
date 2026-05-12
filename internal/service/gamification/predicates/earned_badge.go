package predicates

import (
	"context"
)

// EarnedBadge tests whether the actor's EarnedBadges slice contains BadgeID.
// Set-membership only; the snapshot loader is responsible for hydrating the
// slice from whichever source-of-truth survives badge issuance in Wave 2.
type EarnedBadge struct {
	BadgeID uint `json:"badge_id"`
}

func (p EarnedBadge) Kind() string { return "EarnedBadge" }

func (p EarnedBadge) Needs() Needs {
	return Needs{BadgeIDs: []uint{p.BadgeID}}
}

func (p EarnedBadge) Evaluate(_ context.Context, actor ActorSnapshot) (bool, Trace) {
	trace := Trace{
		Kind: p.Kind(),
		Params: map[string]any{
			"badge_id": p.BadgeID,
		},
	}
	for _, id := range actor.EarnedBadges {
		if id == p.BadgeID {
			trace.Result = true
			return true, trace
		}
	}
	trace.Reason = "badge not in actor's EarnedBadges"
	return false, trace
}
