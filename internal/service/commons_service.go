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

// ErrCommonsNotFound is returned when the source resource or target
// course is missing, deleted, in another course, or in another tenant.
// Handlers map it to a 404 so the caller can't distinguish the cases.
var ErrCommonsNotFound = errors.New("not found")

// CommonsService handles publishing course content to the Commons,
// browsing it, importing it back into a course, and toggling favorites.
//
// Snapshot strategy: we serialize each resource to inline JSON stored in
// shared_content.content_snapshot (jsonb). Quizzes carry their questions
// and modules carry their items plus the content those items point at,
// so an import recreates a working copy rather than empty shells.
type CommonsService struct {
	sharedRepo     repository.SharedContentRepository
	courseRepo     repository.CourseRepository
	assignmentRepo repository.AssignmentRepository
	pageRepo       repository.PageRepository
	quizRepo       repository.QuizRepository
	moduleRepo     repository.ModuleRepository
	discussionRepo repository.DiscussionTopicRepository

	// Optional deep-copy dependencies (SetContentDeps). Without them a
	// quiz snapshot has no questions and a module snapshot no items.
	questionRepo repository.QuizQuestionRepository
	itemRepo     repository.ModuleItemRepository
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

// SetContentDeps wires the quiz-question and module-item repositories
// the snapshot/import paths need to copy quiz questions and module
// items (B145).
func (s *CommonsService) SetContentDeps(questionRepo repository.QuizQuestionRepository, itemRepo repository.ModuleItemRepository) {
	s.questionRepo = questionRepo
	s.itemRepo = itemRepo
}

func validResourceType(t string) bool {
	switch t {
	case "course", "assignment", "page", "quiz", "module", "discussion_topic":
		return true
	}
	return false
}

// commonsQuiz is the snapshot shape of a quiz: the quiz row plus its
// live questions. Older snapshots (no "questions" key) still decode.
type commonsQuiz struct {
	models.Quiz
	Questions []models.QuizQuestion `json:"questions,omitempty"`
}

// Publish snapshots a resource (or a whole course) into the Commons
// catalog so other teachers in the same district can import it.
//
// The source course is loaded under the caller's tenant and every
// single-resource snapshot must belong to that course (B038): a teacher
// can only publish content from the course whose instructor gate they
// passed.
func (s *CommonsService) Publish(ctx context.Context, userID, courseID, accountID uint, opts CommonsPublishOptions) (*models.SharedContent, error) {
	if !validResourceType(opts.ResourceType) {
		return nil, errors.New("invalid resource_type")
	}
	if strings.TrimSpace(opts.Title) == "" {
		return nil, errors.New("title is required")
	}
	if opts.ResourceType != "course" && opts.ResourceID == 0 {
		return nil, errors.New("resource_id is required for non-course exports")
	}
	visibility := opts.Visibility
	if visibility == "" {
		visibility = "account"
	}
	if visibility != "account" && visibility != "public" {
		return nil, errors.New("invalid visibility")
	}

	course, err := s.courseRepo.FindByID(ctx, courseID, accountID)
	if err != nil || course == nil {
		return nil, ErrCommonsNotFound
	}

	snapshotBytes, err := s.buildSnapshot(ctx, course, opts.ResourceType, opts.ResourceID)
	if err != nil {
		return nil, err
	}

	tagsJSON, err := encodeTags(opts.Tags)
	if err != nil {
		return nil, err
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

// buildSnapshot serializes the source resource(s) to JSON. Every lookup
// is tied to the source course: a resource id from another course (or
// tenant) is reported as not found.
func (s *CommonsService) buildSnapshot(ctx context.Context, course *models.Course, resourceType string, resourceID uint) ([]byte, error) {
	bundle := map[string]interface{}{
		"version":       2,
		"resource_type": resourceType,
		"course_meta": map[string]interface{}{
			"name":        course.Name,
			"course_code": course.CourseCode,
			"ui_mode":     course.UIMode,
		},
	}
	acct := course.AccountID

	switch resourceType {
	case "assignment":
		a, err := s.assignmentRepo.FindByID(ctx, resourceID, acct)
		if err != nil || a == nil || a.CourseID != course.ID || a.WorkflowState == models.AssignmentDeleted {
			return nil, ErrCommonsNotFound
		}
		bundle["assignment"] = a
	case "page":
		p, err := s.pageRepo.FindByID(ctx, resourceID, acct)
		if err != nil || p == nil || p.CourseID != course.ID || p.WorkflowState == "deleted" {
			return nil, ErrCommonsNotFound
		}
		bundle["page"] = p
	case "quiz":
		q, err := s.quizRepo.FindByID(ctx, resourceID, acct)
		if err != nil || q == nil || q.CourseID != course.ID || q.WorkflowState == "deleted" {
			return nil, ErrCommonsNotFound
		}
		bundle["quiz"] = s.snapshotQuiz(ctx, q)
	case "module":
		m, err := s.moduleRepo.FindByID(ctx, resourceID, acct)
		if err != nil || m == nil || m.CourseID != course.ID || m.WorkflowState == "deleted" {
			return nil, ErrCommonsNotFound
		}
		m.Items = liveItems(m.Items)
		bundle["module"] = m
		// A module is only useful with the content its items point at.
		s.snapshotModuleContent(ctx, course, []models.ContextModule{*m}, bundle)
	case "discussion_topic":
		d, err := s.discussionRepo.FindByID(ctx, resourceID, acct)
		if err != nil || d == nil || d.CourseID != course.ID || d.WorkflowState == "deleted" {
			return nil, ErrCommonsNotFound
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
			out := make([]commonsQuiz, 0, len(quizzes.Items))
			for i := range quizzes.Items {
				out = append(out, s.snapshotQuiz(ctx, &quizzes.Items[i]))
			}
			bundle["quizzes"] = out
		}
		if mods, err := s.moduleRepo.ListByCourseID(ctx, course.ID, params); err == nil {
			for i := range mods.Items {
				mods.Items[i].Items = liveItems(mods.Items[i].Items)
			}
			bundle["modules"] = mods.Items
		}
		if discussions, err := s.discussionRepo.ListByCourseID(ctx, course.ID, 0, params); err == nil {
			bundle["discussion_topics"] = discussions.Items
		}
	}

	return json.Marshal(bundle)
}

// snapshotQuiz pairs a quiz with its live questions.
func (s *CommonsService) snapshotQuiz(ctx context.Context, q *models.Quiz) commonsQuiz {
	out := commonsQuiz{Quiz: *q}
	if s.questionRepo != nil {
		if qs, err := s.questionRepo.ListByQuizID(ctx, q.ID, repository.PaginationParams{Page: 1, PerPage: 10000}); err == nil {
			out.Questions = qs.Items
		}
	}
	return out
}

// snapshotModuleContent adds the content referenced by the given
// modules' items to the bundle's resource arrays. Content outside the
// source course is skipped.
func (s *CommonsService) snapshotModuleContent(ctx context.Context, course *models.Course, mods []models.ContextModule, bundle map[string]interface{}) {
	acct := course.AccountID
	var (
		assignments []models.Assignment
		pages       []models.WikiPage
		quizzes     []commonsQuiz
		discussions []models.DiscussionTopic
	)
	seen := map[string]bool{}
	for _, m := range mods {
		for _, it := range m.Items {
			if it.ContentID == nil {
				continue
			}
			key := contentKey(it.ContentType, *it.ContentID)
			if seen[key] {
				continue
			}
			seen[key] = true
			switch normalizeModuleContentType(it.ContentType) {
			case "Assignment":
				if a, err := s.assignmentRepo.FindByID(ctx, *it.ContentID, acct); err == nil && a != nil && a.CourseID == course.ID {
					assignments = append(assignments, *a)
				}
			case "WikiPage":
				if p, err := s.pageRepo.FindByID(ctx, *it.ContentID, acct); err == nil && p != nil && p.CourseID == course.ID {
					pages = append(pages, *p)
				}
			case "Quiz":
				if q, err := s.quizRepo.FindByID(ctx, *it.ContentID, acct); err == nil && q != nil && q.CourseID == course.ID {
					quizzes = append(quizzes, s.snapshotQuiz(ctx, q))
				}
			case "DiscussionTopic":
				if d, err := s.discussionRepo.FindByID(ctx, *it.ContentID, acct); err == nil && d != nil && d.CourseID == course.ID {
					discussions = append(discussions, *d)
				}
			}
		}
	}
	bundle["assignments"] = assignments
	bundle["pages"] = pages
	bundle["quizzes"] = quizzes
	bundle["discussion_topics"] = discussions
}

// liveItems drops deleted module items from a preloaded item slice.
func liveItems(items []models.ContentTag) []models.ContentTag {
	out := make([]models.ContentTag, 0, len(items))
	for _, it := range items {
		if it.WorkflowState != models.ContentTagDeleted {
			out = append(out, it)
		}
	}
	return out
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

// commonsImporter carries the per-import state: the target course, the
// importing user, and the old→new id map module items are remapped
// through.
type commonsImporter struct {
	s        *CommonsService
	target   uint
	userID   uint
	result   *CommonsImportResult
	idMap    map[string]uint
	firstErr error
}

func (im *commonsImporter) note(err error) {
	if err != nil && im.firstErr == nil {
		im.firstErr = err
	}
}

func (im *commonsImporter) assignment(ctx context.Context, raw json.RawMessage) error {
	var a models.Assignment
	if err := json.Unmarshal(raw, &a); err != nil {
		return err
	}
	oldID := a.ID
	a.ID = 0
	a.CourseID = im.target
	a.AssignmentGroupID = nil
	a.GroupCategoryID = nil
	if err := im.s.assignmentRepo.Create(ctx, &a); err != nil {
		return err
	}
	im.idMap[contentKey("Assignment", oldID)] = a.ID
	im.result.CreatedIDs["assignments"] = append(im.result.CreatedIDs["assignments"], a.ID)
	return nil
}

func (im *commonsImporter) page(ctx context.Context, raw json.RawMessage) error {
	var p models.WikiPage
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	oldID := p.ID
	p.ID = 0
	p.CourseID = im.target
	// The target course keeps its own front page.
	p.FrontPage = false
	if err := im.s.pageRepo.Create(ctx, &p); err != nil {
		return err
	}
	im.idMap[contentKey("WikiPage", oldID)] = p.ID
	im.result.CreatedIDs["pages"] = append(im.result.CreatedIDs["pages"], p.ID)
	return nil
}

func (im *commonsImporter) quiz(ctx context.Context, raw json.RawMessage) error {
	var cq commonsQuiz
	if err := json.Unmarshal(raw, &cq); err != nil {
		return err
	}
	q := cq.Quiz
	oldID := q.ID
	q.ID = 0
	q.CourseID = im.target
	if err := im.s.quizRepo.Create(ctx, &q); err != nil {
		return err
	}
	if im.s.questionRepo != nil {
		for _, src := range cq.Questions {
			nq := src
			nq.ID = 0
			nq.QuizID = q.ID
			// Group, bank-item and stimulus rows live with the source
			// quiz/course; they are not part of the snapshot.
			nq.QuizQuestionGroupID = nil
			nq.BankItemID = nil
			nq.StimulusID = nil
			if err := im.s.questionRepo.Create(ctx, &nq); err != nil {
				return fmt.Errorf("copy question: %w", err)
			}
		}
	}
	im.idMap[contentKey("Quiz", oldID)] = q.ID
	im.result.CreatedIDs["quizzes"] = append(im.result.CreatedIDs["quizzes"], q.ID)
	return nil
}

func (im *commonsImporter) discussion(ctx context.Context, raw json.RawMessage) error {
	var d models.DiscussionTopic
	if err := json.Unmarshal(raw, &d); err != nil {
		return err
	}
	oldID := d.ID
	d.ID = 0
	d.CourseID = im.target
	if im.userID != 0 {
		// The importing teacher owns the copy; the original author may
		// not even be in this course.
		d.UserID = im.userID
	}
	if err := im.s.discussionRepo.Create(ctx, &d); err != nil {
		return err
	}
	im.idMap[contentKey("DiscussionTopic", oldID)] = d.ID
	im.result.CreatedIDs["discussion_topics"] = append(im.result.CreatedIDs["discussion_topics"], d.ID)
	return nil
}

// module creates the module and its items, remapping each item's
// content_id through the id map. Items whose content was not part of the
// snapshot keep no content link (rather than pointing back into the
// source course).
func (im *commonsImporter) module(ctx context.Context, raw json.RawMessage) error {
	var m models.ContextModule
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	items := m.Items
	m.ID = 0
	m.CourseID = im.target
	m.Items = nil
	if err := im.s.moduleRepo.Create(ctx, &m); err != nil {
		return err
	}
	im.result.CreatedIDs["modules"] = append(im.result.CreatedIDs["modules"], m.ID)
	if im.s.itemRepo == nil {
		return nil
	}
	for _, src := range items {
		if src.WorkflowState == models.ContentTagDeleted {
			continue
		}
		it := models.ContentTag{
			ContextModuleID: m.ID,
			ContentType:     src.ContentType,
			Title:           src.Title,
			Position:        src.Position,
			URL:             src.URL,
			Indent:          src.Indent,
			NewTab:          src.NewTab,
			WorkflowState:   src.WorkflowState,
			CompletionType:  src.CompletionType,
			MinScore:        src.MinScore,
		}
		if src.ContentID != nil {
			newID, ok := im.idMap[contentKey(src.ContentType, *src.ContentID)]
			if !ok {
				// Content not in the snapshot (file, LTI tool, …):
				// skip rather than create a dangling link.
				continue
			}
			it.ContentID = &newID
			if normalizeModuleContentType(src.ContentType) == "WikiPage" {
				if p, err := im.s.pageRepo.FindByID(ctx, newID, 0); err == nil && p != nil {
					it.URL = p.URL
				}
			}
		}
		if err := im.s.itemRepo.Create(ctx, &it); err != nil {
			return fmt.Errorf("copy module item %q: %w", src.Title, err)
		}
		im.result.CreatedIDs["module_items"] = append(im.result.CreatedIDs["module_items"], it.ID)
	}
	return nil
}

// eachRaw decodes a JSON array and runs fn on each element, recording
// (not aborting on) per-element failures.
func (im *commonsImporter) eachRaw(ctx context.Context, raw json.RawMessage, fn func(context.Context, json.RawMessage) error) {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return
	}
	for _, r := range arr {
		im.note(fn(ctx, r))
	}
}

// Import clones the snapshotted resource(s) into the target course. It
// re-creates rows via the standard repository constructors so all the
// existing validation and indexing applies. Download count is bumped.
//
// Both the Commons item and the target course are resolved under the
// caller's tenant (B036): an item or course in another tenant is
// ErrCommonsNotFound. The caller's write permission on the target course
// is enforced by the instructor route middleware on
// POST /courses/:course_id/commons/:id/import.
func (s *CommonsService) Import(ctx context.Context, userID, targetCourseID, sharedContentID, accountID uint) (*CommonsImportResult, error) {
	if accountID == 0 {
		return nil, ErrCommonsNotFound
	}
	item, err := s.sharedRepo.FindByID(ctx, sharedContentID, accountID)
	if err != nil || item == nil {
		return nil, ErrCommonsNotFound
	}
	target, err := s.courseRepo.FindByID(ctx, targetCourseID, accountID)
	if err != nil || target == nil || target.AccountID != accountID {
		return nil, ErrCommonsNotFound
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
	im := &commonsImporter{s: s, target: targetCourseID, userID: userID, result: result, idMap: map[string]uint{}}

	switch item.ResourceType {
	case "assignment":
		if raw, ok := bundle["assignment"]; ok {
			if err := im.assignment(ctx, raw); err != nil {
				return nil, err
			}
		}
	case "page":
		if raw, ok := bundle["page"]; ok {
			if err := im.page(ctx, raw); err != nil {
				return nil, err
			}
		}
	case "quiz":
		if raw, ok := bundle["quiz"]; ok {
			if err := im.quiz(ctx, raw); err != nil {
				return nil, err
			}
		}
	case "discussion_topic":
		if raw, ok := bundle["discussion_topic"]; ok {
			if err := im.discussion(ctx, raw); err != nil {
				return nil, err
			}
		}
	case "module", "course":
		// Content first so the module items can be remapped onto it.
		if raw, ok := bundle["assignments"]; ok {
			im.eachRaw(ctx, raw, im.assignment)
		}
		if raw, ok := bundle["pages"]; ok {
			im.eachRaw(ctx, raw, im.page)
		}
		if raw, ok := bundle["quizzes"]; ok {
			im.eachRaw(ctx, raw, im.quiz)
		}
		if raw, ok := bundle["discussion_topics"]; ok {
			im.eachRaw(ctx, raw, im.discussion)
		}
		if item.ResourceType == "module" {
			if raw, ok := bundle["module"]; ok {
				if err := im.module(ctx, raw); err != nil {
					return nil, err
				}
			}
		} else if raw, ok := bundle["modules"]; ok {
			im.eachRaw(ctx, raw, im.module)
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
