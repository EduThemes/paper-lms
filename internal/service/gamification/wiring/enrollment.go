// Package wiring assembles the gamification.Emitter into the
// service-layer callback hooks introduced in Sprint D-1 Phase 0. Each
// function in this package returns a service.OnX callback that, when
// invoked by the originating service, loads the canonical entity for
// the action, builds an xAPI-shaped GamificationEvent, and hands it to
// the Emitter.
//
// All callbacks here run inside goroutines spawned by the originating
// service's fire helper; the originating call site already detached the
// context and recovered from panic. These callbacks therefore never
// propagate errors — every failure path logs via slog.Error and returns,
// so a gamification glitch can never break the originating write.
package wiring

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

// EnrolledCourseEmitCallback returns an EnrollmentCreatedCallback that,
// given an enrollmentID, loads the enrollment, walks to its course for
// the tenant_id (Course.AccountID), constructs a
// `verb=enrolled, object_type=Course` event, and calls emitter.Emit.
//
// Errors are logged via slog.Error and NEVER propagated — the originating
// EnrollmentService.fireOnCreated runs this callback in a detached
// goroutine and has no caller to surface errors to.
func EnrolledCourseEmitCallback(
	emitter *gamification.Emitter,
	enrollmentRepo repository.EnrollmentRepository,
	courseRepo repository.CourseRepository,
) service.EnrollmentCreatedCallback {
	return func(ctx context.Context, enrollmentID uint) {
		enrollment, err := enrollmentRepo.FindByID(ctx, enrollmentID)
		if err != nil {
			slog.Error("enrolled course emit: load enrollment",
				"enrollment_id", enrollmentID, "error", err)
			return
		}
		course, err := courseRepo.FindByID(ctx, enrollment.CourseID, 0)
		if err != nil {
			slog.Error("enrolled course emit: load course",
				"enrollment_id", enrollmentID, "error", err)
			return
		}

		result, err := json.Marshal(map[string]any{
			"enrollment_type": enrollment.Type,
			"role":            enrollment.Role,
			"workflow_state":  enrollment.WorkflowState,
		})
		if err != nil {
			slog.Error("enrolled course emit: marshal result",
				"enrollment_id", enrollmentID, "error", err)
			return
		}
		contextJSON, err := json.Marshal(map[string]any{
			"course_id":     enrollment.CourseID,
			"enrollment_id": enrollment.ID,
		})
		if err != nil {
			slog.Error("enrolled course emit: marshal context",
				"enrollment_id", enrollmentID, "error", err)
			return
		}

		objectID := enrollment.CourseID
		event := &models.GamificationEvent{
			OccurredAt: time.Now(),
			TenantID:   course.AccountID,
			ActorID:    enrollment.UserID,
			Verb:       gamification.VerbEnrolled,
			ObjectType: gamification.ObjectCourse,
			ObjectID:   &objectID,
			Result:     result,
			Context:    contextJSON,
			Source:     gamification.EmitterSource,
		}

		if _, err := emitter.Emit(ctx, event); err != nil {
			slog.Error("enrolled course emit: emit failed",
				"enrollment_id", enrollmentID, "error", err)
			return
		}
	}
}
