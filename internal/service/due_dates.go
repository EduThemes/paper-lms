package service

// Effective due dates (G019 / G022 / G025 / G040): the dates that apply
// to each student on each assignment once assignment overrides and
// extra-day accommodations are applied, the grading period those dates
// fall in, and whether the student is assigned the assignment at all.
// This is the bulk twin of SubmissionService.EffectiveDates.

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// EffectiveDueDate is one student's dates on one assignment.
type EffectiveDueDate struct {
	DueAt                 *time.Time `json:"due_at"`
	UnlockAt              *time.Time `json:"unlock_at"`
	LockAt                *time.Time `json:"lock_at"`
	GradingPeriodID       uint       `json:"grading_period_id,omitempty"`
	InClosedGradingPeriod bool       `json:"in_closed_grading_period"`
	// OverrideID is the override the dates come from (0 = the
	// assignment's own dates).
	OverrideID uint `json:"override_id,omitempty"`
	ExtraDays  int  `json:"extra_days,omitempty"`
}

// DueDateInput is everything the resolver needs for one course.
type DueDateInput struct {
	Assignments      []models.Assignment
	StudentIDs       []uint
	StudentSections  map[uint][]uint // user -> section ids in the course
	Overrides        []models.AssignmentOverride
	OverrideStudents []models.AssignmentOverrideStudent
	// ExtraDays is each student's largest accommodation extension.
	ExtraDays map[uint]int
	Periods   []models.GradingPeriod
	Now       time.Time
}

// DueDateMap is assignment id -> user id -> dates. Students the
// assignment is not assigned to (only_visible_to_overrides without an
// applicable override) are absent.
type DueDateMap map[uint]map[uint]EffectiveDueDate

// AccommodationExtraDays reduces accommodations to each user's largest
// extra_days.
func AccommodationExtraDays(accs []models.StudentAccommodation) map[uint]int {
	out := map[uint]int{}
	for _, a := range accs {
		if a.ExtraDays != nil && *a.ExtraDays > out[a.UserID] {
			out[a.UserID] = *a.ExtraDays
		}
	}
	return out
}

// ResolveDueDates computes the effective dates for every assignment and
// student.
func ResolveDueDates(in DueDateInput) DueDateMap {
	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	overridesByAssignment := map[uint][]models.AssignmentOverride{}
	overrideByID := map[uint]models.AssignmentOverride{}
	for _, o := range in.Overrides {
		if o.WorkflowState == "deleted" {
			continue
		}
		overridesByAssignment[o.AssignmentID] = append(overridesByAssignment[o.AssignmentID], o)
		overrideByID[o.ID] = o
	}
	// assignment -> user -> ad-hoc override ids
	adhoc := map[uint]map[uint][]uint{}
	for _, os := range in.OverrideStudents {
		if _, ok := overrideByID[os.AssignmentOverrideID]; !ok {
			continue
		}
		if adhoc[os.AssignmentID] == nil {
			adhoc[os.AssignmentID] = map[uint][]uint{}
		}
		adhoc[os.AssignmentID][os.UserID] = append(adhoc[os.AssignmentID][os.UserID], os.AssignmentOverrideID)
	}

	out := DueDateMap{}
	for i := range in.Assignments {
		a := &in.Assignments[i]
		perUser := map[uint]EffectiveDueDate{}
		for _, uid := range in.StudentIDs {
			var applicable []models.AssignmentOverride
			for _, oid := range adhoc[a.ID][uid] {
				applicable = append(applicable, overrideByID[oid])
			}
			for _, o := range overridesByAssignment[a.ID] {
				if o.CourseSectionID == nil {
					continue
				}
				for _, sid := range in.StudentSections[uid] {
					if *o.CourseSectionID == sid {
						applicable = append(applicable, o)
						break
					}
				}
			}
			if len(applicable) == 0 && a.OnlyVisibleToOverrides {
				continue // not assigned to this student
			}
			d := EffectiveDueDate{DueAt: a.DueAt, UnlockAt: a.UnlockAt, LockAt: a.LockAt}
			if len(applicable) > 0 {
				best := applicable[0]
				for _, o := range applicable[1:] {
					if laterDue(o.DueAt, best.DueAt) {
						best = o
					}
				}
				d.DueAt, d.UnlockAt, d.LockAt, d.OverrideID = best.DueAt, best.UnlockAt, best.LockAt, best.ID
			}
			if extra := in.ExtraDays[uid]; extra > 0 && d.DueAt != nil {
				due := d.DueAt.AddDate(0, 0, extra)
				d.DueAt = &due
				if d.LockAt != nil {
					lock := d.LockAt.AddDate(0, 0, extra)
					d.LockAt = &lock
				}
				d.ExtraDays = extra
			}
			if p := GradingPeriodForDate(in.Periods, d.DueAt); p != nil {
				d.GradingPeriodID = p.ID
				d.InClosedGradingPeriod = GradingPeriodClosed(p, now)
			}
			perUser[uid] = d
		}
		out[a.ID] = perUser
	}
	return out
}

// GradingPeriodForDate returns the period a due date falls in (start <
// due <= end, Canvas's rule). An assignment with no due date belongs to
// the last period. Nil when no period matches.
func GradingPeriodForDate(periods []models.GradingPeriod, due *time.Time) *models.GradingPeriod {
	var last *models.GradingPeriod
	for i := range periods {
		p := &periods[i]
		if p.WorkflowState != "" && p.WorkflowState != "active" {
			continue
		}
		if last == nil || p.EndDate.After(last.EndDate) {
			last = p
		}
		if due != nil && due.After(p.StartDate) && !due.After(p.EndDate) {
			return p
		}
	}
	if due == nil {
		return last
	}
	return nil
}

// GradingPeriodClosed reports whether grades in the period are locked.
func GradingPeriodClosed(p *models.GradingPeriod, now time.Time) bool {
	if p == nil {
		return false
	}
	if p.IsClosed {
		return true
	}
	return p.CloseDate != nil && now.After(*p.CloseDate)
}

// StudentDueDateResolver resolves one student's effective dates across
// courses (GradingPolicyService.StudentDueDates): assignment id -> dates,
// with unassigned assignments absent.
type StudentDueDateResolver interface {
	StudentDueDates(ctx context.Context, userID uint, courses []models.Course) (map[uint]EffectiveDueDate, error)
}

// studentDates is one student's dates as the planner and the missing-work
// list read them.
type studentDates struct {
	resolved map[uint]EffectiveDueDate
	// overrides is the fallback when no resolver is wired: the due dates
	// of the overrides that apply to the student.
	overrides map[uint][]*time.Time
}

// due returns the student's due date on a, and whether a is assigned to
// them at all.
func (d studentDates) due(a *models.Assignment) (*time.Time, bool) {
	if d.resolved != nil {
		dd, ok := d.resolved[a.ID]
		return dd.DueAt, ok
	}
	ov := d.overrides[a.ID]
	if a.OnlyVisibleToOverrides && len(ov) == 0 {
		return nil, false
	}
	return effectiveDueAt(a.DueAt, ov), true
}

func loadStudentDates(ctx context.Context, resolver StudentDueDateResolver, learner learnerOverrideDates, userID uint,
	sectionIDs []uint, courses []models.Course, assignmentIDs []uint) (studentDates, error) {
	if resolver != nil {
		m, err := resolver.StudentDueDates(ctx, userID, courses)
		if m == nil {
			m = map[uint]EffectiveDueDate{}
		}
		return studentDates{resolved: m}, err
	}
	ov, err := learner.OverrideDueDates(ctx, userID, sectionIDs, assignmentIDs)
	return studentDates{overrides: ov}, err
}

// learnerOverrideDates is the slice of LearnerRepository the fallback uses.
type learnerOverrideDates interface {
	OverrideDueDates(ctx context.Context, userID uint, sectionIDs, assignmentIDs []uint) (map[uint][]*time.Time, error)
}
