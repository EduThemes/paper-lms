package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// GradeSummaryService builds the student Grades page ("grade summary"):
// every assignment with the student's posted result, status badges,
// feedback comments and rubric assessment, and course / group totals with
// assignment-group drop rules and grading periods applied. Only posted
// grades are ever counted or shown to a student or observer, so the
// totals match what the student is allowed to know.
type GradeSummaryService struct {
	learner       repository.LearnerRepository
	enrollments   repository.EnrollmentRepository
	standardsRepo repository.GradingStandardRepository
}

func NewGradeSummaryService(learner repository.LearnerRepository, enrollments repository.EnrollmentRepository, standards repository.GradingStandardRepository) *GradeSummaryService {
	return &GradeSummaryService{learner: learner, enrollments: enrollments, standardsRepo: standards}
}

// GroupRules are an assignment group's drop rules (Canvas rules hash).
type GroupRules struct {
	DropLowest  int    `json:"drop_lowest"`
	DropHighest int    `json:"drop_highest"`
	NeverDrop   []uint `json:"never_drop"`
}

// ParseGroupRules reads AssignmentGroup.Rules. Numbers may arrive as JSON
// numbers or strings (Canvas forms post strings); unknown keys are ignored
// and malformed JSON means "no rules".
func ParseGroupRules(raw string) GroupRules {
	var r GroupRules
	if raw == "" {
		return r
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return r
	}
	toInt := func(v any) int {
		switch x := v.(type) {
		case float64:
			return int(x)
		case string:
			n, _ := strconv.Atoi(x)
			return n
		}
		return 0
	}
	r.DropLowest = max(0, toInt(m["drop_lowest"]))
	r.DropHighest = max(0, toInt(m["drop_highest"]))
	if nd, ok := m["never_drop"].([]any); ok {
		for _, v := range nd {
			if n := toInt(v); n > 0 {
				r.NeverDrop = append(r.NeverDrop, uint(n))
			}
		}
	}
	return r
}

// GradedItem is one assignment's contribution to a group total.
type GradedItem struct {
	AssignmentID uint
	Score        float64 // earned points (0 for ungraded work in the final total)
	Possible     float64
}

// keepByRatio picks `keep` items maximizing (best=true) or minimizing the
// group ratio sum(score)/sum(possible). This is the Canvas "drop the
// assignments that help / hurt the grade most" selection; a plain sort by
// percentage is wrong when points possible differ. Solved with Dinkelbach
// iterations: for a fixed ratio q, the best subset is the top `keep` items
// by score - q*possible, and q converges in a few rounds.
func keepByRatio(items []GradedItem, keep int, best bool) []GradedItem {
	if keep >= len(items) {
		return items
	}
	if keep <= 0 {
		return nil
	}
	ratio := func(set []GradedItem) float64 {
		var s, p float64
		for _, it := range set {
			s += it.Score
			p += it.Possible
		}
		if p == 0 {
			return 0
		}
		return s / p
	}
	q := ratio(items)
	var chosen []GradedItem
	for iter := 0; iter < 100; iter++ {
		sorted := append([]GradedItem(nil), items...)
		sort.SliceStable(sorted, func(i, j int) bool {
			vi := sorted[i].Score - q*sorted[i].Possible
			vj := sorted[j].Score - q*sorted[j].Possible
			if vi != vj {
				if best {
					return vi > vj
				}
				return vi < vj
			}
			return sorted[i].AssignmentID < sorted[j].AssignmentID
		})
		next := sorted[:keep]
		nq := ratio(next)
		chosen = next
		if math.Abs(nq-q) < 1e-12 {
			break
		}
		q = nq
	}
	return chosen
}

// ApplyDropRules returns the items that count after the group's rules, and
// the ids of the dropped ones. Lowest are dropped first, then highest from
// what remains; never_drop and extra-credit (0-point) items are always
// kept, and at least one droppable item always remains.
func ApplyDropRules(items []GradedItem, rules GroupRules) (kept []GradedItem, dropped map[uint]bool) {
	dropped = map[uint]bool{}
	if rules.DropLowest == 0 && rules.DropHighest == 0 {
		return items, dropped
	}
	never := map[uint]bool{}
	for _, id := range rules.NeverDrop {
		never[id] = true
	}
	var fixed, cands []GradedItem
	for _, it := range items {
		if never[it.AssignmentID] || it.Possible <= 0 {
			fixed = append(fixed, it)
		} else {
			cands = append(cands, it)
		}
	}
	remaining := cands
	if rules.DropLowest > 0 && len(remaining) > 1 {
		remaining = keepByRatio(remaining, max(1, len(remaining)-rules.DropLowest), true)
	}
	if rules.DropHighest > 0 && len(remaining) > 1 {
		remaining = keepByRatio(remaining, max(1, len(remaining)-rules.DropHighest), false)
	}
	keptIDs := map[uint]bool{}
	for _, it := range remaining {
		keptIDs[it.AssignmentID] = true
	}
	for _, it := range cands {
		if !keptIDs[it.AssignmentID] {
			dropped[it.AssignmentID] = true
		}
	}
	return append(fixed, remaining...), dropped
}

// --- Summary types ---

// GradeSummaryGroup is one assignment group with its totals.
type GradeSummaryGroup struct {
	ID       uint       `json:"id"`
	Name     string     `json:"name"`
	Weight   float64    `json:"group_weight"`
	Rules    GroupRules `json:"rules"`
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
	ID                uint                  `json:"id"`
	Name              string                `json:"name"`
	AssignmentGroupID *uint                 `json:"assignment_group_id"`
	DueAt             *time.Time            `json:"due_at"`
	PointsPossible    *float64              `json:"points_possible"`
	GradingType       string                `json:"grading_type"`
	GradingPeriodID   *uint                 `json:"grading_period_id"`
	Score             *float64              `json:"score"`
	Grade             *string               `json:"grade"`
	PointsDeducted    *float64              `json:"points_deducted"`
	SubmittedAt       *time.Time            `json:"submitted_at"`
	GradedAt          *time.Time            `json:"graded_at"`
	Posted            bool                  `json:"posted"`
	Status            string                `json:"status"` // graded | submitted | missing | late | excused | not_posted | unsubmitted
	Late              bool                  `json:"late"`
	Missing           bool                  `json:"missing"`
	Excused           bool                  `json:"excused"`
	Dropped           bool                  `json:"dropped"`
	CountsTowardTotal bool                  `json:"counts_toward_total"`
	Comments          []GradeSummaryComment `json:"comments"`
	Rubric            *GradeSummaryRubric   `json:"rubric,omitempty"`
}

// GradeSummaryTotals is the course total.
type GradeSummaryTotals struct {
	CurrentScore *float64 `json:"current_score"`
	CurrentGrade string   `json:"current_grade"`
	FinalScore   *float64 `json:"final_score"`
	FinalGrade   string   `json:"final_grade"`
	Earned       float64  `json:"earned"`
	Possible     float64  `json:"possible"`
}

// GradeSummaryPeriod is a grading period the student can filter by.
type GradeSummaryPeriod struct {
	ID        uint      `json:"id"`
	Title     string    `json:"title"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	IsClosed  bool      `json:"is_closed"`
	Weight    *float64  `json:"weight"`
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

// periodFor places a due date in a grading period: by date range, with
// undated work in the last period (Canvas).
func periodFor(due *time.Time, periods []models.GradingPeriod) *uint {
	if len(periods) == 0 {
		return nil
	}
	if due == nil {
		id := periods[len(periods)-1].ID
		return &id
	}
	for i := range periods {
		p := periods[i]
		if !due.Before(p.StartDate) && !due.After(p.EndDate) {
			id := p.ID
			return &id
		}
	}
	return nil
}

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

	out := &GradeSummary{
		CourseID:          course.ID,
		CourseName:        course.Name,
		StudentID:         studentID,
		ApplyGroupWeights: course.ApplyGroupWeights,
		HideTotals:        course.HideFinalGrades && !viewer.IsStaff,
		GradingPeriods:    []GradeSummaryPeriod{},
		Groups:            []GradeSummaryGroup{},
		Assignments:       []GradeSummaryAssignment{},
	}
	if users, _ := s.learner.UsersByIDs(ctx, []uint{studentID}); len(users) == 1 {
		out.StudentName = users[0].Name
	}
	if s.standardsRepo != nil {
		if st, err := s.standardsRepo.FindActiveByCourse(ctx, courseID); err == nil && st != nil {
			out.GradingScale = parseGradingStandardData(st.Data)
		}
	}
	if out.GradingScale == nil {
		out.GradingScale = defaultGradingScale
	}

	// Grading periods come from the course's term.
	var periods []models.GradingPeriod
	displayAllTotals := true
	groupShowsAll := false
	if course.EnrollmentTermID != nil {
		terms, _ := s.learner.EnrollmentTermsByIDs(ctx, []uint{*course.EnrollmentTermID})
		if len(terms) == 1 && terms[0].GradingPeriodGroupID != nil {
			gid := *terms[0].GradingPeriodGroupID
			if grp, err := s.learner.GradingPeriodGroup(ctx, gid); err == nil {
				periods, _ = s.learner.GradingPeriodsForGroup(ctx, gid)
				groupShowsAll = grp.DisplayTotals || grp.Weighted
			}
		}
	}
	periodWeights := map[uint]float64{}
	if len(periods) > 0 {
		weighted := false
		for _, p := range periods {
			out.GradingPeriods = append(out.GradingPeriods, GradeSummaryPeriod{
				ID: p.ID, Title: p.Title, StartDate: p.StartDate, EndDate: p.EndDate, IsClosed: p.IsClosed, Weight: p.Weight,
			})
			if p.Weight != nil {
				weighted = true
				periodWeights[p.ID] = *p.Weight
			}
		}
		out.PeriodsWeighted = weighted
		// The period group's "display totals for All Grading Periods" flag
		// decides whether the all-periods total is shown (Canvas); weighted
		// periods always produce one.
		displayAllTotals = groupShowsAll || weighted
		switch {
		case periodID == nil:
			now := time.Now()
			for _, p := range periods {
				if !now.Before(p.StartDate) && !now.After(p.EndDate) {
					id := p.ID
					out.SelectedPeriodID = &id
				}
			}
		case *periodID != 0:
			found := false
			for _, p := range periods {
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
	var sectionIDs []uint
	if enr.CourseSectionID != nil {
		sectionIDs = []uint{*enr.CourseSectionID}
	}
	dues, err := s.learner.OverrideDueDates(ctx, studentID, sectionIDs, aIDs)
	if err != nil {
		return nil, err
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
	groupByID := map[uint]models.AssignmentGroup{}
	for _, g := range groups {
		groupByID[g.ID] = g
	}
	type rowRef struct {
		idx     int
		groupID uint
	}
	var refs []rowRef
	for i := range assignments {
		a := &assignments[i]
		due := effectiveDueAt(a.DueAt, dues[a.ID])
		pid := periodFor(due, periods)
		if out.SelectedPeriodID != nil && (pid == nil || *pid != *out.SelectedPeriodID) {
			continue
		}
		sub := subBy[a.ID]
		posted := sub != nil && sub.PostedAt != nil
		row := GradeSummaryAssignment{
			ID: a.ID, Name: a.Name, AssignmentGroupID: a.AssignmentGroupID, DueAt: due,
			PointsPossible: a.PointsPossible, GradingType: a.GradingType, GradingPeriodID: pid,
			Posted: posted, Comments: commentsBy[a.ID], Rubric: rubricBy[a.ID],
		}
		if row.Comments == nil {
			row.Comments = []GradeSummaryComment{}
		}
		if sub != nil {
			row.SubmittedAt = sub.SubmittedAt
			row.Late = sub.Late
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
		_, counts := countedItem(a, &models.Submission{Excused: row.Excused, Score: row.Score}, nil)
		row.CountsTowardTotal = counts
		gid := uint(0)
		if a.AssignmentGroupID != nil {
			if _, ok := groupByID[*a.AssignmentGroupID]; ok {
				gid = *a.AssignmentGroupID
			}
		}
		out.Assignments = append(out.Assignments, row)
		refs = append(refs, rowRef{idx: len(out.Assignments) - 1, groupID: gid})
	}

	// Totals per group (current: graded work only; final: ungraded as 0).
	type acc struct {
		current, final []GradedItem
	}
	perGroup := map[uint]*acc{}
	for _, ref := range refs {
		row := &out.Assignments[ref.idx]
		if !row.CountsTowardTotal {
			continue
		}
		possible := 0.0
		if row.PointsPossible != nil {
			possible = *row.PointsPossible
		}
		a := perGroup[ref.groupID]
		if a == nil {
			a = &acc{}
			perGroup[ref.groupID] = a
		}
		earned := 0.0
		if row.Score != nil {
			earned = *row.Score
			a.current = append(a.current, GradedItem{AssignmentID: row.ID, Score: earned, Possible: possible})
		}
		a.final = append(a.final, GradedItem{AssignmentID: row.ID, Score: earned, Possible: possible})
	}

	type groupTotal struct {
		curEarned, curPossible, finEarned, finPossible float64
		weight                                         float64
	}
	totals := map[uint]*groupTotal{}
	droppedIDs := map[uint]bool{}
	sumItems := func(items []GradedItem) (e, p float64) {
		for _, it := range items {
			e += it.Score
			p += it.Possible
		}
		return
	}
	for gid, a := range perGroup {
		rules := ParseGroupRules(groupByID[gid].Rules)
		cur, drops := ApplyDropRules(a.current, rules)
		for id := range drops {
			droppedIDs[id] = true
		}
		fin, _ := ApplyDropRules(a.final, rules)
		gt := &groupTotal{weight: groupByID[gid].GroupWeight}
		gt.curEarned, gt.curPossible = sumItems(cur)
		gt.finEarned, gt.finPossible = sumItems(fin)
		totals[gid] = gt
	}
	for i := range out.Assignments {
		if droppedIDs[out.Assignments[i].ID] {
			out.Assignments[i].Dropped = true
		}
	}
	for _, g := range groups {
		sg := GradeSummaryGroup{ID: g.ID, Name: g.Name, Weight: g.GroupWeight, Rules: ParseGroupRules(g.Rules)}
		if sg.Rules.NeverDrop == nil {
			sg.Rules.NeverDrop = []uint{}
		}
		if gt := totals[g.ID]; gt != nil {
			sg.Earned, sg.Possible = gt.curEarned, gt.curPossible
			if gt.curPossible > 0 {
				v := roundPct(gt.curEarned / gt.curPossible * 100)
				sg.Score = &v
			}
		}
		out.Groups = append(out.Groups, sg)
	}

	if out.HideTotals || (len(periods) > 0 && out.SelectedPeriodID == nil && !displayAllTotals) {
		return out, nil
	}
	var t GradeSummaryTotals
	useWeights := course.ApplyGroupWeights && len(groups) > 0
	var cur, fin *float64
	if useWeights {
		var cSum, cW, fSum, fW float64
		for gid, gt := range totals {
			if gid == 0 || gt.weight <= 0 {
				continue
			}
			if gt.curPossible > 0 {
				cSum += gt.curEarned / gt.curPossible * 100 * gt.weight
				cW += gt.weight
			}
			if gt.finPossible > 0 {
				fSum += gt.finEarned / gt.finPossible * 100 * gt.weight
				fW += gt.weight
			}
			t.Earned += gt.curEarned
			t.Possible += gt.curPossible
		}
		if cW > 0 {
			v := roundPct(cSum / cW)
			cur = &v
		}
		if fW > 0 {
			v := roundPct(fSum / fW)
			fin = &v
		}
	} else {
		var fe, fp float64
		for _, gt := range totals {
			t.Earned += gt.curEarned
			t.Possible += gt.curPossible
			fe += gt.finEarned
			fp += gt.finPossible
		}
		if t.Possible > 0 {
			v := roundPct(t.Earned / t.Possible * 100)
			cur = &v
		}
		if fp > 0 {
			v := roundPct(fe / fp * 100)
			fin = &v
		}
	}
	// All-periods total with weighted grading periods: weight the per-period
	// current scores instead of pooling the assignments.
	if len(periods) > 0 && out.SelectedPeriodID == nil && out.PeriodsWeighted {
		cur = s.weightedPeriodTotal(ctx, courseID, studentID, accountID, periods, periodWeights, viewer)
	}
	t.CurrentScore, t.FinalScore = cur, fin
	if cur != nil {
		t.CurrentGrade = scoreToLetterGradeWithScale(*cur, out.GradingScale)
	}
	if fin != nil {
		t.FinalGrade = scoreToLetterGradeWithScale(*fin, out.GradingScale)
	}
	out.Totals = &t
	return out, nil
}

// weightedPeriodTotal combines per-period current scores by period weight
// (periods without graded work are left out and the weights renormalized).
func (s *GradeSummaryService) weightedPeriodTotal(ctx context.Context, courseID, studentID, accountID uint, periods []models.GradingPeriod, weights map[uint]float64, viewer GradeSummaryViewer) *float64 {
	var sum, w float64
	for _, p := range periods {
		pid := p.ID
		ps, err := s.Summary(ctx, courseID, studentID, accountID, &pid, viewer)
		if err != nil || ps.Totals == nil || ps.Totals.CurrentScore == nil {
			continue
		}
		wt := weights[p.ID]
		if wt <= 0 {
			continue
		}
		sum += *ps.Totals.CurrentScore * wt
		w += wt
	}
	if w == 0 {
		return nil
	}
	v := roundPct(sum / w)
	return &v
}

func roundPct(v float64) float64 { return math.Round(v*100) / 100 }
