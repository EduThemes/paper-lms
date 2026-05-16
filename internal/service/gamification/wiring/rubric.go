// Package wiring — rubric assessment adapter.
//
// RubricAssessmentCreatedEmitCallback bridges
// RubricService.fireOnAssessmentCreated into the gamification.Emitter.
// The originating service runs this callback inside a detached goroutine
// (with panic recovery); we therefore log every failure via slog.Error
// and never propagate errors.
package wiring

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
)

// RubricAssessmentCreatedEmitCallback returns a
// RubricAssessmentCreatedCallback that loads the assessment, walks to its
// rubric (and, when ContextType=="Course", on to the course) for the
// tenant_id, then emits `verb=assessed, object_type=Rubric`.
//
// Tenant resolution:
//   - rubric.ContextType == "Course": tenant_id = course.AccountID
//   - rubric.ContextType == "Account": tenant_id = rubric.ContextID
//   - otherwise: log a structured error and return without emitting.
//
// Event shape:
//
//	{
//	  occurred_at: assessment.UpdatedAt,
//	  tenant_id:   <resolved per above>,
//	  actor_id:    assessment.UserID  (the student being assessed — the
//	                                   gamification subject, not the assessor),
//	  verb:        "assessed",
//	  object_type: "Rubric",
//	  object_id:   assessment.RubricID  (the rubric is the object;
//	                                     the assessment is the result),
//	  result: {
//	    "score":           <*float64>,
//	    "data":            <raw JSON from assessment.Data, or null if empty>,
//	    "assessment_type": <string>,
//	    "assessment_id":   <assessmentID>,
//	    "assessor_id":     <assessorID>,
//	  },
//	  context: {
//	    "rubric_id":             <rubricID>,
//	    "rubric_association_id": <assoc id>,
//	    "context_type":          <string>,
//	    "context_id":            <uint>,
//	  },
//	  source: "internal",
//	}
//
// assessment.Data is a JSON string containing per-criterion ratings; we
// pass it through as json.RawMessage so the inner object surfaces inline
// rather than being double-encoded as a quoted string. Rules can then
// predicate on per-criterion thresholds (e.g. data.criterion_1.points >= 4).
func RubricAssessmentCreatedEmitCallback(
	emitter *gamification.Emitter,
	assessRepo repository.RubricAssessmentRepository,
	rubricRepo repository.RubricRepository,
	courseRepo repository.CourseRepository,
) service.RubricAssessmentCreatedCallback {
	return func(ctx context.Context, assessmentID uint) {
		assessment, err := assessRepo.FindByID(ctx, assessmentID)
		if err != nil {
			slog.Error("rubric assessment emit: load assessment",
				"assessment_id", assessmentID, "error", err)
			return
		}
		rubric, err := rubricRepo.FindByID(ctx, assessment.RubricID, 0)
		if err != nil {
			slog.Error("rubric assessment emit: load rubric",
				"assessment_id", assessmentID,
				"rubric_id", assessment.RubricID, "error", err)
			return
		}

		var tenantID uint
		switch rubric.ContextType {
		case "Course":
			course, err := courseRepo.FindByID(ctx, rubric.ContextID, 0)
			if err != nil {
				slog.Error("rubric assessment emit: load course",
					"assessment_id", assessmentID,
					"rubric_id", rubric.ID,
					"context_id", rubric.ContextID, "error", err)
				return
			}
			tenantID = course.AccountID
		case "Account":
			tenantID = rubric.ContextID
		default:
			slog.Error("rubric assessment emit: unknown rubric context_type",
				"assessment_id", assessmentID,
				"rubric_id", rubric.ID,
				"context_type", rubric.ContextType)
			return
		}

		// assessment.Data is a JSON string. Pass it through as RawMessage so
		// the per-criterion blob is emitted inline (not as a quoted string).
		// Empty string → null so downstream predicates can distinguish "no
		// criterion data" from "empty object".
		var dataField any
		if assessment.Data != "" {
			dataField = json.RawMessage([]byte(assessment.Data))
		}

		resultBlob, err := json.Marshal(map[string]any{
			"score":           assessment.Score,
			"data":            dataField,
			"assessment_type": assessment.AssessmentType,
			"assessment_id":   assessmentID,
			"assessor_id":     assessment.AssessorID,
		})
		if err != nil {
			slog.Error("rubric assessment emit: marshal result",
				"assessment_id", assessmentID, "error", err)
			return
		}
		contextBlob, err := json.Marshal(map[string]any{
			"rubric_id":             assessment.RubricID,
			"rubric_association_id": assessment.RubricAssociationID,
			"context_type":          rubric.ContextType,
			"context_id":            rubric.ContextID,
		})
		if err != nil {
			slog.Error("rubric assessment emit: marshal context",
				"assessment_id", assessmentID, "error", err)
			return
		}

		objectID := assessment.RubricID
		event := &models.GamificationEvent{
			OccurredAt: assessment.UpdatedAt,
			TenantID:   tenantID,
			ActorID:    assessment.UserID,
			Verb:       gamification.VerbAssessed,
			ObjectType: gamification.ObjectRubric,
			ObjectID:   &objectID,
			Result:     resultBlob,
			Context:    contextBlob,
			Source:     gamification.EmitterSource,
		}
		if _, err := emitter.Emit(ctx, event); err != nil {
			slog.Error("rubric assessment emit: emit failed",
				"assessment_id", assessmentID,
				"verb", gamification.VerbAssessed,
				"error", err)
			return
		}
	}
}
