package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// LearnerDashboardService powers the student dashboard: course cards with
// the user's favorites / nicknames / colors, the recent activity stream and
// the missing-submissions list.
type LearnerDashboardService struct {
	learner       repository.LearnerRepository
	enrollments   repository.EnrollmentRepository
	announcements PlannerAnnouncementVisibility
	dueDates      StudentDueDateResolver
}

func NewLearnerDashboardService(learner repository.LearnerRepository, enrollments repository.EnrollmentRepository) *LearnerDashboardService {
	return &LearnerDashboardService{learner: learner, enrollments: enrollments}
}

// SetAnnouncementVisibility applies announcement audience targeting to the
// activity stream (optional; without it no announcements are streamed).
func (s *LearnerDashboardService) SetAnnouncementVisibility(v PlannerAnnouncementVisibility) *LearnerDashboardService {
	s.announcements = v
	return s
}

// SetDueDateResolver wires the gradebook's effective-date resolver
// (overrides, accommodations, assign-to visibility) for missing work.
func (s *LearnerDashboardService) SetDueDateResolver(r StudentDueDateResolver) *LearnerDashboardService {
	s.dueDates = r
	return s
}

// ErrDashboardCourseNotFound: the course is not one the caller is actively
// enrolled in (handlers answer 404).
var ErrDashboardCourseNotFound = errors.New("course not found")

// ErrInvalidColor: a course color must be a #rrggbb hex code.
var ErrInvalidColor = errors.New("hexcode must be a color like #1a2b3c")

var hexColorRe = regexp.MustCompile(`^#?[0-9a-fA-F]{6}$`)

// NormalizeHexColor validates and canonicalizes a color to "#rrggbb".
func NormalizeHexColor(in string) (string, error) {
	in = strings.TrimSpace(in)
	if !hexColorRe.MatchString(in) {
		return "", ErrInvalidColor
	}
	return "#" + strings.ToLower(strings.TrimPrefix(in, "#")), nil
}

// DashboardCard is one course tile.
type DashboardCard struct {
	ID              uint     `json:"id"`
	LongName        string   `json:"longName"`
	ShortName       string   `json:"shortName"`
	OriginalName    string   `json:"originalName"`
	Nickname        *string  `json:"nickname"`
	CourseCode      string   `json:"courseCode"`
	AssetString     string   `json:"assetString"`
	Href            string   `json:"href"`
	Term            string   `json:"term"`
	Color           *string  `json:"color"`
	Position        *int     `json:"position"`
	IsFavorited     bool     `json:"isFavorited"`
	Published       bool     `json:"published"`
	EnrollmentTypes []string `json:"enrollmentTypes"`
	IsStaff         bool     `json:"isStaff"`
	// Look is the course's Page Designer look (courses.look; null when
	// unset), so the card can use the course banner and colours.
	Look models.NullJSON `json:"look"`
}

// userCourses returns the caller's active enrollments grouped by course and
// the course rows (tenant scoped). Observers' observer enrollments count —
// an observer's dashboard shows the courses they observe.
func (s *LearnerDashboardService) userCourses(ctx context.Context, userID, accountID uint) (map[uint][]models.Enrollment, map[uint]models.Course, error) {
	enrollments, err := s.enrollments.ListByUserID(ctx, userID, accountID)
	if err != nil {
		return nil, nil, err
	}
	byCourse := map[uint][]models.Enrollment{}
	var ids []uint
	for _, e := range enrollments {
		if e.WorkflowState != models.EnrollmentActive {
			continue
		}
		if _, ok := byCourse[e.CourseID]; !ok {
			ids = append(ids, e.CourseID)
		}
		byCourse[e.CourseID] = append(byCourse[e.CourseID], e)
	}
	courses, err := s.learner.CoursesByIDs(ctx, ids, accountID)
	if err != nil {
		return nil, nil, err
	}
	out := map[uint]models.Course{}
	for _, c := range courses {
		staff := false
		for _, e := range byCourse[c.ID] {
			if e.Type == "TeacherEnrollment" || e.Type == "TaEnrollment" || e.Type == "DesignerEnrollment" {
				staff = true
			}
		}
		// Unpublished courses are visible only to their staff.
		if c.WorkflowState == models.CourseAvailable || c.WorkflowState == models.CourseCompleted || staff {
			out[c.ID] = c
		}
	}
	return byCourse, out, nil
}

// requireCourse checks the caller holds an active enrollment in courseID.
func (s *LearnerDashboardService) requireCourse(ctx context.Context, userID, accountID, courseID uint) error {
	_, courses, err := s.userCourses(ctx, userID, accountID)
	if err != nil {
		return err
	}
	if _, ok := courses[courseID]; !ok {
		return ErrDashboardCourseNotFound
	}
	return nil
}

// Cards returns the dashboard course cards. Like Canvas, once the user has
// favorited any course the dashboard shows only favorites; all=true returns
// every course (the "All courses" list where favorites are picked).
func (s *LearnerDashboardService) Cards(ctx context.Context, userID, accountID uint, all bool) ([]DashboardCard, error) {
	byCourse, courses, err := s.userCourses(ctx, userID, accountID)
	if err != nil {
		return nil, err
	}
	prefs, err := s.learner.ListCoursePreferences(ctx, userID)
	if err != nil {
		return nil, err
	}
	prefByCourse := map[uint]models.UserCoursePreference{}
	anyFavorite := false
	for _, p := range prefs {
		if _, ok := courses[p.CourseID]; !ok {
			continue
		}
		prefByCourse[p.CourseID] = p
		if p.Favorite {
			anyFavorite = true
		}
	}
	var termIDs []uint
	for _, c := range courses {
		if c.EnrollmentTermID != nil {
			termIDs = append(termIDs, *c.EnrollmentTermID)
		}
	}
	terms, _ := s.learner.EnrollmentTermsByIDs(ctx, termIDs)
	termName := map[uint]string{}
	for _, t := range terms {
		termName[t.ID] = t.Name
	}

	cards := make([]DashboardCard, 0, len(courses))
	for id, c := range courses {
		p, hasPref := prefByCourse[id]
		fav := hasPref && p.Favorite
		if anyFavorite && !all && !fav {
			continue
		}
		card := DashboardCard{
			ID:           c.ID,
			LongName:     c.Name,
			ShortName:    c.Name,
			OriginalName: c.Name,
			CourseCode:   c.CourseCode,
			AssetString:  fmt.Sprintf("course_%d", c.ID),
			Href:         fmt.Sprintf("/courses/%d", c.ID),
			IsFavorited:  fav,
			Published:    c.WorkflowState == models.CourseAvailable,
			Look:         c.Look,
		}
		if c.EnrollmentTermID != nil {
			card.Term = termName[*c.EnrollmentTermID]
		}
		if hasPref {
			if p.Nickname != nil && *p.Nickname != "" {
				nick := *p.Nickname
				card.Nickname = &nick
				card.ShortName = nick
			}
			card.Color = p.Color
			card.Position = p.Position
		}
		seen := map[string]bool{}
		for _, e := range byCourse[id] {
			if !seen[e.Type] {
				seen[e.Type] = true
				card.EnrollmentTypes = append(card.EnrollmentTypes, e.Type)
			}
			if e.Type == "TeacherEnrollment" || e.Type == "TaEnrollment" || e.Type == "DesignerEnrollment" {
				card.IsStaff = true
			}
		}
		sort.Strings(card.EnrollmentTypes)
		cards = append(cards, card)
	}
	sort.SliceStable(cards, func(i, j int) bool {
		pi, pj := cards[i].Position, cards[j].Position
		if (pi != nil) != (pj != nil) {
			return pi != nil
		}
		if pi != nil && pj != nil && *pi != *pj {
			return *pi < *pj
		}
		ni, nj := strings.ToLower(cards[i].ShortName), strings.ToLower(cards[j].ShortName)
		if ni != nj {
			return ni < nj
		}
		return cards[i].ID < cards[j].ID
	})
	return cards, nil
}

// prefFor loads the user's preference row for a course, or a fresh one.
func (s *LearnerDashboardService) prefFor(ctx context.Context, userID, courseID uint) (*models.UserCoursePreference, error) {
	p, err := s.learner.GetCoursePreference(ctx, userID, courseID)
	if errors.Is(err, repository.ErrLearnerNotFound) {
		return &models.UserCoursePreference{UserID: userID, CourseID: courseID}, nil
	}
	return p, err
}

// SetFavorite favorites / unfavorites a course the caller is enrolled in.
func (s *LearnerDashboardService) SetFavorite(ctx context.Context, userID, accountID, courseID uint, favorite bool) (*models.UserCoursePreference, error) {
	if err := s.requireCourse(ctx, userID, accountID, courseID); err != nil {
		return nil, err
	}
	p, err := s.prefFor(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}
	p.Favorite = favorite
	return p, s.learner.SaveCoursePreference(ctx, p)
}

// ListFavoriteCourseIDs returns the favorited courses the caller can still see.
func (s *LearnerDashboardService) ListFavoriteCourseIDs(ctx context.Context, userID, accountID uint) ([]uint, error) {
	cards, err := s.Cards(ctx, userID, accountID, true)
	if err != nil {
		return nil, err
	}
	ids := []uint{}
	for _, c := range cards {
		if c.IsFavorited {
			ids = append(ids, c.ID)
		}
	}
	return ids, nil
}

// SetNickname sets (or with "" clears) the caller's nickname for a course.
func (s *LearnerDashboardService) SetNickname(ctx context.Context, userID, accountID, courseID uint, nickname string) (*models.UserCoursePreference, error) {
	if err := s.requireCourse(ctx, userID, accountID, courseID); err != nil {
		return nil, err
	}
	nickname = strings.TrimSpace(SanitizePlainText(nickname))
	if len([]rune(nickname)) > 60 {
		return nil, errors.New("nickname must be 60 characters or fewer")
	}
	p, err := s.prefFor(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}
	if nickname == "" {
		p.Nickname = nil
	} else {
		p.Nickname = &nickname
	}
	return p, s.learner.SaveCoursePreference(ctx, p)
}

// SetColor sets (or with "" clears) the caller's card color for a course.
func (s *LearnerDashboardService) SetColor(ctx context.Context, userID, accountID, courseID uint, hex string) (*models.UserCoursePreference, error) {
	if err := s.requireCourse(ctx, userID, accountID, courseID); err != nil {
		return nil, err
	}
	p, err := s.prefFor(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(hex) == "" {
		p.Color = nil
	} else {
		c, err := NormalizeHexColor(hex)
		if err != nil {
			return nil, err
		}
		p.Color = &c
	}
	return p, s.learner.SaveCoursePreference(ctx, p)
}

// SetPositions stores the dashboard card order (course ids, first = top).
// Ids the caller is not enrolled in are ignored.
func (s *LearnerDashboardService) SetPositions(ctx context.Context, userID, accountID uint, order []uint) error {
	_, courses, err := s.userCourses(ctx, userID, accountID)
	if err != nil {
		return err
	}
	pos := 0
	for _, id := range order {
		if _, ok := courses[id]; !ok {
			continue
		}
		p, err := s.prefFor(ctx, userID, id)
		if err != nil {
			return err
		}
		n := pos
		p.Position = &n
		if err := s.learner.SaveCoursePreference(ctx, p); err != nil {
			return err
		}
		pos++
	}
	return nil
}

// Preferences returns the caller's preferences for the courses they can see
// (nicknames and colors endpoints).
func (s *LearnerDashboardService) Preferences(ctx context.Context, userID, accountID uint) ([]models.UserCoursePreference, map[uint]models.Course, error) {
	_, courses, err := s.userCourses(ctx, userID, accountID)
	if err != nil {
		return nil, nil, err
	}
	prefs, err := s.learner.ListCoursePreferences(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	kept := prefs[:0]
	for _, p := range prefs {
		if _, ok := courses[p.CourseID]; ok {
			kept = append(kept, p)
		}
	}
	return kept, courses, nil
}

// --- Activity stream ---

// ActivityStreamItem is one entry of the recent activity stream (Canvas
// /users/self/activity_stream shape, trimmed to what Paper produces).
type ActivityStreamItem struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"` // Announcement | DiscussionTopic | Submission | SubmissionComment
	Title      string    `json:"title"`
	Message    string    `json:"message,omitempty"`
	CourseID   uint      `json:"course_id"`
	CourseName string    `json:"context_name"`
	HTMLURL    string    `json:"html_url"`
	CreatedAt  time.Time `json:"created_at"`
	Score      *float64  `json:"score,omitempty"`
	Grade      *string   `json:"grade,omitempty"`
	Excused    bool      `json:"excused,omitempty"`
	Points     *float64  `json:"points_possible,omitempty"`
	AuthorName string    `json:"author_name,omitempty"`
}

// ActivityStream returns the newest activity across the caller's courses
// over the last `days` days (default 28), newest first, at most limit items.
func (s *LearnerDashboardService) ActivityStream(ctx context.Context, userID, accountID uint, days, limit int) ([]ActivityStreamItem, error) {
	if days <= 0 {
		days = 28
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	since := time.Now().AddDate(0, 0, -days)
	_, courses, err := s.userCourses(ctx, userID, accountID)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(courses))
	for id := range courses {
		ids = append(ids, id)
	}
	items := []ActivityStreamItem{}

	if s.announcements != nil {
		anns, err := s.learner.RecentAnnouncements(ctx, ids, since)
		if err != nil {
			return nil, err
		}
		for i := range anns {
			a := &anns[i]
			if a.CourseID == nil || !s.announcements.CanView(ctx, a, userID) {
				continue
			}
			when := a.CreatedAt
			if a.PostedAt != nil {
				when = *a.PostedAt
			}
			items = append(items, ActivityStreamItem{
				ID: fmt.Sprintf("announcement_%d", a.ID), Type: "Announcement", Title: a.Title,
				Message:  SanitizePlainText(a.Message),
				CourseID: *a.CourseID, CourseName: courses[*a.CourseID].Name,
				HTMLURL: fmt.Sprintf("/courses/%d/announcements", *a.CourseID), CreatedAt: when,
			})
		}
	}

	topics, err := s.learner.RecentDiscussionTopics(ctx, ids, since)
	if err != nil {
		return nil, err
	}
	for _, t := range topics {
		when := t.CreatedAt
		if t.PostedAt != nil {
			when = *t.PostedAt
		}
		items = append(items, ActivityStreamItem{
			ID: fmt.Sprintf("discussion_topic_%d", t.ID), Type: "DiscussionTopic", Title: t.Title,
			CourseID: t.CourseID, CourseName: courses[t.CourseID].Name,
			HTMLURL: fmt.Sprintf("/courses/%d/discussions/%d", t.CourseID, t.ID), CreatedAt: when,
		})
	}

	graded, err := s.learner.RecentlyPostedSubmissions(ctx, userID, ids, since)
	if err != nil {
		return nil, err
	}
	comments, err := s.learner.RecentCommentsOnSubmissions(ctx, userID, ids, since)
	if err != nil {
		return nil, err
	}
	aIDs := map[uint]bool{}
	for _, sub := range graded {
		aIDs[sub.AssignmentID] = true
	}
	for _, c := range comments {
		aIDs[c.AssignmentID] = true
	}
	assignmentByID := map[uint]models.Assignment{}
	if len(aIDs) > 0 {
		all, err := s.learner.PublishedAssignments(ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, a := range all {
			if aIDs[a.ID] {
				assignmentByID[a.ID] = a
			}
		}
	}
	for _, sub := range graded {
		a, ok := assignmentByID[sub.AssignmentID]
		if !ok {
			continue
		}
		when := *sub.PostedAt
		if sub.GradedAt != nil && sub.GradedAt.After(when) {
			when = *sub.GradedAt
		}
		items = append(items, ActivityStreamItem{
			ID: fmt.Sprintf("submission_%d", sub.ID), Type: "Submission", Title: a.Name,
			CourseID: a.CourseID, CourseName: courses[a.CourseID].Name,
			HTMLURL: fmt.Sprintf("/courses/%d/assignments/%d", a.CourseID, a.ID), CreatedAt: when,
			Score: sub.Score, Grade: sub.Grade, Excused: sub.Excused, Points: a.PointsPossible,
		})
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
	for _, c := range comments {
		a, ok := assignmentByID[c.AssignmentID]
		if !ok {
			continue
		}
		items = append(items, ActivityStreamItem{
			ID: fmt.Sprintf("submission_comment_%d", c.CommentID), Type: "SubmissionComment", Title: a.Name,
			Message:  SanitizePlainText(c.Comment),
			CourseID: c.CourseID, CourseName: courses[c.CourseID].Name,
			HTMLURL: fmt.Sprintf("/courses/%d/assignments/%d", c.CourseID, c.AssignmentID), CreatedAt: c.CreatedAt,
			AuthorName: authorName[c.AuthorID],
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		return items[i].ID < items[j].ID
	})
	if len(items) > limit {
		items = items[:limit]
	}
	for i := range items {
		if len(items[i].Message) > 400 {
			items[i].Message = items[i].Message[:400] + "…"
		}
	}
	return items, nil
}

// ActivitySummary counts stream items per type.
func ActivitySummary(items []ActivityStreamItem) []map[string]any {
	counts := map[string]int{}
	var order []string
	for _, it := range items {
		if _, ok := counts[it.Type]; !ok {
			order = append(order, it.Type)
		}
		counts[it.Type]++
	}
	sort.Strings(order)
	out := make([]map[string]any, 0, len(order))
	for _, t := range order {
		out = append(out, map[string]any{"type": t, "count": counts[t]})
	}
	return out
}

// --- Missing submissions ---

// MissingSubmission is one past-due assignment the student has not turned in.
type MissingSubmission struct {
	ID             uint       `json:"id"`
	Name           string     `json:"name"`
	CourseID       uint       `json:"course_id"`
	CourseName     string     `json:"course_name"`
	DueAt          *time.Time `json:"due_at"`
	PointsPossible *float64   `json:"points_possible"`
	HTMLURL        string     `json:"html_url"`
	// Dismissed is true when the student dismissed it from the list
	// (planner override); it still counts as missing.
	Dismissed bool `json:"dismissed"`
}

// MissingSubmissions lists the student's missing work. courseFilter, when
// non-nil, limits it to those course ids (an observer's observed courses).
func (s *LearnerDashboardService) MissingSubmissions(ctx context.Context, studentID, accountID uint, courseFilter map[uint]bool, overrides []models.PlannerOverride) ([]MissingSubmission, error) {
	enrollments, err := s.enrollments.ListByUserID(ctx, studentID, accountID)
	if err != nil {
		return nil, err
	}
	var courseIDs, sectionIDs []uint
	seen := map[uint]bool{}
	for _, e := range enrollments {
		if e.WorkflowState != models.EnrollmentActive || e.Type != "StudentEnrollment" {
			continue
		}
		if courseFilter != nil && !courseFilter[e.CourseID] {
			continue
		}
		if e.CourseSectionID != nil {
			sectionIDs = append(sectionIDs, *e.CourseSectionID)
		}
		if !seen[e.CourseID] {
			seen[e.CourseID] = true
			courseIDs = append(courseIDs, e.CourseID)
		}
	}
	courses, err := s.learner.CoursesByIDs(ctx, courseIDs, accountID)
	if err != nil {
		return nil, err
	}
	live := map[uint]models.Course{}
	var liveIDs []uint
	var liveList []models.Course
	for _, c := range courses {
		if c.WorkflowState == models.CourseAvailable {
			live[c.ID] = c
			liveIDs = append(liveIDs, c.ID)
			liveList = append(liveList, c)
		}
	}
	assignments, err := s.learner.PublishedAssignments(ctx, liveIDs)
	if err != nil {
		return nil, err
	}
	aIDs := make([]uint, len(assignments))
	for i, a := range assignments {
		aIDs[i] = a.ID
	}
	dates, err := loadStudentDates(ctx, s.dueDates, s.learner, studentID, sectionIDs, liveList, aIDs)
	if err != nil {
		return nil, err
	}
	subs, err := s.learner.SubmissionsForUser(ctx, studentID, aIDs)
	if err != nil {
		return nil, err
	}
	subBy := map[uint]*models.Submission{}
	for i := range subs {
		subBy[subs[i].AssignmentID] = &subs[i]
	}
	dismissed := map[uint]bool{}
	for _, o := range overrides {
		if o.PlannableType == "assignment" && o.Dismissed {
			dismissed[o.PlannableID] = true
		}
	}
	now := time.Now()
	out := []MissingSubmission{}
	for i := range assignments {
		a := &assignments[i]
		due, assigned := dates.due(a)
		if !assigned || !isMissing(a, due, subBy[a.ID], now) {
			continue
		}
		out = append(out, MissingSubmission{
			ID: a.ID, Name: a.Name, CourseID: a.CourseID, CourseName: live[a.CourseID].Name,
			DueAt: due, PointsPossible: a.PointsPossible,
			HTMLURL:   fmt.Sprintf("/courses/%d/assignments/%d", a.CourseID, a.ID),
			Dismissed: dismissed[a.ID],
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		di, dj := out[i].DueAt, out[j].DueAt
		if di != nil && dj != nil && !di.Equal(*dj) {
			return di.Before(*dj)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}
