package models

import (
	"testing"
	"time"
)

// Date-based access (G106), multi-enrollment union (G110) and section
// limits (G107). All data is made up.

var accessNow = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

func at(days int) *time.Time {
	t := accessNow.AddDate(0, 0, days)
	return &t
}

func uptr(v uint) *uint { return &v }

func TestEffectiveEnrollmentState_Dates(t *testing.T) {
	course := &Course{WorkflowState: CourseAvailable}
	student := func(start, end *time.Time) *Enrollment {
		return &Enrollment{Type: StudentEnrollmentType, WorkflowState: EnrollmentActive, StartAt: start, EndAt: end}
	}
	cases := []struct {
		name    string
		e       *Enrollment
		section *CourseSection
		course  *Course
		term    *EnrollmentTerm
		want    EnrollmentEffectiveState
	}{
		{"no dates anywhere", student(nil, nil), nil, course, nil, EffectiveActive},
		{"inside enrollment dates", student(at(-1), at(1)), nil, course, nil, EffectiveActive},
		{"after enrollment end", student(at(-10), at(-1)), nil, course, nil, EffectiveCompleted},
		{"before enrollment start", student(at(1), at(10)), nil, course, nil, EffectivePendingActive},
		{"only one enrollment date is ignored (needs both)", student(nil, at(-1)), nil, course, nil, EffectiveActive},
		{"term ended", student(nil, nil), nil, course, &EnrollmentTerm{StartAt: at(-100), EndAt: at(-1)}, EffectiveCompleted},
		{"term not started", student(nil, nil), nil, course, &EnrollmentTerm{StartAt: at(5), EndAt: at(100)}, EffectivePendingActive},
		{"deleted term ignored", student(nil, nil), nil, course, &EnrollmentTerm{StartAt: at(-100), EndAt: at(-1), WorkflowState: "deleted"}, EffectiveActive},
		{"unrestricted section dates ignored", student(nil, nil), &CourseSection{StartAt: at(-10), EndAt: at(-1)}, course, nil, EffectiveActive},
		{"restricted section ended", student(nil, nil), &CourseSection{StartAt: at(-10), EndAt: at(-1), RestrictEnrollmentsToSectionDates: true}, course, nil, EffectiveCompleted},
		{"restricted section overrides an open term", student(nil, nil), &CourseSection{StartAt: at(-10), EndAt: at(-1), RestrictEnrollmentsToSectionDates: true}, course, &EnrollmentTerm{StartAt: at(-100), EndAt: at(100)}, EffectiveCompleted},
		{"unrestricted course dates ignored", student(nil, nil), nil, &Course{WorkflowState: CourseAvailable, EndAt: at(-1)}, nil, EffectiveActive},
		{"restricted course ended", student(nil, nil), nil, &Course{WorkflowState: CourseAvailable, StartAt: at(-30), EndAt: at(-1), RestrictEnrollmentsToCourseDates: true}, nil, EffectiveCompleted},
		{"enrollment dates beat section dates", student(at(-1), at(1)), &CourseSection{EndAt: at(-1), RestrictEnrollmentsToSectionDates: true}, course, nil, EffectiveActive},
		{"concluded course", student(nil, nil), nil, &Course{WorkflowState: CourseCompleted}, nil, EffectiveCompleted},
		{"stored completed stays completed", &Enrollment{Type: StudentEnrollmentType, WorkflowState: EnrollmentCompleted}, nil, course, nil, EffectiveCompleted},
		{"inactive stays inactive", &Enrollment{Type: StudentEnrollmentType, WorkflowState: EnrollmentInactive}, nil, course, nil, EffectiveInactive},
		{"invited before start", &Enrollment{Type: StudentEnrollmentType, WorkflowState: EnrollmentInvited, StartAt: at(1), EndAt: at(5)}, nil, course, nil, EffectivePendingInvited},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EffectiveEnrollmentState(tc.e, tc.section, tc.course, tc.term, accessNow); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestEffectiveEnrollmentState_StaffIgnoreTermStart(t *testing.T) {
	course := &Course{WorkflowState: CourseAvailable}
	term := &EnrollmentTerm{StartAt: at(5), EndAt: at(100)}
	teacher := &Enrollment{Type: TeacherEnrollmentType, WorkflowState: EnrollmentActive}
	if got := EffectiveEnrollmentState(teacher, nil, course, term, accessNow); got != EffectiveActive {
		t.Fatalf("a teacher can prepare a course before its term starts; got %s", got)
	}
	// A teacher held back by restricted course dates is inactive (not
	// pending), unless the term window still covers today.
	restricted := &Course{WorkflowState: CourseAvailable, StartAt: at(5), EndAt: at(50), RestrictEnrollmentsToCourseDates: true}
	if got := EffectiveEnrollmentState(teacher, nil, restricted, nil, accessNow); got != EffectiveInactive {
		t.Fatalf("staff before restricted course dates: got %s, want inactive", got)
	}
	if got := EffectiveEnrollmentState(teacher, nil, restricted, term, accessNow); got != EffectiveActive {
		t.Fatalf("staff keep the term window alongside course dates: got %s", got)
	}
	// Term ended: staff are concluded too.
	ended := &EnrollmentTerm{StartAt: at(-100), EndAt: at(-1)}
	if got := EffectiveEnrollmentState(teacher, nil, course, ended, accessNow); got != EffectiveCompleted {
		t.Fatalf("staff after term end: got %s", got)
	}
}

func TestAccessForEffectiveState(t *testing.T) {
	open := &Course{}
	strict := &Course{RestrictStudentFutureView: true, RestrictStudentPastView: true}
	cases := []struct {
		state  EnrollmentEffectiveState
		typ    string
		course *Course
		want   EnrollmentAccess
	}{
		{EffectiveActive, StudentEnrollmentType, open, EnrollmentAccessFull},
		{EffectiveCompleted, StudentEnrollmentType, open, EnrollmentAccessReadOnly},
		{EffectiveCompleted, StudentEnrollmentType, strict, EnrollmentAccessNone},
		{EffectiveCompleted, TeacherEnrollmentType, strict, EnrollmentAccessReadOnly},
		{EffectivePendingActive, StudentEnrollmentType, open, EnrollmentAccessReadOnly},
		{EffectivePendingActive, StudentEnrollmentType, strict, EnrollmentAccessNone},
		{EffectivePendingInvited, StudentEnrollmentType, open, EnrollmentAccessNone},
		{EffectiveInvited, StudentEnrollmentType, open, EnrollmentAccessNone},
		{EffectiveInactive, TeacherEnrollmentType, open, EnrollmentAccessNone},
		{EffectiveDeleted, StudentEnrollmentType, open, EnrollmentAccessNone},
	}
	for _, tc := range cases {
		if got := AccessForEffectiveState(tc.state, tc.typ, tc.course); got != tc.want {
			t.Errorf("%s/%s: got %s, want %s", tc.state, tc.typ, got, tc.want)
		}
	}
}

func TestResolveCourseAccess_UnionOfEnrollments(t *testing.T) {
	course := &Course{WorkflowState: CourseAvailable}
	data := &CourseAccessData{Enrollments: []Enrollment{
		{ID: 1, Type: StudentEnrollmentType, WorkflowState: EnrollmentActive, CourseSectionID: uptr(10)},
		{ID: 2, Type: TaEnrollmentType, WorkflowState: EnrollmentActive, CourseSectionID: uptr(11)},
	}}

	any := ResolveCourseAccess(data, course, nil, accessNow)
	if any.Enrollment == nil || any.Enrollment.ID != 2 {
		t.Fatalf("any-role gate should act under the most privileged enrollment, got %+v", any.Enrollment)
	}
	instr := ResolveCourseAccess(data, course, map[string]bool{TeacherEnrollmentType: true, TaEnrollmentType: true}, accessNow)
	if instr.Enrollment == nil || instr.Enrollment.Type != TaEnrollmentType {
		t.Fatal("the TA enrollment must satisfy an instructor gate")
	}
	stud := ResolveCourseAccess(data, course, map[string]bool{StudentEnrollmentType: true}, accessNow)
	if stud.Enrollment == nil || stud.Enrollment.Type != StudentEnrollmentType {
		t.Fatal("the student enrollment must satisfy a student-only gate")
	}
	teacherOnly := ResolveCourseAccess(data, course, map[string]bool{TeacherEnrollmentType: true}, accessNow)
	if teacherOnly.Enrollment != nil || !teacherOnly.RoleMismatch {
		t.Fatal("no teacher enrollment: role mismatch")
	}
	if len(any.UsableTypes) != 2 {
		t.Fatalf("usable types: %v", any.UsableTypes)
	}
}

func TestResolveCourseAccess_PrefersFullOverReadOnly(t *testing.T) {
	course := &Course{WorkflowState: CourseAvailable}
	data := &CourseAccessData{Enrollments: []Enrollment{
		// Concluded teacher enrollment (more privileged, read-only)...
		{ID: 1, Type: TeacherEnrollmentType, WorkflowState: EnrollmentCompleted},
		// ...and an active student enrollment.
		{ID: 2, Type: StudentEnrollmentType, WorkflowState: EnrollmentActive},
	}}
	dec := ResolveCourseAccess(data, course, nil, accessNow)
	if dec.Enrollment.ID != 2 || dec.Access != EnrollmentAccessFull {
		t.Fatalf("full access beats a more privileged read-only enrollment, got id=%d access=%s", dec.Enrollment.ID, dec.Access)
	}
	instr := ResolveCourseAccess(data, course, map[string]bool{TeacherEnrollmentType: true}, accessNow)
	if instr.Enrollment.ID != 1 || instr.Access != EnrollmentAccessReadOnly {
		t.Fatal("an instructor gate falls back to the concluded (read-only) teacher enrollment")
	}
}

func TestResolveCourseAccess_SectionLimits(t *testing.T) {
	course := &Course{WorkflowState: CourseAvailable}
	limited := &CourseAccessData{Enrollments: []Enrollment{
		{ID: 1, Type: TeacherEnrollmentType, WorkflowState: EnrollmentActive, CourseSectionID: uptr(10), LimitPrivilegesToCourseSection: true},
		{ID: 2, Type: TeacherEnrollmentType, WorkflowState: EnrollmentActive, LimitPrivilegesToCourseSection: true},
	}}
	dec := ResolveCourseAccess(limited, course, nil, accessNow)
	if !dec.SectionLimited || len(dec.VisibleSectionIDs) != 2 {
		t.Fatalf("expected limit to sections [10 0], got limited=%v %v", dec.SectionLimited, dec.VisibleSectionIDs)
	}

	mixed := &CourseAccessData{Enrollments: []Enrollment{
		{ID: 1, Type: TeacherEnrollmentType, WorkflowState: EnrollmentActive, CourseSectionID: uptr(10), LimitPrivilegesToCourseSection: true},
		{ID: 2, Type: TaEnrollmentType, WorkflowState: EnrollmentActive, CourseSectionID: uptr(11)},
	}}
	if dec := ResolveCourseAccess(mixed, course, nil, accessNow); dec.SectionLimited {
		t.Fatal("one unlimited enrollment lifts the section limit")
	}

	// An unusable (deactivated) unlimited enrollment doesn't lift it.
	withInactive := &CourseAccessData{Enrollments: []Enrollment{
		{ID: 1, Type: TeacherEnrollmentType, WorkflowState: EnrollmentActive, CourseSectionID: uptr(10), LimitPrivilegesToCourseSection: true},
		{ID: 2, Type: TaEnrollmentType, WorkflowState: EnrollmentInactive, CourseSectionID: uptr(11)},
	}}
	if dec := ResolveCourseAccess(withInactive, course, nil, accessNow); !dec.SectionLimited {
		t.Fatal("an inactive enrollment grants nothing, so the limit stays")
	}
}

func TestResolveCourseAccess_NoUsableEnrollment(t *testing.T) {
	course := &Course{WorkflowState: CourseAvailable}
	data := &CourseAccessData{Enrollments: []Enrollment{
		{ID: 1, Type: StudentEnrollmentType, WorkflowState: EnrollmentActive, StartAt: at(3), EndAt: at(30)},
	}}
	strict := &Course{WorkflowState: CourseAvailable, RestrictStudentFutureView: true}
	dec := ResolveCourseAccess(data, strict, nil, accessNow)
	if dec.Enrollment != nil || dec.State != EffectivePendingActive {
		t.Fatalf("future view restricted: no access, state pending; got %+v", dec)
	}
	dec = ResolveCourseAccess(data, course, nil, accessNow)
	if dec.Access != EnrollmentAccessReadOnly {
		t.Fatalf("future view allowed: read-only, got %s", dec.Access)
	}
}
