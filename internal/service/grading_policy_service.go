package service

// GradingPolicyService is the course-level grading policy surface:
// server-computed course grades (G020/G021/G024/G025/G029), effective due
// dates (G019/G040), final grade overrides (G029), course post policy and
// post/hide by section or graded-only (G018), due-date driven late /
// missing refresh and the missing-submission policy job (G022), and
// submission history (G028).

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type GradingPolicyService struct {
	repo        repository.GradingPolicyRepository
	standards   repository.GradingStandardRepository
	submissions *SubmissionService
	now         func() time.Time
}

func NewGradingPolicyService(repo repository.GradingPolicyRepository, standards repository.GradingStandardRepository, submissions *SubmissionService) *GradingPolicyService {
	return &GradingPolicyService{repo: repo, standards: standards, submissions: submissions, now: time.Now}
}

// Repo exposes the store (handlers use it for history reads).
func (s *GradingPolicyService) Repo() repository.GradingPolicyRepository { return s.repo }

// CoursePeriods implements CoursePeriodResolver.
func (s *GradingPolicyService) CoursePeriods(ctx context.Context, courseID uint) ([]models.GradingPeriod, bool, error) {
	course, err := s.repo.FindCourse(ctx, courseID)
	if err != nil {
		return nil, false, err
	}
	group, periods, err := s.repo.CourseGradingPeriods(ctx, course)
	if err != nil || group == nil {
		return nil, false, err
	}
	return periods, group.Weighted, nil
}

// courseData is everything loaded once per course request.
type courseData struct {
	course      *models.Course
	assignments []models.Assignment
	groups      []models.AssignmentGroup
	students    []models.Enrollment
	studentIDs  []uint
	sections    map[uint][]uint
	submissions map[uint]map[uint]*models.Submission // user -> assignment -> sub
	periodGroup *models.GradingPeriodGroup
	periods     []models.GradingPeriod
	dueDates    DueDateMap
}

func (s *GradingPolicyService) load(ctx context.Context, course *models.Course, onlyUsers []uint, withSubmissions bool) (*courseData, error) {
	d := &courseData{course: course, sections: map[uint][]uint{}, submissions: map[uint]map[uint]*models.Submission{}}
	var err error
	if d.assignments, err = s.repo.ListCourseAssignments(ctx, course.ID); err != nil {
		return nil, err
	}
	if d.groups, err = s.repo.ListCourseGroups(ctx, course.ID); err != nil {
		return nil, err
	}
	enrollments, err := s.repo.ListCourseStudents(ctx, course.ID)
	if err != nil {
		return nil, err
	}
	want := map[uint]bool{}
	for _, id := range onlyUsers {
		want[id] = true
	}
	seen := map[uint]bool{}
	for _, e := range enrollments {
		if len(want) > 0 && !want[e.UserID] {
			continue
		}
		if e.CourseSectionID != nil {
			d.sections[e.UserID] = append(d.sections[e.UserID], *e.CourseSectionID)
		}
		if seen[e.UserID] {
			continue
		}
		seen[e.UserID] = true
		d.students = append(d.students, e)
		d.studentIDs = append(d.studentIDs, e.UserID)
	}
	if d.periodGroup, d.periods, err = s.repo.CourseGradingPeriods(ctx, course); err != nil {
		return nil, err
	}
	overrides, err := s.repo.ListCourseOverrides(ctx, course.ID)
	if err != nil {
		return nil, err
	}
	overrideStudents, err := s.repo.ListCourseOverrideStudents(ctx, course.ID)
	if err != nil {
		return nil, err
	}
	accs, err := s.repo.ListCourseAccommodations(ctx, course.ID, d.studentIDs)
	if err != nil {
		return nil, err
	}
	d.dueDates = ResolveDueDates(DueDateInput{
		Assignments: d.assignments, StudentIDs: d.studentIDs, StudentSections: d.sections,
		Overrides: overrides, OverrideStudents: overrideStudents,
		ExtraDays: AccommodationExtraDays(accs), Periods: d.periods, Now: s.now(),
	})
	if withSubmissions && len(d.studentIDs) > 0 {
		subs, err := s.repo.ListCourseSubmissions(ctx, course.ID, d.studentIDs)
		if err != nil {
			return nil, err
		}
		for i := range subs {
			sub := &subs[i]
			if d.submissions[sub.UserID] == nil {
				d.submissions[sub.UserID] = map[uint]*models.Submission{}
			}
			d.submissions[sub.UserID][sub.AssignmentID] = sub
		}
	}
	return d, nil
}

// GradingPeriodInfo describes one grading period of the course's set.
type GradingPeriodInfo struct {
	ID        uint       `json:"id"`
	Title     string     `json:"title"`
	StartDate time.Time  `json:"start_date"`
	EndDate   time.Time  `json:"end_date"`
	CloseDate *time.Time `json:"close_date"`
	Weight    *float64   `json:"weight"`
	IsClosed  bool       `json:"is_closed"`
}

// SchemeInfo is a grading scheme as the UI consumes it.
type SchemeInfo struct {
	ID            uint            `json:"id,omitempty"`
	Title         string          `json:"title"`
	Data          [][]interface{} `json:"data"`
	PointsBased   bool            `json:"points_based"`
	ScalingFactor float64         `json:"scaling_factor"`
}

// SchemeInfoFor is the JSON shape of a resolved scheme.
func SchemeInfoFor(s *GradingScheme) *SchemeInfo { return schemeInfo(s) }

func schemeInfo(s *GradingScheme) *SchemeInfo {
	if s == nil {
		return nil
	}
	info := &SchemeInfo{ID: s.ID, Title: s.Title, PointsBased: s.PointsBased, ScalingFactor: s.ScalingFactor}
	for _, e := range s.Entries {
		info.Data = append(info.Data, []interface{}{e.Name, e.Value})
	}
	return info
}

// StudentCourseGrade is one student's computed grade plus overrides.
type StudentCourseGrade struct {
	UserID       uint   `json:"user_id"`
	Name         string `json:"name"`
	SortableName string `json:"sortable_name"`
	CourseGradeResult
	CurrentGrade string `json:"current_grade,omitempty"`
	FinalGrade   string `json:"final_grade,omitempty"`
	// OverrideScore is the course-level final grade override.
	OverrideScore *float64 `json:"override_score"`
	OverrideGrade string   `json:"override_grade,omitempty"`
	// PeriodOverrides maps grading period id -> override score.
	PeriodOverrides map[uint]float64 `json:"period_overrides,omitempty"`
	// Unassigned lists assignments not assigned to this student.
	Unassigned []uint `json:"unassigned_assignment_ids"`
}

// CourseGrades is the course grade summary.
type CourseGrades struct {
	CourseID                uint                 `json:"course_id"`
	ApplyGroupWeights       bool                 `json:"apply_assignment_group_weights"`
	GradingPeriods          []GradingPeriodInfo  `json:"grading_periods"`
	WeightedGradingPeriods  bool                 `json:"weighted_grading_periods"`
	DisplayTotalsAllPeriods bool                 `json:"display_totals_for_all_grading_periods"`
	GradingScheme           *SchemeInfo          `json:"grading_scheme"`
	Students                []StudentCourseGrade `json:"students"`
}

// CourseGradesOptions narrows the summary.
type CourseGradesOptions struct {
	UserIDs []uint
	// PostedOnly hides unposted grades (the student / observer view).
	PostedOnly bool
}

// CourseScheme returns the scheme for course totals; nil when the course
// turned its grading scheme off.
func (s *GradingPolicyService) CourseScheme(ctx context.Context, course *models.Course) *GradingScheme {
	if course == nil || course.GradingStandardDisabled {
		return nil
	}
	return ResolveGradingScheme(ctx, s.standards, course, nil)
}

// CourseGrades computes every (or the selected) student's course grade.
func (s *GradingPolicyService) CourseGrades(ctx context.Context, course *models.Course, opts CourseGradesOptions) (*CourseGrades, error) {
	d, err := s.load(ctx, course, opts.UserIDs, true)
	if err != nil {
		return nil, err
	}
	scheme := s.CourseScheme(ctx, course)
	out := &CourseGrades{CourseID: course.ID, ApplyGroupWeights: course.ApplyGroupWeights, GradingScheme: schemeInfo(scheme), Students: []StudentCourseGrade{}}
	now := s.now()
	for _, p := range d.periods {
		out.GradingPeriods = append(out.GradingPeriods, GradingPeriodInfo{ID: p.ID, Title: p.Title, StartDate: p.StartDate, EndDate: p.EndDate,
			CloseDate: p.CloseDate, Weight: p.Weight, IsClosed: GradingPeriodClosed(&p, now)})
	}
	if d.periodGroup != nil {
		out.WeightedGradingPeriods = d.periodGroup.Weighted
		out.DisplayTotalsAllPeriods = d.periodGroup.DisplayTotals
	}

	overrides, err := s.repo.ListScoreOverrides(ctx, course.ID)
	if err != nil {
		return nil, err
	}
	overrideBy := map[uint]map[uint]float64{}
	for _, o := range overrides {
		if overrideBy[o.UserID] == nil {
			overrideBy[o.UserID] = map[uint]float64{}
		}
		overrideBy[o.UserID][o.GradingPeriodID] = o.OverrideScore
	}

	for _, e := range d.students {
		uid := e.UserID
		subs := d.submissions[uid]
		if opts.PostedOnly {
			subs = postedView(subs)
		}
		unassigned := map[uint]bool{}
		periodOf := map[uint]uint{}
		var unassignedList []uint
		for _, a := range d.assignments {
			dd, ok := d.dueDates[a.ID][uid]
			if !ok {
				unassigned[a.ID] = true
				unassignedList = append(unassignedList, a.ID)
				continue
			}
			periodOf[a.ID] = dd.GradingPeriodID
		}
		res := CalculateCourseGrade(CourseGradeInput{
			Assignments: d.assignments, Groups: d.groups, Submissions: subs,
			ApplyGroupWeights: course.ApplyGroupWeights, Unassigned: unassigned,
			PeriodOf: periodOf, Periods: d.periods, WeightedPeriods: out.WeightedGradingPeriods,
		})
		if opts.PostedOnly && len(d.periods) > 0 && !out.DisplayTotalsAllPeriods {
			// G025: with "Display totals for All Grading Periods" off,
			// students see per-period grades only (Canvas).
			res.ScoreSet = ScoreSet{Groups: res.Groups}
		}
		sg := StudentCourseGrade{UserID: uid, CourseGradeResult: res, Unassigned: unassignedList}
		if sg.Unassigned == nil {
			sg.Unassigned = []uint{}
		}
		if e.User != nil {
			sg.Name = e.User.Name
			sg.SortableName = e.User.Name
		}
		if scheme != nil {
			if res.CurrentScore != nil {
				sg.CurrentGrade = scheme.ScoreToGrade(*res.CurrentScore)
			}
			if res.FinalScore != nil {
				sg.FinalGrade = scheme.ScoreToGrade(*res.FinalScore)
			}
		}
		if ov, ok := overrideBy[uid]; ok {
			if v, ok := ov[0]; ok {
				vv := v
				sg.OverrideScore = &vv
				if scheme != nil {
					sg.OverrideGrade = scheme.ScoreToGrade(v)
				}
			}
			for pid, v := range ov {
				if pid == 0 {
					continue
				}
				if sg.PeriodOverrides == nil {
					sg.PeriodOverrides = map[uint]float64{}
				}
				sg.PeriodOverrides[pid] = v
			}
		}
		out.Students = append(out.Students, sg)
	}
	sort.SliceStable(out.Students, func(i, j int) bool { return out.Students[i].SortableName < out.Students[j].SortableName })
	return out, nil
}

// postedView is the student's view of their submissions: an unposted
// grade (or excusal) does not exist yet.
func postedView(subs map[uint]*models.Submission) map[uint]*models.Submission {
	out := make(map[uint]*models.Submission, len(subs))
	for aid, sub := range subs {
		if sub.PostedAt != nil {
			out[aid] = sub
			continue
		}
		cp := *sub
		cp.Score = nil
		cp.Grade = nil
		cp.Excused = false
		out[aid] = &cp
	}
	return out
}

// DueDates returns the course's effective due dates for every student
// (or the given ones).
func (s *GradingPolicyService) DueDates(ctx context.Context, course *models.Course, userIDs []uint) (DueDateMap, error) {
	d, err := s.load(ctx, course, userIDs, false)
	if err != nil {
		return nil, err
	}
	return d.dueDates, nil
}

// StudentAssignmentDates returns one student's effective dates for every
// assignment of the course (unassigned assignments are absent).
func (s *GradingPolicyService) StudentAssignmentDates(ctx context.Context, course *models.Course, userID uint) (map[uint]EffectiveDueDate, error) {
	dd, err := s.DueDates(ctx, course, []uint{userID})
	if err != nil {
		return nil, err
	}
	out := map[uint]EffectiveDueDate{}
	for aid, per := range dd {
		if d, ok := per[userID]; ok {
			out[aid] = d
		}
	}
	return out, nil
}

// ErrNotCourseStudent is returned when an override targets a user who is
// not a student of the course.
var ErrNotCourseStudent = errors.New("user is not a student in this course")

// ErrUnknownGradingPeriod is returned for a grading period outside the
// course's grading period set.
var ErrUnknownGradingPeriod = errors.New("grading period not found")

// SetScoreOverride sets (score != nil) or clears a student's final grade
// override for the course (gradingPeriodID 0) or one grading period.
func (s *GradingPolicyService) SetScoreOverride(ctx context.Context, course *models.Course, userID, gradingPeriodID, actorID uint, score *float64) (*models.CourseScoreOverride, error) {
	d, err := s.load(ctx, course, []uint{userID}, false)
	if err != nil {
		return nil, err
	}
	if len(d.studentIDs) == 0 {
		return nil, ErrNotCourseStudent
	}
	if gradingPeriodID != 0 {
		found := false
		for _, p := range d.periods {
			if p.ID == gradingPeriodID {
				found = true
			}
		}
		if !found {
			return nil, ErrUnknownGradingPeriod
		}
	}
	if score == nil {
		return nil, s.repo.DeleteScoreOverride(ctx, course.ID, userID, gradingPeriodID)
	}
	if math.IsNaN(*score) || math.IsInf(*score, 0) || *score < 0 || *score > 1000 {
		return nil, fmt.Errorf("override score must be a percentage between 0 and 1000")
	}
	o := &models.CourseScoreOverride{CourseID: course.ID, UserID: userID, GradingPeriodID: gradingPeriodID,
		OverrideScore: math.Round(*score*100) / 100, UpdatedByID: actorID}
	if err := s.repo.UpsertScoreOverride(ctx, o); err != nil {
		return nil, err
	}
	return o, nil
}

// SetCoursePostPolicy sets the course default post policy and applies it
// to the course's (non-anonymous) assignments, as Canvas does.
func (s *GradingPolicyService) SetCoursePostPolicy(ctx context.Context, course *models.Course, postManually bool) error {
	if err := s.repo.SetCoursePostManually(ctx, course.ID, postManually); err != nil {
		return err
	}
	policy := "automatic"
	if postManually {
		policy = "manual"
	}
	course.PostManually = postManually
	return s.repo.SetAssignmentsPostPolicy(ctx, course.ID, policy)
}

// PostOptions narrows Post / Hide grades.
type PostOptions struct {
	// GradedOnly posts only graded or excused submissions ("Graded" in
	// Canvas); otherwise every assigned student's submission is posted
	// ("Everyone"), so grades entered later show at once.
	GradedOnly bool
	// SectionIDs limits the action to students in these sections.
	SectionIDs []uint
}

// PostGrades posts the assignment's grades per opts and returns the
// submissions whose grades became visible (for notifications).
func (s *GradingPolicyService) PostGrades(ctx context.Context, course *models.Course, assignment *models.Assignment, opts PostOptions) ([]models.Submission, error) {
	return s.setPosted(ctx, course, assignment, opts, true)
}

// HideGrades hides the assignment's posted grades (optionally for some
// sections only).
func (s *GradingPolicyService) HideGrades(ctx context.Context, course *models.Course, assignment *models.Assignment, opts PostOptions) error {
	_, err := s.setPosted(ctx, course, assignment, opts, false)
	return err
}

func (s *GradingPolicyService) setPosted(ctx context.Context, course *models.Course, assignment *models.Assignment, opts PostOptions, post bool) ([]models.Submission, error) {
	d, err := s.load(ctx, course, nil, false)
	if err != nil {
		return nil, err
	}
	inScope := map[uint]bool{}
	sectionFilter := map[uint]bool{}
	for _, sid := range opts.SectionIDs {
		sectionFilter[sid] = true
	}
	for _, uid := range d.studentIDs {
		if _, assigned := d.dueDates[assignment.ID][uid]; !assigned {
			continue
		}
		if len(sectionFilter) > 0 {
			match := false
			for _, sid := range d.sections[uid] {
				if sectionFilter[sid] {
					match = true
				}
			}
			if !match {
				continue
			}
		}
		inScope[uid] = true
	}
	subs, err := s.repo.ListAssignmentSubmissions(ctx, assignment.ID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	have := map[uint]bool{}
	var ids []uint
	var released []models.Submission
	for _, sub := range subs {
		have[sub.UserID] = true
		if !inScope[sub.UserID] {
			continue
		}
		if !post {
			if sub.PostedAt != nil {
				ids = append(ids, sub.ID)
			}
			continue
		}
		gradedOrExcused := sub.Score != nil || sub.Excused
		if sub.PostedAt != nil || (opts.GradedOnly && !gradedOrExcused) {
			continue
		}
		ids = append(ids, sub.ID)
		if gradedOrExcused {
			sub.PostedAt = &now
			released = append(released, sub)
		}
	}
	var postedAt *time.Time
	if post {
		postedAt = &now
	}
	if err := s.repo.SetSubmissionsPostedAt(ctx, assignment.ID, ids, postedAt); err != nil {
		return nil, err
	}
	if post && !opts.GradedOnly {
		// "Everyone": students without a submission row get a posted,
		// unsubmitted one so a later grade is visible immediately.
		for uid := range inScope {
			if have[uid] {
				continue
			}
			at := now
			row := &models.Submission{AssignmentID: assignment.ID, UserID: uid, WorkflowState: models.SubmissionUnsubmitted, PostedAt: &at}
			if dd, ok := d.dueDates[assignment.ID][uid]; ok {
				row.CachedDueDate = dd.DueAt
				t := ComputeTardiness(assignment, row, dd.DueAt, now)
				row.Missing = t.Missing
			}
			_ = s.repo.CreateSubmission(ctx, row)
		}
	}
	return released, nil
}

// RefreshDueStatus recomputes cached_due_date, late and missing on the
// existing submissions of the given assignments (all when empty) after
// due dates, overrides or accommodations change.
func (s *GradingPolicyService) RefreshDueStatus(ctx context.Context, courseID uint, assignmentIDs []uint) error {
	course, err := s.repo.FindCourse(ctx, courseID)
	if err != nil {
		return err
	}
	_, err = s.refresh(ctx, course, assignmentIDs, false)
	return err
}

// refresh is RefreshDueStatus; with applyMissing it also creates missing
// rows for assigned students past due without a submission and applies
// the course missing policy. Returns how many rows changed.
func (s *GradingPolicyService) refresh(ctx context.Context, course *models.Course, assignmentIDs []uint, applyMissing bool) (int, error) {
	d, err := s.load(ctx, course, nil, false)
	if err != nil {
		return 0, err
	}
	want := map[uint]bool{}
	for _, id := range assignmentIDs {
		want[id] = true
	}
	// The late policy re-derives late deductions after date changes and
	// drives the missing policy.
	policy, err := s.repo.FindLatePolicy(ctx, course.ID)
	if err != nil {
		return 0, err
	}
	now := s.now()
	changed := 0
	for i := range d.assignments {
		a := &d.assignments[i]
		if len(want) > 0 && !want[a.ID] {
			continue
		}
		// The scheme is per assignment (an assignment may pick its own).
		var scheme *GradingScheme
		schemeFor := func() *GradingScheme {
			if scheme == nil {
				scheme = ResolveGradingScheme(ctx, s.standards, course, a)
			}
			return scheme
		}
		subs, err := s.repo.ListAssignmentSubmissions(ctx, a.ID)
		if err != nil {
			return changed, err
		}
		have := map[uint]bool{}
		for j := range subs {
			sub := &subs[j]
			have[sub.UserID] = true
			dd, assigned := d.dueDates[a.ID][sub.UserID]
			due := dd.DueAt
			if !assigned {
				due = a.DueAt
			}
			t := ComputeTardiness(a, sub, due, now)
			if !assigned {
				t.Missing = false
			}
			if !sameTime(sub.CachedDueDate, due) || sub.Late != t.Late || sub.Missing != t.Missing {
				sub.CachedDueDate, sub.Late, sub.Missing = due, t.Late, t.Missing
				if err := s.repo.UpdateSubmissionStatus(ctx, sub); err != nil {
					return changed, err
				}
				changed++
			}
			// G022: a graded submission's late deduction follows its
			// effective due date (and the current late policy), as Canvas
			// re-applies the late policy when due dates change. Closed
			// grading periods are left alone.
			if !inClosedPeriod(dd) && s.reapplyLateDeduction(ctx, a, sub, policy, due, schemeFor) {
				changed++
			}
			if applyMissing && t.Missing && sub.Score == nil && a.IsPublished() {
				if points, ok := MissingPoints(policy, a); ok && !inClosedPeriod(dd) {
					s.applyMissingRow(ctx, a, sub, schemeFor(), points, now)
					changed++
				}
			}
		}
		if !applyMissing || !a.IsPublished() || !ExpectsSubmission(a) {
			continue
		}
		for _, uid := range d.studentIDs {
			if have[uid] {
				continue
			}
			dd, assigned := d.dueDates[a.ID][uid]
			if !assigned || dd.DueAt == nil || !now.After(*dd.DueAt) || inClosedPeriod(dd) {
				continue
			}
			row := &models.Submission{AssignmentID: a.ID, UserID: uid, WorkflowState: models.SubmissionUnsubmitted,
				CachedDueDate: dd.DueAt, Missing: true}
			scored := false
			if points, ok := MissingPoints(policy, a); ok {
				applyMissingScore(a, schemeFor(), row, points, now)
				scored = true
			}
			if err := s.repo.CreateSubmission(ctx, row); err != nil {
				return changed, err
			}
			changed++
			if row.ID != 0 && s.submissions != nil {
				if scored {
					// Missing-policy scores are grade changes too (G028).
					s.submissions.logGradeChange(ctx, a, row, 0, nil, nil)
				}
				if row.PostedAt != nil {
					s.submissions.FirePosted(row.ID)
				}
			}
		}
	}
	return changed, nil
}

func inClosedPeriod(dd EffectiveDueDate) bool { return dd.InClosedGradingPeriod }

// reapplyLateDeduction recomputes a graded submission's late deduction
// from its entered score against dueAt and the course late policy, and
// saves the new score when it changed. Missing-policy scores (no grader)
// and excused work are left alone. Returns whether the row changed.
func (s *GradingPolicyService) reapplyLateDeduction(ctx context.Context, a *models.Assignment, sub *models.Submission, policy *models.LatePolicy,
	dueAt *time.Time, schemeFor func() *GradingScheme) bool {
	if s.submissions == nil || sub.Excused || sub.EnteredScore == nil || sub.Score == nil || sub.GraderID == nil {
		return false
	}
	deducted := LateDeductionFor(policy, a, sub, *sub.EnteredScore, dueAt)
	current := 0.0
	if sub.PointsDeducted != nil {
		current = *sub.PointsDeducted
	}
	if math.Abs(deducted-current) < 1e-9 {
		return false
	}
	oldGrade, oldScore := sub.Grade, sub.Score
	final := *sub.EnteredScore - deducted
	grade := FormatGrade(a, schemeFor(), final, "")
	sub.Score = &final
	sub.Grade = &grade
	if deducted > 0 {
		d := deducted
		sub.PointsDeducted = &d
	} else {
		sub.PointsDeducted = nil
	}
	if err := s.submissions.submissionRepo.Update(ctx, sub); err != nil {
		return false
	}
	s.submissions.recordAttemptGrade(ctx, sub)
	s.submissions.logGradeChange(ctx, a, sub, 0, oldGrade, oldScore)
	s.submissions.fireIfPosted(sub)
	return true
}

// applyMissingRow scores an existing missing submission per the policy.
func (s *GradingPolicyService) applyMissingRow(ctx context.Context, a *models.Assignment, sub *models.Submission, scheme *GradingScheme, points float64, now time.Time) {
	if s.submissions == nil {
		return
	}
	applyMissingScore(a, scheme, sub, points, now)
	if err := s.submissions.submissionRepo.Update(ctx, sub); err == nil {
		s.submissions.logGradeChange(ctx, a, sub, 0, nil, nil)
		s.submissions.fireIfPosted(sub)
	}
}

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

// ApplyMissingPolicy is the periodic job: for assignments due in the last
// lookback window it marks past-due unsubmitted work missing (creating
// the row when needed) and applies each course's missing-submission
// policy. Returns how many rows changed.
func (s *GradingPolicyService) ApplyMissingPolicy(ctx context.Context, lookback time.Duration) (int, error) {
	now := s.now()
	assignments, err := s.repo.ListAssignmentsDueBetween(ctx, now.Add(-lookback), now)
	if err != nil {
		return 0, err
	}
	byCourse := map[uint][]uint{}
	var order []uint
	for _, a := range assignments {
		if _, ok := byCourse[a.CourseID]; !ok {
			order = append(order, a.CourseID)
		}
		byCourse[a.CourseID] = append(byCourse[a.CourseID], a.ID)
	}
	total := 0
	for _, cid := range order {
		course, err := s.repo.FindCourse(ctx, cid)
		if err != nil {
			continue
		}
		n, err := s.refresh(ctx, course, byCourse[cid], true)
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// ApplyCourseMissingPolicy re-evaluates every assignment of one course
// now (e.g. right after its late policy changed).
func (s *GradingPolicyService) ApplyCourseMissingPolicy(ctx context.Context, course *models.Course) (int, error) {
	return s.refresh(ctx, course, nil, true)
}

// SubmissionHistory returns a submission's attempts, oldest first.
func (s *GradingPolicyService) SubmissionHistory(ctx context.Context, submissionID uint) ([]models.SubmissionVersion, error) {
	return s.repo.ListSubmissionVersions(ctx, submissionID)
}

// Submissions exposes the submission service (late-status writes).
func (s *GradingPolicyService) Submissions() *SubmissionService { return s.submissions }

// IsCourseStudent reports whether userID holds an active student
// enrollment in the course.
func (s *GradingPolicyService) IsCourseStudent(ctx context.Context, courseID, userID uint) (bool, error) {
	enrollments, err := s.repo.ListCourseStudents(ctx, courseID)
	if err != nil {
		return false, err
	}
	for _, e := range enrollments {
		if e.UserID == userID {
			return true, nil
		}
	}
	return false, nil
}

// CoursePostManually returns the course's default post policy (false
// when the course cannot be loaded).
func (s *GradingPolicyService) CoursePostManually(ctx context.Context, courseID uint) bool {
	course, err := s.repo.FindCourse(ctx, courseID)
	return err == nil && course != nil && course.PostManually
}

// SchemeUsableByCourse reports whether grading standard id may be used by
// the course (its own schemes and its account's, not deleted).
func (s *GradingPolicyService) SchemeUsableByCourse(ctx context.Context, courseID, standardID uint) bool {
	if s.standards == nil || standardID == 0 {
		return false
	}
	course, err := s.repo.FindCourse(ctx, courseID)
	if err != nil || course == nil {
		return false
	}
	gs, err := s.standards.FindByID(ctx, standardID)
	return err == nil && gs != nil && gs.WorkflowState != "deleted" && StandardVisibleToCourse(gs, course)
}

// RefreshUserDueStatus recomputes late / missing on a student's
// submissions after their accommodations change: in courseID when set,
// otherwise in every course they are a student of.
func (s *GradingPolicyService) RefreshUserDueStatus(ctx context.Context, userID uint, courseID *uint) error {
	var courses []uint
	if courseID != nil && *courseID != 0 {
		courses = []uint{*courseID}
	} else {
		ids, err := s.repo.ListUserStudentCourseIDs(ctx, userID)
		if err != nil {
			return err
		}
		courses = ids
	}
	for _, cid := range courses {
		if err := s.RefreshDueStatus(ctx, cid, nil); err != nil {
			return err
		}
	}
	return nil
}

// ValidGradingType reports whether t is a supported assignment grading
// type (Canvas: points, percent, letter_grade, gpa_scale, pass_fail,
// not_graded).
func ValidGradingType(t string) bool {
	switch t {
	case "points", "percent", "letter_grade", "gpa_scale", "pass_fail", "not_graded":
		return true
	}
	return false
}
