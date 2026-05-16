package wiring

// outcome_mastery.go — emit adapter for the per-row mastery transition
// callback fired by LearningOutcomeService.CreateResult. The service
// already guards the false/nil → true transition on the same
// (user, outcome, asset) composite, so this adapter does NOT re-check
// the transition; it simply walks the outcome's tenancy chain and
// emits a `verb=mastered, object_type=Outcome` event.
//
// Tenant resolution uses the OUTCOME's ContextType/ContextID, not the
// result row's — the outcome is the thing whose ownership defines the
// tenancy. (A result row may be recorded under a sub-context for
// cross-context outcome alignments, so trusting the result's context
// would misroute events on shared outcomes.)
//
// Errors at every step are logged via slog.Error and never propagated:
// the OutcomeMasteryCrossedCallback signature has no error return and a
// gamification emit failure must not break outcome result writes.

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
)

// OutcomeMasteryCrossedEmitCallback returns an OutcomeMasteryCrossedCallback
// that:
//
//  1. Loads the just-transitioned LearningOutcomeResult row (for score /
//     possible / percent / asset linkage / assessed_at timestamp).
//  2. Loads the LearningOutcome (for context type/ID + calculation_method).
//  3. Resolves the tenant: ContextType=="Course" → courseRepo lookup →
//     course.AccountID; ContextType=="Account" → ContextID directly.
//     Anything else is logged and skipped (no emit).
//  4. Emits a verb=mastered, object_type=Outcome event keyed on the
//     OUTCOME ID (the thing being mastered), not the result row ID.
//
// Event shape:
//
//	{
//	  occurred_at: result.AssessedAt → result.UpdatedAt → time.Now() (first non-nil),
//	  tenant_id:   resolved per ContextType above,
//	  actor_id:    userID,
//	  verb:        "mastered",
//	  object_type: "Outcome",
//	  object_id:   outcomeID,
//	  result: {
//	    "score":     <*float64>,
//	    "possible":  <*float64>,
//	    "percent":   <*float64>,
//	    "mastery":   true,
//	    "result_id": <resultID>,
//	  },
//	  context: {
//	    "context_type":          <outcome.ContextType>,
//	    "context_id":            <outcome.ContextID>,
//	    "associated_asset_type": <result.AssociatedAssetType>,
//	    "associated_asset_id":   <result.AssociatedAssetID>,
//	    "calculation_method":    <outcome.CalculationMethod>,
//	  },
//	  source: "internal",
//	}
//
// JSON keys are snake_case lowercase throughout — PR #8 caught a casing
// drift bug; do not re-introduce it.
func OutcomeMasteryCrossedEmitCallback(
	emitter *gamification.Emitter,
	resultRepo repository.LearningOutcomeResultRepository,
	outcomeRepo repository.LearningOutcomeRepository,
	courseRepo repository.CourseRepository,
) service.OutcomeMasteryCrossedCallback {
	return func(ctx context.Context, userID, outcomeID, resultID uint) {
		result, err := resultRepo.FindByID(ctx, resultID)
		if err != nil {
			slog.Error("outcome mastery emit: load result",
				"result_id", resultID,
				"outcome_id", outcomeID,
				"user_id", userID,
				"error", err,
			)
			return
		}
		outcome, err := outcomeRepo.FindByID(ctx, outcomeID, 0)
		if err != nil {
			slog.Error("outcome mastery emit: load outcome",
				"outcome_id", outcomeID,
				"result_id", resultID,
				"user_id", userID,
				"error", err,
			)
			return
		}

		// Tenant resolution. Prefer the outcome's context (defines outcome
		// tenancy) over the result's context (may be a sub-context under a
		// cross-context alignment).
		var tenantID uint
		switch outcome.ContextType {
		case "Course":
			course, err := courseRepo.FindByID(ctx, outcome.ContextID, 0)
			if err != nil {
				slog.Error("outcome mastery emit: load course for tenancy",
					"outcome_id", outcomeID,
					"context_id", outcome.ContextID,
					"user_id", userID,
					"error", err,
				)
				return
			}
			tenantID = course.AccountID
		case "Account":
			tenantID = outcome.ContextID
		default:
			slog.Error("outcome mastery emit: unsupported outcome context type",
				"outcome_id", outcomeID,
				"context_type", outcome.ContextType,
				"context_id", outcome.ContextID,
				"user_id", userID,
			)
			return
		}

		resultBlob, err := json.Marshal(map[string]any{
			"score":     result.Score,
			"possible":  result.Possible,
			"percent":   result.Percent,
			"mastery":   true,
			"result_id": resultID,
		})
		if err != nil {
			slog.Error("outcome mastery emit: marshal result blob",
				"outcome_id", outcomeID,
				"result_id", resultID,
				"user_id", userID,
				"error", err,
			)
			return
		}
		contextBlob, err := json.Marshal(map[string]any{
			"context_type":          result.ContextType,
			"context_id":            result.ContextID,
			"associated_asset_type": result.AssociatedAssetType,
			"associated_asset_id":   result.AssociatedAssetID,
			"calculation_method":    outcome.CalculationMethod,
		})
		if err != nil {
			slog.Error("outcome mastery emit: marshal context blob",
				"outcome_id", outcomeID,
				"result_id", resultID,
				"user_id", userID,
				"error", err,
			)
			return
		}

		// occurred_at: AssessedAt → UpdatedAt → time.Now().
		occurredAt := time.Now()
		if !result.UpdatedAt.IsZero() {
			occurredAt = result.UpdatedAt
		}
		if result.AssessedAt != nil {
			occurredAt = *result.AssessedAt
		}

		objectID := outcomeID
		event := &models.GamificationEvent{
			OccurredAt: occurredAt,
			TenantID:   tenantID,
			ActorID:    userID,
			Verb:       gamification.VerbMastered,
			ObjectType: gamification.ObjectOutcome,
			ObjectID:   &objectID,
			Result:     resultBlob,
			Context:    contextBlob,
			Source:     gamification.EmitterSource,
		}
		if _, err := emitter.Emit(ctx, event); err != nil {
			slog.Error("outcome mastery emit: emit failed",
				"outcome_id", outcomeID,
				"result_id", resultID,
				"user_id", userID,
				"tenant_id", tenantID,
				"verb", gamification.VerbMastered,
				"error", err,
			)
			return
		}
	}
}
