package service

import (
	"context"
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type AnalyticsService struct {
	pageViewRepo   repository.PageViewRepository
	submissionRepo repository.SubmissionRepository
	enrollmentRepo repository.EnrollmentRepository
	assignmentRepo repository.AssignmentRepository

	// participation de-duplication: an autosaving editor or quiz posts the
	// same course URL many times a minute; count it once per window.
	partMu   sync.Mutex
	partSeen map[string]time.Time
	now      func() time.Time
}

func NewAnalyticsService(
	pageViewRepo repository.PageViewRepository,
	submissionRepo repository.SubmissionRepository,
	enrollmentRepo repository.EnrollmentRepository,
	assignmentRepo repository.AssignmentRepository,
) *AnalyticsService {
	return &AnalyticsService{
		pageViewRepo:   pageViewRepo,
		submissionRepo: submissionRepo,
		enrollmentRepo: enrollmentRepo,
		assignmentRepo: assignmentRepo,
		partSeen:       map[string]time.Time{},
		now:            time.Now,
	}
}

// ErrPageViewNotTracked means the caller has no enrollment in the course,
// so the page view is not recorded (the handler answers 204 either way, so
// nothing leaks about whether the course exists).
var ErrPageViewNotTracked = errors.New("page view not tracked")

// ErrAnalyticsStudentNotFound means the user is not a student of the course
// (in the caller's tenant); handlers answer 404.
var ErrAnalyticsStudentNotFound = errors.New("student not found in course")

// participationWindow is how long a repeated write to the same course URL
// by the same user counts as one participation.
const participationWindow = 60 * time.Second

// maxInteractionSeconds caps one page view's reported time on page (a tab
// left open overnight is not eight hours of study).
const maxInteractionSeconds = 2 * 60 * 60

// AnalyticsCategory maps the first path segment under a course
// (e.g. "assignments" in /courses/5/assignments/3) to the content category
// analytics groups page views by. Unknown segments are "other"; the course
// home is "home".
func AnalyticsCategory(segment string) string {
	switch strings.ToLower(strings.TrimSpace(segment)) {
	case "":
		return "home"
	case "assignments", "assignment_groups", "overrides":
		return "assignments"
	case "discussion_topics", "discussions", "discussions_v2", "entries":
		return "discussions"
	case "quizzes", "quiz_submissions", "item-banks", "item_banks", "question_banks":
		return "quizzes"
	case "pages", "wiki_pages", "front_page", "p":
		return "pages"
	case "files", "folders":
		return "files"
	case "modules", "module_items":
		return "modules"
	case "announcements":
		return "announcements"
	case "grades", "gradebook", "submissions", "students", "speedgrader", "outcome_results", "outcome_rollups":
		return "grades"
	case "syllabus":
		return "syllabus"
	case "calendar", "calendar_events", "appointment_groups":
		return "calendar"
	case "conferences":
		return "conferences"
	case "collaborations":
		return "collaborations"
	case "groups":
		return "groups"
	case "outcomes", "outcome_groups", "mastery", "learning_mastery":
		return "outcomes"
	case "people", "users", "enrollments", "sections":
		return "people"
	case "external_tools", "lti":
		return "external_tools"
	}
	return "other"
}

// AnalyticsCategoryForPath derives the category from a course URL path such
// as "/courses/5/assignments/3" or "/api/v1/courses/5/assignments/3".
func AnalyticsCategoryForPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "courses" {
			if i+2 < len(parts) {
				return AnalyticsCategory(parts[i+2])
			}
			return AnalyticsCategory("")
		}
	}
	return "other"
}

// GetCourseActivity returns the course's student page views and
// participations per day (Canvas "weekly online activity" — the client
// groups the days into weeks). since bounds the range (nil: all time).
func (s *AnalyticsService) GetCourseActivity(ctx context.Context, courseID uint, since *time.Time) ([]repository.PageViewDayCount, error) {
	return s.pageViewRepo.ActivityByDay(ctx, courseID, 0, since)
}

// GetCourseActivityCategories groups the course's student page views by the
// kind of content viewed.
func (s *AnalyticsService) GetCourseActivityCategories(ctx context.Context, courseID uint) ([]repository.PageViewCategoryCount, error) {
	return s.pageViewRepo.CategoryCounts(ctx, courseID, 0)
}

// courseStudents returns the course's (non-deleted) student enrollments,
// one per user, ordered by name.
func (s *AnalyticsService) courseStudents(ctx context.Context, courseID, accountID uint) ([]models.Enrollment, error) {
	seen := map[uint]bool{}
	var out []models.Enrollment
	for page := 1; page <= 200; page++ {
		res, err := s.enrollmentRepo.ListByCourseID(ctx, courseID, accountID, repository.PaginationParams{Page: page, PerPage: 100})
		if err != nil {
			return nil, err
		}
		for _, e := range res.Items {
			if e.Type != "StudentEnrollment" || seen[e.UserID] {
				continue
			}
			seen[e.UserID] = true
			out = append(out, e)
		}
		if len(res.Items) == 0 || int64(page*100) >= res.TotalCount {
			break
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(enrollmentUserName(out[i])) < strings.ToLower(enrollmentUserName(out[j]))
	})
	return out, nil
}

func enrollmentUserName(e models.Enrollment) string {
	if e.User != nil {
		if e.User.SortableName != "" {
			return e.User.SortableName
		}
		return e.User.Name
	}
	return ""
}

// publishedAssignments lists the course's published, graded-or-submittable
// assignments.
func (s *AnalyticsService) publishedAssignments(ctx context.Context, courseID uint) ([]models.Assignment, error) {
	res, err := s.assignmentRepo.ListByCourseID(ctx, courseID, repository.PaginationParams{Page: 1, PerPage: 1000})
	if err != nil {
		return nil, err
	}
	out := make([]models.Assignment, 0, len(res.Items))
	for _, a := range res.Items {
		if a.IsPublished() {
			out = append(out, a)
		}
	}
	return out, nil
}

// expectsSubmission reports whether students are expected to turn the
// assignment in online (so "not turned in after the due date" is missing).
func expectsSubmission(a *models.Assignment) bool {
	types := strings.ToLower(a.SubmissionTypes)
	return types != "" && !strings.Contains(types, "none") && !strings.Contains(types, "on_paper") && !strings.Contains(types, "not_graded")
}

// Tardiness statuses (Canvas tardiness_breakdown).
const (
	TardinessOnTime   = "on_time"
	TardinessLate     = "late"
	TardinessMissing  = "missing"
	TardinessFloating = "floating" // not submitted and not yet due (or no due date)
	TardinessExcused  = "excused"
)

// SubmissionTardiness classifies one student's submission to an assignment.
func SubmissionTardiness(a *models.Assignment, sub *models.Submission, now time.Time) string {
	if sub != nil && sub.Excused {
		return TardinessExcused
	}
	// A status a grader set by hand wins (same rule as ComputeTardiness).
	if sub != nil && sub.LatePolicyStatus != nil {
		switch *sub.LatePolicyStatus {
		case models.LatePolicyStatusLate:
			return TardinessLate
		case models.LatePolicyStatusMissing:
			return TardinessMissing
		case models.LatePolicyStatusExtended, models.LatePolicyStatusNone:
			if sub.SubmittedAt != nil || sub.Score != nil {
				return TardinessOnTime
			}
			return TardinessFloating
		}
	}
	if sub != nil && sub.SubmittedAt != nil {
		if sub.Late {
			return TardinessLate
		}
		if a.DueAt != nil && sub.SubmittedAt.After(*a.DueAt) {
			return TardinessLate
		}
		return TardinessOnTime
	}
	if sub != nil && sub.Missing {
		return TardinessMissing
	}
	if sub != nil && sub.Score != nil {
		// Graded without an online submission (on paper, or entered
		// directly): counts as turned in.
		return TardinessOnTime
	}
	if a.DueAt != nil && a.DueAt.Before(now) && expectsSubmission(a) {
		return TardinessMissing
	}
	return TardinessFloating
}

// TardinessBreakdown counts a student's assignments by tardiness status.
type TardinessBreakdown struct {
	OnTime   int `json:"on_time"`
	Late     int `json:"late"`
	Missing  int `json:"missing"`
	Floating int `json:"floating"`
	Excused  int `json:"excused"`
	Total    int `json:"total"`
}

func (b *TardinessBreakdown) add(status string) {
	b.Total++
	switch status {
	case TardinessOnTime:
		b.OnTime++
	case TardinessLate:
		b.Late++
	case TardinessMissing:
		b.Missing++
	case TardinessExcused:
		b.Excused++
	default:
		b.Floating++
	}
}

// GetCourseAssignmentStats returns per-assignment statistics over the
// course's students: score min / quartiles / median / max / average and the
// on-time / late / missing breakdown (Canvas course analytics "assignments").
func (s *AnalyticsService) GetCourseAssignmentStats(ctx context.Context, courseID, accountID uint) ([]map[string]interface{}, error) {
	assignments, err := s.publishedAssignments(ctx, courseID)
	if err != nil {
		return nil, err
	}
	students, err := s.courseStudents(ctx, courseID, accountID)
	if err != nil {
		return nil, err
	}
	isStudent := make(map[uint]bool, len(students))
	for _, e := range students {
		isStudent[e.UserID] = true
	}

	submissionResult, err := s.submissionRepo.BulkListByCourse(ctx, courseID, repository.PaginationParams{Page: 1, PerPage: 100000})
	if err != nil {
		return nil, err
	}
	subs := map[uint]map[uint]*models.Submission{} // assignment -> user -> submission
	for i := range submissionResult.Items {
		sub := &submissionResult.Items[i]
		if !isStudent[sub.UserID] {
			continue
		}
		if subs[sub.AssignmentID] == nil {
			subs[sub.AssignmentID] = map[uint]*models.Submission{}
		}
		subs[sub.AssignmentID][sub.UserID] = sub
	}

	now := s.now()
	stats := make([]map[string]interface{}, 0, len(assignments))
	for i := range assignments {
		a := &assignments[i]
		var scores []float64
		var breakdown TardinessBreakdown
		submitted := 0
		for _, st := range students {
			sub := subs[a.ID][st.UserID]
			breakdown.add(SubmissionTardiness(a, sub, now))
			if sub != nil && sub.SubmittedAt != nil {
				submitted++
			}
			if sub != nil && sub.Score != nil && sub.PostedAt != nil && !sub.Excused {
				scores = append(scores, *sub.Score)
			}
		}
		stat := map[string]interface{}{
			"assignment_id":    a.ID,
			"title":            a.Name,
			"points_possible":  a.PointsPossible,
			"due_at":           a.DueAt,
			"submission_count": submitted,
			"graded_count":     len(scores),
			"tardiness":        breakdown,
			"min_score":        nil,
			"max_score":        nil,
			"avg_score":        nil,
			"median":           nil,
			"first_quartile":   nil,
			"third_quartile":   nil,
		}
		if len(scores) > 0 {
			sort.Float64s(scores)
			stat["min_score"] = scores[0]
			stat["max_score"] = scores[len(scores)-1]
			stat["avg_score"] = round2(mean(scores))
			stat["median"] = round2(quantile(scores, 0.5))
			stat["first_quartile"] = round2(quantile(scores, 0.25))
			stat["third_quartile"] = round2(quantile(scores, 0.75))
		}
		stats = append(stats, stat)
	}
	return stats, nil
}

// quantile is the linear-interpolated q-quantile of sorted xs.
func quantile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	pos := q * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	return sorted[lo] + (sorted[hi]-sorted[lo])*(pos-float64(lo))
}

// StudentSummary is one student's row in course analytics.
type StudentSummary struct {
	ID                 uint               `json:"id"`
	Name               string             `json:"name"`
	SortableName       string             `json:"sortable_name"`
	PageViews          int64              `json:"page_views"`
	Participations     int64              `json:"participations"`
	InteractionSeconds int64              `json:"interaction_seconds"`
	LastActivityAt     *time.Time         `json:"last_activity_at"`
	Submissions        int                `json:"submissions"`
	Tardiness          TardinessBreakdown `json:"tardiness_breakdown"`
	// Scores keyed by assignment id (posted scores only) so the client can
	// filter "students who scored more/less than N on X".
	Scores map[uint]*float64 `json:"scores"`
	// Statuses keyed by assignment id (on_time / late / missing / ...).
	Statuses map[uint]string `json:"statuses"`
}

// GetStudentSummaries returns analytics summaries for the course's students
// (page views, participations, last activity, submission tardiness), sorted
// by name and paged. It returns the page and the total student count.
func (s *AnalyticsService) GetStudentSummaries(ctx context.Context, courseID, accountID uint, params repository.PaginationParams) ([]StudentSummary, int64, error) {
	students, err := s.courseStudents(ctx, courseID, accountID)
	if err != nil {
		return nil, 0, err
	}
	total := int64(len(students))
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PerPage < 1 {
		params.PerPage = 50
	}
	start := (params.Page - 1) * params.PerPage
	if start > len(students) {
		start = len(students)
	}
	end := start + params.PerPage
	if end > len(students) {
		end = len(students)
	}
	page := students[start:end]

	totals, err := s.pageViewRepo.TotalsByUser(ctx, courseID)
	if err != nil {
		return nil, 0, err
	}
	assignments, err := s.publishedAssignments(ctx, courseID)
	if err != nil {
		return nil, 0, err
	}
	submissionResult, err := s.submissionRepo.BulkListByCourse(ctx, courseID, repository.PaginationParams{Page: 1, PerPage: 100000})
	if err != nil {
		return nil, 0, err
	}
	subs := map[uint]map[uint]*models.Submission{} // user -> assignment -> submission
	for i := range submissionResult.Items {
		sub := &submissionResult.Items[i]
		if subs[sub.UserID] == nil {
			subs[sub.UserID] = map[uint]*models.Submission{}
		}
		subs[sub.UserID][sub.AssignmentID] = sub
	}

	now := s.now()
	out := make([]StudentSummary, 0, len(page))
	for _, e := range page {
		sum := StudentSummary{ID: e.UserID, Scores: map[uint]*float64{}, Statuses: map[uint]string{}}
		if e.User != nil {
			sum.Name = e.User.Name
			sum.SortableName = e.User.SortableName
		}
		if t, ok := totals[e.UserID]; ok {
			sum.PageViews = t.Views
			sum.Participations = t.Participations
			sum.InteractionSeconds = t.InteractionSeconds
			sum.LastActivityAt = t.LastViewedAt
		}
		if e.LastActivityAt != nil && (sum.LastActivityAt == nil || e.LastActivityAt.After(*sum.LastActivityAt)) {
			sum.LastActivityAt = e.LastActivityAt
		}
		for i := range assignments {
			a := &assignments[i]
			sub := subs[e.UserID][a.ID]
			status := SubmissionTardiness(a, sub, now)
			sum.Tardiness.add(status)
			sum.Statuses[a.ID] = status
			if sub != nil && sub.SubmittedAt != nil {
				sum.Submissions++
			}
			if sub != nil && sub.Score != nil && sub.PostedAt != nil {
				v := *sub.Score
				sum.Scores[a.ID] = &v
			}
		}
		out = append(out, sum)
	}
	return out, total, nil
}

// StudentActivity is the per-student analytics drill-down.
type StudentActivity struct {
	UserID     uint                               `json:"user_id"`
	ByDay      []repository.PageViewDayCount      `json:"by_day"`
	Categories []repository.PageViewCategoryCount `json:"categories"`
	Recent     []models.PageView                  `json:"recent_page_views"`
	Totals     repository.PageViewUserTotals      `json:"totals"`
}

// GetStudentActivity returns one student's activity in the course: page
// views and participations per day, by content category, and the most
// recent page views. The student must have a (non-deleted) student
// enrollment in the course; any other user is reported as not found.
func (s *AnalyticsService) GetStudentActivity(ctx context.Context, courseID, userID, accountID uint) (*StudentActivity, error) {
	if !s.isCourseStudent(ctx, courseID, userID, accountID) {
		return nil, ErrAnalyticsStudentNotFound
	}
	byDay, err := s.pageViewRepo.ActivityByDay(ctx, courseID, userID, nil)
	if err != nil {
		return nil, err
	}
	cats, err := s.pageViewRepo.CategoryCounts(ctx, courseID, userID)
	if err != nil {
		return nil, err
	}
	recent, err := s.pageViewRepo.ListByUserAndContext(ctx, userID, "Course", courseID, repository.PaginationParams{Page: 1, PerPage: 50})
	if err != nil {
		return nil, err
	}
	out := &StudentActivity{UserID: userID, ByDay: byDay, Categories: cats, Recent: recent.Items}
	if totals, err := s.pageViewRepo.TotalsByUser(ctx, courseID); err == nil {
		out.Totals = totals[userID]
	}
	if out.Recent == nil {
		out.Recent = []models.PageView{}
	}
	return out, nil
}

// isCourseStudent reports whether userID has a non-deleted student
// enrollment in the course (in the caller's tenant).
func (s *AnalyticsService) isCourseStudent(ctx context.Context, courseID, userID, accountID uint) bool {
	students, err := s.courseStudents(ctx, courseID, accountID)
	if err != nil {
		return false
	}
	for _, e := range students {
		if e.UserID == userID {
			return true
		}
	}
	return false
}

// GetStudentAssignments returns one student's assignments in the course with
// their submission and tardiness status. Non-students are not found.
func (s *AnalyticsService) GetStudentAssignments(ctx context.Context, courseID, userID, accountID uint) ([]map[string]interface{}, error) {
	if !s.isCourseStudent(ctx, courseID, userID, accountID) {
		return nil, ErrAnalyticsStudentNotFound
	}
	assignments, err := s.publishedAssignments(ctx, courseID)
	if err != nil {
		return nil, err
	}
	submissions, err := s.submissionRepo.ListByUserAndCourse(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}
	subByAssignment := make(map[uint]*models.Submission)
	for i := range submissions {
		subByAssignment[submissions[i].AssignmentID] = &submissions[i]
	}

	now := s.now()
	results := make([]map[string]interface{}, 0, len(assignments))
	for i := range assignments {
		a := &assignments[i]
		sub := subByAssignment[a.ID]
		entry := map[string]interface{}{
			"assignment_id":   a.ID,
			"title":           a.Name,
			"points_possible": a.PointsPossible,
			"due_at":          a.DueAt,
			"status":          SubmissionTardiness(a, sub, now),
			"submission":      nil,
		}
		if sub != nil {
			// Only a posted grade is shown (a hidden grade is not yet
			// part of the student's record).
			var score *float64
			if sub.PostedAt != nil {
				score = sub.Score
			}
			entry["submission"] = map[string]interface{}{
				"score":          score,
				"submitted_at":   sub.SubmittedAt,
				"workflow_state": sub.WorkflowState,
				"late":           sub.Late,
				"missing":        sub.Missing,
				"excused":        sub.Excused,
			}
		}
		results = append(results, entry)
	}
	return results, nil
}

// GetDepartmentActivity returns placeholder activity stats for a department/account.
func (s *AnalyticsService) GetDepartmentActivity(ctx context.Context, accountID uint) ([]map[string]interface{}, error) {
	return []map[string]interface{}{
		{
			"date":           "2024-01-01",
			"views":          0,
			"participations": 0,
		},
	}, nil
}

// GetDepartmentGrades returns placeholder grade distribution for a department/account.
func (s *AnalyticsService) GetDepartmentGrades(ctx context.Context, accountID uint) ([]map[string]interface{}, error) {
	return []map[string]interface{}{
		{
			"score_bucket": "90-100",
			"count":        0,
		},
	}, nil
}

// GetDepartmentStatistics returns placeholder statistics for a department/account.
func (s *AnalyticsService) GetDepartmentStatistics(ctx context.Context, accountID uint) (map[string]interface{}, error) {
	return map[string]interface{}{
		"courses":     0,
		"teachers":    0,
		"students":    0,
		"assignments": 0,
		"submissions": 0,
	}, nil
}

// RecordPageView records a page view the web client reports. Only course
// page views of users enrolled in that course (in the caller's tenant) are
// kept; anything else returns ErrPageViewNotTracked. Participation is never
// taken from the client — the server records it (RecordParticipation).
func (s *AnalyticsService) RecordPageView(ctx context.Context, pageView *models.PageView, accountID uint) error {
	if pageView.UserID == 0 || pageView.ContextType != "Course" || pageView.ContextID == 0 {
		return ErrPageViewNotTracked
	}
	if _, err := s.enrollmentRepo.FindByUserAndCourse(ctx, pageView.UserID, pageView.ContextID, accountID); err != nil {
		return ErrPageViewNotTracked
	}
	pageView.Participated = false
	if pageView.InteractionSeconds < 0 {
		pageView.InteractionSeconds = 0
	}
	if pageView.InteractionSeconds > maxInteractionSeconds {
		pageView.InteractionSeconds = maxInteractionSeconds
	}
	if pageView.AssetCategory == "" {
		pageView.AssetCategory = AnalyticsCategoryForPath(pageView.URL)
	}
	if err := s.pageViewRepo.Create(ctx, pageView); err != nil {
		return err
	}
	_ = s.pageViewRepo.TouchEnrollmentActivity(ctx, pageView.UserID, pageView.ContextID)
	return nil
}

// UpdatePageViewInteraction records how long the user spent on a page view
// they reported earlier (capped). Another user's page view is not found.
func (s *AnalyticsService) UpdatePageViewInteraction(ctx context.Context, id, userID uint, seconds int) error {
	if seconds < 0 {
		seconds = 0
	}
	if seconds > maxInteractionSeconds {
		seconds = maxInteractionSeconds
	}
	return s.pageViewRepo.UpdateInteraction(ctx, id, userID, seconds)
}

// RecordParticipation records a participation — a student's successful
// write in a course (a submission, a discussion post, a quiz answer...).
// Only active student enrollments count; a repeated write to the same URL
// within participationWindow counts once. Errors are swallowed: analytics
// must never fail the request that triggered it.
func (s *AnalyticsService) RecordParticipation(ctx context.Context, userID, courseID, accountID uint, method, path, routePath string) {
	if userID == 0 || courseID == 0 {
		return
	}
	key := strings.Join([]string{strconv.FormatUint(uint64(userID), 10), strconv.FormatUint(uint64(courseID), 10), method, path}, "|")
	now := s.now()
	s.partMu.Lock()
	if last, ok := s.partSeen[key]; ok && now.Sub(last) < participationWindow {
		s.partMu.Unlock()
		return
	}
	s.partSeen[key] = now
	if len(s.partSeen) > 10000 {
		for k, t := range s.partSeen {
			if now.Sub(t) >= participationWindow {
				delete(s.partSeen, k)
			}
		}
	}
	s.partMu.Unlock()

	enrollment, err := s.enrollmentRepo.FindByUserAndCourse(ctx, userID, courseID, accountID)
	if err != nil || enrollment == nil || enrollment.Type != "StudentEnrollment" {
		return
	}
	if len(path) > 2000 {
		path = path[:2000]
	}
	pv := &models.PageView{
		UserID:        userID,
		ContextType:   "Course",
		ContextID:     courseID,
		URL:           path,
		Action:        strings.ToLower(method) + " " + routePath,
		Participated:  true,
		AssetCategory: AnalyticsCategoryForPath(routePath),
	}
	if err := s.pageViewRepo.Create(ctx, pv); err != nil {
		return
	}
	_ = s.pageViewRepo.TouchEnrollmentActivity(ctx, userID, courseID)
}

// GetUserPageViews returns a paginated list of page views for a user.
// accountID==0 disables the tenant scope (privileged internal callers).
func (s *AnalyticsService) GetUserPageViews(ctx context.Context, userID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PageView], error) {
	return s.pageViewRepo.ListByUserID(ctx, userID, accountID, params)
}
