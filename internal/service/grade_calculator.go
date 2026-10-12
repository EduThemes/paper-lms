package service

// Course grade calculator (G020/G021/G025/G029): the server-side twin of
// web/src/utils/gradeCalculator.js. It follows Canvas's documented
// grading rules:
//
//   - unpublished, deleted and not_graded assignments, assignments not
//     assigned to the student (only_visible_to_overrides) and excused
//     submissions never count;
//   - a 0-point assignment's score is extra credit;
//   - assignment groups drop their lowest / highest scores per the
//     group's rules (never_drop assignments are kept), choosing the
//     drop set that gives the student the best (drop lowest) or the
//     least favourable (drop highest) group percentage;
//   - the "current" grade counts graded work only, the "final" grade
//     counts ungraded work as zero;
//   - weighted groups scale up when their weights total less than 100;
//   - with grading periods, each period gets its own grade and the
//     course total either combines the periods by weight (weighted
//     grading period sets) or is computed over every assignment.

import (
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// DropRules are an assignment group's drop rules (Canvas
// assignment_group.rules).
type DropRules struct {
	DropLowest  int    `json:"drop_lowest"`
	DropHighest int    `json:"drop_highest"`
	NeverDrop   []uint `json:"never_drop"`
}

// IsZero reports whether the rules drop nothing.
func (r DropRules) IsZero() bool { return r.DropLowest <= 0 && r.DropHighest <= 0 }

// ParseDropRules reads an assignment group's stored rules. It accepts the
// JSON object Paper stores ({"drop_lowest":1,"never_drop":[3]}, with ids
// as numbers or strings) and Canvas's line format
// ("drop_lowest:1\ndrop_highest:0\nnever_drop:3\n"). Unknown or invalid
// input yields no rules.
func ParseDropRules(raw string) DropRules {
	raw = strings.TrimSpace(raw)
	var out DropRules
	if raw == "" || raw == "{}" || raw == "null" {
		return out
	}
	if strings.HasPrefix(raw, "{") {
		var m map[string]json.RawMessage
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			return out
		}
		out.DropLowest = jsonInt(m["drop_lowest"])
		out.DropHighest = jsonInt(m["drop_highest"])
		if v, ok := m["never_drop"]; ok {
			var items []json.RawMessage
			if json.Unmarshal(v, &items) == nil {
				for _, it := range items {
					if id := jsonInt(it); id > 0 {
						out.NeverDrop = append(out.NeverDrop, uint(id))
					}
				}
			}
		}
	} else {
		for _, line := range strings.Split(raw, "\n") {
			k, v, ok := strings.Cut(strings.TrimSpace(line), ":")
			if !ok {
				continue
			}
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil || n < 0 {
				continue
			}
			switch strings.TrimSpace(k) {
			case "drop_lowest":
				out.DropLowest = n
			case "drop_highest":
				out.DropHighest = n
			case "never_drop":
				if n > 0 {
					out.NeverDrop = append(out.NeverDrop, uint(n))
				}
			}
		}
	}
	if out.DropLowest < 0 {
		out.DropLowest = 0
	}
	if out.DropHighest < 0 {
		out.DropHighest = 0
	}
	return out
}

// MarshalDropRules stores rules in the JSON object form.
func MarshalDropRules(r DropRules) string {
	if r.NeverDrop == nil {
		r.NeverDrop = []uint{}
	}
	b, _ := json.Marshal(r)
	return string(b)
}

func jsonInt(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var f float64
	if json.Unmarshal(raw, &f) == nil {
		return int(f)
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			return n
		}
	}
	return 0
}

// GroupGrade is one assignment group's contribution for one student.
type GroupGrade struct {
	AssignmentGroupID uint    `json:"assignment_group_id"`
	Name              string  `json:"name"`
	Weight            float64 `json:"group_weight"`
	CurrentScore      float64 `json:"current_score"`
	CurrentPossible   float64 `json:"current_possible"`
	FinalScore        float64 `json:"final_score"`
	FinalPossible     float64 `json:"final_possible"`
	// Dropped / FinalDropped list the assignments removed by drop rules
	// for the current and final grades.
	Dropped      []uint `json:"dropped"`
	FinalDropped []uint `json:"final_dropped"`
	// SubmissionCount is the number of graded, counted submissions.
	SubmissionCount int `json:"submission_count"`
}

// ScoreSet is a course (or grading period) grade: percentages are nil
// when nothing counts yet.
type ScoreSet struct {
	CurrentScore    *float64     `json:"current_score"`
	FinalScore      *float64     `json:"final_score"`
	CurrentPoints   float64      `json:"current_points"`
	CurrentPossible float64      `json:"current_possible"`
	FinalPoints     float64      `json:"final_points"`
	FinalPossible   float64      `json:"final_possible"`
	Groups          []GroupGrade `json:"assignment_groups"`
}

// PeriodGrade is one grading period's grade.
type PeriodGrade struct {
	GradingPeriodID uint    `json:"grading_period_id"`
	Title           string  `json:"title"`
	Weight          float64 `json:"weight"`
	ScoreSet
}

// CourseGradeInput is everything the calculator needs for one student.
type CourseGradeInput struct {
	Assignments       []models.Assignment
	Groups            []models.AssignmentGroup
	Submissions       map[uint]*models.Submission // by assignment id
	ApplyGroupWeights bool
	// Unassigned holds assignment ids the student is not assigned to.
	Unassigned map[uint]bool
	// PeriodOf maps assignment id -> the grading period its effective due
	// date falls in for this student (0 / missing = none).
	PeriodOf        map[uint]uint
	Periods         []models.GradingPeriod
	WeightedPeriods bool
}

// CourseGradeResult is a student's course grade.
type CourseGradeResult struct {
	ScoreSet
	Periods []PeriodGrade `json:"grading_periods"`
}

// calcItem is one counted assignment for one student.
type calcItem struct {
	assignmentID uint
	possible     float64
	score        float64
	graded       bool
}

// countedItem applies the per-assignment counting rules; ok=false means
// the assignment does not take part in the grade at all.
func countedItem(a *models.Assignment, sub *models.Submission, unassigned map[uint]bool) (calcItem, bool) {
	if a == nil || !a.IsPublished() || a.GradingType == "not_graded" || a.OmitFromFinalGrade || unassigned[a.ID] {
		return calcItem{}, false
	}
	if sub != nil && sub.Excused {
		return calcItem{}, false
	}
	it := calcItem{assignmentID: a.ID}
	if a.PointsPossible != nil && *a.PointsPossible > 0 {
		it.possible = *a.PointsPossible
	}
	if sub != nil && sub.Score != nil && sub.WorkflowState != models.SubmissionPendingReview {
		it.score = *sub.Score
		it.graded = true
	}
	if it.possible == 0 && !it.graded {
		// A point-less assignment only matters as extra credit.
		return calcItem{}, false
	}
	return it, true
}

// CalculateCourseGrade computes one student's course grade.
func CalculateCourseGrade(in CourseGradeInput) CourseGradeResult {
	var res CourseGradeResult
	all := make([]models.Assignment, 0, len(in.Assignments))
	all = append(all, in.Assignments...)

	if len(in.Periods) == 0 {
		res.ScoreSet = scoreAssignments(all, in)
		return res
	}

	periods := append([]models.GradingPeriod(nil), in.Periods...)
	sort.SliceStable(periods, func(i, j int) bool { return periods[i].StartDate.Before(periods[j].StartDate) })
	for _, p := range periods {
		var inPeriod []models.Assignment
		for _, a := range all {
			if in.PeriodOf[a.ID] == p.ID {
				inPeriod = append(inPeriod, a)
			}
		}
		pg := PeriodGrade{GradingPeriodID: p.ID, Title: p.Title, ScoreSet: scoreAssignments(inPeriod, in)}
		if p.Weight != nil {
			pg.Weight = *p.Weight
		}
		res.Periods = append(res.Periods, pg)
	}

	if !in.WeightedPeriods {
		res.ScoreSet = scoreAssignments(all, in)
		return res
	}

	// Weighted grading periods: the course total combines the period
	// grades by weight; groups are reported per period.
	res.CurrentScore = combinePeriods(res.Periods, false)
	res.FinalScore = combinePeriods(res.Periods, true)
	for _, pg := range res.Periods {
		res.CurrentPoints += pg.CurrentPoints
		res.CurrentPossible += pg.CurrentPossible
		res.FinalPoints += pg.FinalPoints
		res.FinalPossible += pg.FinalPossible
	}
	res.Groups = mergeGroupGrades(res.Periods)
	return res
}

// combinePeriods is Canvas's weighted grading period total: the
// weight-scaled sum of period percentages, scaled up when the weights in
// play total less than 100. The current grade only uses periods with
// graded work.
func combinePeriods(periods []PeriodGrade, final bool) *float64 {
	var sum, weight float64
	used := 0
	for _, p := range periods {
		pct := p.CurrentScore
		if final {
			pct = p.FinalScore
		}
		if pct == nil {
			if final {
				weight += p.Weight
			}
			continue
		}
		sum += *pct * p.Weight / 100
		weight += p.Weight
		used++
	}
	if used == 0 {
		return nil
	}
	if weight == 0 {
		z := 0.0
		return &z
	}
	v := round2(sum * 100 / math.Min(weight, 100))
	return &v
}

// mergeGroupGrades sums each group's per-period grades into one row per
// group (weighted grading periods report groups that way).
func mergeGroupGrades(periods []PeriodGrade) []GroupGrade {
	byID := map[uint]*GroupGrade{}
	var order []uint
	for _, p := range periods {
		for _, g := range p.Groups {
			cur, ok := byID[g.AssignmentGroupID]
			if !ok {
				c := GroupGrade{AssignmentGroupID: g.AssignmentGroupID, Name: g.Name, Weight: g.Weight, Dropped: []uint{}, FinalDropped: []uint{}}
				cur = &c
				byID[g.AssignmentGroupID] = cur
				order = append(order, g.AssignmentGroupID)
			}
			cur.CurrentScore += g.CurrentScore
			cur.CurrentPossible += g.CurrentPossible
			cur.FinalScore += g.FinalScore
			cur.FinalPossible += g.FinalPossible
			cur.SubmissionCount += g.SubmissionCount
			cur.Dropped = append(cur.Dropped, g.Dropped...)
			cur.FinalDropped = append(cur.FinalDropped, g.FinalDropped...)
		}
	}
	out := make([]GroupGrade, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out
}

// scoreAssignments grades one set of assignments (all of them, or one
// grading period's).
func scoreAssignments(assignments []models.Assignment, in CourseGradeInput) ScoreSet {
	groupIndex := map[uint]int{}
	groups := make([]GroupGrade, 0, len(in.Groups)+1)
	rules := map[uint]DropRules{}
	for _, g := range in.Groups {
		if g.WorkflowState == "deleted" {
			continue
		}
		groupIndex[g.ID] = len(groups)
		groups = append(groups, GroupGrade{AssignmentGroupID: g.ID, Name: g.Name, Weight: g.GroupWeight, Dropped: []uint{}, FinalDropped: []uint{}})
		rules[g.ID] = ParseDropRules(g.Rules)
	}

	itemsByGroup := map[uint][]calcItem{}
	for i := range assignments {
		a := &assignments[i]
		it, ok := countedItem(a, in.Submissions[a.ID], in.Unassigned)
		if !ok {
			continue
		}
		gid := uint(0)
		if a.AssignmentGroupID != nil {
			gid = *a.AssignmentGroupID
		}
		if _, known := groupIndex[gid]; !known {
			gid = 0
		}
		itemsByGroup[gid] = append(itemsByGroup[gid], it)
	}
	if items, ok := itemsByGroup[0]; ok && len(items) > 0 {
		groupIndex[0] = len(groups)
		groups = append(groups, GroupGrade{Name: "Ungrouped", Dropped: []uint{}, FinalDropped: []uint{}})
	}

	for gid, items := range itemsByGroup {
		g := &groups[groupIndex[gid]]
		r := rules[gid]
		var graded []calcItem
		for _, it := range items {
			if it.graded {
				graded = append(graded, it)
			}
		}
		g.SubmissionCount = len(graded)
		keptCur, droppedCur := applyDropRules(graded, r)
		for _, it := range keptCur {
			g.CurrentScore += it.score
			g.CurrentPossible += it.possible
		}
		g.Dropped = append(g.Dropped, droppedCur...)
		keptFin, droppedFin := applyDropRules(items, r)
		for _, it := range keptFin {
			g.FinalScore += it.score // ungraded items carry score 0
			g.FinalPossible += it.possible
		}
		g.FinalDropped = append(g.FinalDropped, droppedFin...)
	}

	var set ScoreSet
	set.Groups = groups
	weighted := in.ApplyGroupWeights && len(in.Groups) > 0
	for _, g := range groups {
		if weighted && g.AssignmentGroupID == 0 {
			continue // ungrouped work has no weight
		}
		set.CurrentPoints += g.CurrentScore
		set.CurrentPossible += g.CurrentPossible
		set.FinalPoints += g.FinalScore
		set.FinalPossible += g.FinalPossible
	}
	if weighted {
		set.CurrentScore = combineGroups(groups, false)
		set.FinalScore = combineGroups(groups, true)
		return set
	}
	if set.CurrentPossible > 0 {
		v := round2(set.CurrentPoints / set.CurrentPossible * 100)
		set.CurrentScore = &v
	} else if set.CurrentPoints > 0 {
		// Only extra credit graded so far: Canvas shows no percentage.
		set.CurrentScore = nil
	}
	if set.FinalPossible > 0 {
		v := round2(set.FinalPoints / set.FinalPossible * 100)
		set.FinalScore = &v
	}
	return set
}

// combineGroups is Canvas's weighted assignment group total.
func combineGroups(groups []GroupGrade, final bool) *float64 {
	var sum, weight float64
	used := 0
	for _, g := range groups {
		if g.AssignmentGroupID == 0 {
			continue
		}
		score, possible := g.CurrentScore, g.CurrentPossible
		if final {
			score, possible = g.FinalScore, g.FinalPossible
		}
		if possible <= 0 {
			continue
		}
		sum += score / possible * g.Weight
		weight += g.Weight
		used++
	}
	if used == 0 || weight == 0 {
		return nil
	}
	if weight < 100 {
		sum = sum * 100 / weight
	}
	v := round2(sum)
	return &v
}

// applyDropRules returns the kept items and the dropped assignment ids.
// never_drop items are always kept; at least one droppable item is kept
// (Canvas caps drop_lowest at n-1 and ignores drop_highest when both
// together would drop everything).
func applyDropRules(items []calcItem, r DropRules) (kept []calcItem, dropped []uint) {
	if r.IsZero() || len(items) == 0 {
		return items, nil
	}
	never := map[uint]bool{}
	for _, id := range r.NeverDrop {
		never[id] = true
	}
	var fixed, droppable []calcItem
	for _, it := range items {
		if never[it.assignmentID] {
			fixed = append(fixed, it)
		} else {
			droppable = append(droppable, it)
		}
	}
	n := len(droppable)
	if n == 0 {
		return items, nil
	}
	dropLowest := r.DropLowest
	if dropLowest > n-1 {
		dropLowest = n - 1
	}
	dropHighest := r.DropHighest
	if dropLowest+dropHighest >= n {
		dropHighest = 0
	}
	keep := droppable
	if dropLowest > 0 {
		keep = bestSubset(keep, fixed, len(keep)-dropLowest, true)
	}
	if dropHighest > 0 {
		keep = bestSubset(keep, fixed, len(keep)-dropHighest, false)
	}
	keptIDs := map[uint]bool{}
	for _, it := range keep {
		keptIDs[it.assignmentID] = true
	}
	for _, it := range droppable {
		if !keptIDs[it.assignmentID] {
			dropped = append(dropped, it.assignmentID)
		}
	}
	sort.Slice(dropped, func(i, j int) bool { return dropped[i] < dropped[j] })
	kept = append(append([]calcItem{}, keep...), fixed...)
	return kept, dropped
}

// bestSubset picks keepCount of candidates so that, together with the
// fixed (never-drop) items, the ratio earned/possible is maximised
// (maximise=true, used for drop-lowest) or minimised (drop-highest).
// Point-less (extra credit) candidates are ranked by raw score. For
// pointed candidates this is Dinkelbach's method for fractional
// programming: for a ratio guess q, the best subset maximises
// sum(score - q*possible), which is a sort; iterate q to its fixed
// point (finite: each step strictly improves the ratio).
func bestSubset(candidates, fixed []calcItem, keepCount int, maximise bool) []calcItem {
	if keepCount < 1 {
		keepCount = 1
	}
	if len(candidates) <= keepCount {
		return candidates
	}
	var fixedScore, fixedPossible float64
	for _, it := range fixed {
		fixedScore += it.score
		fixedPossible += it.possible
	}
	pointed := false
	for _, it := range candidates {
		if it.possible > 0 {
			pointed = true
			break
		}
	}
	if !pointed && fixedPossible == 0 {
		sorted := append([]calcItem(nil), candidates...)
		sort.SliceStable(sorted, func(i, j int) bool {
			if sorted[i].score != sorted[j].score {
				if maximise {
					return sorted[i].score > sorted[j].score
				}
				return sorted[i].score < sorted[j].score
			}
			return sorted[i].assignmentID < sorted[j].assignmentID
		})
		return sorted[:keepCount]
	}

	pick := func(q float64) []calcItem {
		sorted := append([]calcItem(nil), candidates...)
		sort.SliceStable(sorted, func(i, j int) bool {
			vi := sorted[i].score - q*sorted[i].possible
			vj := sorted[j].score - q*sorted[j].possible
			if vi != vj {
				if maximise {
					return vi > vj
				}
				return vi < vj
			}
			return sorted[i].assignmentID < sorted[j].assignmentID
		})
		return sorted[:keepCount]
	}
	ratio := func(set []calcItem) float64 {
		s, p := fixedScore, fixedPossible
		for _, it := range set {
			s += it.score
			p += it.possible
		}
		if p == 0 {
			if maximise {
				return math.Inf(1)
			}
			return math.Inf(-1)
		}
		return s / p
	}

	// Start from the subset with the best raw percentages.
	q := 0.0
	if maximise {
		q = math.Inf(-1)
	} else {
		q = math.Inf(1)
	}
	current := pick(0)
	for iter := 0; iter < 100; iter++ {
		r := ratio(current)
		if math.IsInf(r, 0) {
			return current
		}
		if (maximise && r <= q+1e-12) || (!maximise && r >= q-1e-12) {
			break
		}
		q = r
		current = pick(q)
	}
	return current
}
