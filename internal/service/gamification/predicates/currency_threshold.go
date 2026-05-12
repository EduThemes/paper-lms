package predicates

import (
	"context"
)

// CurrencyThreshold tests whether the actor's balance in a code-identified
// currency meets or exceeds MinAmount. Rules reference currencies by code
// ("xp", "coins") for portability across tenants; the snapshot loader
// resolves the code to a currency_type_id via ActorSnapshot.CurrencyByCode
// before evaluation.
type CurrencyThreshold struct {
	Code      string
	MinAmount int64
}

func (p CurrencyThreshold) Kind() string { return "CurrencyThreshold" }

func (p CurrencyThreshold) Evaluate(_ context.Context, actor ActorSnapshot) (bool, Trace) {
	return p.evaluateWithKind(actor, p.Kind())
}

// evaluateWithKind is the shared body so ReputationThreshold can delegate
// without losing its own Kind() in the trace.
func (p CurrencyThreshold) evaluateWithKind(actor ActorSnapshot, kind string) (bool, Trace) {
	trace := Trace{
		Kind: kind,
		Params: map[string]any{
			"code":       p.Code,
			"min_amount": p.MinAmount,
		},
	}

	id, ok := actor.CurrencyByCode[p.Code]
	if !ok {
		trace.Reason = "currency code not present in actor snapshot"
		return false, trace
	}
	balance := actor.WalletBalances[id]
	trace.Params["balance"] = balance

	if balance < p.MinAmount {
		trace.Reason = "balance below MinAmount"
		return false, trace
	}
	trace.Result = true
	return true, trace
}
