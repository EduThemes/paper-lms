package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// GradeSummaryService builds the student Grades page ("grade summary"):
// every assignment assigned to the student with their posted result,
// status badges, feedback comments and rubric assessment, plus course /
// group / grading period totals. The numbers come from the gradebook's
// engine (GradingPolicyService.CourseGrades): the same drop rules, group
// weights, grading periods, assign-to visibility, effective due dates
// (overrides and accommodations) and final grade overrides the teacher
// sees, limited to posted grades for students and observers.
type GradeSummaryService struct {
	learner     repository.LearnerRepository
	enrollments repository.EnrollmentRepository
	policy      *GradingPolicyService
}

func NewGradeSummaryService(learner repository.LearnerRepository, enrollments repository.EnrollmentRepository, policy *GradingPolicyService) *GradeSummaryService {
	return &GradeSummaryService{learner: learner, enrollments: enrollments, policy: policy}
}

// --- Summary types ---

// GradeSummaryGroup is one assignment group with its totals.
type GradeSummaryGroup struct {
	ID       uint      `json:"id"`
	Name     string    `json:"name"`
	Weight   float64   `json:"group_weight"`
	Rules    DropRules `json:"rules"`
	Earned   float64    `json:"earned"`
	Possible float64    `json:"possible"`
	// Score is the group percentage over graded work (nil: nothing graded).
	Score *float64 `json:"score"`
}

// GradeSummaryComment is a feedback comment visible to the student.
type GradeSummaryComment struct {
	ID         uint      `json:"id"`
	AuthorID   uint      `json:"author_id"`
	AuthorName string    `json:"author_name"`
	Comment    string    `json:"comment"`
	CreatedAt  time.Time `json:"created_at"`
}

// GradeSummaryRubric is the posted rubric assessment for an assignment.
type GradeSummaryRubric struct {
	Title          string          `json:"title"`
	Criteria       json.RawMessage `json:"criteria"`
	Assessment     json.RawMessage `json:"assessment"`
	Score          *float64        `json:"score"`
	PointsPossible float64         `json:"points_possible"`
	HidePoints     bool            `json:"hide_points"`
	HideScoreTotal bool            `json:"hide_score_total"`
}

// GradeSummaryAssignment is one row of the Grades page.
type GradeSummaryAssignment struct {
	ID                uint       `json:"id"`
	Name              string     `json:"name"`
	AssignmentGroupID *uint      `json:"assignment_group_id"`
	DueAt             *time.Time `json:"due_at"`
	PointsPossible    *float64   `json:"points_possible"`
	GradingType       string     `json:"grading_type"`
	GradingPeriodID   *uint      `json:"grading_period_id"`
	Score             *float64   `json:"score"`
	Grade             *string    `json:"grade"`
	PointsDeducted    *float64   `json:"points_deducted"`
	SubmittedAt       *time.Time `json:"submitted_at"`
	GradedAt          *time.Time `json:"graded_at"`
	Posted            bool       `json:"posted"`
	Status            string     `json:"status"` // graded | pending_review | submitted | missing | late | excused | not_posted | unsubmitted
	Late              bool       `json:"late"`
	Missing           bool       `json:"missing"`
	Excused           bool       `json:"excused"`
	Dropped           bool       `json:"dropped"`
	CountsTowardTotal bool       `json:"counts_toward_total"`
	// PendingReview: a quiz awaiting manual grading; its partial score is
	// shown but does not count toward the current grade yet.
	PendingReview bool `json:"pending_review"`
	// OmitFromFinalGrade: never counts, even with a What-If score.
	OmitFromFinalGrade bool                  `json:"omit_from_final_grade"`
	Comments           []GradeSummaryComment `json:"comments"`
	Rubric             *GradeSummaryRubric   `json:"rubric,omitempty"`
}

// GradeSummaryTotals is the course total.
type GradeSummaryTotals struct {
	CurrentScore *float64 `json:"current_score"`
	CurrentGrade string   `json:"current_grade"`
	FinalScore   *float64 `json:"final_score"`
	FinalGrade   string   `json:"final_grade"`
	Earned       float64  `json:"earned"`
	Possible     float64  `json:"possible"`
	// OverrideScore / OverrideGrade: the instructor's final grade
	// override for this scope (the course, or the selected period).
	OverrideScore *float64 `json:"override_score"`
	OverrideGrade string   `json:"override_grade,omitempty"`
}

// GradeSummaryPeriod is a grading period the student can filter by.
type GradeSummaryPeriod struct {
	ID        uint       `json:"id"`
	Title     string     `json:"title"`
	StartDate time.Time  `json:"start_date"`
	EndDate   time.Time  `json:"end_date"`
	CloseDate *time.Time `json:"close_date"`
	IsClosed  bool       `json:"is_closed"`
	Weight    *float64   `json:"weight"`
	// OverrideScore / OverrideGrade: the period's final grade override.
	OverrideScore *float64 `json:"override_score"`
	OverrideGrade string   `json:"override_grade,omitempty"`
}

// GradeSummary is the whole Grades page payload.
type GradeSummary struct {
	CourseID          uint                     `json:"course_id"`
	CourseName        string                   `json:"course_name"`
	StudentID         uint                     `json:"student_id"`
	StudentName       string                   `json:"student_name"`
	ApplyGroupWeights bool                     `json:"apply_assignment_group_weights"`
	HideTotals        bool                     `json:"hide_totals"`
	GradingScale      []GradeEntry             `json:"grading_scale"`
	GradingPeriods    []GradeSummaryPeriod     `json:"grading_periods"`
	SelectedPeriodID  *uint                    `json:"grading_period_id"`
	PeriodsWeighted   bool                     `json:"grading_periods_weighted"`
	Groups            []GradeSummaryGroup      `json:"assignment_groups"`
	Assignments       []GradeSummaryAssignment `json:"assignments"`
	Totals            *GradeSummaryTotals      `json:"totals"`
}

// GradeSummaryViewer describes who asks: staff see totals even when the
// course hides them from students.
type GradeSummaryViewer struct {
	IsStaff bool
}

// ErrGradeSummaryNotFound: the student is not actively enrolled in the
// course (or the course is not in the caller's tenant).
var ErrGradeSummaryNotFound = errors.New("not found")

// Summary builds the grade summary for studentID in courseID. periodID
// selects a grading period: nil = the current period (or all when none is
// current), 0 = all periods.
func (s *GradeSummaryService) Summary(ctx context.Context, courseID, studentID, accountID uint, periodID *uint, viewer GradeSummaryViewer) (*GradeSummary, error) {
	enr, err := s.enrollments.FindByUserAndCourse(ctx, studentID, courseID, accountID)
	if err != nil || enr == nil || enr.Type != "StudentEnrollment" ||
		(enr.WorkflowState != models.EnrollmentActive && enr.WorkflowState != models.EnrollmentCompleted) {
		return nil, ErrGradeSummaryNotFound
	}
	courses, err := s.learner.CoursesByIDs(ctx, []uint{courseID}, accountID)
	if err != nil || len(courses) == 0 {
		return nil, ErrGradeSummaryNotFound
	}
	course := courses[0]
	if s.policy == nil {
		return nil, errors.New("grade summary: grading policy service not configured")
	}
	// The gradebook's engine, restricted to what this viewer may see.
	grades, err := s.policy.CourseGrades(ctx, &course, CourseGradesOptions{UserIDs: []uint{studentID}, PostedOnly: !viewer.IsStaff})
	if err != nil {
		return nil, err
	}
	if len(grades.Students) == 0 {
		return nil, ErrGradeSummaryNotFound
	}
	st := grades.Students[0]
	scheme := s.policy.CourseScheme(ctx, &course)

	out := &GradeSummary{
		CourseID:          course.ID,
		CourseName:        course.Name,
		StudentID:         studentID,
		ApplyGroupWeights: course.ApplyGroupWeights,
		HideTotals:        course.HideFinalGrades && !viewer.IsStaff,
		PeriodsWeighted:   grades.WeightedGradingPeriods,
		GradingScale:      []GradeEntry{},
		GradingPeriods:    []GradeSummaryPeriod{},
		Groups:            []GradeSummaryGroup{},
		Assignments:       []GradeSummaryAssignment{},
	}
	if scheme != nil {
		// A course that turned its grading scheme off shows percentages
		// only (empty scale, no letter grades).
		out.GradingScale = scheme.Entries
	}
	if users, _ := s.learner.UsersByIDs(ctx, []uint{studentID}); len(users) == 1 {
		out.StudentName = users[0].Name
	}
	gradeFor := func(v *float64) string {
		if v == nil || scheme == nil {
			return ""
		}
		return scheme.ScoreToGrade(*v)
	}

	// Grading periods: the course term's set (the gradebook's resolver).
	for _, p := range grades.GradingPeriods {
		gp := GradeSummaryPeriod{ID: p.ID, Title: p.Title, StartDate: p.StartDate, EndDate: p.EndDate,
			CloseDate: p.CloseDate, IsClosed: p.IsClosed, Weight: p.Weight}
		if v, ok := st.PeriodOverrides[p.ID]; ok {
			vv := v
			gp.OverrideScore = &vv
			gp.OverrideGrade = gradeFor(&vv)
		}
		out.GradingPeriods = append(out.GradingPeriods, gp)
	}
	if len(grades.GradingPeriods) > 0 {
		switch {
		case periodID == nil:
			if pid := grades.CurrentPeriodID(time.Now()); pid != 0 {
				out.SelectedPeriodID = &pid
			}
		case *periodID != 0:
			found := false
			for _, p := range grades.GradingPeriods {
				if p.ID == *periodID {
					found = true
				}
			}
			if !found {
				return nil, ErrGradeSummaryNotFound
			}
			id := *periodID
			out.SelectedPeriodID = &id
		}
	}

	// The score set for the selected scope: one period, or the course.
	set := st.ScoreSet
	overrideScore := st.OverrideScore
	if out.SelectedPeriodID != nil {
		set = ScoreSet{}
		overrideScore = nil
		for _, pg := range st.Periods {
			if pg.GradingPeriodID == *out.SelectedPeriodID {
				set = pg.ScoreSet
			}
		}
		if v, ok := st.PeriodOverrides[*out.SelectedPeriodID]; ok {
			overrideScore = &v
		}
	}
	groupGrade := map[uint]GroupGrade{}
	dropped := map[uint]bool{}
	for _, g := range set.Groups {
		groupGrade[g.AssignmentGroupID] = g
		for _, id := range g.Dropped {
			dropped[id] = true
		}
	}

	groups, err := s.learner.AssignmentGroups(ctx, courseID)
	if err != nil {
		return nil, err
	}
	assignments, err := s.learner.PublishedAssignments(ctx, []uint{courseID})
	if err != nil {
		return nil, err
	}
	aIDs := make([]uint, len(assignments))
	for i, a := range assignments {
		aIDs[i] = a.ID
	}
	subs, err := s.learner.SubmissionsForUser(ctx, studentID, aIDs)
	if err != nil {
		return nil, err
	}
	subBy := map[uint]*models.Submission{}
	var subIDs []uint
	for i := range subs {
		subBy[subs[i].AssignmentID] = &subs[i]
		subIDs = append(subIDs, subs[i].ID)
	}

	// Feedback comments: the student's own always; others' once posted.
	comments, err := s.learner.SubmissionComments(ctx, subIDs)
	if err != nil {
		return nil, err
	}
	subByID := map[uint]*models.Submission{}
	for i := range subs {
		subByID[subs[i].ID] = &subs[i]
	}
	authorIDs := []uint{}
	for _, c := range comments {
		authorIDs = append(authorIDs, c.AuthorID)
	}
	authors, _ := s.learner.UsersByIDs(ctx, authorIDs)
	authorName := map[uint]string{}
	for _, u := range authors {
		authorName[u.ID] = u.Name
	}
	commentsBy := map[uint][]GradeSummaryComment{}
	for _, c := range comments {
		sub := subByID[c.SubmissionID]
		if sub == nil {
			continue
		}
		if c.AuthorID != studentID && sub.PostedAt == nil && !viewer.IsStaff {
			continue
		}
		commentsBy[sub.AssignmentID] = append(commentsBy[sub.AssignmentID], GradeSummaryComment{
			ID: c.ID, AuthorID: c.AuthorID, AuthorName: authorName[c.AuthorID],
			Comment: c.Comment, CreatedAt: c.CreatedAt,
		})
	}
	rubrics, err := s.learner.RubricAssessmentsForUser(ctx, studentID, aIDs)
	if err != nil {
		return nil, err
	}
	rubricBy := map[uint]*GradeSummaryRubric{}
	for _, r := range rubrics {
		sub := subBy[r.AssignmentID]
		if !viewer.IsStaff && (sub == nil || sub.PostedAt == nil) {
			continue
		}
		data := r.Assessment.Data
		if data == "" {
			data = "{}"
		}
		criteria := r.RubricData
		if criteria == "" {
			criteria = "[]"
		}
		rb := &GradeSummaryRubric{
			Title: r.RubricTitle, Criteria: json.RawMessage(criteria), Assessment: json.RawMessage(data),
			Score: r.Assessment.Score, PointsPossible: r.PointsPossible,
			HidePoints: r.HidePoints, HideScoreTotal: r.HideScoreTotal,
		}
		if !json.Valid(rb.Criteria) {
			rb.Criteria = json.RawMessage("[]")
		}
		if !json.Valid(rb.Assessment) {
			rb.Assessment = json.RawMessage("{}")
		}
		rubricBy[r.AssignmentID] = rb
	}

	now := time.Now()
	for i := range assignments {
		a := &assignments[i]
		dd, assigned := st.DueDates[a.ID]
		if !assigned {
			continue // assign-to: not this student's work
		}
		due := dd.DueAt
		var pid *uint
		if dd.GradingPeriodID != 0 {
			v := dd.GradingPeriodID
			pid = &v
		}
		if out.SelectedPeriodID != nil && (pid == nil || *pid != *out.SelectedPeriodID) {
			continue
		}
		sub := subBy[a.ID]
		posted := sub != nil && sub.PostedAt != nil
		row := GradeSummaryAssignment{
			ID: a.ID, Name: a.Name, AssignmentGroupID: a.AssignmentGroupID, DueAt: due,
			PointsPossible: a.PointsPossible, GradingType: a.GradingType, GradingPeriodID: pid,
			OmitFromFinalGrade: a.OmitFromFinalGrade, Dropped: dropped[a.ID],
			Posted: posted, Comments: commentsBy[a.ID], Rubric: rubricBy[a.ID],
		}
		if row.Comments == nil {
			row.Comments = []GradeSummaryComment{}
		}
		var workflow models.SubmissionWorkflow
		if sub != nil {
			row.SubmittedAt = sub.SubmittedAt
			row.Late = sub.Late
			row.PendingReview = sub.WorkflowState == models.SubmissionPendingReview
			workflow = sub.WorkflowState
		}
		row.Missing = isMissing(a, due, sub, now)
		if posted || viewer.IsStaff {
			if sub != nil {
				row.Score = sub.Score
				row.Grade = sub.Grade
				row.PointsDeducted = sub.PointsDeducted
				row.GradedAt = sub.GradedAt
				row.Excused = sub.Excused
			}
		}
		switch {
		case row.Excused:
			row.Status = "excused"
		case row.PendingReview:
			row.Status = "pending_review"
		case row.Score != nil:
			row.Status = "graded"
		case sub != nil && !posted && (sub.Score != nil || sub.Excused):
			row.Status = "not_posted"
		case row.Missing:
			row.Status = "missing"
		case sub != nil && isSubmitted(sub) && sub.Late:
			row.Status = "late"
		case sub != nil && isSubmitted(sub):
			row.Status = "submitted"
		default:
			row.Status = "unsubmitted"
		}
		_, counts := countedItem(a, &models.Submission{Excused: row.Excused, Score: row.Score, WorkflowState: workflow}, nil)
		row.CountsTowardTotal = counts
		out.Assignments = append(out.Assignments, row)
	}

	for _, g := range groups {
		sg := GradeSummaryGroup{ID: g.ID, Name: g.Name, Weight: g.GroupWeight, Rules: ParseDropRules(g.Rules)}
		if sg.Rules.NeverDrop == nil {
			sg.Rules.NeverDrop = []uint{}
		}
		if gg, ok := groupGrade[g.ID]; ok {
			sg.Earned, sg.Possible = roundPct(gg.CurrentScore), roundPct(gg.CurrentPossible)
			if gg.CurrentPossible > 0 {
				v := roundPct(gg.CurrentScore / gg.CurrentPossible * 100)
				sg.Score = &v
			}
		}
		out.Groups = append(out.Groups, sg)
	}

	// Students see no total when the course hides totals, and no
	// all-periods total when the period set does not display one (the
	// engine already withheld those numbers for them).
	if out.HideTotals || (len(grades.GradingPeriods) > 0 && out.SelectedPeriodID == nil && !grades.DisplayTotalsAllPeriods && !viewer.IsStaff) {
		return out, nil
	}
	t := GradeSummaryTotals{
		CurrentScore: set.CurrentScore, FinalScore: set.FinalScore,
		CurrentGrade: gradeFor(set.CurrentScore), FinalGrade: gradeFor(set.FinalScore),
		Earned: roundPct(set.CurrentPoints), Possible: roundPct(set.CurrentPossible),
		OverrideScore: overrideScore, OverrideGrade: gradeFor(overrideScore),
	}
	out.Totals = &t
	return out, nil
}

func roundPct(v float64) float64 { return math.Round(v*100) / 100 }
