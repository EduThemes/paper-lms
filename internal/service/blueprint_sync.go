package service

// Blueprint sync orchestration. It copies content (modules, assignments,
// pages, quizzes with their questions, discussion topics and module
// items) from a blueprint template course to each associated child
// course, and on later syncs pushes blueprint edits to the existing
// copies (B041).
//
// Identity: each copy is recorded in blueprint_content_tags (source id →
// child id per child course), so a renamed blueprint item updates its
// copy instead of spawning a duplicate. Copies made before the identity
// map existed are adopted by title on the first sync that sees them.
//
// Change propagation follows Canvas's master-courses rules:
//   - a blueprint item changed since the last completed sync is pushed;
//   - if the child copy was edited locally since it was last synced (a
//     "downstream change"), the local edit wins — unless the blueprint
//     locks content (default restrictions {"content": true}), in which
//     case the blueprint version is forced.
//
// Wave 5 split (chore/wave5-split-quiz-blueprint). All methods hang off
// *BlueprintService so the handler call surface is unchanged.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// downstreamSlack absorbs clock/rounding noise between a child row's
// updated_at (written by the sync) and the tag's synced_at.
const downstreamSlack = time.Second

// TriggerSync creates a migration record and performs actual content sync from the
// blueprint template course to all associated child courses.
func (s *BlueprintService) TriggerSync(ctx context.Context, templateID, userID uint, comment string) (*models.BlueprintMigration, error) {
	if templateID == 0 {
		return nil, errors.New("template_id is required")
	}
	if userID == 0 {
		return nil, errors.New("user_id is required")
	}

	// Look up the template to get the source course ID
	template, err := s.tmplRepo.FindByID(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}
	sourceCourseID := template.CourseID

	// The previous completed sync is the baseline for "changed since".
	lastSync := s.lastCompletedSync(ctx, templateID)

	// Create a migration record in "running" state
	migration := &models.BlueprintMigration{
		BlueprintTemplateID: templateID,
		UserID:              userID,
		WorkflowState:       "running",
		Comment:             comment,
		ExportSettings:      "{}",
	}
	if err := s.migRepo.Create(ctx, migration); err != nil {
		return nil, err
	}

	// Fetch all subscriptions (child courses)
	subs, err := s.subRepo.ListByTemplateID(ctx, templateID, repository.PaginationParams{Page: 1, PerPage: 10000})
	if err != nil {
		s.failMigration(ctx, migration, err)
		return migration, err
	}

	// Perform content sync for each child course
	var syncErrors []string
	for _, sub := range subs.Items {
		if sub.WorkflowState != "active" {
			continue
		}
		sc := &syncCtx{
			templateID:  templateID,
			child:       sub.ChildCourseID,
			lastSync:    lastSync,
			lockContent: contentLocked(template),
			idMap:       make(map[string]uint),
		}
		if err := s.syncCourseContent(ctx, sourceCourseID, sc); err != nil {
			slog.Error("blueprint sync failed for child course",
				"template_id", templateID,
				"source_course_id", sourceCourseID,
				"child_course_id", sub.ChildCourseID,
				"error", err,
			)
			syncErrors = append(syncErrors, fmt.Sprintf("course %d: %v", sub.ChildCourseID, err))
		}
	}

	// Mark migration as completed (or failed if any child course errored)
	now := time.Now()
	migration.CompletedAt = &now
	if len(syncErrors) > 0 {
		migration.WorkflowState = "completed" // partial success — still mark completed
		migration.Comment = comment + " | sync warnings: " + strings.Join(syncErrors, "; ")
	} else {
		migration.WorkflowState = "completed"
	}
	if err := s.migRepo.Update(ctx, migration); err != nil {
		return migration, err
	}

	return migration, nil
}

// lastCompletedSync returns the completion time of the template's most
// recent completed migration, or the zero time when it was never synced.
func (s *BlueprintService) lastCompletedSync(ctx context.Context, templateID uint) time.Time {
	migrations, err := s.migRepo.ListByTemplateID(ctx, templateID, repository.PaginationParams{Page: 1, PerPage: 100})
	if err != nil {
		return time.Time{}
	}
	for _, m := range migrations.Items {
		if m.WorkflowState == "completed" && m.CompletedAt != nil {
			return *m.CompletedAt
		}
	}
	return time.Time{}
}

// contentLocked reports whether the template's default restrictions
// lock content, which makes blueprint edits overwrite local child edits.
func contentLocked(t *models.BlueprintTemplate) bool {
	if t == nil || !t.UseDefaultRestrictions || strings.TrimSpace(t.DefaultRestrictions) == "" {
		return false
	}
	var r map[string]interface{}
	if err := json.Unmarshal([]byte(t.DefaultRestrictions), &r); err != nil {
		return false
	}
	locked, _ := r["content"].(bool)
	return locked
}

// failMigration marks a migration as failed.
func (s *BlueprintService) failMigration(ctx context.Context, migration *models.BlueprintMigration, reason error) {
	now := time.Now()
	migration.WorkflowState = "failed"
	migration.CompletedAt = &now
	migration.Comment = migration.Comment + " | error: " + reason.Error()
	_ = s.migRepo.Update(ctx, migration)
}

// syncCtx is the per-child-course sync state.
type syncCtx struct {
	templateID  uint
	child       uint
	lastSync    time.Time // zero = first sync
	lockContent bool
	// idMap maps contentKey(type, sourceID) → child id, for remapping
	// module item content ids.
	idMap map[string]uint
}

// sourceChanged reports whether a blueprint row changed since the last
// completed sync.
func (sc *syncCtx) sourceChanged(updatedAt time.Time) bool {
	return sc.lastSync.IsZero() || updatedAt.After(sc.lastSync)
}

// shouldPush decides whether a changed blueprint row overwrites its
// existing child copy: yes unless the copy was edited locally since it
// was last synced and the blueprint does not lock content.
func (sc *syncCtx) shouldPush(srcUpdated time.Time, tag *models.BlueprintContentTag, childUpdated time.Time) bool {
	if !sc.sourceChanged(srcUpdated) {
		return false
	}
	if sc.lockContent {
		return true
	}
	var baseline time.Time
	switch {
	case tag != nil:
		baseline = tag.SyncedAt
	case !sc.lastSync.IsZero():
		baseline = sc.lastSync
	default:
		return true
	}
	downstreamChanged := childUpdated.After(baseline.Add(downstreamSlack))
	return !downstreamChanged
}

// findTag returns the identity-map row for a source item (nil if none or
// the map is not wired).
func (s *BlueprintService) findTag(ctx context.Context, sc *syncCtx, contentType string, sourceID uint) *models.BlueprintContentTag {
	if s.tagRepo == nil {
		return nil
	}
	tag, err := s.tagRepo.Find(ctx, sc.templateID, sc.child, contentType, sourceID)
	if err != nil {
		return nil
	}
	return tag
}

// noteSynced maintains the identity-map row for one source item:
//   - pushed (copy created or overwritten): synced now;
//   - adopted without a push (no tag yet, or the tag pointed at a copy
//     that no longer exists): record the copy with the previous sync as
//     its baseline, so a local edit made since then still counts as a
//     downstream change next time;
//   - otherwise the row is left alone — refreshing synced_at for a copy
//     that was NOT overwritten would hide its local edits.
func (s *BlueprintService) noteSynced(ctx context.Context, sc *syncCtx, contentType string, sourceID, childID uint, tag *models.BlueprintContentTag, pushed bool) error {
	if s.tagRepo == nil {
		return nil
	}
	var syncedAt time.Time
	switch {
	case pushed:
		syncedAt = time.Now()
	case tag == nil || tag.ChildID != childID:
		syncedAt = sc.lastSync
		if syncedAt.IsZero() {
			syncedAt = time.Now()
		}
	default:
		return nil
	}
	return s.tagRepo.Upsert(ctx, &models.BlueprintContentTag{
		BlueprintTemplateID: sc.templateID,
		ChildCourseID:       sc.child,
		ContentType:         contentType,
		SourceID:            sourceID,
		ChildID:             childID,
		SyncedAt:            syncedAt,
	})
}

// syncCourseContent copies / updates all content types from
// sourceCourseID into sc.child.
func (s *BlueprintService) syncCourseContent(ctx context.Context, sourceCourseID uint, sc *syncCtx) error {
	if err := s.syncModules(ctx, sourceCourseID, sc); err != nil {
		return fmt.Errorf("sync modules: %w", err)
	}
	if err := s.syncAssignments(ctx, sourceCourseID, sc); err != nil {
		return fmt.Errorf("sync assignments: %w", err)
	}
	if err := s.syncPages(ctx, sourceCourseID, sc); err != nil {
		return fmt.Errorf("sync pages: %w", err)
	}
	if err := s.syncQuizzes(ctx, sourceCourseID, sc); err != nil {
		return fmt.Errorf("sync quizzes: %w", err)
	}
	if err := s.syncDiscussions(ctx, sourceCourseID, sc); err != nil {
		return fmt.Errorf("sync discussions: %w", err)
	}
	// Module items last, so idMap holds every content copy.
	if err := s.syncModuleItems(ctx, sourceCourseID, sc); err != nil {
		return fmt.Errorf("sync module items: %w", err)
	}
	return nil
}

// ---------- Modules ----------

func applyModuleFields(dst *models.ContextModule, src *models.ContextModule) {
	dst.Name = src.Name
	dst.Position = src.Position
	dst.UnlockAt = src.UnlockAt
	dst.EndAt = src.EndAt
	dst.RequireSequentialProgress = src.RequireSequentialProgress
	dst.WorkflowState = src.WorkflowState
}

func (s *BlueprintService) syncModules(ctx context.Context, sourceCourseID uint, sc *syncCtx) error {
	sourceModules, err := s.moduleRepo.ListByCourseID(ctx, sourceCourseID, bigPage)
	if err != nil {
		return err
	}
	childModules, err := s.moduleRepo.ListByCourseID(ctx, sc.child, bigPage)
	if err != nil {
		return err
	}
	byID := make(map[uint]*models.ContextModule, len(childModules.Items))
	byTitle := make(map[string]*models.ContextModule, len(childModules.Items))
	for i := range childModules.Items {
		m := &childModules.Items[i]
		byID[m.ID] = m
		if _, ok := byTitle[m.Name]; !ok {
			byTitle[m.Name] = m
		}
	}

	for i := range sourceModules.Items {
		src := &sourceModules.Items[i]
		tag := s.findTag(ctx, sc, "ContextModule", src.ID)
		pushed := false
		var child *models.ContextModule
		if tag != nil {
			child = byID[tag.ChildID]
		} else if !sc.lastSync.IsZero() {
			child = byTitle[src.Name] // adopt a pre-identity-map copy
		}

		switch {
		case child == nil:
			if tag != nil && !sc.sourceChanged(src.UpdatedAt) {
				// Deleted in the child course and unchanged in the
				// blueprint: respect the local deletion.
				continue
			}
			nm := &models.ContextModule{CourseID: sc.child}
			applyModuleFields(nm, src)
			if err := s.moduleRepo.Create(ctx, nm); err != nil {
				return fmt.Errorf("create module %q: %w", src.Name, err)
			}
			child = nm
			pushed = true
		case sc.shouldPush(src.UpdatedAt, tag, child.UpdatedAt):
			applyModuleFields(child, src)
			child.Items = nil
			if err := s.moduleRepo.Update(ctx, child); err != nil {
				return fmt.Errorf("update module %q: %w", src.Name, err)
			}
			pushed = true
		}
		sc.idMap[contentKey("ContextModule", src.ID)] = child.ID
		if err := s.noteSynced(ctx, sc, "ContextModule", src.ID, child.ID, tag, pushed); err != nil {
			return err
		}
	}
	return nil
}

// ---------- Assignments ----------

func applyAssignmentFields(dst, src *models.Assignment) {
	dst.Name = src.Name
	dst.Description = src.Description
	dst.DueAt = src.DueAt
	dst.UnlockAt = src.UnlockAt
	dst.LockAt = src.LockAt
	dst.PointsPossible = src.PointsPossible
	dst.GradingType = src.GradingType
	dst.SubmissionTypes = src.SubmissionTypes
	dst.Position = src.Position
	dst.WorkflowState = src.WorkflowState
	dst.Published = src.Published
	dst.AnonymousGrading = src.AnonymousGrading
	dst.PostPolicy = src.PostPolicy
	dst.PeerReviewsEnabled = src.PeerReviewsEnabled
	dst.PeerReviewCount = src.PeerReviewCount
}

func (s *BlueprintService) syncAssignments(ctx context.Context, sourceCourseID uint, sc *syncCtx) error {
	sourceAssignments, err := s.assignRepo.ListByCourseID(ctx, sourceCourseID, bigPage)
	if err != nil {
		return err
	}
	childAssignments, err := s.assignRepo.ListByCourseID(ctx, sc.child, bigPage)
	if err != nil {
		return err
	}
	byID := make(map[uint]*models.Assignment, len(childAssignments.Items))
	byName := make(map[string]*models.Assignment, len(childAssignments.Items))
	for i := range childAssignments.Items {
		a := &childAssignments.Items[i]
		byID[a.ID] = a
		if _, ok := byName[a.Name]; !ok {
			byName[a.Name] = a
		}
	}

	for i := range sourceAssignments.Items {
		src := &sourceAssignments.Items[i]
		tag := s.findTag(ctx, sc, "Assignment", src.ID)
		pushed := false
		var child *models.Assignment
		if tag != nil {
			child = byID[tag.ChildID]
		} else if !sc.lastSync.IsZero() {
			child = byName[src.Name]
		}

		switch {
		case child == nil:
			if tag != nil && !sc.sourceChanged(src.UpdatedAt) {
				continue
			}
			na := &models.Assignment{CourseID: sc.child}
			applyAssignmentFields(na, src)
			if err := s.assignRepo.Create(ctx, na); err != nil {
				return fmt.Errorf("create assignment %q: %w", src.Name, err)
			}
			child = na
			pushed = true
		case sc.shouldPush(src.UpdatedAt, tag, child.UpdatedAt):
			applyAssignmentFields(child, src)
			if err := s.assignRepo.Update(ctx, child); err != nil {
				return fmt.Errorf("update assignment %q: %w", src.Name, err)
			}
			pushed = true
		}
		sc.idMap[contentKey("Assignment", src.ID)] = child.ID
		if err := s.noteSynced(ctx, sc, "Assignment", src.ID, child.ID, tag, pushed); err != nil {
			return err
		}
	}
	return nil
}

// ---------- Pages ----------

func applyPageFields(dst, src *models.WikiPage) {
	dst.Title = src.Title
	dst.Body = src.Body
	dst.WorkflowState = src.WorkflowState
	dst.EditingRoles = src.EditingRoles
	dst.Public = src.Public
	dst.WebsiteMode = src.WebsiteMode
}

func (s *BlueprintService) syncPages(ctx context.Context, sourceCourseID uint, sc *syncCtx) error {
	sourcePages, err := s.pageRepo.ListByCourseID(ctx, sourceCourseID, bigPage)
	if err != nil {
		return err
	}
	childPages, err := s.pageRepo.ListByCourseID(ctx, sc.child, bigPage)
	if err != nil {
		return err
	}
	byID := make(map[uint]*models.WikiPage, len(childPages.Items))
	byTitle := make(map[string]*models.WikiPage, len(childPages.Items))
	for i := range childPages.Items {
		p := &childPages.Items[i]
		byID[p.ID] = p
		if _, ok := byTitle[p.Title]; !ok {
			byTitle[p.Title] = p
		}
	}

	for i := range sourcePages.Items {
		src := &sourcePages.Items[i]
		tag := s.findTag(ctx, sc, "WikiPage", src.ID)
		pushed := false
		var child *models.WikiPage
		if tag != nil {
			child = byID[tag.ChildID]
		} else if !sc.lastSync.IsZero() {
			child = byTitle[src.Title]
		}

		switch {
		case child == nil:
			if tag != nil && !sc.sourceChanged(src.UpdatedAt) {
				continue
			}
			np := &models.WikiPage{CourseID: sc.child, URL: src.URL, FrontPage: src.FrontPage}
			applyPageFields(np, src)
			if err := s.pageRepo.Create(ctx, np); err != nil {
				return fmt.Errorf("create page %q: %w", src.Title, err)
			}
			child = np
			pushed = true
		case sc.shouldPush(src.UpdatedAt, tag, child.UpdatedAt):
			applyPageFields(child, src)
			if err := s.pageRepo.Update(ctx, child); err != nil {
				return fmt.Errorf("update page %q: %w", src.Title, err)
			}
			pushed = true
		}
		sc.idMap[contentKey("WikiPage", src.ID)] = child.ID
		if err := s.noteSynced(ctx, sc, "WikiPage", src.ID, child.ID, tag, pushed); err != nil {
			return err
		}
	}
	return nil
}

// ---------- Quizzes (with questions) ----------

func applyQuizFields(dst, src *models.Quiz) {
	dst.Title = src.Title
	dst.Description = src.Description
	dst.QuizType = src.QuizType
	dst.TimeLimit = src.TimeLimit
	dst.AllowedAttempts = src.AllowedAttempts
	dst.DueAt = src.DueAt
	dst.UnlockAt = src.UnlockAt
	dst.LockAt = src.LockAt
	dst.PointsPossible = src.PointsPossible
	dst.ShuffleAnswers = src.ShuffleAnswers
	dst.ScoringPolicy = src.ScoringPolicy
	dst.ShowCorrectAnswers = src.ShowCorrectAnswers
	dst.HideResults = src.HideResults
	dst.OneQuestionAtATime = src.OneQuestionAtATime
	dst.CantGoBack = src.CantGoBack
	dst.Published = src.Published
	dst.WorkflowState = src.WorkflowState
}

func applyQuestionFields(dst, src *models.QuizQuestion) {
	dst.Position = src.Position
	dst.QuestionType = src.QuestionType
	dst.QuestionText = src.QuestionText
	dst.PointsPossible = src.PointsPossible
	dst.Answers = src.Answers
	dst.CorrectComments = src.CorrectComments
	dst.IncorrectComments = src.IncorrectComments
	dst.NeutralComments = src.NeutralComments
	dst.WorkflowState = src.WorkflowState
}

// quizChangedAt is the latest of the quiz row's and its questions'
// updated_at — editing a question is editing the quiz.
func (s *BlueprintService) quizChangedAt(ctx context.Context, q *models.Quiz) time.Time {
	latest := q.UpdatedAt
	if qs, err := s.qqRepo.ListByQuizID(ctx, q.ID, bigPage); err == nil {
		// Bank-draw copies are written when students take the quiz; they
		// are not authoring changes.
		for _, qq := range models.AuthoredQuizQuestions(qs.Items) {
			if qq.UpdatedAt.After(latest) {
				latest = qq.UpdatedAt
			}
		}
	}
	return latest
}

func (s *BlueprintService) syncQuizzes(ctx context.Context, sourceCourseID uint, sc *syncCtx) error {
	sourceQuizzes, err := s.quizRepo.ListByCourseID(ctx, sourceCourseID, bigPage)
	if err != nil {
		return err
	}
	childQuizzes, err := s.quizRepo.ListByCourseID(ctx, sc.child, bigPage)
	if err != nil {
		return err
	}
	byID := make(map[uint]*models.Quiz, len(childQuizzes.Items))
	byTitle := make(map[string]*models.Quiz, len(childQuizzes.Items))
	for i := range childQuizzes.Items {
		q := &childQuizzes.Items[i]
		byID[q.ID] = q
		if _, ok := byTitle[q.Title]; !ok {
			byTitle[q.Title] = q
		}
	}

	for i := range sourceQuizzes.Items {
		src := &sourceQuizzes.Items[i]
		changedAt := s.quizChangedAt(ctx, src)
		tag := s.findTag(ctx, sc, "Quiz", src.ID)
		pushed := false
		var child *models.Quiz
		if tag != nil {
			child = byID[tag.ChildID]
		} else if !sc.lastSync.IsZero() {
			child = byTitle[src.Title]
		}

		switch {
		case child == nil:
			if tag != nil && !sc.sourceChanged(changedAt) {
				continue
			}
			nq := &models.Quiz{CourseID: sc.child}
			applyQuizFields(nq, src)
			if err := s.quizRepo.Create(ctx, nq); err != nil {
				return fmt.Errorf("create quiz %q: %w", src.Title, err)
			}
			if err := s.syncQuizQuestions(ctx, src.ID, nq.ID); err != nil {
				return fmt.Errorf("sync questions for quiz %q: %w", src.Title, err)
			}
			child = nq
			pushed = true
		case sc.shouldPush(changedAt, tag, s.quizChangedAt(ctx, child)):
			applyQuizFields(child, src)
			if err := s.quizRepo.Update(ctx, child); err != nil {
				return fmt.Errorf("update quiz %q: %w", src.Title, err)
			}
			if err := s.syncQuizQuestions(ctx, src.ID, child.ID); err != nil {
				return fmt.Errorf("sync questions for quiz %q: %w", src.Title, err)
			}
			pushed = true
		}
		sc.idMap[contentKey("Quiz", src.ID)] = child.ID
		if err := s.noteSynced(ctx, sc, "Quiz", src.ID, child.ID, tag, pushed); err != nil {
			return err
		}
	}
	return nil
}

// syncQuizQuestions makes the child quiz's live questions mirror the
// source quiz's, matching by order: existing child questions are
// updated in place, missing ones created, surplus ones deleted. Only
// authored questions take part on either side: per-attempt bank-draw
// copies (QuestionBankEntryID) are neither copied nor matched/deleted.
func (s *BlueprintService) syncQuizQuestions(ctx context.Context, sourceQuizID, childQuizID uint) error {
	srcPage, err := s.qqRepo.ListByQuizID(ctx, sourceQuizID, bigPage)
	if err != nil {
		return err
	}
	childPage, err := s.qqRepo.ListByQuizID(ctx, childQuizID, bigPage)
	if err != nil {
		return err
	}
	srcItems := models.AuthoredQuizQuestions(srcPage.Items)
	childItems := models.AuthoredQuizQuestions(childPage.Items)
	for i := range srcItems {
		src := &srcItems[i]
		if i < len(childItems) {
			dst := childItems[i]
			applyQuestionFields(&dst, src)
			if err := s.qqRepo.Update(ctx, &dst); err != nil {
				return fmt.Errorf("update question (position %d): %w", src.Position, err)
			}
			continue
		}
		// Question groups, bank items and stimuli belong to the source
		// course; they are not copied.
		nq := &models.QuizQuestion{QuizID: childQuizID}
		applyQuestionFields(nq, src)
		if err := s.qqRepo.Create(ctx, nq); err != nil {
			return fmt.Errorf("create question (position %d): %w", src.Position, err)
		}
	}
	for i := len(srcItems); i < len(childItems); i++ {
		if err := s.qqRepo.Delete(ctx, childItems[i].ID); err != nil {
			return err
		}
	}
	return nil
}

// ---------- Discussion topics ----------

func applyDiscussionFields(dst, src *models.DiscussionTopic) {
	dst.Title = src.Title
	dst.Message = src.Message
	dst.DiscussionType = src.DiscussionType
	dst.PostedAt = src.PostedAt
	dst.DelayedPostAt = src.DelayedPostAt
	dst.LockAt = src.LockAt
	dst.Pinned = src.Pinned
	dst.Locked = src.Locked
	dst.AllowRating = src.AllowRating
	dst.OnlyGradersCanRate = src.OnlyGradersCanRate
	dst.SortByRating = src.SortByRating
	dst.RequireInitialPost = src.RequireInitialPost
	dst.WorkflowState = src.WorkflowState
}

func (s *BlueprintService) syncDiscussions(ctx context.Context, sourceCourseID uint, sc *syncCtx) error {
	sourceTopics, err := s.discRepo.ListByCourseID(ctx, sourceCourseID, 0, bigPage)
	if err != nil {
		return err
	}
	childTopics, err := s.discRepo.ListByCourseID(ctx, sc.child, 0, bigPage)
	if err != nil {
		return err
	}
	byID := make(map[uint]*models.DiscussionTopic, len(childTopics.Items))
	byTitle := make(map[string]*models.DiscussionTopic, len(childTopics.Items))
	for i := range childTopics.Items {
		t := &childTopics.Items[i]
		byID[t.ID] = t
		if _, ok := byTitle[t.Title]; !ok {
			byTitle[t.Title] = t
		}
	}

	for i := range sourceTopics.Items {
		src := &sourceTopics.Items[i]
		tag := s.findTag(ctx, sc, "DiscussionTopic", src.ID)
		pushed := false
		var child *models.DiscussionTopic
		if tag != nil {
			child = byID[tag.ChildID]
		} else if !sc.lastSync.IsZero() {
			child = byTitle[src.Title]
		}

		switch {
		case child == nil:
			if tag != nil && !sc.sourceChanged(src.UpdatedAt) {
				continue
			}
			nt := &models.DiscussionTopic{CourseID: sc.child, UserID: src.UserID}
			applyDiscussionFields(nt, src)
			if err := s.discRepo.Create(ctx, nt); err != nil {
				return fmt.Errorf("create discussion %q: %w", src.Title, err)
			}
			child = nt
			pushed = true
		case sc.shouldPush(src.UpdatedAt, tag, child.UpdatedAt):
			applyDiscussionFields(child, src)
			if err := s.discRepo.Update(ctx, child); err != nil {
				return fmt.Errorf("update discussion %q: %w", src.Title, err)
			}
			pushed = true
		}
		sc.idMap[contentKey("DiscussionTopic", src.ID)] = child.ID
		if err := s.noteSynced(ctx, sc, "DiscussionTopic", src.ID, child.ID, tag, pushed); err != nil {
			return err
		}
	}
	return nil
}

// ---------- Module items ----------

// applyItemFields copies a source module item onto its child copy,
// remapping the module and content ids. ok=false when the item links
// content that was not synced (the copy would dangle).
func (sc *syncCtx) applyItemFields(dst, src *models.ContentTag, childModuleID uint) (ok bool) {
	dst.ContextModuleID = childModuleID
	dst.ContentType = src.ContentType
	dst.Title = src.Title
	dst.Position = src.Position
	dst.URL = src.URL
	dst.Indent = src.Indent
	dst.NewTab = src.NewTab
	dst.WorkflowState = src.WorkflowState
	dst.CompletionType = src.CompletionType
	dst.MinScore = src.MinScore
	dst.ContentID = nil
	if src.ContentID != nil {
		newID, found := sc.idMap[contentKey(src.ContentType, *src.ContentID)]
		if !found {
			// ExternalUrl / SubHeader carry no content; anything else
			// (files, tools) is not part of blueprint sync.
			return false
		}
		dst.ContentID = &newID
	}
	return true
}

func (s *BlueprintService) syncModuleItems(ctx context.Context, sourceCourseID uint, sc *syncCtx) error {
	sourceModules, err := s.moduleRepo.ListByCourseID(ctx, sourceCourseID, bigPage)
	if err != nil {
		return err
	}

	for _, srcModule := range sourceModules.Items {
		childModuleID, ok := sc.idMap[contentKey("ContextModule", srcModule.ID)]
		if !ok {
			continue // module copy deleted downstream
		}

		sourceItems, err := s.itemRepo.ListByModuleID(ctx, srcModule.ID, bigPage)
		if err != nil {
			return fmt.Errorf("list items for module %d: %w", srcModule.ID, err)
		}
		childItems, err := s.itemRepo.ListByModuleID(ctx, childModuleID, bigPage)
		if err != nil {
			return fmt.Errorf("list child items for module %d: %w", childModuleID, err)
		}
		byTitle := make(map[string]*models.ContentTag, len(childItems.Items))
		for i := range childItems.Items {
			it := &childItems.Items[i]
			if _, ok := byTitle[it.Title]; !ok {
				byTitle[it.Title] = it
			}
		}

		for i := range sourceItems.Items {
			srcItem := &sourceItems.Items[i]
			tag := s.findTag(ctx, sc, "ContentTag", srcItem.ID)
			pushed := false
			var child *models.ContentTag
			if tag != nil {
				// The copy may have been moved to another child module.
				if it, err := s.itemRepo.FindByID(ctx, tag.ChildID); err == nil && it != nil && it.WorkflowState != models.ContentTagDeleted {
					child = it
				}
			} else if !sc.lastSync.IsZero() {
				child = byTitle[srcItem.Title]
			}

			switch {
			case child == nil:
				if tag != nil && !sc.sourceChanged(srcItem.UpdatedAt) {
					continue
				}
				ni := &models.ContentTag{}
				if !sc.applyItemFields(ni, srcItem, childModuleID) {
					continue
				}
				if err := s.itemRepo.Create(ctx, ni); err != nil {
					return fmt.Errorf("create item %q in module %d: %w", srcItem.Title, childModuleID, err)
				}
				child = ni
				pushed = true
			case sc.shouldPush(srcItem.UpdatedAt, tag, child.UpdatedAt):
				updated := *child
				if !sc.applyItemFields(&updated, srcItem, childModuleID) {
					continue
				}
				if err := s.itemRepo.Update(ctx, &updated); err != nil {
					return fmt.Errorf("update item %q in module %d: %w", srcItem.Title, childModuleID, err)
				}
				child = &updated
				pushed = true
			}
			if err := s.noteSynced(ctx, sc, "ContentTag", srcItem.ID, child.ID, tag, pushed); err != nil {
				return err
			}
		}
	}

	return nil
}
