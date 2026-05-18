package service

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// PlannerItem is a unified representation of any plannable object (assignment,
// quiz, discussion, calendar event, planner note, etc.) that appears on a
// student's planner / to-do list.
type PlannerItem struct {
	PlannableType string      `json:"plannable_type"` // "assignment", "quiz", "discussion_topic", "calendar_event", "planner_note", "announcement"
	PlannableID   uint        `json:"plannable_id"`
	PlannableDate *time.Time  `json:"plannable_date"`
	Plannable     interface{} `json:"plannable"`
	CourseID      *uint       `json:"course_id,omitempty"`
	ContextName   string      `json:"context_name,omitempty"`
	NewActivity   bool        `json:"new_activity"`
	Submissions   interface{} `json:"submissions,omitempty"` // submission status for graded items

	// PlannerOverride fields merged in when an override exists
	PlannerOverride *PlannerOverrideInfo `json:"planner_override,omitempty"`
}

// PlannerOverrideInfo is a lightweight copy of the override for embedding in
// planner item responses.
type PlannerOverrideInfo struct {
	ID             uint  `json:"id"`
	MarkedComplete bool  `json:"marked_complete"`
	Dismissed      bool  `json:"dismissed"`
}

type PlannerService struct {
	noteRepo       repository.PlannerNoteRepository
	overrideRepo   repository.PlannerOverrideRepository
	enrollmentRepo repository.EnrollmentRepository
	assignmentRepo repository.AssignmentRepository
	quizRepo       repository.QuizRepository
	calendarRepo   repository.CalendarEventRepository
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

// GetPlannerItems aggregates assignments, quizzes, calendar events, and
// personal planner notes from all of the user's enrolled courses into a single
// chronological list within the given date range.
func (s *PlannerService) GetPlannerItems(ctx context.Context, userID uint, startDate, endDate time.Time) ([]PlannerItem, error) {
	// 1. Fetch the user's active enrollments to discover their courses.
	enrollments, err := s.enrollmentRepo.ListByUserID(ctx, userID, 0)
	if err != nil {
		return nil, err
	}

	courseIDs := make([]uint, 0, len(enrollments))
	for _, e := range enrollments {
		courseIDs = append(courseIDs, e.CourseID)
	}

	// 2. Fetch all overrides for this user so we can merge them later.
	overrides, err := s.overrideRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Build a lookup map: "type:id" -> override
	overrideMap := make(map[string]*models.PlannerOverride, len(overrides))
	for i := range overrides {
		o := &overrides[i]
		key := overrideKey(o.PlannableType, o.PlannableID)
		overrideMap[key] = o
	}

	var items []PlannerItem

	// Use a large page size to fetch all items per course.
	// For a student planner this is bounded by the date range.
	bigPage := repository.PaginationParams{Page: 1, PerPage: 500}

	// 3. For each course, gather assignments, quizzes, and calendar events.
	for _, courseID := range courseIDs {
		// Assignments
		assignmentResult, err := s.assignmentRepo.ListByCourseID(ctx, courseID, bigPage)
		if err == nil {
			for _, a := range assignmentResult.Items {
				if a.WorkflowState == "deleted" || !a.Published {
					continue
				}
				if a.DueAt == nil || !inDateRange(a.DueAt, startDate, endDate) {
					continue
				}
				cid := courseID
				item := PlannerItem{
					PlannableType: "assignment",
					PlannableID:   a.ID,
					PlannableDate: a.DueAt,
					CourseID:      &cid,
					Plannable:     a,
					Submissions:   false, // caller can enrich with submission status
				}
				mergeOverride(&item, overrideMap)
				items = append(items, item)
			}
		}

		// Quizzes
		quizResult, err := s.quizRepo.ListByCourseID(ctx, courseID, bigPage)
		if err == nil {
			for _, q := range quizResult.Items {
				if q.WorkflowState == "deleted" || !q.Published {
					continue
				}
				if q.DueAt == nil || !inDateRange(q.DueAt, startDate, endDate) {
					continue
				}
				cid := courseID
				item := PlannerItem{
					PlannableType: "quiz",
					PlannableID:   q.ID,
					PlannableDate: q.DueAt,
					CourseID:      &cid,
					Plannable:     q,
				}
				mergeOverride(&item, overrideMap)
				items = append(items, item)
			}
		}

		// Calendar events (course-scoped)
		events, err := s.calendarRepo.ListByContextAndDateRange(ctx, "Course", courseID, startDate, endDate)
		if err == nil {
			for _, e := range events {
				cid := courseID
				startAt := e.StartAt
				item := PlannerItem{
					PlannableType: "calendar_event",
					PlannableID:   e.ID,
					PlannableDate: &startAt,
					CourseID:      &cid,
					Plannable:     e,
				}
				mergeOverride(&item, overrideMap)
				items = append(items, item)
			}
		}
	}

	// 4. Personal calendar events (user-scoped)
	userEvents, err := s.calendarRepo.ListByContextAndDateRange(ctx, "User", userID, startDate, endDate)
	if err == nil {
		for _, e := range userEvents {
			startAt := e.StartAt
			item := PlannerItem{
				PlannableType: "calendar_event",
				PlannableID:   e.ID,
				PlannableDate: &startAt,
				Plannable:     e,
			}
			mergeOverride(&item, overrideMap)
			items = append(items, item)
		}
	}

	// 5. Personal planner notes
	noteResult, err := s.noteRepo.ListByUserID(ctx, userID, bigPage)
	if err == nil {
		for _, n := range noteResult.Items {
			if !inDateRange(&n.TodoDate, startDate, endDate) {
				continue
			}
			todoDate := n.TodoDate
			item := PlannerItem{
				PlannableType: "planner_note",
				PlannableID:   n.ID,
				PlannableDate: &todoDate,
				CourseID:      n.CourseID,
				Plannable:     n,
			}
			mergeOverride(&item, overrideMap)
			items = append(items, item)
		}
	}

	// 6. Sort by date ascending.
	sort.Slice(items, func(i, j int) bool {
		di := items[i].PlannableDate
		dj := items[j].PlannableDate
		if di == nil && dj == nil {
			return items[i].PlannableID < items[j].PlannableID
		}
		if di == nil {
			return false
		}
		if dj == nil {
			return true
		}
		return di.Before(*dj)
	})

	return items, nil
}

// --- Planner Notes CRUD ---

func (s *PlannerService) CreateNote(ctx context.Context, note *models.PlannerNote) error {
	if note.Title == "" {
		return errors.New("planner note title is required")
	}
	if note.WorkflowState == "" {
		note.WorkflowState = "active"
	}
	return s.noteRepo.Create(ctx, note)
}

func (s *PlannerService) UpdateNote(ctx context.Context, note *models.PlannerNote) error {
	if note.Title == "" {
		return errors.New("planner note title is required")
	}
	return s.noteRepo.Update(ctx, note)
}

func (s *PlannerService) DeleteNote(ctx context.Context, noteID, userID uint) error {
	note, err := s.noteRepo.FindByID(ctx, noteID)
	if err != nil {
		return errors.New("planner note not found")
	}
	if note.UserID != userID {
		return errors.New("you can only delete your own planner notes")
	}
	return s.noteRepo.Delete(ctx, noteID)
}

func (s *PlannerService) GetNoteByID(ctx context.Context, noteID uint) (*models.PlannerNote, error) {
	return s.noteRepo.FindByID(ctx, noteID)
}

// --- Planner Overrides ---

func (s *PlannerService) CreateOrUpdateOverride(ctx context.Context, override *models.PlannerOverride) error {
	if override.PlannableType == "" {
		return errors.New("plannable_type is required")
	}
	if override.PlannableID == 0 {
		return errors.New("plannable_id is required")
	}

	// Check if an override already exists for this user + plannable
	existing, _ := s.overrideRepo.FindByUserAndPlannable(ctx, override.UserID, override.PlannableType, override.PlannableID)
	if existing != nil {
		existing.MarkedComplete = override.MarkedComplete
		existing.Dismissed = override.Dismissed
		return s.overrideRepo.Update(ctx, existing)
	}
	return s.overrideRepo.Create(ctx, override)
}

func (s *PlannerService) DeleteOverride(ctx context.Context, overrideID, userID uint) error {
	override, err := s.overrideRepo.FindByID(ctx, overrideID)
	if err != nil {
		return errors.New("planner override not found")
	}
	if override.UserID != userID {
		return errors.New("you can only delete your own planner overrides")
	}
	return s.overrideRepo.Delete(ctx, overrideID)
}

func (s *PlannerService) GetOverrideByID(ctx context.Context, overrideID uint) (*models.PlannerOverride, error) {
	return s.overrideRepo.FindByID(ctx, overrideID)
}

// --- Helpers ---

func overrideKey(plannableType string, plannableID uint) string {
	return plannableType + ":" + uintToStr(plannableID)
}

func uintToStr(v uint) string {
	buf := make([]byte, 0, 10)
	if v == 0 {
		return "0"
	}
	for v > 0 {
		buf = append(buf, byte('0'+v%10))
		v /= 10
	}
	// reverse
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}

func mergeOverride(item *PlannerItem, overrideMap map[string]*models.PlannerOverride) {
	key := overrideKey(item.PlannableType, item.PlannableID)
	if o, ok := overrideMap[key]; ok {
		item.PlannerOverride = &PlannerOverrideInfo{
			ID:             o.ID,
			MarkedComplete: o.MarkedComplete,
			Dismissed:      o.Dismissed,
		}
	}
}

func inDateRange(t *time.Time, start, end time.Time) bool {
	if t == nil {
		return false
	}
	return !t.Before(start) && !t.After(end)
}
