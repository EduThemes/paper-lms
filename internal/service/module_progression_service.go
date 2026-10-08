package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
)

// ModuleProgressionStore is the persistence the progression engine needs
// (implemented by postgres.ModuleProgressionRepository).
type ModuleProgressionStore interface {
	ListPublishedModules(ctx context.Context, courseID uint) ([]models.ContextModule, error)
	ListPrerequisites(ctx context.Context, moduleIDs []uint) (map[uint][]uint, error)
	ListItemActions(ctx context.Context, userID uint, itemIDs []uint) (map[uint]models.ModuleItemUserAction, error)
	RecordViews(ctx context.Context, userID uint, itemIDs []uint, at time.Time) error
	SetMarkedDone(ctx context.Context, userID, itemID uint, done bool, at time.Time) error
	ListSubmissionFacts(ctx context.Context, userID uint, assignmentIDs []uint) (map[uint]postgres.SubmissionFact, error)
	ListQuizFacts(ctx context.Context, userID uint, quizIDs []uint) (map[uint]postgres.QuizFact, error)
	ListDiscussionFacts(ctx context.Context, userID uint, topicIDs []uint) (map[uint]postgres.DiscussionFact, error)
	ListProgressions(ctx context.Context, userID uint, moduleIDs []uint) (map[uint]models.ContextModuleProgression, error)
	UpsertProgressions(ctx context.Context, rows []models.ContextModuleProgression) error
	ListActiveStudents(ctx context.Context, courseID uint) ([]postgres.CourseStudent, error)
	ActiveEnrollmentTypes(ctx context.Context, userID, courseID uint) ([]string, error)
	ObservedStudentIDs(ctx context.Context, observerID, courseID uint) ([]uint, error)
	CourseInAccount(ctx context.Context, courseID, accountID uint) (bool, error)
	ListItemsForContent(ctx context.Context, courseID uint, contentTypes []string, contentID uint) ([]models.ContentTag, error)
	ResolvePageID(ctx context.Context, courseID uint, key string) (uint, bool, error)
	AttachmentCourseID(ctx context.Context, attachmentID uint) (uint, bool, error)
	UnpublishedContent(ctx context.Context, courseID uint) (map[string]map[uint]bool, error)
}

// AssignmentVisibility hides assignments a student may not see yet —
// Mastery Paths assignments that have not been released to them.
// Implemented by MasteryPathService.
type AssignmentVisibility interface {
	HiddenAssignmentIDs(ctx context.Context, courseID, studentID uint) (map[uint]bool, error)
}

// Errors returned by the progression write paths.
var (
	ErrModuleItemNotFound    = errors.New("module item not found")
	ErrModuleItemLocked      = errors.New("module item is locked")
	ErrModuleItemNotMarkable = errors.New("module item does not have a mark-as-done requirement")
)

// ModuleProgressionService evaluates and records students' progress
// through a course's modules (Canvas ContextModuleProgression) and
// answers "may this student open this content yet?" for the content
// gate.
type ModuleProgressionService struct {
	store      ModuleProgressionStore
	visibility AssignmentVisibility
	now        func() time.Time
}

func NewModuleProgressionService(store ModuleProgressionStore) *ModuleProgressionService {
	return &ModuleProgressionService{store: store, now: time.Now}
}

// SetAssignmentVisibility wires Mastery Paths visibility so released-
// only assignments are hidden from students' module lists, progress and
// content access until released.
func (s *ModuleProgressionService) SetAssignmentVisibility(v AssignmentVisibility) {
	s.visibility = v
}

// CourseProgress is one student's progress through a course's modules.
type CourseProgress struct {
	UserID  uint             `json:"user_id"`
	Modules []ModuleProgress `json:"modules"`
}

// ModuleNamer returns a lookup of module names for lock explanations.
func (p *CourseProgress) ModuleNamer() func(uint) string {
	names := make(map[uint]string, len(p.Modules))
	for _, m := range p.Modules {
		names[m.ModuleID] = m.Name
	}
	return func(id uint) string {
		if n, ok := names[id]; ok {
			return n
		}
		return "a prerequisite module"
	}
}

// Module returns the progress for moduleID, or nil.
func (p *CourseProgress) Module(moduleID uint) *ModuleProgress {
	for i := range p.Modules {
		if p.Modules[i].ModuleID == moduleID {
			return &p.Modules[i]
		}
	}
	return nil
}

// Item returns the module and item progress for itemID, or nils.
func (p *CourseProgress) Item(itemID uint) (*ModuleProgress, *ItemProgress) {
	for i := range p.Modules {
		for j := range p.Modules[i].Items {
			if p.Modules[i].Items[j].ItemID == itemID {
				return &p.Modules[i], &p.Modules[i].Items[j]
			}
		}
	}
	return nil, nil
}

// CourseInAccount reports whether the course belongs to the tenant.
func (s *ModuleProgressionService) CourseInAccount(ctx context.Context, courseID, accountID uint) (bool, error) {
	return s.store.CourseInAccount(ctx, courseID, accountID)
}

// IsLearner reports whether the user takes the course as a student: an
// active StudentEnrollment and no staff (teacher/TA/designer)
// enrollment. Only learners are subject to module locks and progress.
func (s *ModuleProgressionService) IsLearner(ctx context.Context, userID, courseID uint) (bool, error) {
	types, err := s.store.ActiveEnrollmentTypes(ctx, userID, courseID)
	if err != nil {
		return false, err
	}
	student := false
	for _, t := range types {
		switch t {
		case "StudentEnrollment":
			student = true
		case "TeacherEnrollment", "TaEnrollment", "DesignerEnrollment":
			return false, nil
		}
	}
	return student, nil
}

// ObservesStudent reports whether observerID observes studentID in the
// course.
func (s *ModuleProgressionService) ObservesStudent(ctx context.Context, observerID, studentID, courseID uint) (bool, error) {
	ids, err := s.store.ObservedStudentIDs(ctx, observerID, courseID)
	if err != nil {
		return false, err
	}
	for _, id := range ids {
		if id == studentID {
			return true, nil
		}
	}
	return false, nil
}

// hiddenAssignments returns the assignments hidden from the student.
func (s *ModuleProgressionService) hiddenAssignments(ctx context.Context, courseID, userID uint) (map[uint]bool, error) {
	if s.visibility == nil {
		return map[uint]bool{}, nil
	}
	return s.visibility.HiddenAssignmentIDs(ctx, courseID, userID)
}

// contentKind maps a module item's content type to the kind keys of
// ModuleProgressionStore.UnpublishedContent ("" for items with no
// publishable content: URLs, tools, sub headers).
func contentKind(contentType string) string {
	switch contentType {
	case "Assignment", "DiscussionTopic", "WikiPage", "Attachment":
		return contentType
	case "Quizzes::Quiz", "Quiz":
		return "Quiz"
	}
	return ""
}

// VisibleItemFilter returns a predicate that drops module items the
// student may not see: Mastery Paths assignments not released to them
// (G080) and items whose linked content is a draft or deleted (G063).
func (s *ModuleProgressionService) VisibleItemFilter(ctx context.Context, courseID, userID uint) (func(models.ContentTag) bool, error) {
	hidden, err := s.hiddenAssignments(ctx, courseID, userID)
	if err != nil {
		return nil, err
	}
	drafts, err := s.store.UnpublishedContent(ctx, courseID)
	if err != nil {
		return nil, err
	}
	return func(it models.ContentTag) bool {
		if it.ContentID == nil {
			return true
		}
		if it.ContentType == "Assignment" && hidden[*it.ContentID] {
			return false
		}
		if kind := contentKind(it.ContentType); kind != "" && drafts[kind][*it.ContentID] {
			return false
		}
		return true
	}, nil
}

// loadStructure returns the course's published modules with the items
// visible to the student, and the prerequisite map.
func (s *ModuleProgressionService) loadStructure(ctx context.Context, courseID, userID uint) ([]models.ContextModule, map[uint][]uint, error) {
	mods, err := s.store.ListPublishedModules(ctx, courseID)
	if err != nil {
		return nil, nil, err
	}
	keep, err := s.VisibleItemFilter(ctx, courseID, userID)
	if err != nil {
		return nil, nil, err
	}
	ids := make([]uint, len(mods))
	for i := range mods {
		ids[i] = mods[i].ID
		items := mods[i].Items[:0:0]
		for _, it := range mods[i].Items {
			if keep(it) {
				items = append(items, it)
			}
		}
		mods[i].Items = items
	}
	prereqs, err := s.store.ListPrerequisites(ctx, ids)
	if err != nil {
		return nil, nil, err
	}
	return mods, prereqs, nil
}

func isQuizContentType(ct string) bool { return ct == "Quizzes::Quiz" || ct == "Quiz" }

// applySubmission folds an assignment submission into item facts.
func applySubmission(f *ItemFacts, sub postgres.SubmissionFact, ok bool) {
	if !ok {
		return
	}
	if sub.Excused {
		f.Excused = true
	}
	switch models.SubmissionWorkflow(sub.WorkflowState) {
	case models.SubmissionSubmitted, models.SubmissionPendingReview:
		f.Submitted = true
	case models.SubmissionGraded:
		// A grade entered with nothing turned in (attempt 0, no
		// submitted_at) is a manual grade, not a submission (Canvas).
		if sub.Attempt > 0 || sub.SubmittedAt != nil {
			f.Submitted = true
		}
	}
	// Only a posted grade may satisfy min_score; an unposted one would
	// leak the hidden grade through the module state.
	if sub.Score != nil && sub.PostedAt != nil {
		v := *sub.Score
		if f.Score == nil || v > *f.Score {
			f.Score = &v
		}
	}
}

// buildFacts gathers the student's facts for every item of mods.
func (s *ModuleProgressionService) buildFacts(ctx context.Context, userID uint, mods []models.ContextModule) (map[uint]ItemFacts, error) {
	var itemIDs, assignmentIDs, quizIDs, topicIDs []uint
	for _, m := range mods {
		for _, it := range m.Items {
			itemIDs = append(itemIDs, it.ID)
			if it.ContentID == nil {
				continue
			}
			switch {
			case it.ContentType == "Assignment":
				assignmentIDs = append(assignmentIDs, *it.ContentID)
			case isQuizContentType(it.ContentType):
				quizIDs = append(quizIDs, *it.ContentID)
			case it.ContentType == "DiscussionTopic":
				topicIDs = append(topicIDs, *it.ContentID)
			}
		}
	}

	actions, err := s.store.ListItemActions(ctx, userID, itemIDs)
	if err != nil {
		return nil, err
	}
	topics, err := s.store.ListDiscussionFacts(ctx, userID, topicIDs)
	if err != nil {
		return nil, err
	}
	for _, t := range topics {
		if t.AssignmentID != nil {
			assignmentIDs = append(assignmentIDs, *t.AssignmentID)
		}
	}
	subs, err := s.store.ListSubmissionFacts(ctx, userID, assignmentIDs)
	if err != nil {
		return nil, err
	}
	quizzes, err := s.store.ListQuizFacts(ctx, userID, quizIDs)
	if err != nil {
		return nil, err
	}

	facts := make(map[uint]ItemFacts, len(itemIDs))
	for _, m := range mods {
		for _, it := range m.Items {
			var f ItemFacts
			if a, ok := actions[it.ID]; ok {
				f.Viewed = a.ViewedAt != nil
				f.Done = a.MarkedDoneAt != nil
			}
			if it.ContentID != nil {
				id := *it.ContentID
				switch {
				case it.ContentType == "Assignment":
					sub, ok := subs[id]
					applySubmission(&f, sub, ok)
				case isQuizContentType(it.ContentType):
					if q, ok := quizzes[id]; ok {
						f.Submitted = q.Finished
						f.Score = q.BestScore
						f.Excused = q.Excused
					}
				case it.ContentType == "DiscussionTopic":
					if t, ok := topics[id]; ok {
						f.Contributed = t.Contributed
						if t.AssignmentID != nil {
							sub, ok := subs[*t.AssignmentID]
							applySubmission(&f, sub, ok)
						}
						// An ungraded discussion is "submitted" by posting.
						if t.Contributed {
							f.Submitted = true
						}
					}
				}
			}
			facts[it.ID] = f
		}
	}
	return facts, nil
}

// Evaluate computes the student's progress through the course's modules
// and records it in context_module_progressions.
func (s *ModuleProgressionService) Evaluate(ctx context.Context, courseID, userID uint) (*CourseProgress, error) {
	progress, _, err := s.evaluate(ctx, courseID, userID)
	return progress, err
}

// evaluate is Evaluate that also returns the module structure (the
// student-visible items) the progress was computed from.
func (s *ModuleProgressionService) evaluate(ctx context.Context, courseID, userID uint) (*CourseProgress, []models.ContextModule, error) {
	mods, prereqs, err := s.loadStructure(ctx, courseID, userID)
	if err != nil {
		return nil, nil, err
	}
	facts, err := s.buildFacts(ctx, userID, mods)
	if err != nil {
		return nil, nil, err
	}
	now := s.now()
	progress := EvaluateModuleProgress(ModuleEvalInput{Now: now, Modules: mods, Prerequisites: prereqs, Facts: facts})

	if err := s.persist(ctx, userID, progress, now); err != nil {
		return nil, nil, err
	}
	return &CourseProgress{UserID: userID, Modules: progress}, mods, nil
}

// persist upserts the evaluated progressions, keeping the first
// completion time of a module that stays completed.
func (s *ModuleProgressionService) persist(ctx context.Context, userID uint, progress []ModuleProgress, now time.Time) error {
	if len(progress) == 0 {
		return nil
	}
	ids := make([]uint, len(progress))
	for i, p := range progress {
		ids[i] = p.ModuleID
	}
	existing, err := s.store.ListProgressions(ctx, userID, ids)
	if err != nil {
		return err
	}
	rows := make([]models.ContextModuleProgression, 0, len(progress))
	for _, p := range progress {
		met, _ := json.Marshal(p.RequirementsMet)
		row := models.ContextModuleProgression{
			ContextModuleID: p.ModuleID,
			UserID:          userID,
			WorkflowState:   p.State,
			RequirementsMet: string(met),
			CurrentPosition: p.CurrentPosition,
		}
		evaluated := now
		row.EvaluatedAt = &evaluated
		if p.State == models.ModuleProgressionCompleted {
			if prev, ok := existing[p.ModuleID]; ok && prev.CompletedAt != nil && prev.WorkflowState == models.ModuleProgressionCompleted {
				row.CompletedAt = prev.CompletedAt
			} else {
				done := now
				row.CompletedAt = &done
			}
		}
		rows = append(rows, row)
	}
	return s.store.UpsertProgressions(ctx, rows)
}

// CompletedAt returns the recorded completion times of the student's
// modules (after Evaluate has persisted them).
func (s *ModuleProgressionService) CompletedAt(ctx context.Context, userID uint, moduleIDs []uint) (map[uint]*time.Time, error) {
	rows, err := s.store.ListProgressions(ctx, userID, moduleIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[uint]*time.Time, len(rows))
	for id, r := range rows {
		out[id] = r.CompletedAt
	}
	return out, nil
}

// RecordItemView marks a module item viewed by the student (Canvas
// mark_read). The item must be visible and unlocked.
func (s *ModuleProgressionService) RecordItemView(ctx context.Context, courseID, userID, itemID uint) error {
	progress, err := s.Evaluate(ctx, courseID, userID)
	if err != nil {
		return err
	}
	_, ip := progress.Item(itemID)
	if ip == nil {
		return ErrModuleItemNotFound
	}
	if ip.Locked {
		return ErrModuleItemLocked
	}
	return s.store.RecordViews(ctx, userID, []uint{itemID}, s.now())
}

// SetItemDone marks (done=true) or unmarks a must_mark_done item.
func (s *ModuleProgressionService) SetItemDone(ctx context.Context, courseID, userID, itemID uint, done bool) error {
	progress, err := s.Evaluate(ctx, courseID, userID)
	if err != nil {
		return err
	}
	_, ip := progress.Item(itemID)
	if ip == nil {
		return ErrModuleItemNotFound
	}
	if ip.RequirementType != models.ModuleCompletionMustMarkDone {
		return ErrModuleItemNotMarkable
	}
	if ip.Locked {
		return ErrModuleItemLocked
	}
	now := s.now()
	if done {
		// Marking done implies the student has opened the item.
		if err := s.store.RecordViews(ctx, userID, []uint{itemID}, now); err != nil {
			return err
		}
	}
	return s.store.SetMarkedDone(ctx, userID, itemID, done, now)
}

// RosterEntry is one student's module states for the teacher view.
type RosterEntry struct {
	UserID       uint                                   `json:"user_id"`
	Name         string                                 `json:"name"`
	SortableName string                                 `json:"sortable_name"`
	States       map[uint]models.ModuleProgressionState `json:"states"`
	Completed    int                                    `json:"completed"`
}

// RosterModule names a module column of the roster.
type RosterModule struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Position int    `json:"position"`
}

// ProgressRoster is the teacher's "student progress" grid.
type ProgressRoster struct {
	Modules  []RosterModule `json:"modules"`
	Students []RosterEntry  `json:"students"`
}

// CourseRoster evaluates every active student of the course.
func (s *ModuleProgressionService) CourseRoster(ctx context.Context, courseID uint) (*ProgressRoster, error) {
	mods, err := s.store.ListPublishedModules(ctx, courseID)
	if err != nil {
		return nil, err
	}
	roster := &ProgressRoster{Modules: make([]RosterModule, 0, len(mods)), Students: []RosterEntry{}}
	for _, m := range mods {
		roster.Modules = append(roster.Modules, RosterModule{ID: m.ID, Name: m.Name, Position: m.Position})
	}
	students, err := s.store.ListActiveStudents(ctx, courseID)
	if err != nil {
		return nil, err
	}
	for _, st := range students {
		p, err := s.Evaluate(ctx, courseID, st.ID)
		if err != nil {
			return nil, err
		}
		entry := RosterEntry{UserID: st.ID, Name: st.Name, SortableName: st.SortableName, States: map[uint]models.ModuleProgressionState{}}
		for _, mp := range p.Modules {
			entry.States[mp.ModuleID] = mp.State
			if mp.State == models.ModuleProgressionCompleted {
				entry.Completed++
			}
		}
		roster.Students = append(roster.Students, entry)
	}
	return roster, nil
}

// Content kinds the gate checks.
const (
	GatedAssignment = "Assignment"
	GatedQuiz       = "Quiz"
	GatedDiscussion = "DiscussionTopic"
	GatedPage       = "WikiPage"
	GatedFile       = "Attachment"
)

func gatedContentTypes(kind string) []string {
	switch kind {
	case GatedQuiz:
		return []string{"Quizzes::Quiz", "Quiz"}
	default:
		return []string{kind}
	}
}

// ContentAccess is the gate's decision for one piece of content.
type ContentAccess struct {
	// Hidden: the student may not know the content exists (404).
	Hidden bool
	// Locked: every module item linking the content is locked.
	Locked      bool
	Explanation string
	ModuleID    uint
	UnlockAt    *time.Time
	// ItemIDs are the unlocked module items that link the content; a
	// successful view records must_view on them.
	ItemIDs []uint
}

// CheckContentAccess decides whether the student may open the content.
// Content that no published module item links is never module-locked
// (Canvas locks content only through its module items, and only when
// every item linking it is locked).
func (s *ModuleProgressionService) CheckContentAccess(ctx context.Context, courseID, userID uint, kind string, contentID uint) (*ContentAccess, error) {
	acc := &ContentAccess{}
	if kind == GatedAssignment {
		hidden, err := s.hiddenAssignments(ctx, courseID, userID)
		if err != nil {
			return nil, err
		}
		if hidden[contentID] {
			acc.Hidden = true
			return acc, nil
		}
	}
	items, err := s.store.ListItemsForContent(ctx, courseID, gatedContentTypes(kind), contentID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return acc, nil
	}
	progress, err := s.Evaluate(ctx, courseID, userID)
	if err != nil {
		return nil, err
	}
	var firstLocked *ModuleProgress
	found := false
	for _, it := range items {
		mp, ip := progress.Item(it.ID)
		if ip == nil {
			continue
		}
		found = true
		if !ip.Locked {
			acc.ItemIDs = append(acc.ItemIDs, it.ID)
		} else if firstLocked == nil {
			firstLocked = mp
		}
	}
	if !found || len(acc.ItemIDs) > 0 || firstLocked == nil {
		return acc, nil
	}
	acc.Locked = true
	acc.ModuleID = firstLocked.ModuleID
	acc.UnlockAt = firstLocked.UnlockAt
	acc.Explanation = contentLockExplanation(firstLocked, progress.ModuleNamer())
	return acc, nil
}

// contentLockExplanation explains why content in a locked item of mp is
// closed to the student.
func contentLockExplanation(mp *ModuleProgress, namer func(uint) string) string {
	if mp.State == models.ModuleProgressionLocked {
		return "This content is part of the module \"" + mp.Name + "\", which is locked. " + mp.LockExplanation(namer)
	}
	return "This content is part of the module \"" + mp.Name + "\". Complete the earlier items of the module first."
}

// LockedContent returns the course content the student may not open
// because of module locks, keyed by gated kind (GatedAssignment, …) and
// content id, with the lock explanation. As in CheckContentAccess,
// content is locked only when it is linked by at least one visible
// module item and every such item is locked. Index endpoints use it to
// withhold the bodies of locked content (Canvas locked_for_user).
func (s *ModuleProgressionService) LockedContent(ctx context.Context, courseID, userID uint) (map[string]map[uint]string, error) {
	progress, mods, err := s.evaluate(ctx, courseID, userID)
	if err != nil {
		return nil, err
	}
	namer := progress.ModuleNamer()
	type state struct {
		open        bool
		explanation string
	}
	seen := map[string]map[uint]*state{}
	for _, m := range mods {
		for _, it := range m.Items {
			if it.ContentID == nil {
				continue
			}
			kind := contentKind(it.ContentType)
			if kind == "" {
				continue
			}
			mp, ip := progress.Item(it.ID)
			if ip == nil {
				continue
			}
			if seen[kind] == nil {
				seen[kind] = map[uint]*state{}
			}
			st := seen[kind][*it.ContentID]
			if st == nil {
				st = &state{}
				seen[kind][*it.ContentID] = st
			}
			if !ip.Locked {
				st.open = true
			} else if st.explanation == "" {
				st.explanation = contentLockExplanation(mp, namer)
			}
		}
	}
	out := map[string]map[uint]string{}
	for kind, byID := range seen {
		for id, st := range byID {
			if st.open {
				continue
			}
			if out[kind] == nil {
				out[kind] = map[uint]string{}
			}
			out[kind][id] = st.explanation
		}
	}
	return out, nil
}

// RecordContentView records must_view on the given module items after
// the student opened their content.
func (s *ModuleProgressionService) RecordContentView(ctx context.Context, userID uint, itemIDs []uint) error {
	return s.store.RecordViews(ctx, userID, itemIDs, s.now())
}

// ResolvePageID maps a page URL key (slug or id) to a page id.
func (s *ModuleProgressionService) ResolvePageID(ctx context.Context, courseID uint, key string) (uint, bool, error) {
	return s.store.ResolvePageID(ctx, courseID, key)
}

// AttachmentCourseID returns the course an attachment belongs to.
func (s *ModuleProgressionService) AttachmentCourseID(ctx context.Context, attachmentID uint) (uint, bool, error) {
	return s.store.AttachmentCourseID(ctx, attachmentID)
}
