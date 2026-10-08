package service

import (
	"context"
	"errors"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// CoursePacingOverrideTitle is the title of the assignment overrides a
// published student/section pace writes (Canvas uses the same title).
const CoursePacingOverrideTitle = "Course Pacing"

// ErrPaceItemNotInCourse is returned when a pace names a module item
// that does not belong to a module of the pace's course.
var ErrPaceItemNotInCourse = errors.New("module item does not belong to this course")

// ErrPaceTargetNotInCourse is returned when a pace's user_id or
// course_section_id is not part of the pace's course.
var ErrPaceTargetNotInCourse = errors.New("pace user or section does not belong to this course")

type CoursePaceService struct {
	paceRepo       repository.CoursePaceRepository
	paceItemRepo   repository.CoursePaceModuleItemRepository
	moduleItemRepo repository.ModuleItemRepository
	assignmentRepo repository.AssignmentRepository

	// Optional dependencies (SetDeps). moduleRepo enables the
	// module-item → course tie; the override repos let student/section
	// paces write overrides instead of the shared due date; courseRepo,
	// enrollmentRepo and sectionRepo give the schedule its start date.
	moduleRepo          repository.ModuleRepository
	overrideRepo        repository.AssignmentOverrideRepository
	overrideStudentRepo repository.AssignmentOverrideStudentRepository
	courseRepo          repository.CourseRepository
	enrollmentRepo      repository.EnrollmentRepository
	sectionRepo         repository.SectionRepository

	// G079 (SetPacingRepos, course_pace_schedule.go): blackout dates and
	// the quiz / discussion repos that let paces date them too.
	blackouts BlackoutDateStore
	quizRepo  repository.QuizRepository
	topicRepo repository.DiscussionTopicRepository
}

func NewCoursePaceService(paceRepo repository.CoursePaceRepository, paceItemRepo repository.CoursePaceModuleItemRepository, moduleItemRepo repository.ModuleItemRepository, assignmentRepo repository.AssignmentRepository) *CoursePaceService {
	return &CoursePaceService{paceRepo: paceRepo, paceItemRepo: paceItemRepo, moduleItemRepo: moduleItemRepo, assignmentRepo: assignmentRepo}
}

// CoursePaceDeps carries the optional repositories wired by SetDeps.
type CoursePaceDeps struct {
	ModuleRepo          repository.ModuleRepository
	OverrideRepo        repository.AssignmentOverrideRepository
	OverrideStudentRepo repository.AssignmentOverrideStudentRepository
	CourseRepo          repository.CourseRepository
	EnrollmentRepo      repository.EnrollmentRepository
	SectionRepo         repository.SectionRepository
}

// SetDeps wires the optional repositories.
func (s *CoursePaceService) SetDeps(d CoursePaceDeps) {
	s.moduleRepo = d.ModuleRepo
	s.overrideRepo = d.OverrideRepo
	s.overrideStudentRepo = d.OverrideStudentRepo
	s.courseRepo = d.CourseRepo
	s.enrollmentRepo = d.EnrollmentRepo
	s.sectionRepo = d.SectionRepo
}

func (s *CoursePaceService) Create(ctx context.Context, pace *models.CoursePace) error {
	if pace.CourseID == 0 {
		return errors.New("course_id is required")
	}
	if pace.UserID != nil && pace.CourseSectionID != nil {
		return errors.New("a pace targets a student or a section, not both")
	}
	if err := s.validateTarget(ctx, pace); err != nil {
		return err
	}
	if pace.WorkflowState == "" {
		pace.WorkflowState = models.CoursePaceUnpublished
	}
	return s.paceRepo.Create(ctx, pace)
}

// validateTarget checks that a student pace's user is enrolled in the
// pace's course and a section pace's section belongs to it.
func (s *CoursePaceService) validateTarget(ctx context.Context, pace *models.CoursePace) error {
	if pace.UserID != nil && s.enrollmentRepo != nil {
		e, err := s.enrollmentRepo.FindByUserAndCourse(ctx, *pace.UserID, pace.CourseID, 0)
		if err != nil || e == nil {
			return ErrPaceTargetNotInCourse
		}
	}
	if pace.CourseSectionID != nil && s.sectionRepo != nil {
		sec, err := s.sectionRepo.FindByID(ctx, *pace.CourseSectionID)
		if err != nil || sec == nil || sec.CourseID != pace.CourseID {
			return ErrPaceTargetNotInCourse
		}
	}
	return nil
}

func (s *CoursePaceService) GetByID(ctx context.Context, id, accountID uint) (*models.CoursePace, error) {
	return s.paceRepo.FindByID(ctx, id, accountID)
}

func (s *CoursePaceService) GetByCourseID(ctx context.Context, courseID uint) (*models.CoursePace, error) {
	return s.paceRepo.FindByCourseID(ctx, courseID)
}

func (s *CoursePaceService) GetByUserID(ctx context.Context, courseID uint, userID uint) (*models.CoursePace, error) {
	return s.paceRepo.FindByUserID(ctx, courseID, userID)
}

func (s *CoursePaceService) GetBySectionID(ctx context.Context, courseID uint, sectionID uint) (*models.CoursePace, error) {
	return s.paceRepo.FindBySectionID(ctx, courseID, sectionID)
}

func (s *CoursePaceService) Update(ctx context.Context, pace *models.CoursePace) error {
	return s.paceRepo.Update(ctx, pace)
}

func (s *CoursePaceService) Delete(ctx context.Context, id uint) error {
	return s.paceRepo.Delete(ctx, id)
}

func (s *CoursePaceService) ListByCourse(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.CoursePace], error) {
	return s.paceRepo.ListByCourseID(ctx, courseID, params)
}

func (s *CoursePaceService) PublishPace(ctx context.Context, id, accountID uint) (*models.CoursePace, error) {
	pace, err := s.paceRepo.FindByID(ctx, id, accountID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	pace.WorkflowState = models.CoursePaceActive
	pace.PublishedAt = &now

	if err := s.paceRepo.Update(ctx, pace); err != nil {
		return nil, err
	}

	// Apply computed dates to linked assignments
	s.applyPaceDates(ctx, pace)

	return pace, nil
}

// paceStartDate is the date the pace's schedule counts from. Like
// Canvas it is anchored to the calendar, not to the publish click, so
// republishing a pace does not shift every date (B144):
//   - student pace: the student's enrollment date
//   - section pace: the section start date
//   - otherwise the course start date
//
// falling back to the pace's creation time when none is set.
func (s *CoursePaceService) paceStartDate(ctx context.Context, pace *models.CoursePace) time.Time {
	if pace.UserID != nil && s.enrollmentRepo != nil {
		if e, err := s.enrollmentRepo.FindByUserAndCourse(ctx, *pace.UserID, pace.CourseID, 0); err == nil && e != nil && !e.CreatedAt.IsZero() {
			return e.CreatedAt
		}
	}
	if pace.CourseSectionID != nil && s.sectionRepo != nil {
		if sec, err := s.sectionRepo.FindByID(ctx, *pace.CourseSectionID); err == nil && sec != nil && sec.StartAt != nil {
			return *sec.StartAt
		}
	}
	if s.courseRepo != nil {
		// Internal hop: the pace was already loaded under the caller's
		// tenant, so its course id is trusted here.
		if c, err := s.courseRepo.FindByID(ctx, pace.CourseID, 0); err == nil && c != nil && c.StartAt != nil {
			return *c.StartAt
		}
	}
	if !pace.CreatedAt.IsZero() {
		return pace.CreatedAt
	}
	return time.Now()
}

// moduleItemInCourse loads a module item and verifies its module belongs
// to courseID. Without a module repository it cannot verify the tie and
// returns an error (fail closed).
func (s *CoursePaceService) moduleItemInCourse(ctx context.Context, itemID, courseID uint) (*models.ContentTag, error) {
	if s.moduleItemRepo == nil || s.moduleRepo == nil {
		return nil, ErrPaceItemNotInCourse
	}
	item, err := s.moduleItemRepo.FindByID(ctx, itemID)
	if err != nil || item == nil || item.WorkflowState == models.ContentTagDeleted {
		return nil, ErrPaceItemNotInCourse
	}
	mod, err := s.moduleRepo.FindByID(ctx, item.ContextModuleID, 0)
	if err != nil || mod == nil || mod.CourseID != courseID || mod.WorkflowState == "deleted" {
		return nil, ErrPaceItemNotInCourse
	}
	return item, nil
}

// applyPaceDates computes the timeline and writes due dates for each
// module item that links to an assignment of the pace's course.
//
//   - The course-wide pace sets the assignment's own due date.
//   - A student pace writes an ad-hoc "Course Pacing" override for that
//     student, and a section pace a section override, so one learner's
//     schedule never moves the date for the whole class (B144, Canvas
//     CoursePace#publish).
//
// Every module item and assignment is tied to the pace's course (B039):
// a pace can never touch another course's (or tenant's) assignments.
func (s *CoursePaceService) applyPaceDates(ctx context.Context, pace *models.CoursePace) {
	if s.moduleItemRepo == nil || s.assignmentRepo == nil {
		return
	}

	items, err := s.paceItemRepo.ListByPaceID(ctx, pace.ID)
	if err != nil || len(items) == 0 {
		return
	}

	// G079: the shared schedule skips weekends (when excluded) and
	// blackout dates, and compresses to the end date for hard end dates.
	dates := s.paceSchedule(ctx, pace, items)
	for i, item := range items {
		moduleItem, err := s.moduleItemInCourse(ctx, item.ModuleItemID, pace.CourseID)
		if err != nil {
			continue
		}
		assignmentID, handled := s.pacedAssignmentID(ctx, pace, moduleItem, dates[i])
		if handled || assignmentID == 0 {
			continue
		}

		assignment, err := s.assignmentRepo.FindByID(ctx, assignmentID, 0)
		if err != nil || assignment == nil || assignment.CourseID != pace.CourseID {
			continue
		}

		projectedDate := dates[i]
		switch {
		case pace.UserID != nil:
			s.upsertStudentPaceOverride(ctx, assignment.ID, *pace.UserID, projectedDate)
		case pace.CourseSectionID != nil:
			s.upsertSectionPaceOverride(ctx, assignment.ID, *pace.CourseSectionID, projectedDate)
		default:
			assignment.DueAt = &projectedDate
			_ = s.assignmentRepo.Update(ctx, assignment)
		}
	}
}

// upsertStudentPaceOverride gives userID the due date dueAt on
// assignmentID through an ad-hoc override, reusing the student's
// existing single-student override when there is one.
func (s *CoursePaceService) upsertStudentPaceOverride(ctx context.Context, assignmentID, userID uint, dueAt time.Time) {
	if s.overrideRepo == nil || s.overrideStudentRepo == nil {
		return
	}
	existing, _ := s.overrideStudentRepo.ListByUserAndAssignment(ctx, userID, assignmentID)
	for _, row := range existing {
		ov, err := s.overrideRepo.FindByID(ctx, row.AssignmentOverrideID)
		if err != nil || ov == nil {
			continue
		}
		members, _ := s.overrideStudentRepo.ListByOverrideID(ctx, ov.ID)
		if len(members) <= 1 {
			// The student's own override: move its date.
			d := dueAt
			ov.DueAt = &d
			ov.WorkflowState = "active"
			_ = s.overrideRepo.Update(ctx, ov)
			return
		}
		// Shared with other students: take this student out of it so
		// the pace date applies to them alone.
		_ = s.overrideStudentRepo.Delete(ctx, ov.ID, userID)
	}
	d := dueAt
	ov := &models.AssignmentOverride{
		AssignmentID:  assignmentID,
		Title:         CoursePacingOverrideTitle,
		DueAt:         &d,
		WorkflowState: "active",
	}
	if err := s.overrideRepo.Create(ctx, ov); err != nil {
		return
	}
	_ = s.overrideStudentRepo.Create(ctx, &models.AssignmentOverrideStudent{
		AssignmentOverrideID: ov.ID,
		UserID:               userID,
		AssignmentID:         assignmentID,
	})
}

// upsertSectionPaceOverride gives sectionID the due date dueAt on
// assignmentID through its section override (created when missing).
func (s *CoursePaceService) upsertSectionPaceOverride(ctx context.Context, assignmentID, sectionID uint, dueAt time.Time) {
	if s.overrideRepo == nil {
		return
	}
	overrides, _ := s.overrideRepo.ListByAssignmentID(ctx, assignmentID)
	for i := range overrides {
		ov := overrides[i]
		if ov.CourseSectionID != nil && *ov.CourseSectionID == sectionID && ov.WorkflowState != "deleted" {
			d := dueAt
			ov.DueAt = &d
			_ = s.overrideRepo.Update(ctx, &ov)
			return
		}
	}
	d := dueAt
	sid := sectionID
	_ = s.overrideRepo.Create(ctx, &models.AssignmentOverride{
		AssignmentID:    assignmentID,
		Title:           CoursePacingOverrideTitle,
		DueAt:           &d,
		CourseSectionID: &sid,
		WorkflowState:   "active",
	})
}

func (s *CoursePaceService) GetPaceItems(ctx context.Context, paceID uint) ([]models.CoursePaceModuleItem, error) {
	return s.paceItemRepo.ListByPaceID(ctx, paceID)
}

// UpdatePaceItems upserts the durations of the pace's module items.
// Every module item must belong to a module of the pace's course (B039);
// a foreign id fails the whole request with ErrPaceItemNotInCourse.
func (s *CoursePaceService) UpdatePaceItems(ctx context.Context, pace *models.CoursePace, items []models.CoursePaceModuleItem) ([]models.CoursePaceModuleItem, error) {
	for i := range items {
		if items[i].Duration < 0 {
			return nil, errors.New("duration must be zero or more days")
		}
		if _, err := s.moduleItemInCourse(ctx, items[i].ModuleItemID, pace.CourseID); err != nil {
			return nil, err
		}
		items[i].CoursePaceID = pace.ID
	}

	if err := s.paceItemRepo.BulkUpsert(ctx, items); err != nil {
		return nil, err
	}

	return s.paceItemRepo.ListByPaceID(ctx, pace.ID)
}

// ComputeTimeline computes projected dates for each module item based on duration
// and whether weekends are excluded.
func (s *CoursePaceService) ComputeTimeline(ctx context.Context, paceID, accountID uint) ([]map[string]interface{}, error) {
	pace, err := s.paceRepo.FindByID(ctx, paceID, accountID)
	if err != nil {
		return nil, err
	}

	items, err := s.paceItemRepo.ListByPaceID(ctx, paceID)
	if err != nil {
		return nil, err
	}

	timeline := make([]map[string]interface{}, len(items))
	dates := s.paceSchedule(ctx, pace, items)

	for i, item := range items {
		timeline[i] = map[string]interface{}{
			"module_item_id": item.ModuleItemID,
			"duration":       item.Duration,
			"projected_date": dates[i].Format(time.RFC3339),
		}
	}

	return timeline, nil
}
