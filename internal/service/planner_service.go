package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// PlannerItem is a unified representation of any plannable object (assignment,
// quiz, discussion, calendar event, planner note, announcement) that appears
// on a student's planner / to-do list. The shape follows Canvas's
// /api/v1/planner/items.
type PlannerItem struct {
	PlannableType string      `json:"plannable_type"` // "assignment", "quiz", "discussion_topic", "calendar_event", "planner_note", "announcement"
	PlannableID   uint        `json:"plannable_id"`
	PlannableDate *time.Time  `json:"plannable_date"`
	Plannable     interface{} `json:"plannable"`
	CourseID      *uint       `json:"course_id,omitempty"`
	ContextType   string      `json:"context_type"`
	ContextName   string      `json:"context_name,omitempty"`
	HTMLURL       string      `json:"html_url,omitempty"`
	NewActivity   bool        `json:"new_activity"`
	Submissions   interface{} `json:"submissions"` // *PlannerSubmissionStatus for graded work, false otherwise

	// PlannerOverride is the user's explicit done / dismissed choice.
	PlannerOverride *PlannerOverrideInfo `json:"planner_override,omitempty"`
	// Completed is the effective "done" state: the override when there is
	// one, otherwise whether the work was turned in (Canvas marks submitted
	// work complete by default).
	Completed bool `json:"completed"`
}

// PlannerSubmissionStatus is the student's state on a graded plannable.
type PlannerSubmissionStatus struct {
	Submitted   bool `json:"submitted"`
	Excused     bool `json:"excused"`
	Graded      bool `json:"graded"`
	Late        bool `json:"late"`
	Missing     bool `json:"missing"`
	HasFeedback bool `json:"has_feedback"`
}

// PlannerOverrideInfo is a lightweight copy of the override for embedding in
// planner item responses.
type PlannerOverrideInfo struct {
	ID             uint `json:"id"`
	MarkedComplete bool `json:"marked_complete"`
	Dismissed      bool `json:"dismissed"`
}

// PlannerQuery narrows a planner fetch.
type PlannerQuery struct {
	Start, End time.Time
	// IncompleteOnly drops completed and dismissed items (Canvas
	// filter=incomplete_items).
	IncompleteOnly bool
}

// PlannerAnnouncementVisibility decides whether a user may see an
// announcement (audience / section targeting). AnnouncementService
// satisfies it.
type PlannerAnnouncementVisibility interface {
	CanView(ctx context.Context, a *models.Announcement, userID uint) bool
}

// PlannableTypes are the plannable_type values an override may target.
var PlannableTypes = map[string]bool{
	"assignment": true, "quiz": true, "discussion_topic": true, "calendar_event": true,
	"planner_note": true, "announcement": true, "wiki_page": true,
}

// ErrPlannerNotFound is returned when a note / override / course is not the
// caller's (handlers answer 404 so ids are not probeable).
var ErrPlannerNotFound = errors.New("not found")

type PlannerService struct {
	noteRepo       repository.PlannerNoteRepository
	overrideRepo   repository.PlannerOverrideRepository
	enrollmentRepo repository.EnrollmentRepository
	assignmentRepo repository.AssignmentRepository
	quizRepo       repository.QuizRepository
	calendarRepo   repository.CalendarEventRepository

	learner       repository.LearnerRepository
	announcements PlannerAnnouncementVisibility
	dueDates      StudentDueDateResolver
}

func NewPlannerService(
	noteRepo repository.PlannerNoteRepository,
	overrideRepo repository.PlannerOverrideRepository,
	enrollmentRepo repository.EnrollmentRepository,
	assignmentRepo repository.AssignmentRepository,
	quizRepo repository.QuizRepository,
	calendarRepo repository.CalendarEventRepository,
) *PlannerService {
	return &PlannerService{
		noteRepo:       noteRepo,
		overrideRepo:   overrideRepo,
		enrollmentRepo: enrollmentRepo,
		assignmentRepo: assignmentRepo,
		quizRepo:       quizRepo,
		calendarRepo:   calendarRepo,
	}
}

// SetLearnerRepository wires the set-based read model (required).
func (s *PlannerService) SetLearnerRepository(r repository.LearnerRepository) *PlannerService {
	s.learner = r
	return s
}

// SetAnnouncementVisibility lets the planner list announcements the user
// may see (optional; without it announcements are left out).
func (s *PlannerService) SetAnnouncementVisibility(v PlannerAnnouncementVisibility) *PlannerService {
	s.announcements = v
	return s
}

// SetDueDateResolver wires the gradebook's effective-date resolver
// (overrides, accommodations, assign-to visibility). Without it the
// planner falls back to override dates only.
func (s *PlannerService) SetDueDateResolver(r StudentDueDateResolver) *PlannerService {
	s.dueDates = r
	return s
}

// GetPlannerItems is the tenant-agnostic entry point kept for the Canvas
// /users/self/todo alias. accountID=0: the user id is the authenticated
// caller, whose enrollments already define the course set.
func (s *PlannerService) GetPlannerItems(ctx context.Context, userID uint, startDate, endDate time.Time) ([]PlannerItem, error) {
	return s.Items(ctx, userID, 0, PlannerQuery{Start: startDate, End: endDate})
}

// studentCourses returns the caller's active, non-observer enrollments in
// their tenant and the course rows behind them.
func studentCourses(ctx context.Context, enrollments repository.EnrollmentRepository, learner repository.LearnerRepository, userID, accountID uint, includeStaff bool) ([]models.Enrollment, map[uint]models.Course, error) {
	all, err := enrollments.ListByUserID(ctx, userID, accountID)
	if err != nil {
		return nil, nil, err
	}
	var kept []models.Enrollment
	ids := []uint{}
	seen := map[uint]bool{}
	staffIn := map[uint]bool{}
	for _, e := range all {
		if e.WorkflowState != models.EnrollmentActive || e.Type == "ObserverEnrollment" {
			continue
		}
		if !includeStaff && e.Type != "StudentEnrollment" {
			continue
		}
		if e.Type != "StudentEnrollment" {
			staffIn[e.CourseID] = true
		}
		kept = append(kept, e)
		if !seen[e.CourseID] {
			seen[e.CourseID] = true
			ids = append(ids, e.CourseID)
		}
	}
	courses, err := learner.CoursesByIDs(ctx, ids, accountID)
	if err != nil {
		return nil, nil, err
	}
	byID := make(map[uint]models.Course, len(courses))
	for _, c := range courses {
		// Students only see a published course; an unpublished course's work
		// is not theirs to plan yet. Its staff still plan in it.
		if c.WorkflowState == models.CourseAvailable || (staffIn[c.ID] && c.WorkflowState != models.CourseDeleted) {
			byID[c.ID] = c
		}
	}
	// Drop enrollments whose course was filtered out, so callers that
	// derive course lists from the enrollments see the same set.
	filtered := kept[:0]
	for _, e := range kept {
		if _, ok := byID[e.CourseID]; ok {
			filtered = append(filtered, e)
		}
	}
	return filtered, byID, nil
}

// effectiveDueAt resolves the due date the student actually has: the most
// lenient (latest) of the overrides that apply to them, or the assignment's
// own due date when none does. An applicable override without a due date
// means "no due date".
func effectiveDueAt(base *time.Time, overrides []*time.Time) *time.Time {
	if len(overrides) == 0 {
		return base
	}
	var latest *time.Time
	for _, d := range overrides {
		if d == nil {
			return nil
		}
		if latest == nil || d.After(*latest) {
			latest = d
		}
	}
	return latest
}

// isSubmitted reports whether the student turned the work in.
func isSubmitted(sub *models.Submission) bool {
	if sub == nil {
		return false
	}
	return sub.SubmittedAt != nil || sub.WorkflowState == models.SubmissionSubmitted ||
		sub.WorkflowState == models.SubmissionPendingReview
}

// isMissing is the student's missing status against their effective due
// date, the same rule the gradebook stores (ComputeTardiness): a manual
// late_policy_status wins, excused work is never missing, and otherwise
// work is missing when the due date has passed, the assignment expects an
// online submission, nothing was turned in and no grader has graded it.
func isMissing(a *models.Assignment, due *time.Time, sub *models.Submission, now time.Time) bool {
	if sub != nil && sub.LatePolicyStatus != nil {
		return ComputeTardiness(a, sub, due, now).Missing
	}
	if a.GradingType == "not_graded" || isSubmitted(sub) {
		return false
	}
	if sub == nil {
		return due != nil && now.After(*due) && ExpectsSubmission(a)
	}
	return ComputeTardiness(a, sub, due, now).Missing
}

func submissionStatus(a *models.Assignment, due *time.Time, sub *models.Submission, now time.Time) *PlannerSubmissionStatus {
	st := &PlannerSubmissionStatus{Missing: isMissing(a, due, sub, now)}
	if sub == nil {
		return st
	}
	posted := sub.PostedAt != nil
	st.Submitted = isSubmitted(sub)
	st.Excused = posted && sub.Excused
	st.Graded = posted && (sub.Score != nil || sub.Excused)
	st.Late = sub.Late
	return st
}

// Items aggregates the user's plannables in [Start, End].
func (s *PlannerService) Items(ctx context.Context, userID, accountID uint, q PlannerQuery) ([]PlannerItem, error) {
	if s.learner == nil {
		return nil, errors.New("planner read model not configured")
	}
	now := time.Now()
	enrollments, courses, err := studentCourses(ctx, s.enrollmentRepo, s.learner, userID, accountID, true)
	if err != nil {
		return nil, err
	}
	courseIDs := make([]uint, 0, len(courses))
	for id := range courses {
		courseIDs = append(courseIDs, id)
	}
	sort.Slice(courseIDs, func(i, j int) bool { return courseIDs[i] < courseIDs[j] })
	// Only student enrollments make graded work "yours" (a teacher's
	// planner lists course events and notes, not every assignment).
	studentCourse := map[uint]bool{}
	var sectionIDs []uint
	for _, e := range enrollments {
		if e.Type == "StudentEnrollment" {
			studentCourse[e.CourseID] = true
			if e.CourseSectionID != nil {
				sectionIDs = append(sectionIDs, *e.CourseSectionID)
			}
		}
	}
	var studentCourseIDs []uint
	var studentCourseList []models.Course
	for _, id := range courseIDs {
		if studentCourse[id] {
			studentCourseIDs = append(studentCourseIDs, id)
			studentCourseList = append(studentCourseList, courses[id])
		}
	}

	overrides, err := s.overrideRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	overrideMap := make(map[string]*models.PlannerOverride, len(overrides))
	for i := range overrides {
		o := &overrides[i]
		overrideMap[overrideKey(o.PlannableType, o.PlannableID)] = o
	}

	var items []PlannerItem
	courseName := func(id uint) string {
		c := courses[id]
		return c.Name
	}

	// Quizzes are read first: a graded quiz's online_quiz assignment
	// (G041) is listed once, as the quiz.
	quizzes, err := s.learner.PublishedQuizzes(ctx, studentCourseIDs)
	if err != nil {
		return nil, err
	}
	quizAssignments := QuizLinkedAssignmentIDs(quizzes)

	// Assignments (and graded discussions, which plan as the discussion).
	assignments, err := s.learner.PublishedAssignments(ctx, studentCourseIDs)
	if err != nil {
		return nil, err
	}
	aIDs := make([]uint, len(assignments))
	for i, a := range assignments {
		aIDs[i] = a.ID
	}
	dates, err := loadStudentDates(ctx, s.dueDates, s.learner, userID, sectionIDs, studentCourseList, aIDs)
	if err != nil {
		return nil, err
	}
	subs, err := s.learner.SubmissionsForUser(ctx, userID, aIDs)
	if err != nil {
		return nil, err
	}
	subByAssignment := map[uint]*models.Submission{}
	for i := range subs {
		subByAssignment[subs[i].AssignmentID] = &subs[i]
	}
	topicByAssignment, err := s.learner.DiscussionTopicsForAssignments(ctx, aIDs)
	if err != nil {
		return nil, err
	}
	for i := range assignments {
		a := &assignments[i]
		if quizAssignments[a.ID] {
			continue
		}
		due, assigned := dates.due(a)
		if !assigned || due == nil || !inDateRange(due, q.Start, q.End) {
			continue
		}
		cid := a.CourseID
		sub := subByAssignment[a.ID]
		status := submissionStatus(a, due, sub, now)
		item := PlannerItem{
			PlannableType: "assignment",
			PlannableID:   a.ID,
			PlannableDate: due,
			CourseID:      &cid,
			ContextType:   "Course",
			ContextName:   courseName(cid),
			HTMLURL:       fmt.Sprintf("/courses/%d/assignments/%d", cid, a.ID),
			Submissions:   status,
			Plannable: map[string]any{
				"id": a.ID, "title": a.Name, "due_at": due, "points_possible": a.PointsPossible,
				"assignment_id": a.ID, "course_id": cid,
			},
			NewActivity: status.Graded && sub != nil && sub.PostedAt != nil && sub.PostedAt.After(now.AddDate(0, 0, -3)),
		}
		if topicID, ok := topicByAssignment[a.ID]; ok {
			item.PlannableType = "discussion_topic"
			item.PlannableID = topicID
			item.HTMLURL = fmt.Sprintf("/courses/%d/discussions/%d", cid, topicID)
			item.Plannable.(map[string]any)["id"] = topicID
		}
		item.Completed = status.Submitted || status.Excused || status.Graded
		mergeOverride(&item, overrideMap)
		if sub != nil {
			completeIfTurnedInAfterOverride(&item, overrideMap, sub.SubmittedAt)
		}
		items = append(items, item)
	}

	// Quizzes (a graded quiz plans as the quiz, not its assignment).
	qIDs := make([]uint, len(quizzes))
	for i, qz := range quizzes {
		qIDs[i] = qz.ID
	}
	quizSubs, err := s.learner.LatestQuizSubmissions(ctx, userID, qIDs)
	if err != nil {
		return nil, err
	}
	for _, qz := range quizzes {
		if qz.DueAt == nil || !inDateRange(qz.DueAt, q.Start, q.End) {
			continue
		}
		cid := qz.CourseID
		qs, has := quizSubs[qz.ID]
		done := has && (qs.WorkflowState == "complete" || qs.WorkflowState == "pending_review")
		status := &PlannerSubmissionStatus{
			Submitted: done,
			Graded:    has && qs.WorkflowState == "complete",
			Missing:   !done && qz.DueAt.Before(now),
		}
		item := PlannerItem{
			PlannableType: "quiz",
			PlannableID:   qz.ID,
			PlannableDate: qz.DueAt,
			CourseID:      &cid,
			ContextType:   "Course",
			ContextName:   courseName(cid),
			HTMLURL:       fmt.Sprintf("/courses/%d/quizzes/%d", cid, qz.ID),
			Submissions:   status,
			Plannable: map[string]any{
				"id": qz.ID, "title": qz.Title, "due_at": qz.DueAt, "points_possible": qz.PointsPossible, "course_id": cid,
			},
			Completed: done,
		}
		mergeOverride(&item, overrideMap)
		if done {
			completeIfTurnedInAfterOverride(&item, overrideMap, qs.FinishedAt)
		}
		items = append(items, item)
	}

	// Announcements posted in range (audience-filtered).
	if s.announcements != nil {
		anns, err := s.learner.RecentAnnouncements(ctx, courseIDs, q.Start)
		if err != nil {
			return nil, err
		}
		for i := range anns {
			a := &anns[i]
			if a.CourseID == nil {
				continue
			}
			when := a.CreatedAt
			if a.PostedAt != nil {
				when = *a.PostedAt
			}
			if !inDateRange(&when, q.Start, q.End) || !s.announcements.CanView(ctx, a, userID) {
				continue
			}
			cid := *a.CourseID
			w := when
			item := PlannerItem{
				PlannableType: "announcement",
				PlannableID:   a.ID,
				PlannableDate: &w,
				CourseID:      &cid,
				ContextType:   "Course",
				ContextName:   courseName(cid),
				HTMLURL:       fmt.Sprintf("/courses/%d/announcements", cid),
				Submissions:   false,
				Plannable:     map[string]any{"id": a.ID, "title": a.Title, "posted_at": a.PostedAt},
				NewActivity:   when.After(now.AddDate(0, 0, -3)),
			}
			mergeOverride(&item, overrideMap)
			items = append(items, item)
		}
	}

	// Calendar events: course-scoped, then personal.
	if s.calendarRepo != nil {
		for _, cid := range courseIDs {
			events, err := s.calendarRepo.ListByContextAndDateRange(ctx, "Course", cid, q.Start, q.End)
			if err != nil {
				continue
			}
			for _, e := range events {
				if e.WorkflowState == "deleted" {
					continue
				}
				items = append(items, calendarItem(e, &cid, courseName(cid), overrideMap))
			}
		}
		if userEvents, err := s.calendarRepo.ListByContextAndDateRange(ctx, "User", userID, q.Start, q.End); err == nil {
			for _, e := range userEvents {
				if e.WorkflowState == "deleted" {
					continue
				}
				items = append(items, calendarItem(e, nil, "", overrideMap))
			}
		}
	}

	// Personal planner notes.
	noteResult, err := s.noteRepo.ListByUserID(ctx, userID, repository.PaginationParams{Page: 1, PerPage: 1000})
	if err != nil {
		return nil, err
	}
	for _, n := range noteResult.Items {
		if !inDateRange(&n.TodoDate, q.Start, q.End) {
			continue
		}
		todoDate := n.TodoDate
		item := PlannerItem{
			PlannableType: "planner_note",
			PlannableID:   n.ID,
			PlannableDate: &todoDate,
			CourseID:      n.CourseID,
			ContextType:   "User",
			Submissions:   false,
			Plannable:     plannerNotePlannable(&n),
		}
		if n.CourseID != nil {
			item.ContextType = "Course"
			item.ContextName = courseName(*n.CourseID)
		}
		mergeOverride(&item, overrideMap)
		items = append(items, item)
	}

	if q.IncompleteOnly {
		kept := items[:0]
		for _, it := range items {
			if it.Completed || (it.PlannerOverride != nil && it.PlannerOverride.Dismissed) {
				continue
			}
			kept = append(kept, it)
		}
		items = kept
	}

	sort.SliceStable(items, func(i, j int) bool {
		di, dj := items[i].PlannableDate, items[j].PlannableDate
		switch {
		case di == nil && dj == nil:
			return items[i].PlannableID < items[j].PlannableID
		case di == nil:
			return false
		case dj == nil:
			return true
		case !di.Equal(*dj):
			return di.Before(*dj)
		}
		if items[i].PlannableType != items[j].PlannableType {
			return items[i].PlannableType < items[j].PlannableType
		}
		return items[i].PlannableID < items[j].PlannableID
	})
	if items == nil {
		items = []PlannerItem{}
	}
	return items, nil
}

func calendarItem(e models.CalendarEvent, courseID *uint, name string, overrideMap map[string]*models.PlannerOverride) PlannerItem {
	startAt := e.StartAt
	ctxType := "User"
	if courseID != nil {
		ctxType = "Course"
	}
	item := PlannerItem{
		PlannableType: "calendar_event",
		PlannableID:   e.ID,
		PlannableDate: &startAt,
		CourseID:      courseID,
		ContextType:   ctxType,
		ContextName:   name,
		HTMLURL:       "/calendar",
		Submissions:   false,
		Plannable: map[string]any{
			"id": e.ID, "title": e.Title, "start_at": e.StartAt, "end_at": e.EndAt,
			"all_day": e.AllDay, "location_name": e.LocationName,
		},
	}
	mergeOverride(&item, overrideMap)
	return item
}

func plannerNotePlannable(n *models.PlannerNote) map[string]any {
	return map[string]any{
		"id": n.ID, "title": n.Title, "details": n.Details, "todo_date": n.TodoDate,
		"course_id": n.CourseID, "user_id": n.UserID,
	}
}

// --- Planner Notes CRUD ---

// NoteInput is the user-editable part of a planner note.
type NoteInput struct {
	Title    *string
	Details  *string
	TodoDate *time.Time
	CourseID *uint
	// ClearCourse unlinks the note from its course.
	ClearCourse bool
}

// validateNoteCourse checks that a note's course is one the user is actively
// enrolled in (in their tenant). Any other id is ErrPlannerNotFound.
func (s *PlannerService) validateNoteCourse(ctx context.Context, userID, accountID, courseID uint) error {
	e, err := s.enrollmentRepo.FindByUserAndCourse(ctx, userID, courseID, accountID)
	if err != nil || e == nil || e.WorkflowState != models.EnrollmentActive {
		return ErrPlannerNotFound
	}
	return nil
}

func (s *PlannerService) ListNotes(ctx context.Context, userID uint, start, end *time.Time) ([]models.PlannerNote, error) {
	res, err := s.noteRepo.ListByUserID(ctx, userID, repository.PaginationParams{Page: 1, PerPage: 1000})
	if err != nil {
		return nil, err
	}
	out := make([]models.PlannerNote, 0, len(res.Items))
	for _, n := range res.Items {
		if start != nil && n.TodoDate.Before(*start) {
			continue
		}
		if end != nil && n.TodoDate.After(*end) {
			continue
		}
		out = append(out, n)
	}
	return out, nil
}

// CreateNoteFor creates a note owned by userID.
func (s *PlannerService) CreateNoteFor(ctx context.Context, userID, accountID uint, in NoteInput) (*models.PlannerNote, error) {
	note := &models.PlannerNote{UserID: userID, TodoDate: time.Now(), WorkflowState: "active"}
	if in.Title != nil {
		note.Title = SanitizePlainText(strings.TrimSpace(*in.Title))
	}
	if in.Details != nil {
		note.Details = SanitizePlainText(*in.Details)
	}
	if in.TodoDate != nil {
		note.TodoDate = *in.TodoDate
	}
	if in.CourseID != nil && *in.CourseID != 0 {
		if err := s.validateNoteCourse(ctx, userID, accountID, *in.CourseID); err != nil {
			return nil, err
		}
		cid := *in.CourseID
		note.CourseID = &cid
	}
	if err := s.CreateNote(ctx, note); err != nil {
		return nil, err
	}
	return note, nil
}

// UpdateNoteFor edits the caller's own note; someone else's is
// ErrPlannerNotFound.
func (s *PlannerService) UpdateNoteFor(ctx context.Context, userID, accountID, noteID uint, in NoteInput) (*models.PlannerNote, error) {
	note, err := s.noteRepo.FindByID(ctx, noteID)
	if err != nil || note.UserID != userID || note.WorkflowState == "deleted" {
		return nil, ErrPlannerNotFound
	}
	if in.Title != nil {
		note.Title = SanitizePlainText(strings.TrimSpace(*in.Title))
	}
	if in.Details != nil {
		note.Details = SanitizePlainText(*in.Details)
	}
	if in.TodoDate != nil {
		note.TodoDate = *in.TodoDate
	}
	if in.ClearCourse {
		note.CourseID = nil
	} else if in.CourseID != nil && *in.CourseID != 0 {
		if err := s.validateNoteCourse(ctx, userID, accountID, *in.CourseID); err != nil {
			return nil, err
		}
		cid := *in.CourseID
		note.CourseID = &cid
	}
	if err := s.UpdateNote(ctx, note); err != nil {
		return nil, err
	}
	return note, nil
}

func (s *PlannerService) CreateNote(ctx context.Context, note *models.PlannerNote) error {
	if strings.TrimSpace(note.Title) == "" {
		return errors.New("planner note title is required")
	}
	if len(note.Title) > 255 {
		return errors.New("planner note title is too long")
	}
	if note.WorkflowState == "" {
		note.WorkflowState = "active"
	}
	return s.noteRepo.Create(ctx, note)
}

func (s *PlannerService) UpdateNote(ctx context.Context, note *models.PlannerNote) error {
	if strings.TrimSpace(note.Title) == "" {
		return errors.New("planner note title is required")
	}
	if len(note.Title) > 255 {
		return errors.New("planner note title is too long")
	}
	return s.noteRepo.Update(ctx, note)
}

// DeleteNote soft-deletes the caller's own note.
func (s *PlannerService) DeleteNote(ctx context.Context, noteID, userID uint) error {
	note, err := s.noteRepo.FindByID(ctx, noteID)
	if err != nil || note.UserID != userID || note.WorkflowState == "deleted" {
		return ErrPlannerNotFound
	}
	return s.noteRepo.Delete(ctx, noteID)
}

func (s *PlannerService) GetNoteByID(ctx context.Context, noteID uint) (*models.PlannerNote, error) {
	return s.noteRepo.FindByID(ctx, noteID)
}

// --- Planner Overrides ---

func (s *PlannerService) ListOverrides(ctx context.Context, userID uint) ([]models.PlannerOverride, error) {
	return s.overrideRepo.ListByUserID(ctx, userID)
}

// UpsertOverride sets the caller's override for one plannable. Only the
// flags that are given change: dismissing an item from the missing list
// must not clear its "done" mark, and checking it off must not undo a
// dismissal.
func (s *PlannerService) UpsertOverride(ctx context.Context, userID uint, plannableType string, plannableID uint, markedComplete, dismissed *bool) (*models.PlannerOverride, error) {
	o := &models.PlannerOverride{UserID: userID, PlannableType: plannableType, PlannableID: plannableID}
	if existing, err := s.overrideRepo.FindByUserAndPlannable(ctx, userID, plannableType, plannableID); err == nil && existing != nil {
		o.MarkedComplete, o.Dismissed = existing.MarkedComplete, existing.Dismissed
	}
	if markedComplete != nil {
		o.MarkedComplete = *markedComplete
	}
	if dismissed != nil {
		o.Dismissed = *dismissed
	}
	if err := s.CreateOrUpdateOverride(ctx, o); err != nil {
		return nil, err
	}
	return o, nil
}

// CreateOrUpdateOverride upserts the caller's override for one plannable.
func (s *PlannerService) CreateOrUpdateOverride(ctx context.Context, override *models.PlannerOverride) error {
	if override.PlannableType == "" {
		return errors.New("plannable_type is required")
	}
	if !PlannableTypes[override.PlannableType] {
		return errors.New("plannable_type is not supported")
	}
	if override.PlannableID == 0 {
		return errors.New("plannable_id is required")
	}
	existing, _ := s.overrideRepo.FindByUserAndPlannable(ctx, override.UserID, override.PlannableType, override.PlannableID)
	if existing != nil {
		existing.MarkedComplete = override.MarkedComplete
		existing.Dismissed = override.Dismissed
		if err := s.overrideRepo.Update(ctx, existing); err != nil {
			return err
		}
		*override = *existing
		return nil
	}
	return s.overrideRepo.Create(ctx, override)
}

// UpdateOverride edits the caller's own override by id.
func (s *PlannerService) UpdateOverride(ctx context.Context, overrideID, userID uint, markedComplete, dismissed *bool) (*models.PlannerOverride, error) {
	o, err := s.overrideRepo.FindByID(ctx, overrideID)
	if err != nil || o.UserID != userID {
		return nil, ErrPlannerNotFound
	}
	if markedComplete != nil {
		o.MarkedComplete = *markedComplete
	}
	if dismissed != nil {
		o.Dismissed = *dismissed
	}
	if err := s.overrideRepo.Update(ctx, o); err != nil {
		return nil, err
	}
	return o, nil
}

func (s *PlannerService) DeleteOverride(ctx context.Context, overrideID, userID uint) error {
	override, err := s.overrideRepo.FindByID(ctx, overrideID)
	if err != nil || override.UserID != userID {
		return ErrPlannerNotFound
	}
	return s.overrideRepo.Delete(ctx, overrideID)
}

func (s *PlannerService) GetOverrideByID(ctx context.Context, overrideID uint) (*models.PlannerOverride, error) {
	return s.overrideRepo.FindByID(ctx, overrideID)
}

// --- Helpers ---

func overrideKey(plannableType string, plannableID uint) string {
	return fmt.Sprintf("%s:%d", plannableType, plannableID)
}

// mergeOverride attaches the user's override and lets it decide Completed.
func mergeOverride(item *PlannerItem, overrideMap map[string]*models.PlannerOverride) {
	if o, ok := overrideMap[overrideKey(item.PlannableType, item.PlannableID)]; ok {
		item.PlannerOverride = &PlannerOverrideInfo{
			ID:             o.ID,
			MarkedComplete: o.MarkedComplete,
			Dismissed:      o.Dismissed,
		}
		item.Completed = o.MarkedComplete
	}
}

// completeIfTurnedInAfterOverride marks graded work done when the student
// turned it in after their last override change. Canvas does the same when
// a submission is saved (it completes the planner override), so an item
// left unchecked or dismissed from the missing list does not stay "not
// done" once the work is actually submitted.
func completeIfTurnedInAfterOverride(item *PlannerItem, overrideMap map[string]*models.PlannerOverride, turnedInAt *time.Time) {
	if item.PlannerOverride == nil || item.PlannerOverride.MarkedComplete || turnedInAt == nil {
		return
	}
	o := overrideMap[overrideKey(item.PlannableType, item.PlannableID)]
	if o != nil && turnedInAt.After(o.UpdatedAt) {
		item.Completed = true
		item.PlannerOverride.MarkedComplete = true
	}
}

func inDateRange(t *time.Time, start, end time.Time) bool {
	if t == nil {
		return false
	}
	return !t.Before(start) && !t.After(end)
}
