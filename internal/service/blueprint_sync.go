package service

// Blueprint sync orchestration. It copies content (assignment groups,
// files, modules, assignments, pages, quizzes with their questions,
// discussion topics, module items and the syllabus) from a blueprint
// template course to each associated child course, and on later syncs
// pushes blueprint edits and deletions to the existing copies (B041,
// G069).
//
// Identity: each copy is recorded in blueprint_content_tags (source id →
// child id per child course), so a renamed blueprint item updates its
// copy instead of spawning a duplicate. Copies made before the identity
// map existed are adopted by title on the first sync that sees them.
//
// Change propagation follows Canvas's master-courses rules:
//   - a blueprint item changed since the last completed sync is pushed;
//   - if the child copy was edited locally since it was last synced (a
//     "downstream change"), the local edit wins — unless the item is
//     locked (its own lock row, or the template's default restrictions;
//     see blueprint_locks.go), in which case the blueprint version is
//     forced;
//   - a blueprint item deleted since its copy was made deletes the copy,
//     again unless the copy has unlocked local changes, which keep it
//     (it becomes the child course's own content).
//
// Links inside pushed HTML that point at blueprint content are rewritten
// to the child copies.
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

// BlueprintSyncResults counts what one sync did across all children.
type BlueprintSyncResults struct {
	Created  int      `json:"created"`
	Updated  int      `json:"updated"`
	Deleted  int      `json:"deleted"`
	Warnings []string `json:"warnings,omitempty"`
}

// TriggerSync creates a migration record and performs actual content sync from the
// blueprint template course to all associated child courses.
func (s *BlueprintService) TriggerSync(ctx context.Context, templateID, userID uint, comment string) (*models.BlueprintMigration, error) {
	return s.TriggerSyncWithOptions(ctx, templateID, userID, comment, BlueprintSyncOptions{})
}

// TriggerSyncWithOptions is TriggerSync with the per-sync choices.
func (s *BlueprintService) TriggerSyncWithOptions(ctx context.Context, templateID, userID uint, comment string, opts BlueprintSyncOptions) (*models.BlueprintMigration, error) {
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
	if opts.CopySettings {
		migration.ExportSettings = `{"copy_settings":true}`
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

	defaults := templateDefaultRestrictions(template)
	locks := s.loadLocks(ctx, templateID)
	totals := &BlueprintSyncResults{}

	// Perform content sync for each child course
	var syncErrors []string
	for _, sub := range subs.Items {
		if sub.WorkflowState != "active" {
			continue
		}
		sc := &syncCtx{
			templateID:   templateID,
			source:       sourceCourseID,
			child:        sub.ChildCourseID,
			lastSync:     lastSync,
			defaults:     defaults,
			locks:        locks,
			idMap:        make(map[string]uint),
			slugMap:      make(map[string]string),
			results:      totals,
			copySettings: opts.CopySettings,
		}
		if s.tagRepo != nil {
			sc.priorTags, _ = s.tagRepo.ListByChildCourse(ctx, templateID, sub.ChildCourseID)
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
	if b, err := json.Marshal(map[string]interface{}{"results": totals, "copy_settings": opts.CopySettings}); err == nil {
		migration.ExportSettings = string(b)
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
	templateID uint
	source     uint
	child      uint
	lastSync   time.Time // zero = first sync
	defaults   BlueprintRestrictions
	locks      map[string]BlueprintRestrictions
	// idMap maps contentKey(type, sourceID) → child id, for remapping
	// module item content ids and links.
	idMap map[string]uint
	// slugMap maps a blueprint page slug → its child copy's slug.
	slugMap map[string]string
	// priorTags is the identity map as it stood before this sync; the
	// deletion pass walks it for source items that are gone.
	priorTags []models.BlueprintContentTag
	// pushed lists the copies this sync created or overwrote, for the
	// link-rewrite pass.
	pushed  []pushedCopy
	results *BlueprintSyncResults
	// groupsSynced is true once assignment groups have been mapped, so
	// assignment copies take their group from idMap; liveGroups holds the
	// blueprint's current group ids for the group deletion pass.
	groupsSynced bool
	liveGroups   map[uint]bool
	copySettings bool
}

type pushedCopy struct {
	contentType string
	sourceID    uint
	childID     uint
}

// locked reports whether a blueprint item is locked (any restriction).
func (sc *syncCtx) locked(contentType string, sourceID uint) bool {
	if !blueprintLockableTypes[contentType] {
		return false
	}
	if r, ok := sc.locks[contentKey(contentType, sourceID)]; ok {
		return r.Any()
	}
	return sc.defaults.Any()
}

// sourceChanged reports whether a blueprint row changed since the last
// completed sync.
func (sc *syncCtx) sourceChanged(updatedAt time.Time) bool {
	return sc.lastSync.IsZero() || updatedAt.After(sc.lastSync)
}

// shouldPush decides whether a changed blueprint row overwrites its
// existing child copy: yes unless the copy was edited locally since it
// was last synced and the item is not locked.
func (sc *syncCtx) shouldPush(srcUpdated time.Time, tag *models.BlueprintContentTag, childUpdated time.Time, locked bool) bool {
	if !sc.sourceChanged(srcUpdated) {
		return false
	}
	if locked {
		return true
	}
	return !sc.downstreamChanged(tag, childUpdated)
}

// downstreamChanged reports whether a child copy was edited after it was
// last synced.
func (sc *syncCtx) downstreamChanged(tag *models.BlueprintContentTag, childUpdated time.Time) bool {
	var baseline time.Time
	switch {
	case tag != nil:
		baseline = tag.SyncedAt
	case !sc.lastSync.IsZero():
		baseline = sc.lastSync
	default:
		return false
	}
	return childUpdated.After(baseline.Add(downstreamSlack))
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
	if pushed {
		sc.pushed = append(sc.pushed, pushedCopy{contentType: contentType, sourceID: sourceID, childID: childID})
		if sc.results != nil {
			if tag == nil || tag.ChildID != childID {
				sc.results.Created++
			} else {
				sc.results.Updated++
			}
		}
	}
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
	if err := s.syncAssignmentGroups(ctx, sourceCourseID, sc); err != nil {
		return fmt.Errorf("sync assignment groups: %w", err)
	}
	if err := s.syncFiles(ctx, sourceCourseID, sc); err != nil {
		return fmt.Errorf("sync files: %w", err)
	}
	if err := s.syncRubrics(ctx, sourceCourseID, sc); err != nil {
		return fmt.Errorf("sync rubrics: %w", err)
	}
	if err := s.syncModules(ctx, sourceCourseID, sc); err != nil {
		return fmt.Errorf("sync modules: %w", err)
	}
	if err := s.syncAssignments(ctx, sourceCourseID, sc); err != nil {
		return fmt.Errorf("sync assignments: %w", err)
	}
	if err := s.syncAssignmentGroupDeletions(ctx, sc); err != nil {
		return fmt.Errorf("sync assignment group deletions: %w", err)
	}
	if err := s.syncRubricAssociations(ctx, sourceCourseID, sc); err != nil {
		return fmt.Errorf("sync rubric associations: %w", err)
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
	if err := s.syncSyllabus(ctx, sourceCourseID, sc); err != nil {
		return fmt.Errorf("sync syllabus: %w", err)
	}
	if err := s.syncSettings(ctx, sourceCourseID, sc); err != nil {
		return fmt.Errorf("sync settings: %w", err)
	}
	if err := s.rewritePushedLinks(ctx, sc); err != nil {
		return fmt.Errorf("rewrite links: %w", err)
	}
	return nil
}

// ---------- Deletions ----------

// syncDeletions removes child copies whose blueprint item no longer
// exists. live holds the source ids still present. childUpdated returns
// a copy's updated_at (ok=false when the copy is already gone);
// deleteChild removes it. Unlocked copies with local changes are kept
// (and detached from the blueprint).
func (s *BlueprintService) syncDeletions(
	ctx context.Context,
	sc *syncCtx,
	contentType string,
	live map[uint]bool,
	childUpdated func(childID uint) (time.Time, bool),
	deleteChild func(childID uint) error,
) error {
	if s.tagRepo == nil {
		return nil
	}
	for i := range sc.priorTags {
		tag := &sc.priorTags[i]
		if tag.ContentType != contentType || live[tag.SourceID] {
			continue
		}
		if updated, exists := childUpdated(tag.ChildID); exists {
			if sc.locked(contentType, tag.SourceID) || !sc.downstreamChanged(tag, updated) {
				if err := deleteChild(tag.ChildID); err != nil {
					return fmt.Errorf("delete %s %d: %w", contentType, tag.ChildID, err)
				}
				if sc.results != nil {
					sc.results.Deleted++
				}
			}
		}
		if err := s.tagRepo.Delete(ctx, tag.ID); err != nil {
			return err
		}
	}
	return nil
}

// ---------- Assignment groups ----------

func (s *BlueprintService) syncAssignmentGroups(ctx context.Context, sourceCourseID uint, sc *syncCtx) error {
	if s.groupRepo == nil {
		return nil
	}
	src, err := s.groupRepo.ListByCourseID(ctx, sourceCourseID, bigPage)
	if err != nil {
		return err
	}
	childGroups, err := s.groupRepo.ListByCourseID(ctx, sc.child, bigPage)
	if err != nil {
		return err
	}
	byID := make(map[uint]*models.AssignmentGroup, len(childGroups.Items))
	byName := make(map[string]*models.AssignmentGroup, len(childGroups.Items))
	for i := range childGroups.Items {
		g := &childGroups.Items[i]
		byID[g.ID] = g
		if _, ok := byName[g.Name]; !ok {
			byName[g.Name] = g
		}
	}
	live := map[uint]bool{}
	for i := range src.Items {
		g := &src.Items[i]
		live[g.ID] = true
		tag := s.findTag(ctx, sc, "AssignmentGroup", g.ID)
		pushed := false
		var child *models.AssignmentGroup
		if tag != nil {
			child = byID[tag.ChildID]
		} else {
			// Adopt a same-named group (every course starts with its own
			// "Assignments" group).
			child = byName[g.Name]
		}
		switch {
		case child == nil:
			if tag != nil && !sc.sourceChanged(g.UpdatedAt) {
				continue
			}
			ng := &models.AssignmentGroup{CourseID: sc.child, Name: g.Name, Position: g.Position, GroupWeight: g.GroupWeight, Rules: g.Rules, WorkflowState: g.WorkflowState}
			if err := s.groupRepo.Create(ctx, ng); err != nil {
				return fmt.Errorf("create assignment group %q: %w", g.Name, err)
			}
			child = ng
			pushed = true
		case sc.shouldPush(g.UpdatedAt, tag, child.UpdatedAt, false):
			child.Name, child.Position, child.GroupWeight, child.Rules = g.Name, g.Position, g.GroupWeight, g.Rules
			if err := s.groupRepo.Update(ctx, child); err != nil {
				return fmt.Errorf("update assignment group %q: %w", g.Name, err)
			}
			pushed = true
		}
		sc.idMap[contentKey("AssignmentGroup", g.ID)] = child.ID
		if err := s.noteSynced(ctx, sc, "AssignmentGroup", g.ID, child.ID, tag, pushed); err != nil {
			return err
		}
	}
	sc.groupsSynced = true
	sc.liveGroups = live
	return nil
}

// syncAssignmentGroupDeletions runs after the assignments are synced (so
// copies have moved to their new groups): a deleted blueprint group's
// copy goes unless child assignments still sit in it.
func (s *BlueprintService) syncAssignmentGroupDeletions(ctx context.Context, sc *syncCtx) error {
	if s.groupRepo == nil || !sc.groupsSynced {
		return nil
	}
	childGroups, err := s.groupRepo.ListByCourseID(ctx, sc.child, bigPage)
	if err != nil {
		return err
	}
	byID := make(map[uint]*models.AssignmentGroup, len(childGroups.Items))
	for i := range childGroups.Items {
		byID[childGroups.Items[i].ID] = &childGroups.Items[i]
	}
	inUse := map[uint]bool{}
	if as, err := s.assignRepo.ListByCourseID(ctx, sc.child, bigPage); err == nil {
		for _, a := range as.Items {
			if a.AssignmentGroupID != nil {
				inUse[*a.AssignmentGroupID] = true
			}
		}
	}
	return s.syncDeletions(ctx, sc, "AssignmentGroup", sc.liveGroups,
		func(id uint) (time.Time, bool) {
			if g, ok := byID[id]; ok && !inUse[id] {
				return g.UpdatedAt, true
			}
			// Gone already, or still holding assignments: keep it.
			return time.Time{}, false
		},
		func(id uint) error { return s.groupRepo.Delete(ctx, id) })
}

// ---------- Files ----------

func (s *BlueprintService) syncFiles(ctx context.Context, sourceCourseID uint, sc *syncCtx) error {
	if s.attachmentRepo == nil || s.files == nil || s.files.StorageBackend() == nil {
		return nil
	}
	src, err := s.attachmentRepo.ListByContext(ctx, "Course", sourceCourseID, bigPage)
	if err != nil {
		return err
	}
	childFiles, err := s.attachmentRepo.ListByContext(ctx, "Course", sc.child, bigPage)
	if err != nil {
		return err
	}
	byID := make(map[uint]*models.Attachment, len(childFiles.Items))
	for i := range childFiles.Items {
		byID[childFiles.Items[i].ID] = &childFiles.Items[i]
	}
	live := map[uint]bool{}
	var root *models.Folder
	for i := range src.Items {
		f := &src.Items[i]
		live[f.ID] = true
		tag := s.findTag(ctx, sc, "Attachment", f.ID)
		pushed := false
		var child *models.Attachment
		if tag != nil {
			child = byID[tag.ChildID]
		}
		switch {
		case child == nil:
			if tag != nil && !sc.sourceChanged(f.UpdatedAt) {
				continue
			}
			if root == nil {
				if root, err = s.files.GetOrCreateRootFolder(ctx, "Course", sc.child); err != nil {
					return err
				}
			}
			rc, err := s.files.StorageBackend().Get(ctx, f.StoragePath)
			if err != nil {
				if sc.results != nil {
					sc.results.Warnings = append(sc.results.Warnings, fmt.Sprintf("file %q could not be read: %v", f.DisplayName, err))
				}
				continue
			}
			att := &models.Attachment{
				ContextType: "Course", ContextID: sc.child, FolderID: &root.ID, UserID: f.UserID,
				DisplayName: f.DisplayName, Filename: f.Filename, ContentType: f.ContentType, Size: f.Size,
			}
			err = s.files.UploadFileTrusted(ctx, att, rc)
			_ = rc.Close()
			if err != nil {
				return fmt.Errorf("copy file %q: %w", f.DisplayName, err)
			}
			child = att
			pushed = true
		case sc.shouldPush(f.UpdatedAt, tag, child.UpdatedAt, false) && child.DisplayName != f.DisplayName:
			// A rename; file bytes are immutable once uploaded.
			child.DisplayName = f.DisplayName
			if err := s.attachmentRepo.Update(ctx, child); err != nil {
				return fmt.Errorf("rename file %q: %w", f.DisplayName, err)
			}
			pushed = true
		}
		sc.idMap[contentKey("Attachment", f.ID)] = child.ID
		if err := s.noteSynced(ctx, sc, "Attachment", f.ID, child.ID, tag, pushed); err != nil {
			return err
		}
	}
	return s.syncDeletions(ctx, sc, "Attachment", live,
		func(id uint) (time.Time, bool) {
			if a, ok := byID[id]; ok {
				return a.UpdatedAt, true
			}
			return time.Time{}, false
		},
		func(id uint) error { return s.attachmentRepo.Delete(ctx, id) })
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

	live := map[uint]bool{}
	for i := range sourceModules.Items {
		src := &sourceModules.Items[i]
		live[src.ID] = true
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
		case sc.shouldPush(src.UpdatedAt, tag, child.UpdatedAt, false):
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
	return s.syncDeletions(ctx, sc, "ContextModule", live,
		func(id uint) (time.Time, bool) {
			if m, ok := byID[id]; ok {
				return m.UpdatedAt, true
			}
			return time.Time{}, false
		},
		func(id uint) error { return s.moduleRepo.Delete(ctx, id) })
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

// applyAssignmentGroup points a copy at the child copy of its blueprint
// assignment group (when groups are synced).
func (sc *syncCtx) applyAssignmentGroup(dst, src *models.Assignment) {
	if !sc.groupsSynced {
		return
	}
	dst.AssignmentGroupID = nil
	if src.AssignmentGroupID != nil {
		if id, ok := sc.idMap[contentKey("AssignmentGroup", *src.AssignmentGroupID)]; ok {
			dst.AssignmentGroupID = &id
		}
	}
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

	live := map[uint]bool{}
	for i := range sourceAssignments.Items {
		src := &sourceAssignments.Items[i]
		live[src.ID] = true
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
			sc.applyAssignmentGroup(na, src)
			if err := s.assignRepo.Create(ctx, na); err != nil {
				return fmt.Errorf("create assignment %q: %w", src.Name, err)
			}
			child = na
			pushed = true
		case sc.shouldPush(src.UpdatedAt, tag, child.UpdatedAt, sc.locked("Assignment", src.ID)):
			applyAssignmentFields(child, src)
			sc.applyAssignmentGroup(child, src)
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
	return s.syncDeletions(ctx, sc, "Assignment", live,
		func(id uint) (time.Time, bool) {
			if a, ok := byID[id]; ok {
				return a.UpdatedAt, true
			}
			return time.Time{}, false
		},
		func(id uint) error { return s.assignRepo.Delete(ctx, id) })
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

	live := map[uint]bool{}
	for i := range sourcePages.Items {
		src := &sourcePages.Items[i]
		live[src.ID] = true
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
		case sc.shouldPush(src.UpdatedAt, tag, child.UpdatedAt, sc.locked("WikiPage", src.ID)):
			applyPageFields(child, src)
			if err := s.pageRepo.Update(ctx, child); err != nil {
				return fmt.Errorf("update page %q: %w", src.Title, err)
			}
			pushed = true
		}
		sc.idMap[contentKey("WikiPage", src.ID)] = child.ID
		sc.slugMap[src.URL] = child.URL
		if err := s.noteSynced(ctx, sc, "WikiPage", src.ID, child.ID, tag, pushed); err != nil {
			return err
		}
	}
	return s.syncDeletions(ctx, sc, "WikiPage", live,
		func(id uint) (time.Time, bool) {
			if p, ok := byID[id]; ok {
				return p.UpdatedAt, true
			}
			return time.Time{}, false
		},
		func(id uint) error { return s.pageRepo.Delete(ctx, id) })
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

	live := map[uint]bool{}
	for i := range sourceQuizzes.Items {
		src := &sourceQuizzes.Items[i]
		live[src.ID] = true
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
			if err := createQuizPreservingDefaults(ctx, s.quizRepo, nq, src.ShowCorrectAnswers); err != nil {
				return fmt.Errorf("create quiz %q: %w", src.Title, err)
			}
			if err := s.syncQuizQuestions(ctx, src.ID, nq.ID); err != nil {
				return fmt.Errorf("sync questions for quiz %q: %w", src.Title, err)
			}
			child = nq
			pushed = true
		case sc.shouldPush(changedAt, tag, s.quizChangedAt(ctx, child), sc.locked("Quiz", src.ID)):
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
	return s.syncDeletions(ctx, sc, "Quiz", live,
		func(id uint) (time.Time, bool) {
			if q, ok := byID[id]; ok {
				return s.quizChangedAt(ctx, q), true
			}
			return time.Time{}, false
		},
		func(id uint) error { return s.quizRepo.Delete(ctx, id) })
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

	live := map[uint]bool{}
	for i := range sourceTopics.Items {
		src := &sourceTopics.Items[i]
		live[src.ID] = true
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
			if src.AssignmentID != nil {
				if id, ok := sc.idMap[contentKey("Assignment", *src.AssignmentID)]; ok {
					nt.AssignmentID = &id
				}
			}
			if err := s.discRepo.Create(ctx, nt); err != nil {
				return fmt.Errorf("create discussion %q: %w", src.Title, err)
			}
			child = nt
			pushed = true
		case sc.shouldPush(src.UpdatedAt, tag, child.UpdatedAt, sc.locked("DiscussionTopic", src.ID)):
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
	return s.syncDeletions(ctx, sc, "DiscussionTopic", live,
		func(id uint) (time.Time, bool) {
			if t, ok := byID[id]; ok {
				return t.UpdatedAt, true
			}
			return time.Time{}, false
		},
		func(id uint) error { return s.discRepo.Delete(ctx, id) })
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
			// (LTI tools) is not part of blueprint sync.
			return false
		}
		dst.ContentID = &newID
		if normalizeModuleContentType(src.ContentType) == "WikiPage" {
			if slug, ok := sc.slugMap[src.URL]; ok {
				dst.URL = slug
			}
		}
	}
	return true
}

func (s *BlueprintService) syncModuleItems(ctx context.Context, sourceCourseID uint, sc *syncCtx) error {
	sourceModules, err := s.moduleRepo.ListByCourseID(ctx, sourceCourseID, bigPage)
	if err != nil {
		return err
	}

	live := map[uint]bool{}
	for _, srcModule := range sourceModules.Items {
		childModuleID, ok := sc.idMap[contentKey("ContextModule", srcModule.ID)]
		if !ok {
			// Module copy deleted downstream: its items stay out too.
			for _, it := range srcModule.Items {
				live[it.ID] = true
			}
			continue
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
			if srcItem.ContentID != nil {
				if _, synced := sc.idMap[contentKey(srcItem.ContentType, *srcItem.ContentID)]; !synced {
					// The item links content that no longer syncs (deleted
					// in the blueprint, or its copy deleted downstream):
					// not live, so the deletion pass removes the copy
					// instead of leaving it dangling.
					continue
				}
			}
			live[srcItem.ID] = true
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
			case sc.shouldPush(srcItem.UpdatedAt, tag, child.UpdatedAt, false):
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

	return s.syncDeletions(ctx, sc, "ContentTag", live,
		func(id uint) (time.Time, bool) {
			it, err := s.itemRepo.FindByID(ctx, id)
			if err != nil || it == nil || it.WorkflowState == models.ContentTagDeleted {
				return time.Time{}, false
			}
			return it.UpdatedAt, true
		},
		func(id uint) error { return s.itemRepo.Delete(ctx, id) })
}

// ---------- Syllabus ----------

// syncSyllabus pushes the blueprint's syllabus body when it changed since
// the last sync, unless the child course was edited since (and content
// is not locked by default).
func (s *BlueprintService) syncSyllabus(ctx context.Context, sourceCourseID uint, sc *syncCtx) error {
	if s.courseRepo == nil {
		return nil
	}
	// accountID=0: auth-internal — both courses are the template's own
	// course and one of its associated courses (same account, B037).
	src, err := s.courseRepo.FindByID(ctx, sourceCourseID, 0)
	if err != nil || src == nil {
		return nil
	}
	child, err := s.courseRepo.FindByID(ctx, sc.child, 0)
	if err != nil || child == nil {
		return nil
	}
	if child.SyllabusBody == src.SyllabusBody || !sc.sourceChanged(src.UpdatedAt) {
		return nil
	}
	tag := s.findTag(ctx, sc, "Syllabus", sourceCourseID)
	if !sc.defaults.Content && strings.TrimSpace(child.SyllabusBody) != "" && sc.downstreamChanged(tag, child.UpdatedAt) {
		return nil
	}
	child.SyllabusBody = src.SyllabusBody
	if err := s.courseRepo.Update(ctx, child); err != nil {
		return err
	}
	return s.noteSynced(ctx, sc, "Syllabus", sourceCourseID, sc.child, tag, true)
}

// ---------- Links ----------

// rewritePushedLinks rewrites links to blueprint content inside every
// copy this sync wrote, then re-stamps their tags so the rewrite does not
// read as a local edit next time.
func (s *BlueprintService) rewritePushedLinks(ctx context.Context, sc *syncCtx) error {
	if len(sc.pushed) == 0 {
		return nil
	}
	rw := &copier{
		source:  &models.Course{ID: sc.source},
		target:  &models.Course{ID: sc.child},
		idMap:   sc.idMap,
		slugMap: sc.slugMap,
	}
	for _, p := range sc.pushed {
		changed := false
		// accountID=0 below: auth-internal — rows this sync just wrote.
		switch p.contentType {
		case "Assignment":
			if a, err := s.assignRepo.FindByID(ctx, p.childID, 0); err == nil && a != nil {
				if body := rw.rewriteHTML(a.Description); body != a.Description {
					a.Description = body
					if err := s.assignRepo.Update(ctx, a); err != nil {
						return err
					}
					changed = true
				}
			}
		case "WikiPage":
			if pg, err := s.pageRepo.FindByID(ctx, p.childID, 0); err == nil && pg != nil {
				if body := rw.rewriteHTML(pg.Body); body != pg.Body {
					pg.Body = body
					if err := s.pageRepo.Update(ctx, pg); err != nil {
						return err
					}
					changed = true
				}
			}
		case "Quiz":
			if q, err := s.quizRepo.FindByID(ctx, p.childID, 0); err == nil && q != nil {
				if body := rw.rewriteHTML(q.Description); body != q.Description {
					q.Description = body
					if err := s.quizRepo.Update(ctx, q); err != nil {
						return err
					}
					changed = true
				}
			}
		case "DiscussionTopic":
			if t, err := s.discRepo.FindByID(ctx, p.childID, 0); err == nil && t != nil {
				if body := rw.rewriteHTML(t.Message); body != t.Message {
					t.Message = body
					if err := s.discRepo.Update(ctx, t); err != nil {
						return err
					}
					changed = true
				}
			}
		case "Syllabus":
			if s.courseRepo != nil {
				if c, err := s.courseRepo.FindByID(ctx, p.childID, 0); err == nil && c != nil {
					if body := rw.rewriteHTML(c.SyllabusBody); body != c.SyllabusBody {
						c.SyllabusBody = body
						if err := s.courseRepo.Update(ctx, c); err != nil {
							return err
						}
						changed = true
					}
				}
			}
		}
		if changed && s.tagRepo != nil {
			if err := s.tagRepo.Upsert(ctx, &models.BlueprintContentTag{
				BlueprintTemplateID: sc.templateID, ChildCourseID: sc.child,
				ContentType: p.contentType, SourceID: p.sourceID, ChildID: p.childID,
				SyncedAt: time.Now(),
			}); err != nil {
				return err
			}
		}
	}
	return nil
}
