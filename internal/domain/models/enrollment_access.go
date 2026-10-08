package models

import "time"

// Date-based enrollment access (G106) and the multi-enrollment /
// section-limit resolution (G107, G110) used by the course access path
// (middleware.PermissionMiddleware and handlers.ResourceAuthorizer).
//
// The semantics follow Canvas's EnrollmentDateBuilder + EnrollmentState:
//
//   - An enrollment's participation window comes from, in priority order,
//     its own start_at/end_at (only when BOTH are set), its section's dates
//     when the section restricts enrollments to them, the course's dates
//     when the course does, and otherwise the term's dates. Staff
//     (teacher/TA/designer) ignore the term START date, and when bound by
//     section or course dates they also keep the term window.
//   - Inside any window the stored workflow_state applies. Past every
//     window the enrollment is "completed" (concluded). Before every window
//     a learner is "pending" and staff are "inactive".
//   - A concluded enrollment is read-only; a pending learner may view
//     read-only. A course can remove either view with
//     restrict_student_past_view / restrict_student_future_view.
//   - A course in the "completed" workflow state concludes every
//     enrollment in it (read-only); only an admin can reopen it.

// EnrollmentEffectiveState is an enrollment's state after dates apply.
type EnrollmentEffectiveState string

const (
	EffectiveActive         EnrollmentEffectiveState = "active"
	EffectiveInvited        EnrollmentEffectiveState = "invited"
	EffectivePendingActive  EnrollmentEffectiveState = "pending_active"
	EffectivePendingInvited EnrollmentEffectiveState = "pending_invited"
	EffectiveCompleted      EnrollmentEffectiveState = "completed"
	EffectiveInactive       EnrollmentEffectiveState = "inactive"
	EffectiveDeleted        EnrollmentEffectiveState = "deleted"
	EffectiveRejected       EnrollmentEffectiveState = "rejected"
)

// EnrollmentAccess is what an enrollment lets its user do in the course.
type EnrollmentAccess string

const (
	EnrollmentAccessFull     EnrollmentAccess = "full"
	EnrollmentAccessReadOnly EnrollmentAccess = "read_only"
	EnrollmentAccessNone     EnrollmentAccess = "none"
)

func (a EnrollmentAccess) rank() int {
	switch a {
	case EnrollmentAccessFull:
		return 2
	case EnrollmentAccessReadOnly:
		return 1
	}
	return 0
}

// Enrollment type names.
const (
	TeacherEnrollmentType  = "TeacherEnrollment"
	TaEnrollmentType       = "TaEnrollment"
	DesignerEnrollmentType = "DesignerEnrollment"
	StudentEnrollmentType  = "StudentEnrollment"
	ObserverEnrollmentType = "ObserverEnrollment"
)

// IsStaffEnrollmentType reports whether the type is a course-staff type
// (Canvas's "admin" enrollments).
func IsStaffEnrollmentType(t string) bool {
	return t == TeacherEnrollmentType || t == TaEnrollmentType || t == DesignerEnrollmentType
}

// EnrollmentTypePrivilege ranks enrollment types, most privileged first
// (0 = teacher). Mirrors the repository's enrollmentPrivilegeOrder.
func EnrollmentTypePrivilege(t string) int {
	switch t {
	case TeacherEnrollmentType:
		return 0
	case TaEnrollmentType:
		return 1
	case DesignerEnrollmentType:
		return 2
	case StudentEnrollmentType:
		return 3
	case ObserverEnrollmentType:
		return 4
	}
	return 5
}

// DateRange is a participation window; a nil end is open.
type DateRange struct {
	Start *time.Time
	End   *time.Time
}

func (r DateRange) contains(now time.Time) bool {
	if r.Start != nil && now.Before(*r.Start) {
		return false
	}
	if r.End != nil && now.After(*r.End) {
		return false
	}
	return true
}

func termRange(term *EnrollmentTerm, enrollmentType string) (DateRange, bool) {
	if term == nil || term.WorkflowState == "deleted" {
		return DateRange{}, false
	}
	r := DateRange{Start: term.StartAt, End: term.EndAt}
	if IsStaffEnrollmentType(enrollmentType) {
		r.Start = nil // staff can prepare a course before its term starts
	}
	return r, true
}

// EnrollmentDateRanges returns the participation windows for an
// enrollment. section and term may be nil.
func EnrollmentDateRanges(e *Enrollment, section *CourseSection, course *Course, term *EnrollmentTerm) []DateRange {
	staff := IsStaffEnrollmentType(e.Type)
	var out []DateRange
	switch {
	case e.StartAt != nil && e.EndAt != nil:
		out = append(out, DateRange{Start: e.StartAt, End: e.EndAt})
	case section != nil && section.RestrictEnrollmentsToSectionDates:
		out = append(out, DateRange{Start: section.StartAt, End: section.EndAt})
		if tr, ok := termRange(term, e.Type); ok && staff {
			out = append(out, tr)
		}
	case course != nil && course.RestrictEnrollmentsToCourseDates:
		out = append(out, DateRange{Start: course.StartAt, End: course.EndAt})
		if tr, ok := termRange(term, e.Type); ok && staff {
			out = append(out, tr)
		}
	default:
		if tr, ok := termRange(term, e.Type); ok {
			out = append(out, tr)
		} else {
			out = append(out, DateRange{})
		}
	}
	return out
}

// EffectiveEnrollmentState applies the date windows (and a concluded
// course) to the enrollment's stored workflow_state.
func EffectiveEnrollmentState(e *Enrollment, section *CourseSection, course *Course, term *EnrollmentTerm, now time.Time) EnrollmentEffectiveState {
	wf := EnrollmentEffectiveState(e.WorkflowState)
	if wf == "" {
		wf = EffectiveActive
	}
	if wf != EffectiveActive && wf != EffectiveInvited {
		return wf
	}
	if course != nil && course.WorkflowState == CourseCompleted && wf == EffectiveActive {
		return EffectiveCompleted
	}

	ranges := EnrollmentDateRanges(e, section, course, term)
	var globalStart *time.Time
	for _, r := range ranges {
		if r.contains(now) {
			return wf
		}
		// Earliest date of each range (start, or end for an open start),
		// then the earliest across ranges — Canvas's global_start_at.
		earliest := r.Start
		if earliest == nil {
			earliest = r.End
		} else if r.End != nil && r.End.Before(*earliest) {
			earliest = r.End
		}
		if earliest != nil && (globalStart == nil || earliest.Before(*globalStart)) {
			globalStart = earliest
		}
	}
	if globalStart == nil {
		return wf
	}
	if globalStart.Before(now) {
		return EffectiveCompleted
	}
	if !IsStaffEnrollmentType(e.Type) {
		if wf == EffectiveActive {
			return EffectivePendingActive
		}
		return EffectivePendingInvited
	}
	return EffectiveInactive
}

// AccessForEffectiveState maps an effective state to what the user may do.
func AccessForEffectiveState(state EnrollmentEffectiveState, enrollmentType string, course *Course) EnrollmentAccess {
	learner := !IsStaffEnrollmentType(enrollmentType)
	switch state {
	case EffectiveActive:
		return EnrollmentAccessFull
	case EffectiveCompleted:
		if learner && course != nil && course.RestrictStudentPastView {
			return EnrollmentAccessNone
		}
		return EnrollmentAccessReadOnly
	case EffectivePendingActive:
		if course != nil && course.RestrictStudentFutureView {
			return EnrollmentAccessNone
		}
		return EnrollmentAccessReadOnly
	}
	return EnrollmentAccessNone
}

// CourseAccessData is everything the access path needs about one user's
// enrollments in one course: every enrollment that can still grant
// access (active, invited, completed, inactive), their sections, and the
// course's term.
type CourseAccessData struct {
	Enrollments []Enrollment
	Sections    map[uint]*CourseSection
	Term        *EnrollmentTerm
}

// CourseAccessDecision is the resolved access of one user to one course.
type CourseAccessDecision struct {
	// Enrollment is the enrollment the request acts under: the most
	// useful one (full > read-only) matching the requested role set,
	// most privileged first. Nil when no enrollment matched.
	Enrollment *Enrollment
	State      EnrollmentEffectiveState
	Access     EnrollmentAccess
	// RoleMismatch is set when the user holds a usable enrollment but
	// none of the requested types.
	RoleMismatch bool
	// SectionLimited is true when every usable enrollment is limited to
	// its section(s); VisibleSectionIDs then lists them (0 stands for
	// "no section", i.e. the course's default section).
	SectionLimited    bool
	VisibleSectionIDs []uint
	// UsableTypes lists the types of every enrollment granting access.
	UsableTypes []string
}

// ResolveCourseAccess picks the enrollment a request acts under. roleSet
// nil means any type. Permissions are the union of the user's
// enrollments: a TA in one section who is a student in another passes an
// instructor gate as the TA and a student gate as the student.
func ResolveCourseAccess(data *CourseAccessData, course *Course, roleSet map[string]bool, now time.Time) CourseAccessDecision {
	var dec CourseAccessDecision
	if data == nil {
		return dec
	}
	limitedAll := true
	anyUsable := false
	seenSection := map[uint]bool{}
	seenType := map[string]bool{}

	bestRank := -1
	for i := range data.Enrollments {
		e := &data.Enrollments[i]
		var section *CourseSection
		if e.CourseSectionID != nil && data.Sections != nil {
			section = data.Sections[*e.CourseSectionID]
		}
		state := EffectiveEnrollmentState(e, section, course, data.Term, now)
		access := AccessForEffectiveState(state, e.Type, course)
		if access == EnrollmentAccessNone {
			if dec.Enrollment == nil && bestRank < 0 {
				// Remember the state of an unusable match for the
				// error message ("not active" vs "not enrolled").
				if roleSet == nil || roleSet[e.Type] {
					dec.State = state
				}
			}
			continue
		}
		anyUsable = true
		if !seenType[e.Type] {
			seenType[e.Type] = true
			dec.UsableTypes = append(dec.UsableTypes, e.Type)
		}
		if e.LimitPrivilegesToCourseSection {
			sid := uint(0)
			if e.CourseSectionID != nil {
				sid = *e.CourseSectionID
			}
			if !seenSection[sid] {
				seenSection[sid] = true
				dec.VisibleSectionIDs = append(dec.VisibleSectionIDs, sid)
			}
		} else {
			limitedAll = false
		}

		if roleSet != nil && !roleSet[e.Type] {
			continue
		}
		// Higher access first, then higher privilege.
		rank := access.rank()*10 + (9 - EnrollmentTypePrivilege(e.Type))
		if rank > bestRank {
			bestRank = rank
			dec.Enrollment = e
			dec.State = state
			dec.Access = access
		}
	}
	if anyUsable && limitedAll {
		dec.SectionLimited = true
	} else {
		dec.VisibleSectionIDs = nil
	}
	if dec.Enrollment == nil {
		dec.Access = EnrollmentAccessNone
		dec.RoleMismatch = anyUsable
	}
	return dec
}
