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
}

// GetUnsyncedChanges returns content in the template course that has been modified
// since the last completed migration. If no migration exists, all content is returned.
func (s *BlueprintService) GetUnsyncedChanges(ctx context.Context, templateID uint) ([]UnsyncedChange, error) {
	template, err := s.tmplRepo.FindByID(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}
	courseid := template.CourseID

	// Find the last completed migration to get its completed_at timestamp
	var since time.Time
	migrations, err := s.migRepo.ListByTemplateID(ctx, templateID, repository.PaginationParams{Page: 1, PerPage: 1})
	if err == nil && len(migrations.Items) > 0 {
		// Migrations are returned newest first; find last completed one
		for _, m := range migrations.Items {
			if m.WorkflowState == "completed" && m.CompletedAt != nil {
				since = *m.CompletedAt
				break
			}
		}
	}
	// If since is zero, all content is "unsynced" (first sync scenario).
	// We still return it so the UI can show what will be pushed.

	var changes []UnsyncedChange

	// Check modules
	modules, err := s.moduleRepo.ListByCourseID(ctx, courseid, bigPage)
	if err == nil {
		for _, m := range modules.Items {
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
	if err == nil {
		for _, a := range assignments.Items {
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
	if err == nil {
		for _, p := range pages.Items {
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
	if err == nil {
		for _, q := range quizzes.Items {
			if since.IsZero() || q.UpdatedAt.After(since) {
				changes = append(changes, UnsyncedChange{
					AssetType: "quiz",
					AssetID:   q.ID,
					AssetName: q.Title,
					ChangeAt:  q.UpdatedAt,
				})
			}
		}
	}

	// Check discussions
	discussions, err := s.discRepo.ListByCourseID(ctx, courseid, 0, bigPage)
	if err == nil {
		for _, d := range discussions.Items {
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

	return changes, nil
}
