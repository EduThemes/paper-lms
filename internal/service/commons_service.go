package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// CommonsPublishOptions parameterizes a Publish call.
type CommonsPublishOptions struct {
	ResourceType string // course | assignment | page | quiz | module | discussion_topic
	ResourceID   uint   // ignored when ResourceType == "course"
	Title        string
	Description  string
	Subject      string
	GradeLevel   string
	Tags         []string
	ThumbnailURL string
	Visibility   string // "account" (default) | "public"
}

// CommonsService handles publishing course content to the Commons,
// browsing it, importing it back into a course, and toggling favorites.
//
// Snapshot strategy: we serialize each resource to inline JSON stored in
// shared_content.content_snapshot (jsonb). For full-course exports we
// build a multi-resource bundle in the same JSON shape that the existing
// IMSCC content-migrations importer can consume — a Commons import is
// effectively a content_migration with source=commons (the importer in
// internal/api/v1/handlers/content_migrations.go remains the single
// place that knows how to write resources into a target course).
type CommonsService struct {
	sharedRepo     repository.SharedContentRepository
	courseRepo     repository.CourseRepository
	assignmentRepo repository.AssignmentRepository
	pageRepo       repository.PageRepository
	quizRepo       repository.QuizRepository
	moduleRepo     repository.ModuleRepository
	discussionRepo repository.DiscussionTopicRepository
}

func NewCommonsService(
	sharedRepo repository.SharedContentRepository,
	courseRepo repository.CourseRepository,
	assignmentRepo repository.AssignmentRepository,
	pageRepo repository.PageRepository,
	quizRepo repository.QuizRepository,
	moduleRepo repository.ModuleRepository,
	discussionRepo repository.DiscussionTopicRepository,
) *CommonsService {
	return &CommonsService{
		sharedRepo:     sharedRepo,
		courseRepo:     courseRepo,
		assignmentRepo: assignmentRepo,
		pageRepo:       pageRepo,
		quizRepo:       quizRepo,
		moduleRepo:     moduleRepo,
		discussionRepo: discussionRepo,
	}
}

func validResourceType(t string) bool {
	switch t {
	case "course", "assignment", "page", "quiz", "module", "discussion_topic":
		return true
	}
	return false
}

// Publish snapshots a resource (or a whole course) into the Commons
// catalog so other teachers in the same district can import it.
func (s *CommonsService) Publish(ctx context.Context, userID, courseID uint, opts CommonsPublishOptions) (*models.SharedContent, error) {
	if !validResourceType(opts.ResourceType) {
		return nil, errors.New("invalid resource_type")
	}
	if strings.TrimSpace(opts.Title) == "" {
		return nil, errors.New("title is required")
	}
	if opts.ResourceType != "course" && opts.ResourceID == 0 {
		return nil, errors.New("resource_id is required for non-course exports")
	}

	course, err := s.courseRepo.FindByID(ctx, courseID, 0)
	if err != nil {
		return nil, fmt.Errorf("source course not found: %w", err)
	}

	snapshotBytes, err := s.buildSnapshot(ctx, course, opts.ResourceType, opts.ResourceID)
	if err != nil {
		return nil, err
	}

	tagsJSON, err := encodeTags(opts.Tags)
	if err != nil {
		return nil, err
	}

	visibility := opts.Visibility
	if visibility == "" {
		visibility = "account"
	}

	var sourceContentID *uint
	if opts.ResourceType != "course" {
		id := opts.ResourceID
		sourceContentID = &id
	}

	item := &models.SharedContent{
		AccountID:       course.AccountID,
		AuthorUserID:    userID,
		Title:           opts.Title,
		Description:     opts.Description,
		ResourceType:    opts.ResourceType,
		SourceCourseID:  courseID,
		SourceContentID: sourceContentID,
		Subject:         opts.Subject,
		GradeLevel:      opts.GradeLevel,
		Tags:            tagsJSON,
		ThumbnailURL:    opts.ThumbnailURL,
		ContentSnapshot: string(snapshotBytes),
		Visibility:      visibility,
	}

	if err := s.sharedRepo.Create(ctx, item); err != nil {
		return nil, fmt.Errorf("could not publish: %w", err)
	}
	return item, nil
}

// buildSnapshot serializes the source resource(s) to JSON. For
// resource_type=="course" it bundles all major resource families.
func (s *CommonsService) buildSnapshot(ctx context.Context, course *models.Course, resourceType string, resourceID uint) ([]byte, error) {
	bundle := map[string]interface{}{
		"version":       1,
		"resource_type": resourceType,
		"course_meta": map[string]interface{}{
			"name":        course.Name,
			"course_code": course.CourseCode,
			"ui_mode":     course.UIMode,
		},
	}

	switch resourceType {
	case "assignment":
		a, err := s.assignmentRepo.FindByID(ctx, resourceID, 0)
		if err != nil {
			return nil, fmt.Errorf("assignment not found: %w", err)
		}
		bundle["assignment"] = a
	case "page":
		p, err := s.pageRepo.FindByID(ctx, resourceID, 0)
		if err != nil {
			return nil, fmt.Errorf("page not found: %w", err)
		}
		bundle["page"] = p
	case "quiz":
		q, err := s.quizRepo.FindByID(ctx, resourceID, 0)
		if err != nil {
			return nil, fmt.Errorf("quiz not found: %w", err)
		}
		bundle["quiz"] = q
	case "module":
		m, err := s.moduleRepo.FindByID(ctx, resourceID, 0)
		if err != nil {
			return nil, fmt.Errorf("module not found: %w", err)
		}
		bundle["module"] = m
	case "discussion_topic":
		d, err := s.discussionRepo.FindByID(ctx, resourceID, 0)
		if err != nil {
			return nil, fmt.Errorf("discussion topic not found: %w", err)
		}
		bundle["discussion_topic"] = d
	case "course":
		// Bundle all major resources. Each list call uses a generous
		// page size; the snapshot is written once at publish time.
		params := repository.PaginationParams{Page: 1, PerPage: 500}

		if assignments, err := s.assignmentRepo.ListByCourseID(ctx, course.ID, params); err == nil {
			bundle["assignments"] = assignments.Items
		}
		if pages, err := s.pageRepo.ListByCourseID(ctx, course.ID, params); err == nil {
			bundle["pages"] = pages.Items
		}
		if quizzes, err := s.quizRepo.ListByCourseID(ctx, course.ID, params); err == nil {
			bundle["quizzes"] = quizzes.Items
		}
		if mods, err := s.moduleRepo.ListByCourseID(ctx, course.ID, params); err == nil {
			bundle["modules"] = mods.Items
		}
		if discussions, err := s.discussionRepo.ListByCourseID(ctx, course.ID, 0, params); err == nil {
			bundle["discussion_topics"] = discussions.Items
		}
	}

	return json.Marshal(bundle)
}

// Browse returns a paginated catalog scoped to the caller's district.
func (s *CommonsService) Browse(ctx context.Context, accountID uint, filters repository.SharedContentFilters, params repository.PaginationParams) (*repository.PaginatedResult[models.SharedContent], error) {
	return s.sharedRepo.ListByAccount(ctx, accountID, filters, params)
}

// Get returns a single Commons item by ID, scoped to the caller's tenant.
// accountID==0 disables the tenant scope (privileged internal callers).
func (s *CommonsService) Get(ctx context.Context, id, accountID uint) (*models.SharedContent, error) {
	return s.sharedRepo.FindByID(ctx, id, accountID)
}

// CommonsImportResult summarizes what a Commons import created in the
// target course. Named distinctly from the IMSCC ImportResult that lives
// in imscc_parser.go.
type CommonsImportResult struct {
	SharedContentID uint              `json:"shared_content_id"`
	TargetCourseID  uint              `json:"target_course_id"`
	ResourceType    string            `json:"resource_type"`
	CreatedIDs      map[string][]uint `json:"created_ids"` // e.g. {"assignments": [1,2], "pages": [3]}
}

// Import clones the snapshotted resource(s) into the target course. It
// re-creates rows via the standard repository constructors so all the
// existing validation and indexing applies. Download count is bumped.
func (s *CommonsService) Import(ctx context.Context, userID, targetCourseID, sharedContentID, accountID uint) (*CommonsImportResult, error) {
	item, err := s.sharedRepo.FindByID(ctx, sharedContentID, accountID)
	if err != nil {
		return nil, fmt.Errorf("commons item not found: %w", err)
	}
	target, err := s.courseRepo.FindByID(ctx, targetCourseID, 0)
	if err != nil {
		return nil, fmt.Errorf("target course not found: %w", err)
	}
	if target.AccountID != item.AccountID && item.Visibility != "public" {
		return nil, errors.New("commons item is not available in this account")
	}

	var bundle map[string]json.RawMessage
	if err := json.Unmarshal([]byte(item.ContentSnapshot), &bundle); err != nil {
		return nil, fmt.Errorf("could not decode snapshot: %w", err)
	}

	result := &CommonsImportResult{
		SharedContentID: item.ID,
		TargetCourseID:  targetCourseID,
		ResourceType:    item.ResourceType,
		CreatedIDs:      map[string][]uint{},
	}

	// Reset IDs/timestamps to force fresh inserts in the target course.
	importAssignment := func(raw json.RawMessage) error {
		var a models.Assignment
		if err := json.Unmarshal(raw, &a); err != nil {
			return err
		}
		a.ID = 0
		a.CourseID = targetCourseID
		a.AssignmentGroupID = nil
		a.GroupCategoryID = nil
		if err := s.assignmentRepo.Create(ctx, &a); err != nil {
			return err
		}
		result.CreatedIDs["assignments"] = append(result.CreatedIDs["assignments"], a.ID)
		return nil
	}
	importPage := func(raw json.RawMessage) error {
		var p models.WikiPage
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		p.ID = 0
		p.CourseID = targetCourseID
		if err := s.pageRepo.Create(ctx, &p); err != nil {
			return err
		}
		result.CreatedIDs["pages"] = append(result.CreatedIDs["pages"], p.ID)
		return nil
	}
	importQuiz := func(raw json.RawMessage) error {
		var q models.Quiz
		if err := json.Unmarshal(raw, &q); err != nil {
			return err
		}
		q.ID = 0
		q.CourseID = targetCourseID
		if err := s.quizRepo.Create(ctx, &q); err != nil {
			return err
		}
		result.CreatedIDs["quizzes"] = append(result.CreatedIDs["quizzes"], q.ID)
		return nil
	}
	importModule := func(raw json.RawMessage) error {
		var m models.ContextModule
		if err := json.Unmarshal(raw, &m); err != nil {
			return err
		}
		m.ID = 0
		m.CourseID = targetCourseID
		m.Items = nil
		if err := s.moduleRepo.Create(ctx, &m); err != nil {
			return err
		}
		result.CreatedIDs["modules"] = append(result.CreatedIDs["modules"], m.ID)
		return nil
	}
	importDiscussion := func(raw json.RawMessage) error {
		var d models.DiscussionTopic
		if err := json.Unmarshal(raw, &d); err != nil {
			return err
		}
		d.ID = 0
		d.CourseID = targetCourseID
		if err := s.discussionRepo.Create(ctx, &d); err != nil {
			return err
		}
		result.CreatedIDs["discussion_topics"] = append(result.CreatedIDs["discussion_topics"], d.ID)
		return nil
	}

	switch item.ResourceType {
	case "assignment":
		if raw, ok := bundle["assignment"]; ok {
			if err := importAssignment(raw); err != nil {
				return nil, err
			}
		}
	case "page":
		if raw, ok := bundle["page"]; ok {
			if err := importPage(raw); err != nil {
				return nil, err
			}
		}
	case "quiz":
		if raw, ok := bundle["quiz"]; ok {
			if err := importQuiz(raw); err != nil {
				return nil, err
			}
		}
	case "module":
		if raw, ok := bundle["module"]; ok {
			if err := importModule(raw); err != nil {
				return nil, err
			}
		}
	case "discussion_topic":
		if raw, ok := bundle["discussion_topic"]; ok {
			if err := importDiscussion(raw); err != nil {
				return nil, err
			}
		}
	case "course":
		if raw, ok := bundle["assignments"]; ok {
			var arr []json.RawMessage
			_ = json.Unmarshal(raw, &arr)
			for _, r := range arr {
				_ = importAssignment(r)
			}
		}
		if raw, ok := bundle["pages"]; ok {
			var arr []json.RawMessage
			_ = json.Unmarshal(raw, &arr)
			for _, r := range arr {
				_ = importPage(r)
			}
		}
		if raw, ok := bundle["quizzes"]; ok {
			var arr []json.RawMessage
			_ = json.Unmarshal(raw, &arr)
			for _, r := range arr {
				_ = importQuiz(r)
			}
		}
		if raw, ok := bundle["modules"]; ok {
			var arr []json.RawMessage
			_ = json.Unmarshal(raw, &arr)
			for _, r := range arr {
				_ = importModule(r)
			}
		}
		if raw, ok := bundle["discussion_topics"]; ok {
			var arr []json.RawMessage
			_ = json.Unmarshal(raw, &arr)
			for _, r := range arr {
				_ = importDiscussion(r)
			}
		}
	}

	if err := s.sharedRepo.IncrementDownloadCount(ctx, item.ID); err != nil {
		// Non-fatal; return the import result anyway.
		_ = err
	}

	return result, nil
}

// ToggleFavorite flips the caller's favorite for a Commons item.
func (s *CommonsService) ToggleFavorite(ctx context.Context, userID, sharedContentID, accountID uint) (bool, error) {
	if _, err := s.sharedRepo.FindByID(ctx, sharedContentID, accountID); err != nil {
		return false, err
	}
	return s.sharedRepo.ToggleFavorite(ctx, sharedContentID, userID)
}

// ListFavorites returns the caller's favorited Commons items.
func (s *CommonsService) ListFavorites(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.SharedContent], error) {
	return s.sharedRepo.ListUserFavorites(ctx, userID, params)
}

// IsFavorited tells whether the user has favorited this Commons item.
func (s *CommonsService) IsFavorited(ctx context.Context, userID, sharedContentID uint) (bool, error) {
	return s.sharedRepo.IsFavorited(ctx, sharedContentID, userID)
}

// encodeTags serializes a tags slice to a JSON-array string for storage
// in the jsonb column. nil/empty becomes "[]".
func encodeTags(tags []string) (string, error) {
	if len(tags) == 0 {
		return "[]", nil
	}
	b, err := json.Marshal(tags)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
