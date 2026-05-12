package predicates

import (
	"context"
)

// ReputationThreshold is a thin convenience wrapper over CurrencyThreshold
// pinned to the "reputation" code. Rule authors who write
// `ReputationThreshold(20)` don't have to remember the magic code, and the
// trace surfaces the rule's intent (Reputation gates capability unlocks,
// per SYNTHESIS.md §3) rather than a generic currency check.
type ReputationThreshold struct {
	MinAmount int64 `json:"min_amount"`
}

// ReputationCode is the system currency code shared by every tenant.
const ReputationCode = "reputation"

func (p ReputationThreshold) Kind() string { return "ReputationThreshold" }

func (p ReputationThreshold) Needs() Needs {
	return Needs{CurrencyCodes: []string{ReputationCode}}
}

func (p ReputationThreshold) Evaluate(_ context.Context, actor ActorSnapshot) (bool, Trace) {
	delegate := CurrencyThreshold{Code: ReputationCode, MinAmount: p.MinAmount}
	return delegate.evaluateWithKind(actor, p.Kind())
}
