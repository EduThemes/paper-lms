// Package wiring adapts the cross-service "something happened" callbacks
// (SubmissionGradedCallback, ContentViewedCallback, etc.) into
// gamification.Emit calls. Each adapter walks whatever ownership chain
// the source row needs to resolve the tenant_id, builds an xAPI-shaped
// event, and emits it. cmd/server/main.go wires the resulting callbacks
// onto the corresponding services at startup.
//
// Adapters log and swallow errors: a gamification failure must never
// fail the user-visible operation (page render, grade write, etc.).
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

// ViewedContentEmitCallback returns a ContentViewedCallback that, given
// (userID, objectType, objectID, durationSeconds), walks the object's
// owning context (page → course → account) for the tenant_id and emits
// a `verb=viewed` event through the supplied Emitter.
//
// Wave 1 vocabulary supports objectType == gamification.ObjectPage only.
// Other object types are logged as a warning and skipped (no emit, no
// error) so the callback fan-out at the ContentViewService never sees
// an emit failure for a not-yet-supported type. Future content types
// (Quiz, Assignment as a "viewed" target, etc.) extend the switch.
//
// All errors — repo lookups, JSON marshal, emit — are logged via
// slog.Error and swallowed. The callback's contract with
// ContentViewService.RecordView is fire-and-forget; surfacing an error
// here would only crash the goroutine wrapper.
func ViewedContentEmitCallback(
	emitter *gamification.Emitter,
	pageRepo repository.PageRepository,
	courseRepo repository.CourseRepository,
) service.ContentViewedCallback {
	return func(ctx context.Context, userID uint, objectType string, objectID uint, durationSeconds int64) {
		switch objectType {
		case gamification.ObjectPage:
			// supported below
		default:
			slog.Warn("gamification: unsupported content-view object type, skipping emit",
				"object_type", objectType,
				"object_id", objectID,
				"user_id", userID,
			)
			return
		}

		page, err := pageRepo.FindByID(ctx, objectID, 0)
		if err != nil {
			slog.Error("gamification: load page for view emit",
				"object_type", objectType,
				"object_id", objectID,
				"user_id", userID,
				"error", err,
			)
			return
		}
		if page == nil {
			slog.Error("gamification: page not found for view emit",
				"object_id", objectID,
				"user_id", userID,
			)
			return
		}

		course, err := courseRepo.FindByID(ctx, page.CourseID, 0)
		if err != nil {
			slog.Error("gamification: load course for view emit",
				"course_id", page.CourseID,
				"page_id", page.ID,
				"user_id", userID,
				"error", err,
			)
			return
		}
		if course == nil {
			slog.Error("gamification: course not found for view emit",
				"course_id", page.CourseID,
				"page_id", page.ID,
				"user_id", userID,
			)
			return
		}

		resultJSON, err := json.Marshal(map[string]any{
			"duration_seconds": durationSeconds,
		})
		if err != nil {
			slog.Error("gamification: marshal view result",
				"page_id", page.ID,
				"user_id", userID,
				"error", err,
			)
			return
		}
		contextJSON, err := json.Marshal(map[string]any{
			"course_id": page.CourseID,
		})
		if err != nil {
			slog.Error("gamification: marshal view context",
				"page_id", page.ID,
				"user_id", userID,
				"error", err,
			)
			return
		}

		objID := objectID
		event := &models.GamificationEvent{
			OccurredAt: time.Now(),
			TenantID:   course.AccountID,
			ActorID:    userID,
			Verb:       gamification.VerbViewed,
			ObjectType: objectType,
			ObjectID:   &objID,
			Result:     resultJSON,
			Context:    contextJSON,
			Source:     gamification.EmitterSource,
		}
		if _, err := emitter.Emit(ctx, event); err != nil {
			slog.Error("gamification: emit viewed event",
				"page_id", page.ID,
				"user_id", userID,
				"tenant_id", course.AccountID,
				"error", err,
			)
		}
	}
}
