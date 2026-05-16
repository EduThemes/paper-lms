// Package wiring assembles call-site adapters that translate domain
// events (a graded submission, a completed quiz, a viewed page, …) into
// gamification.Emitter calls. Each adapter is a thin function returning
// the SubmissionGradedCallback / QuizCompletedCallback / … shape the
// corresponding service exposes via OnGraded / OnCompleted / OnViewed.
//
// Why a separate package? cmd/server/main.go wires the callbacks at
// startup; keeping them out of the service package keeps the service
// from importing gamification (and its FERPA / event / rule deps).
// Tests in this package exercise the adapters against real Postgres.
//
// Error handling contract for every adapter in this package: errors are
// LOGGED with slog.Error and never propagated. The service-layer
// callback signatures return nothing — emit failures must not block
// the originating action (grading, completing a quiz, viewing a page).
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

// GradedSubmissionEmitCallback returns a SubmissionGradedCallback that
// loads the submission, walks to its assignment + course for the
// tenant_id (course.account_id), constructs a `verb=graded,
// object_type=Submission` event, and calls emitter.Emit. Errors at
// every step are logged via slog.Error and NEVER propagated — the
// SubmissionGradedCallback signature has no error return and emit
// failures must not break grading.
//
// Event shape:
//
//	{
//	  occurred_at: submission.GradedAt (fallback time.Now() if nil),
//	  tenant_id:   course.AccountID,
//	  actor_id:    submission.UserID,
//	  verb:        "graded",
//	  object_type: "Submission",
//	  object_id:   submissionID,
//	  result:      {"score": <float64?>, "workflow_state": <string>},
//	  context:     {"course_id": <uint>, "assignment_id": <uint>},
//	  source:      "internal",
//	}
//
// Score may be nil (ungraded re-save, excused, etc.); when nil it is
// emitted as JSON null so downstream predicates can distinguish "no
// score" from "score 0".
func GradedSubmissionEmitCallback(
	emitter *gamification.Emitter,
	submissionRepo repository.SubmissionRepository,
	assignmentRepo repository.AssignmentRepository,
	courseRepo repository.CourseRepository,
) service.SubmissionGradedCallback {
	return func(ctx context.Context, submissionID uint) {
		submission, err := submissionRepo.FindByID(ctx, submissionID)
		if err != nil {
			slog.Error("graded submission emit: load submission",
				"submission_id", submissionID, "error", err)
			return
		}
		assignment, err := assignmentRepo.FindByID(ctx, submission.AssignmentID, 0)
		if err != nil {
			slog.Error("graded submission emit: load assignment",
				"submission_id", submissionID, "error", err)
			return
		}
		course, err := courseRepo.FindByID(ctx, assignment.CourseID, 0)
		if err != nil {
			slog.Error("graded submission emit: load course",
				"submission_id", submissionID, "error", err)
			return
		}

		// Result blob: score may be nil (excused / ungraded post-write).
		// json.Marshal emits a Go nil *float64 as JSON null, which is the
		// right answer — downstream predicates can distinguish that from
		// score=0 if they care.
		resultBlob, err := json.Marshal(map[string]any{
			"score":          submission.Score,
			"workflow_state": submission.WorkflowState,
		})
		if err != nil {
			slog.Error("graded submission emit: marshal result",
				"submission_id", submissionID, "error", err)
			return
		}
		contextBlob, err := json.Marshal(map[string]any{
			"course_id":     assignment.CourseID,
			"assignment_id": assignment.ID,
		})
		if err != nil {
			slog.Error("graded submission emit: marshal context",
				"submission_id", submissionID, "error", err)
			return
		}

		occurredAt := time.Now()
		if submission.GradedAt != nil {
			occurredAt = *submission.GradedAt
		}

		objectID := submissionID
		event := &models.GamificationEvent{
			OccurredAt: occurredAt,
			TenantID:   course.AccountID,
			ActorID:    submission.UserID,
			Verb:       gamification.VerbGraded,
			ObjectType: gamification.ObjectSubmission,
			ObjectID:   &objectID,
			Result:     resultBlob,
			Context:    contextBlob,
			Source:     gamification.EmitterSource,
		}
		if _, err := emitter.Emit(ctx, event); err != nil {
			slog.Error("graded submission emit: emit failed",
				"submission_id", submissionID,
				"verb", gamification.VerbGraded,
				"error", err)
			return
		}
	}
}
