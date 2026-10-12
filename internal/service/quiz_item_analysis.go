package service

// Quiz item analysis (quiz-items lane, G054).
//
// Classical test theory over each student's latest finished attempt
// (complete or pending_review):
//
//   - difficulty index  — mean item score / item points (Canvas: share of
//     respondents who got it right; for partial-credit items the mean
//     fraction is the natural generalisation),
//   - discrimination    — mean item fraction of the top 27% of respondents
//     (by total score) minus that of the bottom 27%,
//   - point-biserial    — Pearson correlation of the item fraction with the
//     respondent's total score, and per choice the correlation of "chose
//     this option" with the total (Canvas reports the latter per answer),
//   - Cronbach's alpha  — internal consistency over items every respondent
//     saw.
//
// Answer distributions cover every item type: choice ids (MC/TF/multiple
// answer/dropdown per blank), matching pairs per left item, per-blank
// typed answers for fill-in-multiple-blanks, and the most common typed
// responses for short answer / numerical / formula items.

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/security/htmlsanitize"
)

// ItemAnalysisReport is the full item-analysis payload.
type ItemAnalysisReport struct {
	QuizID      uint                `json:"quiz_id"`
	QuizTitle   string              `json:"quiz_title"`
	Anonymous   bool                `json:"anonymous"`
	GeneratedAt time.Time           `json:"generated_at"`
	Summary     ItemAnalysisSummary `json:"summary"`
	Items       []ItemAnalysisItem  `json:"items"`
}

// ItemAnalysisSummary holds quiz-level statistics.
type ItemAnalysisSummary struct {
	Respondents      int      `json:"respondents"`
	PointsPossible   float64  `json:"points_possible"`
	MeanScore        float64  `json:"mean_score"`
	MedianScore      float64  `json:"median_score"`
	StdDev           float64  `json:"standard_deviation"`
	HighScore        float64  `json:"high_score"`
	LowScore         float64  `json:"low_score"`
	MeanTimeSeconds  float64  `json:"mean_time_seconds"`
	Alpha            *float64 `json:"alpha"`
	AlphaItemCount   int      `json:"alpha_item_count"`
	PendingReview    int      `json:"pending_review"`
	CompletedAnswers int      `json:"completed_answers"`
}

// ItemAnalysisItem is one question's statistics.
type ItemAnalysisItem struct {
	QuestionID       uint                 `json:"question_id"`
	Position         int                  `json:"position"`
	QuestionType     string               `json:"question_type"`
	QuestionText     string               `json:"question_text"`
	PointsPossible   float64              `json:"points_possible"`
	Respondents      int                  `json:"respondents"`
	Answered         int                  `json:"answered"`
	Unanswered       int                  `json:"unanswered"`
	Correct          int                  `json:"correct"`
	PartiallyCorrect int                  `json:"partially_correct"`
	Incorrect        int                  `json:"incorrect"`
	PendingReview    int                  `json:"pending_review"`
	AveragePoints    float64              `json:"average_points"`
	StdDev           float64              `json:"standard_deviation"`
	Difficulty       *float64             `json:"difficulty_index"`
	Discrimination   *float64             `json:"discrimination_index"`
	PointBiserial    *float64             `json:"point_biserial"`
	Options          []ItemAnalysisOption `json:"options,omitempty"`
	Responses        []ItemAnalysisOption `json:"responses,omitempty"`
}

// ItemAnalysisOption is one answer choice (or one distinct typed response)
// with how many respondents picked it. Group names the blank / left-hand
// item it belongs to for multi-part items.
type ItemAnalysisOption struct {
	Group         string   `json:"group,omitempty"`
	ID            string   `json:"id,omitempty"`
	Text          string   `json:"text"`
	Correct       bool     `json:"correct"`
	Count         int      `json:"count"`
	Ratio         float64  `json:"ratio"`
	PointBiserial *float64 `json:"point_biserial,omitempty"`
}

// ItemRespondent is one student's latest finished attempt.
type ItemRespondent struct {
	sub     models.QuizSubmission
	total   float64
	answers map[uint]models.QuizSubmissionAnswer
	inSet   func(questionID uint) bool
}

// UserID is the respondent's user id.
func (r ItemRespondent) UserID() uint { return r.sub.UserID }

// LatestFinishedAttempts keeps each student's latest finished attempt.
func LatestFinishedAttempts(subs []models.QuizSubmission) []models.QuizSubmission {
	latest := map[uint]models.QuizSubmission{}
	for _, s := range subs {
		if s.WorkflowState != "complete" && s.WorkflowState != "pending_review" {
			continue
		}
		if cur, ok := latest[s.UserID]; !ok || s.Attempt > cur.Attempt || (s.Attempt == cur.Attempt && s.ID > cur.ID) {
			latest[s.UserID] = s
		}
	}
	out := make([]models.QuizSubmission, 0, len(latest))
	for _, s := range latest {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// BuildItemAnalysis loads the quiz's questions, finished attempts and
// answers, and computes the report. The quiz must already be authorized.
func (s *QuizService) BuildItemAnalysis(ctx context.Context, quiz *models.Quiz) (*ItemAnalysisReport, []ItemRespondent, []models.QuizQuestion, error) {
	result, err := s.questionRepo.ListByQuizID(ctx, quiz.ID, repository.PaginationParams{Page: 1, PerPage: 10000})
	if err != nil {
		return nil, nil, nil, err
	}
	questions := make([]models.QuizQuestion, 0, len(result.Items))
	for _, q := range result.Items {
		if q.WorkflowState != "deleted" {
			questions = append(questions, q)
		}
	}
	sort.SliceStable(questions, func(i, j int) bool {
		if questions[i].Position != questions[j].Position {
			return questions[i].Position < questions[j].Position
		}
		return questions[i].ID < questions[j].ID
	})
	subs, err := s.submissionRepo.ListCompletedByQuizID(ctx, quiz.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	subs = LatestFinishedAttempts(subs)
	ids := make([]uint, len(subs))
	for i, sub := range subs {
		ids[i] = sub.ID
	}
	var answers []models.QuizSubmissionAnswer
	if len(ids) > 0 {
		answers, err = s.answerRepo.ListBySubmissionIDs(ctx, ids)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	report, respondents := ComputeItemAnalysis(quiz, questions, subs, answers)
	return report, respondents, questions, nil
}

// ComputeItemAnalysis is the pure computation behind BuildItemAnalysis.
func ComputeItemAnalysis(quiz *models.Quiz, questions []models.QuizQuestion, subs []models.QuizSubmission, answers []models.QuizSubmissionAnswer) (*ItemAnalysisReport, []ItemRespondent) {
	bySub := map[uint]map[uint]models.QuizSubmissionAnswer{}
	for _, a := range answers {
		if bySub[a.QuizSubmissionID] == nil {
			bySub[a.QuizSubmissionID] = map[uint]models.QuizSubmissionAnswer{}
		}
		bySub[a.QuizSubmissionID][a.QuestionID] = a
	}

	// Questions that are not per-attempt bank copies are "authored"; an
	// attempt without a personalized set saw every authored question.
	authored := map[uint]bool{}
	for _, q := range questions {
		if q.QuestionBankEntryID == nil {
			authored[q.ID] = true
		}
	}

	respondents := make([]ItemRespondent, 0, len(subs))
	pending := 0
	var timeSum float64
	for _, sub := range subs {
		ans := bySub[sub.ID]
		if ans == nil {
			ans = map[uint]models.QuizSubmissionAnswer{}
		}
		total := 0.0
		if sub.Score != nil {
			total = *sub.Score
		} else {
			for _, a := range ans {
				if a.Points != nil {
					total += *a.Points
				}
			}
		}
		sel, hasSel := selectedQuestionSet(&sub)
		inSet := func(qid uint) bool {
			if hasSel {
				return sel[qid]
			}
			return authored[qid]
		}
		if sub.WorkflowState == "pending_review" {
			pending++
		}
		timeSum += float64(sub.TimeSpent)
		respondents = append(respondents, ItemRespondent{sub: sub, total: total, answers: ans, inSet: inSet})
	}

	report := &ItemAnalysisReport{
		QuizID:      quiz.ID,
		QuizTitle:   quiz.Title,
		Anonymous:   quiz.AnonymousSubmissions,
		GeneratedAt: time.Now().UTC(),
	}
	if quiz.PointsPossible != nil {
		report.Summary.PointsPossible = *quiz.PointsPossible
	}
	report.Summary.Respondents = len(respondents)
	report.Summary.PendingReview = pending
	if len(respondents) > 0 {
		totals := make([]float64, len(respondents))
		for i, r := range respondents {
			totals[i] = r.total
		}
		mean, sd := iaMeanStd(totals)
		sorted := append([]float64(nil), totals...)
		sort.Float64s(sorted)
		report.Summary.MeanScore = iaRound2(mean)
		report.Summary.StdDev = iaRound2(sd)
		report.Summary.LowScore = sorted[0]
		report.Summary.HighScore = sorted[len(sorted)-1]
		report.Summary.MedianScore = iaRound2(iaMedian(sorted))
		report.Summary.MeanTimeSeconds = iaRound2(timeSum / float64(len(respondents)))
	}

	// Per-item fractions for alpha: item -> respondent index -> score.
	itemScores := map[uint]map[int]float64{}

	for _, q := range questions {
		pp := 1.0
		if q.PointsPossible != nil {
			pp = *q.PointsPossible
		}
		qType := NormalizeQuizQuestionType(q.QuestionType)
		item := ItemAnalysisItem{
			QuestionID:     q.ID,
			Position:       q.Position,
			QuestionType:   qType,
			QuestionText:   iaTruncate(strings.TrimSpace(htmlsanitize.PlainText(q.QuestionText)), 500),
			PointsPossible: pp,
		}
		var fractions, totals []float64
		var idxs []int
		scores := map[int]float64{}
		pointsSum := 0.0
		for i, r := range respondents {
			if !r.inSet(q.ID) {
				continue
			}
			item.Respondents++
			a, answered := r.answers[q.ID]
			pts := 0.0
			if answered && strings.TrimSpace(a.Answer) != "" {
				item.Answered++
				if a.Points != nil {
					pts = *a.Points
				}
				switch {
				case a.Correct == nil && a.GradedVia == nil:
					item.PendingReview++
				case a.Correct != nil && *a.Correct:
					item.Correct++
				case pts > 0:
					item.PartiallyCorrect++
				default:
					item.Incorrect++
				}
			} else {
				item.Unanswered++
			}
			pointsSum += pts
			frac := 0.0
			if pp > 0 {
				frac = pts / pp
			}
			fractions = append(fractions, frac)
			totals = append(totals, r.total)
			idxs = append(idxs, i)
			scores[i] = pts
		}
		itemScores[q.ID] = scores
		if item.Respondents > 0 {
			item.AveragePoints = iaRound2(pointsSum / float64(item.Respondents))
			pointList := make([]float64, 0, len(scores))
			for _, v := range scores {
				pointList = append(pointList, v)
			}
			_, sd := iaMeanStd(pointList)
			item.StdDev = iaRound2(sd)
			if pp > 0 && !QuizIsSurvey(quiz) {
				d := iaRound2(iaMean(fractions))
				item.Difficulty = &d
				item.Discrimination = iaDiscrimination(fractions, totals)
				item.PointBiserial = iaPearson(fractions, totals)
			}
		}
		item.Options, item.Responses = iaAnswerDistribution(qType, q.Answers, respondents, idxs, totals, q.ID)
		report.Items = append(report.Items, item)
	}

	report.Summary.Alpha, report.Summary.AlphaItemCount = iaCronbachAlpha(questions, itemScores, len(respondents))
	completed := 0
	for _, r := range respondents {
		completed += len(r.answers)
	}
	report.Summary.CompletedAnswers = completed
	return report, respondents
}

// ---------- distributions ----------

type iaDistKey struct{ group, id string }

func iaAnswerDistribution(qType, answersJSON string, respondents []ItemRespondent, idxs []int, totals []float64, qid uint) ([]ItemAnalysisOption, []ItemAnalysisOption) {
	var opts []answerOption
	_ = json.Unmarshal([]byte(strings.TrimSpace(answersJSON)), &opts)
	n := len(idxs)

	// chosen[k][j] = respondent j (in idxs order) chose option k.
	type optAcc struct {
		opt    ItemAnalysisOption
		chosen []float64
	}
	var order []iaDistKey
	accs := map[iaDistKey]*optAcc{}
	add := func(k iaDistKey, text string, correct bool) *optAcc {
		if a, ok := accs[k]; ok {
			return a
		}
		a := &optAcc{opt: ItemAnalysisOption{Group: k.group, ID: k.id, Text: text, Correct: correct}, chosen: make([]float64, n)}
		accs[k] = a
		order = append(order, k)
		return a
	}
	mark := func(k iaDistKey, j int) {
		if a, ok := accs[k]; ok {
			a.chosen[j] = 1
		}
	}
	finish := func() []ItemAnalysisOption {
		out := make([]ItemAnalysisOption, 0, len(order))
		for _, k := range order {
			a := accs[k]
			cnt := 0
			for _, v := range a.chosen {
				if v > 0 {
					cnt++
				}
			}
			a.opt.Count = cnt
			if n > 0 {
				a.opt.Ratio = iaRound2(float64(cnt) / float64(n))
			}
			a.opt.PointBiserial = iaPearson(a.chosen, totals)
			out = append(out, a.opt)
		}
		return out
	}

	switch qType {
	case "multiple_choice", "true_false", "multiple_answer":
		for _, o := range opts {
			add(iaDistKey{"", o.ID}, o.Text.String(), o.Weight > 0)
		}
		for j, i := range idxs {
			raw := strings.TrimSpace(respondents[i].answers[qid].Answer)
			if qType == "multiple_answer" {
				var ids []string
				if json.Unmarshal([]byte(raw), &ids) == nil {
					for _, id := range ids {
						mark(iaDistKey{"", id}, j)
					}
				}
				continue
			}
			mark(iaDistKey{"", raw}, j)
		}
		return finish(), nil

	case "multiple_dropdown":
		for _, o := range opts {
			add(iaDistKey{o.BlankID, o.ID}, o.Text.String(), o.Weight > 0)
		}
		for j, i := range idxs {
			var m map[string]string
			if json.Unmarshal([]byte(respondents[i].answers[qid].Answer), &m) == nil {
				for blank, id := range m {
					mark(iaDistKey{blank, id}, j)
				}
			}
		}
		return finish(), nil

	case "matching":
		pool := map[string]string{}
		correctByLeft := map[string]string{}
		var lefts []string
		labels := map[string]string{}
		for _, o := range opts {
			if o.BlankID == matchOptionsSentinel {
				pool[o.ID] = o.Text.String()
				continue
			}
			right := o.RightID
			if right == "" {
				right = o.MatchID
			}
			key := matchingKey(o)
			if key == "" {
				continue
			}
			lefts = append(lefts, key)
			label := o.Left
			if label == "" {
				label = o.Text.String()
			}
			labels[key] = label
			correctByLeft[key] = right
			if right != "" {
				if rt := strings.TrimSpace(iaRawRight(answersJSON, o.ID)); rt != "" {
					pool[right] = rt
				}
			}
		}
		ids := make([]string, 0, len(pool))
		for id := range pool {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, left := range lefts {
			for _, id := range ids {
				add(iaDistKey{labels[left], id}, pool[id], correctByLeft[left] == id)
			}
		}
		for j, i := range idxs {
			var pairs []struct {
				Left    string `json:"left"`
				RightID string `json:"right_id"`
			}
			if json.Unmarshal([]byte(respondents[i].answers[qid].Answer), &pairs) == nil {
				for _, p := range pairs {
					mark(iaDistKey{labels[p.Left], p.RightID}, j)
				}
			}
		}
		return finish(), nil

	case "fill_in_multiple_blanks":
		accepted, _ := parseBlankAnswers(answersJSON)
		counts := map[iaDistKey]int{}
		var keys []iaDistKey
		for _, i := range idxs {
			var m map[string]string
			if json.Unmarshal([]byte(respondents[i].answers[qid].Answer), &m) != nil {
				continue
			}
			for blank, text := range m {
				t := strings.ToLower(strings.TrimSpace(text))
				if t == "" {
					continue
				}
				k := iaDistKey{blank, t}
				if counts[k] == 0 {
					keys = append(keys, k)
				}
				counts[k]++
			}
		}
		out := make([]ItemAnalysisOption, 0, len(keys))
		for _, k := range keys {
			ok := false
			for _, acc := range accepted[k.group] {
				if strings.ToLower(strings.TrimSpace(acc)) == k.id {
					ok = true
					break
				}
			}
			out = append(out, ItemAnalysisOption{Group: k.group, Text: k.id, Correct: ok, Count: counts[k], Ratio: iaRatio(counts[k], n)})
		}
		sort.SliceStable(out, func(a, b int) bool {
			if out[a].Group != out[b].Group {
				return out[a].Group < out[b].Group
			}
			return out[a].Count > out[b].Count
		})
		return nil, iaCapPerGroup(out, 10)

	case "short_answer", "numerical_question", "fill_in_the_blank", "formula":
		counts := map[string]int{}
		correct := map[string]bool{}
		var keys []string
		for _, i := range idxs {
			a := respondents[i].answers[qid]
			t := strings.TrimSpace(a.Answer)
			if t == "" {
				continue
			}
			if qType == "short_answer" || qType == "fill_in_the_blank" {
				t = strings.ToLower(t)
			}
			if counts[t] == 0 {
				keys = append(keys, t)
			}
			counts[t]++
			if a.Correct != nil && *a.Correct {
				correct[t] = true
			}
		}
		out := make([]ItemAnalysisOption, 0, len(keys))
		for _, k := range keys {
			out = append(out, ItemAnalysisOption{Text: k, Correct: correct[k], Count: counts[k], Ratio: iaRatio(counts[k], n)})
		}
		sort.SliceStable(out, func(a, b int) bool { return out[a].Count > out[b].Count })
		if len(out) > 15 {
			out = out[:15]
		}
		return nil, out
	}
	return nil, nil
}

// rawRight reads the "right" label of a matching pair entry by id (the
// grader's answerOption does not decode it).
func iaRawRight(answersJSON, id string) string {
	var list []struct {
		ID    string `json:"id"`
		Right string `json:"right"`
	}
	if json.Unmarshal([]byte(answersJSON), &list) != nil {
		return ""
	}
	for _, o := range list {
		if o.ID == id {
			return o.Right
		}
	}
	return ""
}

func iaCapPerGroup(in []ItemAnalysisOption, max int) []ItemAnalysisOption {
	out := make([]ItemAnalysisOption, 0, len(in))
	per := map[string]int{}
	for _, o := range in {
		if per[o.Group] >= max {
			continue
		}
		per[o.Group]++
		out = append(out, o)
	}
	return out
}

func iaRatio(count, n int) float64 {
	if n == 0 {
		return 0
	}
	return iaRound2(float64(count) / float64(n))
}

// ---------- statistics ----------

func iaRound2(v float64) float64 { return math.Round(v*100) / 100 }

func iaMean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

// meanStd returns the mean and population standard deviation.
func iaMeanStd(xs []float64) (float64, float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	m := iaMean(xs)
	v := 0.0
	for _, x := range xs {
		v += (x - m) * (x - m)
	}
	return m, math.Sqrt(v / float64(len(xs)))
}

func iaMedian(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

// pearson returns the correlation of xs and ys, or nil when it is
// undefined (fewer than two points or a constant series).
func iaPearson(xs, ys []float64) *float64 {
	n := len(xs)
	if n < 2 || len(ys) != n {
		return nil
	}
	mx, sx := iaMeanStd(xs)
	my, sy := iaMeanStd(ys)
	if sx == 0 || sy == 0 {
		return nil
	}
	cov := 0.0
	for i := range xs {
		cov += (xs[i] - mx) * (ys[i] - my)
	}
	cov /= float64(n)
	r := iaRound2(cov / (sx * sy))
	return &r
}

// discriminationIndex is the upper-27% minus lower-27% mean item fraction.
func iaDiscrimination(fractions, totals []float64) *float64 {
	n := len(fractions)
	if n < 2 {
		return nil
	}
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return totals[idx[a]] > totals[idx[b]] })
	k := int(math.Round(0.27 * float64(n)))
	if k < 1 {
		k = 1
	}
	if k > n/2 {
		k = n / 2
	}
	if k < 1 {
		return nil
	}
	top, bottom := 0.0, 0.0
	for i := 0; i < k; i++ {
		top += fractions[idx[i]]
		bottom += fractions[idx[n-1-i]]
	}
	d := iaRound2((top - bottom) / float64(k))
	return &d
}

// cronbachAlpha over the items every respondent saw.
func iaCronbachAlpha(questions []models.QuizQuestion, itemScores map[uint]map[int]float64, respondents int) (*float64, int) {
	if respondents < 2 {
		return nil, 0
	}
	var items []uint
	for _, q := range questions {
		if len(itemScores[q.ID]) == respondents {
			items = append(items, q.ID)
		}
	}
	k := len(items)
	if k < 2 {
		return nil, k
	}
	sumVar := 0.0
	totals := make([]float64, respondents)
	for _, id := range items {
		col := make([]float64, respondents)
		for i := 0; i < respondents; i++ {
			col[i] = itemScores[id][i]
			totals[i] += col[i]
		}
		_, sd := iaMeanStd(col)
		sumVar += sd * sd
	}
	_, sdT := iaMeanStd(totals)
	if sdT == 0 {
		return nil, k
	}
	a := iaRound2(float64(k) / float64(k-1) * (1 - sumVar/(sdT*sdT)))
	return &a, k
}

func iaTruncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// ---------- CSV ----------

func iaFmtPtr(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', 2, 64)
}

func iaFmtNum(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// WriteItemAnalysisCSV writes one row per question.
func WriteItemAnalysisCSV(w io.Writer, r *ItemAnalysisReport) error {
	cw := csv.NewWriter(w)
	header := []string{"Question Id", "Position", "Question Type", "Question", "Points Possible",
		"Respondents", "Answered", "Correct", "Partially Correct", "Incorrect", "Unanswered", "Pending Review",
		"Average Points", "Standard Deviation", "Difficulty Index", "Discrimination Index", "Point Biserial", "Answer Distribution"}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, it := range r.Items {
		dist := make([]string, 0, len(it.Options)+len(it.Responses))
		for _, o := range append(append([]ItemAnalysisOption{}, it.Options...), it.Responses...) {
			label := o.Text
			if o.Group != "" {
				label = o.Group + ": " + label
			}
			mark := ""
			if o.Correct {
				mark = "*"
			}
			dist = append(dist, fmt.Sprintf("%s%s (%d)", mark, label, o.Count))
		}
		row := []string{
			strconv.FormatUint(uint64(it.QuestionID), 10), strconv.Itoa(it.Position), it.QuestionType, it.QuestionText,
			iaFmtNum(it.PointsPossible), strconv.Itoa(it.Respondents), strconv.Itoa(it.Answered), strconv.Itoa(it.Correct),
			strconv.Itoa(it.PartiallyCorrect), strconv.Itoa(it.Incorrect), strconv.Itoa(it.Unanswered), strconv.Itoa(it.PendingReview),
			iaFmtNum(it.AveragePoints), iaFmtNum(it.StdDev), iaFmtPtr(it.Difficulty), iaFmtPtr(it.Discrimination), iaFmtPtr(it.PointBiserial),
			strings.Join(dist, "; "),
		}
		if err := writeCSVRow(cw, row); err != nil {
			return err
		}
	}
	summary := []string{"Summary", "", "", fmt.Sprintf("Respondents: %d", r.Summary.Respondents), iaFmtNum(r.Summary.PointsPossible),
		strconv.Itoa(r.Summary.Respondents), "", "", "", "", "", strconv.Itoa(r.Summary.PendingReview),
		iaFmtNum(r.Summary.MeanScore), iaFmtNum(r.Summary.StdDev), "", "", "", "Cronbach alpha: " + iaFmtPtr(r.Summary.Alpha)}
	if err := writeCSVRow(cw, summary); err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
}

// StudentAnalysisNames maps user id -> display name (and SIS/login id).
type StudentAnalysisNames map[uint][2]string

// WriteStudentAnalysisCSV writes one row per respondent (latest finished
// attempt) with each question's response and points. Anonymous surveys
// replace names and ids with "Anonymous N".
func WriteStudentAnalysisCSV(w io.Writer, r *ItemAnalysisReport, respondents []ItemRespondent, questions []models.QuizQuestion, names StudentAnalysisNames) error {
	cw := csv.NewWriter(w)
	header := []string{"Name", "User Id", "Login Id", "Attempt", "Submitted At", "Score", "Time Spent (s)"}
	for i, q := range questions {
		label := iaTruncate(strings.TrimSpace(htmlsanitize.PlainText(q.QuestionText)), 60)
		header = append(header, fmt.Sprintf("Q%d (%d): %s", i+1, q.ID, label), fmt.Sprintf("Q%d Points", i+1))
	}
	if err := cw.Write(sanitizeCSVRow(header)); err != nil {
		return err
	}
	for n, resp := range respondents {
		name, userID, login := "", strconv.FormatUint(uint64(resp.sub.UserID), 10), ""
		if r.Anonymous {
			name, userID = fmt.Sprintf("Anonymous %d", n+1), ""
		} else if nm, ok := names[resp.sub.UserID]; ok {
			name, login = nm[0], nm[1]
		}
		finished := ""
		if resp.sub.FinishedAt != nil {
			finished = resp.sub.FinishedAt.UTC().Format(time.RFC3339)
		}
		row := []string{name, userID, login, strconv.Itoa(resp.sub.Attempt), finished, iaFmtNum(resp.total), strconv.Itoa(resp.sub.TimeSpent)}
		for _, q := range questions {
			if !resp.inSet(q.ID) {
				row = append(row, "", "")
				continue
			}
			a, ok := resp.answers[q.ID]
			if !ok {
				row = append(row, "", "0")
				continue
			}
			pts := ""
			if a.Points != nil {
				pts = iaFmtNum(*a.Points)
			}
			row = append(row, iaHumanAnswer(q, a.Answer), pts)
		}
		if err := writeCSVRow(cw, row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// humanAnswer renders a stored answer readably (choice ids -> labels).
func iaHumanAnswer(q models.QuizQuestion, raw string) string {
	var opts []answerOption
	_ = json.Unmarshal([]byte(q.Answers), &opts)
	label := func(id string) string {
		for _, o := range opts {
			if o.ID == id {
				if t := o.Text.String(); t != "" {
					return t
				}
			}
		}
		return id
	}
	switch NormalizeQuizQuestionType(q.QuestionType) {
	case "multiple_choice", "true_false":
		return label(strings.TrimSpace(raw))
	case "multiple_answer":
		var ids []string
		if json.Unmarshal([]byte(raw), &ids) == nil {
			out := make([]string, len(ids))
			for i, id := range ids {
				out[i] = label(id)
			}
			return strings.Join(out, ", ")
		}
	case "multiple_dropdown":
		var m map[string]string
		if json.Unmarshal([]byte(raw), &m) == nil {
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			out := make([]string, len(keys))
			for i, k := range keys {
				out[i] = k + "=" + label(m[k])
			}
			return strings.Join(out, ", ")
		}
	case "essay":
		return iaTruncate(strings.TrimSpace(htmlsanitize.PlainText(raw)), 2000)
	}
	return iaTruncate(raw, 2000)
}
