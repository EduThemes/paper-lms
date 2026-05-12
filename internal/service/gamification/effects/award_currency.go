package effects

import (
	"context"
	"fmt"
	"math"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// AwardCurrency is the effect side of the economy: ledger a positive delta
// to the actor's wallet for a code-identified currency. Rules reference
// currencies by code ("xp", "coins") for portability; this effect resolves
// the code to a currency_type_id via ResolveCurrencyByCode before calling
// wallet.ApplyTransaction.
//
// Multiplier is optional. When set, the final delta is
// `round(Amount × Multiplier)`. Multipliers <= 0 are treated as 1.0 to keep
// rule authors from accidentally zeroing an award.
type AwardCurrency struct {
	Code       string
	Amount     int64
	Multiplier *float64
}

func (a AwardCurrency) Kind() string { return "AwardCurrency" }

func (a AwardCurrency) Apply(ctx context.Context, deps EffectDeps, trig TriggeringContext) (EffectResult, error) {
	if a.Amount <= 0 {
		return EffectResult{}, fmt.Errorf("AwardCurrency.Amount must be > 0, got %d", a.Amount)
	}
	if deps.CurrencyType == nil || deps.Wallet == nil {
		return EffectResult{}, fmt.Errorf("AwardCurrency requires Wallet and CurrencyType deps")
	}

	currency, err := ResolveCurrencyByCode(ctx, deps.CurrencyType, trig.TenantID, trig.ScopeType, trig.ScopeID, a.Code)
	if err != nil {
		return EffectResult{}, fmt.Errorf("resolve currency %q: %w", a.Code, err)
	}
	if currency == nil {
		return EffectResult{}, fmt.Errorf("currency %q not defined in tenant %d at %s/%d or site", a.Code, trig.TenantID, trig.ScopeType, trig.ScopeID)
	}

	mult := 1.0
	if a.Multiplier != nil && *a.Multiplier > 0 {
		mult = *a.Multiplier
	}
	final := int64(math.Round(float64(a.Amount) * mult))
	if final <= 0 {
		// Round-to-zero would reject in ApplyTransaction; surface a clear
		// error here so the rule_evaluation row records why nothing landed.
		return EffectResult{}, fmt.Errorf("AwardCurrency final delta rounded to %d (Amount=%d × Multiplier=%v)", final, a.Amount, a.Multiplier)
	}

	policyFlags := policyFlagsFor(currency)
	tx := &models.GamificationWalletTransaction{
		UserID:            trig.ActorID,
		CurrencyTypeID:    currency.ID,
		Delta:             final,
		Reason:            fmt.Sprintf("rule:%d", trig.RuleID),
		TriggeringEventID: trig.EventID,
		TriggeringRuleID:  &trig.RuleID,
		PolicyFlags:       policyFlags,
	}
	if err := deps.Wallet.ApplyTransaction(ctx, tx); err != nil {
		return EffectResult{}, fmt.Errorf("apply transaction: %w", err)
	}

	detail := map[string]any{
		"code":             a.Code,
		"currency_type_id": currency.ID,
		"amount":           a.Amount,
		"final_delta":      final,
	}
	if a.Multiplier != nil {
		detail["multiplier"] = *a.Multiplier
	}
	return EffectResult{
		Kind:    a.Kind(),
		Summary: fmt.Sprintf("+%d %s to user %d", final, a.Code, trig.ActorID),
		Detail:  detail,
	}, nil
}

// policyFlagsFor derives the wallet transaction's PolicyFlags from the
// currency's FERPA classification. Education-record currencies (e.g.
// mastery_points) carry the ferpa_protected + education_record flags
// downstream so FERPA-aware consumers (Wave 2 leaderboards, exports) can
// filter them out at read time without re-checking the currency_type row.
func policyFlagsFor(currency *models.GamificationCurrencyType) []string {
	switch currency.FerpaClassification {
	case "education_record":
		return []string{"ferpa_protected", "education_record"}
	case "directory_information":
		return []string{"directory_information"}
	case "instructor_metadata":
		return []string{"instructor_metadata"}
	default:
		return nil
	}
}
