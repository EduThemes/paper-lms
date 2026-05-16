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

// DiscussionEntryPostedEmitCallback returns a DiscussionEntryPostedCallback
// that loads the entry, walks to its topic + course for the tenant_id
// (course.account_id), constructs a `verb=posted,
// object_type=DiscussionEntry` event, and calls emitter.Emit. Errors at
// every step are logged via slog.Error and NEVER propagated — the
// DiscussionEntryPostedCallback signature has no error return and emit
// failures must not break entry creation.
//
// Fires on every discussion entry (top-level posts and replies alike).
// The result blob carries parent_id (which is *uint and JSON-encodes to
// null for top-level posts) so a downstream rule can filter to
// reply-only via a `parent_id != null` predicate.
//
// Event shape:
//
//	{
//	  occurred_at: entry.CreatedAt,
//	  tenant_id:   course.AccountID,
//	  actor_id:    entry.UserID,
//	  verb:        "posted",
//	  object_type: "DiscussionEntry",
//	  object_id:   entryID,
//	  result:      {"parent_id": <*uint>},
//	  context:     {"course_id": <uint>, "discussion_topic_id": <uint>},
//	  source:      "internal",
//	}
func DiscussionEntryPostedEmitCallback(
	emitter *gamification.Emitter,
	entryRepo repository.DiscussionEntryRepository,
	topicRepo repository.DiscussionTopicRepository,
	courseRepo repository.CourseRepository,
) service.DiscussionEntryPostedCallback {
	return func(ctx context.Context, entryID uint) {
		entry, err := entryRepo.FindByID(ctx, entryID)
		if err != nil {
			slog.Error("discussion entry posted emit: load entry",
				"entry_id", entryID, "error", err)
			return
		}
		topic, err := topicRepo.FindByID(ctx, entry.DiscussionTopicID)
		if err != nil {
			slog.Error("discussion entry posted emit: load topic",
				"entry_id", entryID, "error", err)
			return
		}
		course, err := courseRepo.FindByID(ctx, topic.CourseID, 0)
		if err != nil {
			slog.Error("discussion entry posted emit: load course",
				"entry_id", entryID, "error", err)
			return
		}

		// Result blob: parent_id may be nil (top-level post). json.Marshal
		// emits a Go nil *uint as JSON null, letting downstream rules
		// distinguish replies (parent_id != null) from top-level posts.
		resultBlob, err := json.Marshal(map[string]any{
			"parent_id": entry.ParentID,
		})
		if err != nil {
			slog.Error("discussion entry posted emit: marshal result",
				"entry_id", entryID, "error", err)
			return
		}
		contextBlob, err := json.Marshal(map[string]any{
			"course_id":           topic.CourseID,
			"discussion_topic_id": topic.ID,
		})
		if err != nil {
			slog.Error("discussion entry posted emit: marshal context",
				"entry_id", entryID, "error", err)
			return
		}

		objectID := entryID
		event := &models.GamificationEvent{
			OccurredAt: entry.CreatedAt,
			TenantID:   course.AccountID,
			ActorID:    entry.UserID,
			Verb:       gamification.VerbPosted,
			ObjectType: gamification.ObjectDiscussionEntry,
			ObjectID:   &objectID,
			Result:     resultBlob,
			Context:    contextBlob,
			Source:     gamification.EmitterSource,
		}
		if _, err := emitter.Emit(ctx, event); err != nil {
			slog.Error("discussion entry posted emit: emit failed",
				"entry_id", entryID,
				"verb", gamification.VerbPosted,
				"error", err)
			return
		}
	}
}
