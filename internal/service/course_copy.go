package service

// Course copy (G067): copy content from one course into another, the
// worker behind content_migrations of type "course_copy". It mirrors
// Canvas's "Copy a Canvas Course" import:
//
//   - everything, or a selection by content type and/or individual item
//     (Canvas's `select`); a selected module brings the content its items
//     link to, a selected assignment its assignment group and rubric, and
//     a graded discussion its assignment;
//   - copies keep their workflow state, settings and order; files are
//     copied in storage, folders are merged by path;
//   - links inside copied HTML that point at copied source content are
//     rewritten to the copies;
//   - dates are shifted or removed per the date-shift options.
//
// Every row is created through the regular repositories, so the
// repository-level defaults and slug de-duplication apply.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// Course copy content types — the keys of MigrationSettings.Select /
// IncludeOnlyTypes / ExcludeTypes and of the result counts.
const (
	CopyTypeCourseSettings   = "course_settings"
	CopyTypeSyllabus         = "syllabus_body"
	CopyTypeAssignmentGroups = "assignment_groups"
	CopyTypeAssignments      = "assignments"
	CopyTypeQuizzes          = "quizzes"
	CopyTypePages            = "pages"
	CopyTypeDiscussions      = "discussion_topics"
	CopyTypeModules          = "modules"
	CopyTypeFiles            = "files"
	CopyTypeEvents           = "calendar_events"
	CopyTypeRubrics          = "rubrics"
)

// AllCopyTypes lists every copyable content type in copy order.
var AllCopyTypes = []string{
	CopyTypeCourseSettings, CopyTypeSyllabus, CopyTypeFiles, CopyTypeAssignmentGroups,
	CopyTypeRubrics, CopyTypeAssignments, CopyTypeQuizzes, CopyTypePages,
	CopyTypeDiscussions, CopyTypeEvents, CopyTypeModules,
}

// copyTypeAliases folds Canvas's spellings onto Paper's type keys.
var copyTypeAliases = map[string]string{
	"all_course_settings": CopyTypeCourseSettings,
	"all_syllabus_body":   CopyTypeSyllabus,
	"wiki_pages":          CopyTypePages,
	"attachments":         CopyTypeFiles,
	"context_modules":     CopyTypeModules,
	"assignment_group":    CopyTypeAssignmentGroups,
	"discussions":         CopyTypeDiscussions,
	"events":              CopyTypeEvents,
	"calendar_event":      CopyTypeEvents,
}

// NormalizeCopyType maps a client-supplied type name onto a copy type
// key, or "" when it is not one Paper copies.
func NormalizeCopyType(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	t = strings.TrimPrefix(t, "all_")
	if alias, ok := copyTypeAliases[t]; ok {
		return alias
	}
	if alias, ok := copyTypeAliases["all_"+t]; ok {
		return alias
	}
	for _, known := range AllCopyTypes {
		if t == known {
			return known
		}
	}
	return ""
}

// ErrCourseCopyAcrossTenants is returned when the source and target
// courses belong to different accounts.
var ErrCourseCopyAcrossTenants = errors.New("source and target courses are in different accounts")

// CourseCopyDeps lists the repositories the copier writes through.
type CourseCopyDeps struct {
	Courses          repository.CourseRepository
	AssignmentGroups repository.AssignmentGroupRepository
	Assignments      repository.AssignmentRepository
	Quizzes          repository.QuizRepository
	Questions        repository.QuizQuestionRepository
	QuestionGroups   repository.QuizQuestionGroupRepository
	Pages            repository.PageRepository
	Discussions      repository.DiscussionTopicRepository
	Modules          repository.ModuleRepository
	ModuleItems      repository.ModuleItemRepository
	Events           repository.CalendarEventRepository
	Rubrics          repository.RubricRepository
	RubricAssocs     repository.RubricAssociationRepository
	Folders          repository.FolderRepository
	Attachments      repository.AttachmentRepository
	Files            *FileService
}

// CourseCopyService copies content between two courses of one account.
type CourseCopyService struct {
	d CourseCopyDeps
	// cssPolicy resolves pages.custom_css_enabled for the target course
	// (Page Designer). Nil: on.
	cssPolicy CourseCSSPolicy
}

func NewCourseCopyService(d CourseCopyDeps) *CourseCopyService {
	return &CourseCopyService{d: d}
}

// SetCourseCSSPolicy attaches the pages.custom_css_enabled resolution a
// copy applies to the pages it creates.
func (s *CourseCopyService) SetCourseCSSPolicy(p CourseCSSPolicy) *CourseCopyService {
	s.cssPolicy = p
	return s
}

// CourseCopyResult reports what a copy created.
type CourseCopyResult struct {
	Counts   map[string]int `json:"counts"`
	Warnings []string       `json:"warnings,omitempty"`
	Created  []EntityRef    `json:"-"`
	Shifted  int            `json:"dates_shifted"`
}

// copySelection is the resolved what-to-copy set.
type copySelection struct {
	all map[string]bool
	ids map[string]map[uint]bool
	// excluded types stay out even when a dependency would pull them in.
	excluded map[string]bool
}

func (s *copySelection) wants(t string, id uint) bool {
	return s.all[t] || s.ids[t][id]
}

func (s *copySelection) wantsType(t string) bool {
	return s.all[t] || len(s.ids[t]) > 0
}

func (s *copySelection) add(t string, id uint) {
	if s.all[t] || s.excluded[t] {
		return
	}
	if s.ids[t] == nil {
		s.ids[t] = map[uint]bool{}
	}
	s.ids[t][id] = true
}

// resolveCopySelection turns the migration settings into a selection.
// No include list and no item selection means "copy everything".
func resolveCopySelection(settings models.MigrationSettings) *copySelection {
	sel := &copySelection{all: map[string]bool{}, ids: map[string]map[uint]bool{}, excluded: map[string]bool{}}
	everything := settings.CopyEverything || (len(settings.IncludeOnlyTypes) == 0 && len(settings.Select) == 0)
	if everything {
		for _, t := range AllCopyTypes {
			sel.all[t] = true
		}
	} else {
		for _, raw := range settings.IncludeOnlyTypes {
			if t := NormalizeCopyType(raw); t != "" {
				sel.all[t] = true
			}
		}
		for raw, ids := range settings.Select {
			t := NormalizeCopyType(raw)
			if t == "" {
				continue
			}
			for _, id := range ids {
				if id != 0 {
					sel.add(t, id)
				}
			}
		}
	}
	for _, raw := range settings.ExcludeTypes {
		if t := NormalizeCopyType(raw); t != "" {
			delete(sel.all, t)
			delete(sel.ids, t)
			sel.excluded[t] = true
		}
	}
	return sel
}

// copier carries one copy run's state.
type copier struct {
	s        *CourseCopyService
	source   *models.Course
	target   *models.Course
	userID   uint
	sel      *copySelection
	result   *CourseCopyResult
	idMap    map[string]uint   // contentKey(type, sourceID) -> copy id
	slugMap  map[string]string // source page slug -> copy slug
	progress func(int)
}

func (c *copier) warn(format string, args ...interface{}) {
	if len(c.result.Warnings) < 200 {
		c.result.Warnings = append(c.result.Warnings, fmt.Sprintf(format, args...))
	}
}

func (c *copier) created(refType, countKey string, id uint) {
	c.result.Created = append(c.result.Created, EntityRef{Type: refType, ID: id})
	if countKey != "" {
		c.result.Counts[countKey]++
	}
}

var copyAll = repository.PaginationParams{Page: 1, PerPage: 10000}

// zeroTime resets CreatedAt/UpdatedAt on a cloned row so GORM stamps it.
var zeroTime time.Time

// Copy copies the selected content of sourceCourseID into targetCourseID.
// The caller has authorized the user on both courses; Copy still refuses
// a pair of courses in different accounts. progress (optional) receives
// 0-100.
//
// On a fatal error the rows created so far are removed again.
func (s *CourseCopyService) Copy(ctx context.Context, sourceCourseID, targetCourseID, userID uint, settings models.MigrationSettings, progress func(int)) (*CourseCopyResult, error) {
	if sourceCourseID == 0 || targetCourseID == 0 {
		return nil, errors.New("source and target course are required")
	}
	if sourceCourseID == targetCourseID {
		return nil, errors.New("a course cannot be copied into itself")
	}
	// accountID=0: auth-internal — the handler authorized the caller on
	// both courses; the account comparison below keeps the copy inside
	// one tenant.
	source, err := s.d.Courses.FindByID(ctx, sourceCourseID, 0)
	if err != nil || source == nil {
		return nil, fmt.Errorf("source course not found")
	}
	target, err := s.d.Courses.FindByID(ctx, targetCourseID, 0)
	if err != nil || target == nil {
		return nil, fmt.Errorf("target course not found")
	}
	if source.AccountID != target.AccountID {
		return nil, ErrCourseCopyAcrossTenants
	}
	if progress == nil {
		progress = func(int) {}
	}

	c := &copier{
		s: s, source: source, target: target, userID: userID,
		sel:      resolveCopySelection(settings),
		result:   &CourseCopyResult{Counts: map[string]int{}},
		idMap:    map[string]uint{},
		slugMap:  map[string]string{},
		progress: progress,
	}

	if err := c.run(ctx, settings); err != nil {
		s.Rollback(context.WithoutCancel(ctx), c.result.Created)
		return c.result, err
	}
	return c.result, nil
}

func (c *copier) run(ctx context.Context, settings models.MigrationSettings) error {
	if err := c.expandDependencies(ctx); err != nil {
		return err
	}
	steps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{CopyTypeCourseSettings, c.copyCourseSettings},
		{CopyTypeFiles, c.copyFiles},
		{CopyTypeAssignmentGroups, c.copyAssignmentGroups},
		{CopyTypeRubrics, c.copyRubrics},
		{CopyTypeAssignments, c.copyAssignments},
		{CopyTypeQuizzes, c.copyQuizzes},
		{CopyTypePages, c.copyPages},
		{CopyTypeDiscussions, c.copyDiscussions},
		{CopyTypeEvents, c.copyEvents},
		{CopyTypeModules, c.copyModules},
	}
	for i, step := range steps {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := step.fn(ctx); err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
		c.progress(5 + (i+1)*80/len(steps))
	}

	// Rewrite links once every copy exists, then shift dates.
	if err := c.rewriteLinks(ctx); err != nil {
		return fmt.Errorf("rewrite links: %w", err)
	}
	c.progress(90)

	shifter := NewDateShifter(settings.DateShiftOptions, c.source, c.target)
	n, err := ShiftEntityDates(ctx, ContentDateRepos{
		Assignments: c.s.d.Assignments, Quizzes: c.s.d.Quizzes, Discussions: c.s.d.Discussions,
		Modules: c.s.d.Modules, Events: c.s.d.Events,
	}, shifter, c.result.Created)
	if err != nil {
		return fmt.Errorf("shift dates: %w", err)
	}
	c.result.Shifted = n
	c.progress(100)
	return nil
}

// expandDependencies adds the content selected items depend on.
func (c *copier) expandDependencies(ctx context.Context) error {
	sel := c.sel
	// Selected modules bring the content their items link to.
	if sel.wantsType(CopyTypeModules) {
		mods, err := c.s.d.Modules.ListByCourseID(ctx, c.source.ID, copyAll)
		if err != nil {
			return fmt.Errorf("list modules: %w", err)
		}
		for _, m := range mods.Items {
			if !sel.wants(CopyTypeModules, m.ID) {
				continue
			}
			for _, it := range m.Items {
				if it.ContentID == nil || it.WorkflowState == models.ContentTagDeleted {
					continue
				}
				if t := copyTypeForModuleContent(it.ContentType); t != "" {
					sel.add(t, *it.ContentID)
				}
			}
		}
	}
	// Graded discussions bring their assignment.
	if sel.wantsType(CopyTypeDiscussions) {
		topics, err := c.s.d.Discussions.ListByCourseID(ctx, c.source.ID, 0, copyAll)
		if err != nil {
			return fmt.Errorf("list discussions: %w", err)
		}
		for _, t := range topics.Items {
			if sel.wants(CopyTypeDiscussions, t.ID) && t.AssignmentID != nil {
				sel.add(CopyTypeAssignments, *t.AssignmentID)
			}
		}
	}
	// Assignments bring their group and rubric.
	if sel.wantsType(CopyTypeAssignments) {
		as, err := c.s.d.Assignments.ListByCourseID(ctx, c.source.ID, copyAll)
		if err != nil {
			return fmt.Errorf("list assignments: %w", err)
		}
		for _, a := range as.Items {
			if !sel.wants(CopyTypeAssignments, a.ID) {
				continue
			}
			if a.AssignmentGroupID != nil {
				sel.add(CopyTypeAssignmentGroups, *a.AssignmentGroupID)
			}
			if c.s.d.RubricAssocs != nil {
				if ra, err := c.s.d.RubricAssocs.FindByAssociation(ctx, a.ID, "Assignment"); err == nil && ra != nil {
					sel.add(CopyTypeRubrics, ra.RubricID)
				}
			}
		}
	}
	return nil
}

// copyTypeForModuleContent maps a module item content type onto the
// copy type that owns its content.
func copyTypeForModuleContent(contentType string) string {
	switch normalizeModuleContentType(contentType) {
	case "Assignment":
		return CopyTypeAssignments
	case "Quiz":
		return CopyTypeQuizzes
	case "WikiPage":
		return CopyTypePages
	case "DiscussionTopic":
		return CopyTypeDiscussions
	case "Attachment":
		return CopyTypeFiles
	}
	return ""
}

// ------------------------------------------------------------ settings

func (c *copier) copyCourseSettings(ctx context.Context) error {
	changed := false
	if c.sel.all[CopyTypeCourseSettings] {
		c.target.DefaultView = c.source.DefaultView
		c.target.ApplyGroupWeights = c.source.ApplyGroupWeights
		c.target.License = c.source.License
		c.target.NavigationTabs = c.source.NavigationTabs
		c.target.UIMode = c.source.UIMode
		// Page Designer course look. A banner that is a source-course file
		// is re-pointed at the copied file in rewriteLinks.
		c.target.Look = c.source.Look
		changed = true
		c.result.Counts[CopyTypeCourseSettings] = 1
	}
	if c.sel.all[CopyTypeSyllabus] && strings.TrimSpace(c.source.SyllabusBody) != "" {
		c.target.SyllabusBody = c.source.SyllabusBody
		changed = true
		c.result.Counts[CopyTypeSyllabus] = 1
	}
	if !changed {
		return nil
	}
	return c.s.d.Courses.Update(ctx, c.target)
}

// --------------------------------------------------------------- files

func (c *copier) copyFiles(ctx context.Context) error {
	if !c.sel.wantsType(CopyTypeFiles) || c.s.d.Attachments == nil || c.s.d.Files == nil || c.s.d.Folders == nil {
		return nil
	}
	files, err := c.s.d.Attachments.ListByContext(ctx, "Course", c.source.ID, copyAll)
	if err != nil {
		return fmt.Errorf("list files: %w", err)
	}
	var wanted []models.Attachment
	for _, f := range files.Items {
		if c.sel.wants(CopyTypeFiles, f.ID) {
			wanted = append(wanted, f)
		}
	}
	if len(wanted) == 0 {
		return nil
	}

	folderMap, err := c.copyFolders(ctx)
	if err != nil {
		return err
	}
	backend := c.s.d.Files.StorageBackend()
	for _, src := range wanted {
		if backend == nil {
			c.warn("file %q: no storage backend", src.DisplayName)
			continue
		}
		rc, err := backend.Get(ctx, src.StoragePath)
		if err != nil {
			c.warn("file %q could not be read: %v", src.DisplayName, err)
			continue
		}
		att := &models.Attachment{
			ContextType: "Course",
			ContextID:   c.target.ID,
			UserID:      c.userID,
			DisplayName: src.DisplayName,
			Filename:    src.Filename,
			ContentType: src.ContentType,
			Size:        src.Size,
		}
		if src.FolderID != nil {
			if fid, ok := folderMap[*src.FolderID]; ok {
				att.FolderID = &fid
			}
		}
		if att.UserID == 0 {
			att.UserID = src.UserID
		}
		err = c.s.d.Files.UploadFileTrusted(ctx, att, rc)
		_ = rc.Close()
		if err != nil {
			c.warn("file %q could not be copied: %v", src.DisplayName, err)
			continue
		}
		c.idMap[contentKey("Attachment", src.ID)] = att.ID
		c.created("Attachment", CopyTypeFiles, att.ID)
	}
	return nil
}

// copyFolders mirrors the source folder tree into the target course,
// reusing target folders with the same path. It returns source folder id
// → target folder id.
func (c *copier) copyFolders(ctx context.Context) (map[uint]uint, error) {
	out := map[uint]uint{}
	srcFolders, err := c.s.d.Folders.ListByContext(ctx, "Course", c.source.ID, nil, copyAll)
	if err != nil {
		return nil, fmt.Errorf("list folders: %w", err)
	}
	targetRoot, err := c.s.d.Files.GetOrCreateRootFolder(ctx, "Course", c.target.ID)
	if err != nil {
		return nil, fmt.Errorf("target root folder: %w", err)
	}
	existing, err := c.s.d.Folders.ListByContext(ctx, "Course", c.target.ID, nil, copyAll)
	if err != nil {
		return nil, fmt.Errorf("list target folders: %w", err)
	}
	byPath := map[string]uint{}
	for _, f := range existing.Items {
		byPath[f.FullName] = f.ID
	}

	folders := append([]models.Folder(nil), srcFolders.Items...)
	// Parents before children: a shorter path is never a descendant.
	sort.SliceStable(folders, func(i, j int) bool {
		return strings.Count(folders[i].FullName, "/") < strings.Count(folders[j].FullName, "/")
	})
	for _, f := range folders {
		if f.ParentFolderID == nil {
			out[f.ID] = targetRoot.ID
			continue
		}
		parent, ok := out[*f.ParentFolderID]
		if !ok {
			parent = targetRoot.ID
		}
		if id, ok := byPath[f.FullName]; ok {
			out[f.ID] = id
			continue
		}
		nf := &models.Folder{
			ContextType:    "Course",
			ContextID:      c.target.ID,
			ParentFolderID: &parent,
			Name:           f.Name,
			FullName:       f.FullName,
			Position:       f.Position,
			WorkflowState:  f.WorkflowState,
		}
		if err := c.s.d.Folders.Create(ctx, nf); err != nil {
			c.warn("folder %q could not be copied: %v", f.FullName, err)
			out[f.ID] = targetRoot.ID
			continue
		}
		byPath[f.FullName] = nf.ID
		out[f.ID] = nf.ID
		c.created("Folder", "", nf.ID)
	}
	return out, nil
}

// --------------------------------------------------- assignment groups

func (c *copier) copyAssignmentGroups(ctx context.Context) error {
	if !c.sel.wantsType(CopyTypeAssignmentGroups) || c.s.d.AssignmentGroups == nil {
		return nil
	}
	groups, err := c.s.d.AssignmentGroups.ListByCourseID(ctx, c.source.ID, copyAll)
	if err != nil {
		return fmt.Errorf("list assignment groups: %w", err)
	}
	for _, src := range groups.Items {
		if !c.sel.wants(CopyTypeAssignmentGroups, src.ID) {
			continue
		}
		g := src
		g.ID = 0
		g.CourseID = c.target.ID
		g.CreatedAt, g.UpdatedAt = zeroTime, zeroTime
		if err := c.s.d.AssignmentGroups.Create(ctx, &g); err != nil {
			c.warn("assignment group %q could not be copied: %v", src.Name, err)
			continue
		}
		c.idMap[contentKey("AssignmentGroup", src.ID)] = g.ID
		c.created("AssignmentGroup", CopyTypeAssignmentGroups, g.ID)
	}
	return nil
}

// ------------------------------------------------------------- rubrics

func (c *copier) copyRubrics(ctx context.Context) error {
	if !c.sel.wantsType(CopyTypeRubrics) || c.s.d.Rubrics == nil {
		return nil
	}
	rubrics, err := c.s.d.Rubrics.ListByContext(ctx, "Course", c.source.ID, c.source.AccountID, copyAll)
	if err != nil {
		return fmt.Errorf("list rubrics: %w", err)
	}
	for _, src := range rubrics.Items {
		if !c.sel.wants(CopyTypeRubrics, src.ID) {
			continue
		}
		r := src
		r.ID = 0
		r.ContextType = "Course"
		r.ContextID = c.target.ID
		r.CreatedAt, r.UpdatedAt = zeroTime, zeroTime
		if err := c.s.d.Rubrics.Create(ctx, &r); err != nil {
			c.warn("rubric %q could not be copied: %v", src.Title, err)
			continue
		}
		c.idMap[contentKey("Rubric", src.ID)] = r.ID
		c.created("Rubric", CopyTypeRubrics, r.ID)
	}
	return nil
}

// --------------------------------------------------------- assignments

func (c *copier) copyAssignments(ctx context.Context) error {
	if !c.sel.wantsType(CopyTypeAssignments) {
		return nil
	}
	as, err := c.s.d.Assignments.ListByCourseID(ctx, c.source.ID, copyAll)
	if err != nil {
		return fmt.Errorf("list assignments: %w", err)
	}
	for _, src := range as.Items {
		if !c.sel.wants(CopyTypeAssignments, src.ID) {
			continue
		}
		a := src
		a.ID = 0
		a.CourseID = c.target.ID
		a.CreatedAt, a.UpdatedAt = zeroTime, zeroTime
		// Group sets are course-specific; a group assignment becomes an
		// individual one until the teacher picks a group set.
		a.GroupCategoryID = nil
		a.AssignmentGroupID = nil
		if src.AssignmentGroupID != nil {
			if gid, ok := c.idMap[contentKey("AssignmentGroup", *src.AssignmentGroupID)]; ok {
				a.AssignmentGroupID = &gid
			}
		}
		if err := c.s.d.Assignments.Create(ctx, &a); err != nil {
			c.warn("assignment %q could not be copied: %v", src.Name, err)
			continue
		}
		c.idMap[contentKey("Assignment", src.ID)] = a.ID
		c.created("Assignment", CopyTypeAssignments, a.ID)

		if c.s.d.RubricAssocs != nil {
			if ra, err := c.s.d.RubricAssocs.FindByAssociation(ctx, src.ID, "Assignment"); err == nil && ra != nil {
				if rid, ok := c.idMap[contentKey("Rubric", ra.RubricID)]; ok {
					na := *ra
					na.ID = 0
					na.RubricID = rid
					na.AssociationID = a.ID
					na.ContextType = "Course"
					na.ContextID = c.target.ID
					na.CreatedAt, na.UpdatedAt = zeroTime, zeroTime
					if err := c.s.d.RubricAssocs.Create(ctx, &na); err != nil {
						c.warn("rubric of assignment %q could not be attached: %v", src.Name, err)
					} else {
						c.created("RubricAssociation", "", na.ID)
					}
				}
			}
		}
	}
	return nil
}

// ------------------------------------------------------------- quizzes

func (c *copier) copyQuizzes(ctx context.Context) error {
	if !c.sel.wantsType(CopyTypeQuizzes) {
		return nil
	}
	quizzes, err := c.s.d.Quizzes.ListByCourseID(ctx, c.source.ID, copyAll)
	if err != nil {
		return fmt.Errorf("list quizzes: %w", err)
	}
	for _, src := range quizzes.Items {
		if !c.sel.wants(CopyTypeQuizzes, src.ID) {
			continue
		}
		q := src
		q.ID = 0
		q.CourseID = c.target.ID
		q.CreatedAt, q.UpdatedAt = zeroTime, zeroTime
		// The source quiz's gradebook assignment belongs to the source
		// course: link the copied assignment when it came along, else
		// leave it unlinked for the quiz lifecycle link job (which reuses
		// an orphaned online_quiz assignment of the same title).
		q.AssignmentID = nil
		if src.AssignmentID != nil {
			if aid, ok := c.idMap[contentKey("Assignment", *src.AssignmentID)]; ok {
				q.AssignmentID = &aid
			}
		}
		if err := createQuizPreservingDefaults(ctx, c.s.d.Quizzes, &q, src.ShowCorrectAnswers); err != nil {
			c.warn("quiz %q could not be copied: %v", src.Title, err)
			continue
		}
		c.idMap[contentKey("Quiz", src.ID)] = q.ID
		c.created("Quiz", CopyTypeQuizzes, q.ID)

		if err := c.copyQuizQuestions(ctx, src.ID, q.ID); err != nil {
			c.warn("questions of quiz %q could not be copied: %v", src.Title, err)
		}
	}
	return nil
}

// createQuizPreservingDefaults creates a quiz copy and re-applies
// show_correct_answers: GORM omits a false bool and the column defaults
// to true.
func createQuizPreservingDefaults(ctx context.Context, repo repository.QuizRepository, q *models.Quiz, showCorrect bool) error {
	if err := repo.Create(ctx, q); err != nil {
		return err
	}
	if q.ShowCorrectAnswers != showCorrect {
		q.ShowCorrectAnswers = showCorrect
		return repo.Update(ctx, q)
	}
	return nil
}

func (c *copier) copyQuizQuestions(ctx context.Context, srcQuizID, dstQuizID uint) error {
	groupMap := map[uint]uint{}
	if c.s.d.QuestionGroups != nil {
		groups, err := c.s.d.QuestionGroups.ListByQuizID(ctx, srcQuizID)
		if err != nil {
			return err
		}
		for _, g := range groups {
			ng := g
			ng.ID = 0
			ng.QuizID = dstQuizID
			// A bank-backed group draws from the source course's bank,
			// which is not copied; the copy keeps its own questions.
			ng.QuestionBankID = nil
			ng.CreatedAt, ng.UpdatedAt = zeroTime, zeroTime
			if err := c.s.d.QuestionGroups.Create(ctx, &ng); err != nil {
				return err
			}
			groupMap[g.ID] = ng.ID
			c.created("QuizQuestionGroup", "", ng.ID)
		}
	}
	qs, err := c.s.d.Questions.ListByQuizID(ctx, srcQuizID, copyAll)
	if err != nil {
		return err
	}
	for _, src := range models.AuthoredQuizQuestions(qs.Items) {
		nq := src
		nq.ID = 0
		nq.QuizID = dstQuizID
		nq.CreatedAt, nq.UpdatedAt = zeroTime, zeroTime
		nq.BankItemID = nil
		nq.StimulusID = nil
		nq.QuestionBankEntryID = nil
		nq.QuizQuestionGroupID = nil
		if src.QuizQuestionGroupID != nil {
			if gid, ok := groupMap[*src.QuizQuestionGroupID]; ok {
				nq.QuizQuestionGroupID = &gid
			}
		}
		if err := c.s.d.Questions.Create(ctx, &nq); err != nil {
			return err
		}
		c.created("QuizQuestion", "quiz_questions", nq.ID)
	}
	return nil
}

// --------------------------------------------------------------- pages

func (c *copier) copyPages(ctx context.Context) error {
	if !c.sel.wantsType(CopyTypePages) {
		return nil
	}
	pages, err := c.s.d.Pages.ListByCourseID(ctx, c.source.ID, copyAll)
	if err != nil {
		return fmt.Errorf("list pages: %w", err)
	}
	hasFront := false
	if fp, err := c.s.d.Pages.FindFrontPage(ctx, c.target.ID); err == nil && fp != nil {
		hasFront = true
	}
	cssAllowed := c.s.cssPolicy.Allowed(ctx, c.target.ID)
	for _, src := range pages.Items {
		if !c.sel.wants(CopyTypePages, src.ID) {
			continue
		}
		p := src
		p.ID = 0
		p.CourseID = c.target.ID
		p.CreatedAt, p.UpdatedAt = zeroTime, zeroTime
		// The target keeps its own front page.
		if p.FrontPage && hasFront {
			p.FrontPage = false
		}
		cleanPageDesign(&p, cssAllowed)
		if err := c.s.d.Pages.Create(ctx, &p); err != nil {
			c.warn("page %q could not be copied: %v", src.Title, err)
			continue
		}
		if p.FrontPage {
			hasFront = true
		}
		c.idMap[contentKey("WikiPage", src.ID)] = p.ID
		c.slugMap[src.URL] = p.URL
		c.created("WikiPage", CopyTypePages, p.ID)
	}
	return nil
}

// --------------------------------------------------------- discussions

func (c *copier) copyDiscussions(ctx context.Context) error {
	if !c.sel.wantsType(CopyTypeDiscussions) {
		return nil
	}
	topics, err := c.s.d.Discussions.ListByCourseID(ctx, c.source.ID, 0, copyAll)
	if err != nil {
		return fmt.Errorf("list discussions: %w", err)
	}
	// Oldest first so pinned/ordering by creation stays stable.
	items := append([]models.DiscussionTopic(nil), topics.Items...)
	sort.SliceStable(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	for _, src := range items {
		if !c.sel.wants(CopyTypeDiscussions, src.ID) {
			continue
		}
		t := src
		t.ID = 0
		t.CourseID = c.target.ID
		t.CreatedAt, t.UpdatedAt = zeroTime, zeroTime
		if c.userID != 0 {
			t.UserID = c.userID
		}
		t.AssignmentID = nil
		if src.AssignmentID != nil {
			if aid, ok := c.idMap[contentKey("Assignment", *src.AssignmentID)]; ok {
				t.AssignmentID = &aid
			}
		}
		if err := c.s.d.Discussions.Create(ctx, &t); err != nil {
			c.warn("discussion %q could not be copied: %v", src.Title, err)
			continue
		}
		c.idMap[contentKey("DiscussionTopic", src.ID)] = t.ID
		c.created("DiscussionTopic", CopyTypeDiscussions, t.ID)
	}
	return nil
}

// -------------------------------------------------------------- events

func (c *copier) copyEvents(ctx context.Context) error {
	if !c.sel.wantsType(CopyTypeEvents) || c.s.d.Events == nil {
		return nil
	}
	events, err := c.s.d.Events.ListByContext(ctx, "Course", c.source.ID, copyAll)
	if err != nil {
		return fmt.Errorf("list calendar events: %w", err)
	}
	for _, src := range events.Items {
		if !c.sel.wants(CopyTypeEvents, src.ID) {
			continue
		}
		e := src
		e.ID = 0
		e.ContextType = "Course"
		e.ContextID = c.target.ID
		e.CreatedAt, e.UpdatedAt = zeroTime, zeroTime
		if c.userID != 0 {
			e.CreatedByUserID = c.userID
		}
		if err := c.s.d.Events.Create(ctx, &e); err != nil {
			c.warn("event %q could not be copied: %v", src.Title, err)
			continue
		}
		c.idMap[contentKey("CalendarEvent", src.ID)] = e.ID
		c.created("CalendarEvent", CopyTypeEvents, e.ID)
	}
	return nil
}

// ------------------------------------------------------------- modules

func (c *copier) copyModules(ctx context.Context) error {
	if !c.sel.wantsType(CopyTypeModules) {
		return nil
	}
	mods, err := c.s.d.Modules.ListByCourseID(ctx, c.source.ID, copyAll)
	if err != nil {
		return fmt.Errorf("list modules: %w", err)
	}
	// Append after the target's existing modules.
	offset := 0
	if existing, err := c.s.d.Modules.ListByCourseID(ctx, c.target.ID, copyAll); err == nil {
		for _, m := range existing.Items {
			if m.Position > offset {
				offset = m.Position
			}
		}
	}
	for _, src := range mods.Items {
		if !c.sel.wants(CopyTypeModules, src.ID) {
			continue
		}
		items := src.Items
		m := src
		m.ID = 0
		m.CourseID = c.target.ID
		m.Items = nil
		m.Position = src.Position + offset
		m.CreatedAt, m.UpdatedAt = zeroTime, zeroTime
		if err := c.s.d.Modules.Create(ctx, &m); err != nil {
			c.warn("module %q could not be copied: %v", src.Name, err)
			continue
		}
		c.idMap[contentKey("ContextModule", src.ID)] = m.ID
		c.created("ContextModule", CopyTypeModules, m.ID)

		for _, it := range items {
			if it.WorkflowState == models.ContentTagDeleted {
				continue
			}
			ni := it
			ni.ID = 0
			ni.ContextModuleID = m.ID
			ni.CreatedAt, ni.UpdatedAt = zeroTime, zeroTime
			if it.ContentID != nil && copyTypeForModuleContent(it.ContentType) != "" {
				newID, ok := c.idMap[contentKey(it.ContentType, *it.ContentID)]
				if !ok {
					c.warn("module %q: item %q links content that was not copied", src.Name, it.Title)
					continue
				}
				ni.ContentID = &newID
				if normalizeModuleContentType(it.ContentType) == "WikiPage" {
					if slug, ok := c.slugMap[it.URL]; ok {
						ni.URL = slug
					}
				}
			}
			if err := c.s.d.ModuleItems.Create(ctx, &ni); err != nil {
				c.warn("module %q: item %q could not be copied: %v", src.Name, it.Title, err)
				continue
			}
			c.created("ContentTag", "module_items", ni.ID)
		}
	}
	return nil
}

// ---------------------------------------------------------- link rewrite

var (
	reCopyCourseLink = regexp.MustCompile(`/courses/(\d+)(?:/([A-Za-z_]+)(?:/([^"'<>\s?#/]+))?)?`)
	reCopyFileLink   = regexp.MustCompile(`/api/v1/files/(\d+)`)
)

var copyLinkTypes = map[string]string{
	"assignments":       "Assignment",
	"quizzes":           "Quiz",
	"discussion_topics": "DiscussionTopic",
	"announcements":     "DiscussionTopic",
	"modules":           "ContextModule",
	"files":             "Attachment",
}

// rewriteHTML rewrites links to source-course content that was copied
// so they point at the copies. Links to content that was not
// copied keep pointing at the source course.
func (c *copier) rewriteHTML(html string) string {
	if html == "" {
		return html
	}
	src := strconv.FormatUint(uint64(c.source.ID), 10)
	dst := strconv.FormatUint(uint64(c.target.ID), 10)
	out := reCopyCourseLink.ReplaceAllStringFunc(html, func(m string) string {
		g := reCopyCourseLink.FindStringSubmatch(m)
		if g[1] != src {
			return m
		}
		section, seg := g[2], g[3]
		switch {
		case section == "":
			return "/courses/" + dst
		case section == "pages" && seg != "":
			if slug, ok := c.slugMap[seg]; ok {
				return "/courses/" + dst + "/pages/" + slug
			}
			return m
		case seg != "":
			typ, ok := copyLinkTypes[section]
			if !ok {
				return m
			}
			id, err := strconv.ParseUint(seg, 10, 64)
			if err != nil {
				return m
			}
			if nid, ok := c.idMap[contentKey(typ, uint(id))]; ok {
				return fmt.Sprintf("/courses/%s/%s/%d", dst, section, nid)
			}
			return m
		default:
			// Course navigation link (/courses/N/assignments, …).
			return "/courses/" + dst + "/" + section
		}
	})
	return reCopyFileLink.ReplaceAllStringFunc(out, func(m string) string {
		g := reCopyFileLink.FindStringSubmatch(m)
		id, err := strconv.ParseUint(g[1], 10, 64)
		if err != nil {
			return m
		}
		if nid, ok := c.idMap[contentKey("Attachment", uint(id))]; ok {
			return fmt.Sprintf("/api/v1/files/%d", nid)
		}
		return m
	})
}

// rewriteJSON is rewriteHTML for a JSON document (a page design or a
// course look); a result that is no longer valid JSON is dropped.
func (c *copier) rewriteJSON(doc models.NullJSON) (models.NullJSON, bool) {
	if doc.IsNull() {
		return doc, false
	}
	out := c.rewriteHTML(string(doc))
	if out == string(doc) || !json.Valid([]byte(out)) {
		return doc, false
	}
	return models.NullJSON(out), true
}

// rewritePage rewrites links in a copied page's body, stylesheet and
// design, and reports whether anything changed.
func (c *copier) rewritePage(p *models.WikiPage) bool {
	changed := false
	if body := c.rewriteHTML(p.Body); body != p.Body {
		p.Body = body
		changed = true
	}
	if css := c.rewriteHTML(p.CSS); css != p.CSS {
		p.CSS = css
		changed = true
	}
	if design, ok := c.rewriteJSON(p.Design); ok {
		// Re-clean the rewritten design like any other stored one.
		p.Design = revalidateDesign(design)
		changed = true
	}
	return changed
}

func (c *copier) rewriteLinks(ctx context.Context) error {
	courseChanged := false
	if c.result.Counts[CopyTypeSyllabus] > 0 {
		if body := c.rewriteHTML(c.target.SyllabusBody); body != c.target.SyllabusBody {
			c.target.SyllabusBody = body
			courseChanged = true
		}
	}
	if c.result.Counts[CopyTypeCourseSettings] > 0 {
		if look, ok := c.rewriteJSON(c.target.Look); ok {
			c.target.Look = look
			courseChanged = true
		}
	}
	if courseChanged {
		if err := c.s.d.Courses.Update(ctx, c.target); err != nil {
			return err
		}
	}
	// accountID=0 below: auth-internal — every ref is a row this copy
	// just created in the target course.
	for _, ref := range c.result.Created {
		switch ref.Type {
		case "Assignment":
			if a, err := c.s.d.Assignments.FindByID(ctx, ref.ID, 0); err == nil && a != nil {
				if body := c.rewriteHTML(a.Description); body != a.Description {
					a.Description = body
					if err := c.s.d.Assignments.Update(ctx, a); err != nil {
						return err
					}
				}
			}
		case "Quiz":
			if q, err := c.s.d.Quizzes.FindByID(ctx, ref.ID, 0); err == nil && q != nil {
				if body := c.rewriteHTML(q.Description); body != q.Description {
					q.Description = body
					if err := c.s.d.Quizzes.Update(ctx, q); err != nil {
						return err
					}
				}
			}
		case "QuizQuestion":
			if q, err := c.s.d.Questions.FindByID(ctx, ref.ID); err == nil && q != nil {
				if body := c.rewriteHTML(q.QuestionText); body != q.QuestionText {
					q.QuestionText = body
					if err := c.s.d.Questions.Update(ctx, q); err != nil {
						return err
					}
				}
			}
		case "WikiPage":
			if p, err := c.s.d.Pages.FindByID(ctx, ref.ID, 0); err == nil && p != nil {
				if c.rewritePage(p) {
					if err := c.s.d.Pages.Update(ctx, p); err != nil {
						return err
					}
				}
			}
		case "DiscussionTopic":
			if t, err := c.s.d.Discussions.FindByID(ctx, ref.ID, 0); err == nil && t != nil {
				if body := c.rewriteHTML(t.Message); body != t.Message {
					t.Message = body
					if err := c.s.d.Discussions.Update(ctx, t); err != nil {
						return err
					}
				}
			}
		case "CalendarEvent":
			if c.s.d.Events == nil {
				continue
			}
			if e, err := c.s.d.Events.FindByID(ctx, ref.ID, 0); err == nil && e != nil {
				if body := c.rewriteHTML(e.Description); body != e.Description {
					e.Description = body
					if err := c.s.d.Events.Update(ctx, e); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// ------------------------------------------------------------- rollback

// Rollback removes rows a failed copy created, newest first. Content is
// soft-deleted through the repositories; best effort.
func (s *CourseCopyService) Rollback(ctx context.Context, refs []EntityRef) {
	for i := len(refs) - 1; i >= 0; i-- {
		ref := refs[i]
		func() {
			defer func() { _ = recover() }()
			switch ref.Type {
			case "ContentTag":
				_ = s.d.ModuleItems.Delete(ctx, ref.ID)
			case "ContextModule":
				_ = s.d.Modules.Delete(ctx, ref.ID)
			case "WikiPage":
				_ = s.d.Pages.Delete(ctx, ref.ID)
			case "Assignment":
				_ = s.d.Assignments.Delete(ctx, ref.ID)
			case "Quiz":
				_ = s.d.Quizzes.Delete(ctx, ref.ID)
			case "QuizQuestion":
				_ = s.d.Questions.Delete(ctx, ref.ID)
			case "QuizQuestionGroup":
				if s.d.QuestionGroups != nil {
					_ = s.d.QuestionGroups.Delete(ctx, ref.ID)
				}
			case "DiscussionTopic":
				_ = s.d.Discussions.Delete(ctx, ref.ID)
			case "CalendarEvent":
				if s.d.Events != nil {
					_ = s.d.Events.Delete(ctx, ref.ID)
				}
			case "AssignmentGroup":
				if s.d.AssignmentGroups != nil {
					_ = s.d.AssignmentGroups.Delete(ctx, ref.ID)
				}
			case "Rubric":
				if s.d.Rubrics != nil {
					// auth-internal: rollback of rows this copy created.
					_ = s.d.Rubrics.Delete(ctx, ref.ID, 0)
				}
			case "RubricAssociation":
				if s.d.RubricAssocs != nil {
					_ = s.d.RubricAssocs.Delete(ctx, ref.ID)
				}
			case "Attachment":
				if s.d.Files != nil {
					_ = s.d.Files.DeleteAttachment(ctx, ref.ID)
				}
			case "Folder":
				if s.d.Folders != nil {
					_ = s.d.Folders.Delete(ctx, ref.ID)
				}
			}
		}()
	}
}

// ------------------------------------------------------ selectable data

// CopySelectableItem is one item a teacher can pick in the selective
// copy dialog.
type CopySelectableItem struct {
	ID    uint   `json:"id"`
	Title string `json:"title"`
}

// CopySelectableType groups the selectable items of one content type.
type CopySelectableType struct {
	Type  string               `json:"type"`
	Count int                  `json:"count"`
	Items []CopySelectableItem `json:"items,omitempty"`
}

// SelectableContent lists the source course's copyable content for the
// selective copy UI (Canvas's selective_data). The caller has authorized
// the user on the source course.
func (s *CourseCopyService) SelectableContent(ctx context.Context, sourceCourseID uint) ([]CopySelectableType, error) {
	// accountID=0: auth-internal — the handler authorized the caller on
	// the source course.
	course, err := s.d.Courses.FindByID(ctx, sourceCourseID, 0)
	if err != nil || course == nil {
		return nil, fmt.Errorf("source course not found")
	}
	out := []CopySelectableType{
		{Type: CopyTypeCourseSettings, Count: 1},
	}
	if strings.TrimSpace(course.SyllabusBody) != "" {
		out = append(out, CopySelectableType{Type: CopyTypeSyllabus, Count: 1})
	}
	add := func(t string, items []CopySelectableItem) {
		out = append(out, CopySelectableType{Type: t, Count: len(items), Items: items})
	}

	if mods, err := s.d.Modules.ListByCourseID(ctx, sourceCourseID, copyAll); err == nil {
		items := make([]CopySelectableItem, 0, len(mods.Items))
		for _, m := range mods.Items {
			items = append(items, CopySelectableItem{ID: m.ID, Title: m.Name})
		}
		add(CopyTypeModules, items)
	}
	if as, err := s.d.Assignments.ListByCourseID(ctx, sourceCourseID, copyAll); err == nil {
		items := make([]CopySelectableItem, 0, len(as.Items))
		for _, a := range as.Items {
			items = append(items, CopySelectableItem{ID: a.ID, Title: a.Name})
		}
		add(CopyTypeAssignments, items)
	}
	if s.d.AssignmentGroups != nil {
		if gs, err := s.d.AssignmentGroups.ListByCourseID(ctx, sourceCourseID, copyAll); err == nil {
			items := make([]CopySelectableItem, 0, len(gs.Items))
			for _, g := range gs.Items {
				items = append(items, CopySelectableItem{ID: g.ID, Title: g.Name})
			}
			add(CopyTypeAssignmentGroups, items)
		}
	}
	if qs, err := s.d.Quizzes.ListByCourseID(ctx, sourceCourseID, copyAll); err == nil {
		items := make([]CopySelectableItem, 0, len(qs.Items))
		for _, q := range qs.Items {
			items = append(items, CopySelectableItem{ID: q.ID, Title: q.Title})
		}
		add(CopyTypeQuizzes, items)
	}
	if ps, err := s.d.Pages.ListByCourseID(ctx, sourceCourseID, copyAll); err == nil {
		items := make([]CopySelectableItem, 0, len(ps.Items))
		for _, p := range ps.Items {
			items = append(items, CopySelectableItem{ID: p.ID, Title: p.Title})
		}
		add(CopyTypePages, items)
	}
	if ts, err := s.d.Discussions.ListByCourseID(ctx, sourceCourseID, 0, copyAll); err == nil {
		items := make([]CopySelectableItem, 0, len(ts.Items))
		for _, t := range ts.Items {
			items = append(items, CopySelectableItem{ID: t.ID, Title: t.Title})
		}
		add(CopyTypeDiscussions, items)
	}
	if s.d.Attachments != nil {
		if fs, err := s.d.Attachments.ListByContext(ctx, "Course", sourceCourseID, copyAll); err == nil {
			items := make([]CopySelectableItem, 0, len(fs.Items))
			for _, f := range fs.Items {
				items = append(items, CopySelectableItem{ID: f.ID, Title: f.DisplayName})
			}
			add(CopyTypeFiles, items)
		}
	}
	if s.d.Events != nil {
		if es, err := s.d.Events.ListByContext(ctx, "Course", sourceCourseID, copyAll); err == nil {
			items := make([]CopySelectableItem, 0, len(es.Items))
			for _, e := range es.Items {
				items = append(items, CopySelectableItem{ID: e.ID, Title: e.Title})
			}
			add(CopyTypeEvents, items)
		}
	}
	if s.d.Rubrics != nil {
		if rs, err := s.d.Rubrics.ListByContext(ctx, "Course", sourceCourseID, course.AccountID, copyAll); err == nil {
			items := make([]CopySelectableItem, 0, len(rs.Items))
			for _, r := range rs.Items {
				items = append(items, CopySelectableItem{ID: r.ID, Title: r.Title})
			}
			add(CopyTypeRubrics, items)
		}
	}
	return out, nil
}
