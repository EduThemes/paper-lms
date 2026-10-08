package service

// Blueprint unsynced-change tracking + migration history. Reports which
// content in the template course has been modified since the last
// completed sync, and the historic migration list.
//
// Wave 5 split (chore/wave5-split-quiz-blueprint).

import (
	"context"
	"fmt"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// GetMigration returns a migration by ID.
func (s *BlueprintService) GetMigration(ctx context.Context, id uint) (*models.BlueprintMigration, error) {
	return s.migRepo.FindByID(ctx, id)
}

// ListMigrations lists migrations for a template.
func (s *BlueprintService) ListMigrations(ctx context.Context, templateID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.BlueprintMigration], error) {
	return s.migRepo.ListByTemplateID(ctx, templateID, params)
}

// UnsyncedChange represents a content item that was modified after the last sync.
type UnsyncedChange struct {
	AssetType string    `json:"asset_type"` // "assignment", "wiki_page", "quiz", "discussion_topic", "context_module"
	AssetID   uint      `json:"asset_id"`
	AssetName string    `json:"asset_name"`
	ChangeAt  time.Time `json:"change_at"`
	// ChangeType is "updated" (created or edited since the last sync) or
	// "deleted" (removed from the blueprint; the next sync removes the
	// copies).
	ChangeType string `json:"change_type"`
}

// unsyncedAssetTypes maps sync content types onto report asset types.
var unsyncedAssetTypes = map[string]string{
	"ContextModule":   "context_module",
	"Assignment":      "assignment",
	"WikiPage":        "wiki_page",
	"Quiz":            "quiz",
	"DiscussionTopic": "discussion_topic",
}

// GetUnsyncedChanges returns content in the template course that has been modified
// since the last completed migration. If no migration exists, all content is returned.
func (s *BlueprintService) GetUnsyncedChanges(ctx context.Context, templateID uint) ([]UnsyncedChange, error) {
	template, err := s.tmplRepo.FindByID(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}
	courseid := template.CourseID

	// Baseline: the most recent COMPLETED migration (a newer failed or
	// running one must not reset the report to "everything").
	since := s.lastCompletedSync(ctx, templateID)
	// If since is zero, all content is "unsynced" (first sync scenario).
	// We still return it so the UI can show what will be pushed.

	var changes []UnsyncedChange
	live := map[string]bool{}

	// Check modules
	modules, err := s.moduleRepo.ListByCourseID(ctx, courseid, bigPage)
	listedAll := err == nil
	if err == nil {
		for _, m := range modules.Items {
			live[contentKey("ContextModule", m.ID)] = true
			if since.IsZero() || m.UpdatedAt.After(since) {
				changes = append(changes, UnsyncedChange{
					AssetType: "context_module",
					AssetID:   m.ID,
					AssetName: m.Name,
					ChangeAt:  m.UpdatedAt,
				})
			}
		}
	}

	// Check assignments
	assignments, err := s.assignRepo.ListByCourseID(ctx, courseid, bigPage)
	listedAll = listedAll && err == nil
	if err == nil {
		for _, a := range assignments.Items {
			live[contentKey("Assignment", a.ID)] = true
			if since.IsZero() || a.UpdatedAt.After(since) {
				changes = append(changes, UnsyncedChange{
					AssetType: "assignment",
					AssetID:   a.ID,
					AssetName: a.Name,
					ChangeAt:  a.UpdatedAt,
				})
			}
		}
	}

	// Check pages
	pages, err := s.pageRepo.ListByCourseID(ctx, courseid, bigPage)
	listedAll = listedAll && err == nil
	if err == nil {
		for _, p := range pages.Items {
			live[contentKey("WikiPage", p.ID)] = true
			if since.IsZero() || p.UpdatedAt.After(since) {
				changes = append(changes, UnsyncedChange{
					AssetType: "wiki_page",
					AssetID:   p.ID,
					AssetName: p.Title,
					ChangeAt:  p.UpdatedAt,
				})
			}
		}
	}

	// Check quizzes
	quizzes, err := s.quizRepo.ListByCourseID(ctx, courseid, bigPage)
	listedAll = listedAll && err == nil
	if err == nil {
		for i := range quizzes.Items {
			q := &quizzes.Items[i]
			live[contentKey("Quiz", q.ID)] = true
			// A question edit is a quiz edit (sync pushes questions too).
			changedAt := s.quizChangedAt(ctx, q)
			if since.IsZero() || changedAt.After(since) {
				changes = append(changes, UnsyncedChange{
					AssetType: "quiz",
					AssetID:   q.ID,
					AssetName: q.Title,
					ChangeAt:  changedAt,
				})
			}
		}
	}

	// Check discussions
	discussions, err := s.discRepo.ListByCourseID(ctx, courseid, 0, bigPage)
	listedAll = listedAll && err == nil
	if err == nil {
		for _, d := range discussions.Items {
			live[contentKey("DiscussionTopic", d.ID)] = true
			if since.IsZero() || d.UpdatedAt.After(since) {
				changes = append(changes, UnsyncedChange{
					AssetType: "discussion_topic",
					AssetID:   d.ID,
					AssetName: d.Title,
					ChangeAt:  d.UpdatedAt,
				})
			}
		}
	}

	for i := range changes {
		changes[i].ChangeType = "updated"
	}
	changes = append(changes, s.unsyncedDeletions(ctx, templateID, live, listedAll)...)
	return changes, nil
}

// unsyncedDeletions reports blueprint items that were synced to some
// associated course and have since been deleted from the blueprint.
func (s *BlueprintService) unsyncedDeletions(ctx context.Context, templateID uint, live map[string]bool, listed bool) []UnsyncedChange {
	if s.tagRepo == nil || !listed {
		return nil
	}
	subs, err := s.subRepo.ListByTemplateID(ctx, templateID, bigPage)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []UnsyncedChange
	for _, sub := range subs.Items {
		if sub.WorkflowState != "active" {
			continue
		}
		tags, err := s.tagRepo.ListByChildCourse(ctx, templateID, sub.ChildCourseID)
		if err != nil {
			continue
		}
		for _, tag := range tags {
			asset, ok := unsyncedAssetTypes[tag.ContentType]
			key := contentKey(tag.ContentType, tag.SourceID)
			if !ok || live[key] || seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, UnsyncedChange{AssetType: asset, AssetID: tag.SourceID, ChangeAt: tag.SyncedAt, ChangeType: "deleted"})
		}
	}
	return out
}
