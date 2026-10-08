package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

type analyticsMocks struct {
	pageViews   *mocks.MockPageViewRepository
	submissions *mocks.MockSubmissionRepository
	enrollments *mocks.MockEnrollmentRepository
	assignments *mocks.MockAssignmentRepository
	svc         *service.AnalyticsService
}

func newAnalyticsMocks() *analyticsMocks {
	m := &analyticsMocks{
		pageViews:   new(mocks.MockPageViewRepository),
		submissions: new(mocks.MockSubmissionRepository),
		enrollments: new(mocks.MockEnrollmentRepository),
		assignments: new(mocks.MockAssignmentRepository),
	}
	m.svc = service.NewAnalyticsService(m.pageViews, m.submissions, m.enrollments, m.assignments)
	return m
}

func f64(v float64) *float64 { return &v }

// B209 + G154: per-student page views in a course's analytics count only
// that course's page views (one grouped query, not a site-wide total per
// student), and the summary carries participations and tardiness.
func TestGetStudentSummaries_CourseScopedTotalsAndTardiness(t *testing.T) {
	m := newAnalyticsMocks()
	past := time.Now().Add(-48 * time.Hour)
	future := time.Now().Add(48 * time.Hour)
	submittedLate := time.Now().Add(-24 * time.Hour)
	posted := time.Now()

	m.enrollments.On("ListByCourseID", mock.Anything, uint(30), uint(7), mock.Anything).Return(&repository.PaginatedResult[models.Enrollment]{
		Items: []models.Enrollment{
			{UserID: 4, CourseID: 30, Type: "StudentEnrollment", User: &models.User{ID: 4, Name: "Zed Doe"}},
			{UserID: 9, CourseID: 30, Type: "TeacherEnrollment", User: &models.User{ID: 9, Name: "Pat Teacher"}},
			{UserID: 5, CourseID: 30, Type: "StudentEnrollment", User: &models.User{ID: 5, Name: "Ann Doe"}},
		},
		TotalCount: 3,
	}, nil)
	m.pageViews.On("TotalsByUser", mock.Anything, uint(30)).Return(map[uint]repository.PageViewUserTotals{
		4: {Views: 12, Participations: 3, InteractionSeconds: 600},
	}, nil)
	m.assignments.On("ListByCourseID", mock.Anything, uint(30), mock.Anything).Return(&repository.PaginatedResult[models.Assignment]{
		Items: []models.Assignment{
			{ID: 1, CourseID: 30, Name: "Essay", WorkflowState: models.AssignmentPublished, DueAt: &past, SubmissionTypes: "online_upload"},
			{ID: 2, CourseID: 30, Name: "Project", WorkflowState: models.AssignmentPublished, DueAt: &future, SubmissionTypes: "online_text_entry"},
			{ID: 3, CourseID: 30, Name: "Draft", WorkflowState: models.AssignmentUnpublished, DueAt: &past, SubmissionTypes: "online_upload"},
		},
	}, nil)
	m.submissions.On("BulkListByCourse", mock.Anything, uint(30), mock.Anything).Return(&repository.PaginatedResult[models.Submission]{
		Items: []models.Submission{
			{AssignmentID: 1, UserID: 4, SubmittedAt: &submittedLate, Late: true, Score: f64(8), PostedAt: &posted},
		},
	}, nil)

	summaries, total, err := m.svc.GetStudentSummaries(context.Background(), 30, 7, repository.PaginationParams{Page: 1, PerPage: 50})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total, "teachers are not students")
	require.Len(t, summaries, 2)

	// Sorted by name: Ann (5) then Zed (4).
	assert.Equal(t, uint(5), summaries[0].ID)
	assert.Equal(t, int64(0), summaries[0].PageViews)
	assert.Equal(t, 1, summaries[0].Tardiness.Missing, "past-due essay never turned in is missing")
	assert.Equal(t, 1, summaries[0].Tardiness.Floating, "project not yet due")
	assert.Equal(t, 2, summaries[0].Tardiness.Total, "unpublished assignments are not counted")

	zed := summaries[1]
	assert.Equal(t, int64(12), zed.PageViews)
	assert.Equal(t, int64(3), zed.Participations)
	assert.Equal(t, 1, zed.Tardiness.Late)
	assert.Equal(t, 1, zed.Submissions)
	require.NotNil(t, zed.Scores[1])
	assert.Equal(t, 8.0, *zed.Scores[1])
	assert.Equal(t, service.TardinessLate, zed.Statuses[1])

	m.pageViews.AssertNotCalled(t, "ListByUserID", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestSubmissionTardiness(t *testing.T) {
	now := time.Now()
	due := now.Add(-time.Hour)
	early := now.Add(-2 * time.Hour)
	later := now.Add(-30 * time.Minute)
	a := &models.Assignment{DueAt: &due, SubmissionTypes: "online_upload"}

	assert.Equal(t, service.TardinessOnTime, service.SubmissionTardiness(a, &models.Submission{SubmittedAt: &early}, now))
	assert.Equal(t, service.TardinessLate, service.SubmissionTardiness(a, &models.Submission{SubmittedAt: &later}, now), "submitted after the due date")
	assert.Equal(t, service.TardinessMissing, service.SubmissionTardiness(a, nil, now))
	assert.Equal(t, service.TardinessExcused, service.SubmissionTardiness(a, &models.Submission{Excused: true}, now))
	assert.Equal(t, service.TardinessOnTime, service.SubmissionTardiness(a, &models.Submission{Score: f64(3)}, now), "graded on paper")

	paper := &models.Assignment{DueAt: &due, SubmissionTypes: "on_paper"}
	assert.Equal(t, service.TardinessFloating, service.SubmissionTardiness(paper, nil, now), "nothing to turn in online")
	noDue := &models.Assignment{SubmissionTypes: "online_upload"}
	assert.Equal(t, service.TardinessFloating, service.SubmissionTardiness(noDue, nil, now))
}

func TestAnalyticsCategoryForPath(t *testing.T) {
	cases := map[string]string{
		"/courses/5":               "home",
		"/courses/5/assignments/3": "assignments",
		"/api/v1/courses/5/discussion_topics/2/entries": "discussions",
		"/courses/5/discussions/2":                      "discussions",
		"/courses/5/quizzes/1/take":                     "quizzes",
		"/courses/5/pages/intro":                        "pages",
		"/courses/5/files":                              "files",
		"/courses/5/modules":                            "modules",
		"/courses/5/grades":                             "grades",
		"/courses/5/something_new":                      "other",
		"/dashboard":                                    "other",
	}
	for path, want := range cases {
		assert.Equal(t, want, service.AnalyticsCategoryForPath(path), path)
	}
}

// G154: a client-reported page view is kept only when the user is enrolled
// in the course; participation is never taken from the client.
func TestRecordPageView_RequiresEnrollmentAndIgnoresClientParticipation(t *testing.T) {
	m := newAnalyticsMocks()
	ctx := context.Background()

	m.enrollments.On("FindByUserAndCourse", mock.Anything, uint(4), uint(99), uint(7)).Return(nil, assert.AnError)
	err := m.svc.RecordPageView(ctx, &models.PageView{UserID: 4, ContextType: "Course", ContextID: 99, URL: "/courses/99"}, 7)
	assert.ErrorIs(t, err, service.ErrPageViewNotTracked)

	err = m.svc.RecordPageView(ctx, &models.PageView{UserID: 4, ContextType: "Account", ContextID: 7, URL: "/accounts/7"}, 7)
	assert.ErrorIs(t, err, service.ErrPageViewNotTracked, "only course page views are tracked")

	m.enrollments.On("FindByUserAndCourse", mock.Anything, uint(4), uint(30), uint(7)).Return(&models.Enrollment{UserID: 4, CourseID: 30, Type: "StudentEnrollment"}, nil)
	var created *models.PageView
	m.pageViews.On("Create", mock.Anything, mock.AnythingOfType("*models.PageView")).Run(func(args mock.Arguments) {
		created = args.Get(1).(*models.PageView)
	}).Return(nil)
	m.pageViews.On("TouchEnrollmentActivity", mock.Anything, uint(4), uint(30)).Return(nil)

	pv := &models.PageView{UserID: 4, ContextType: "Course", ContextID: 30, URL: "/courses/30/assignments/2", Participated: true, InteractionSeconds: 999999}
	require.NoError(t, m.svc.RecordPageView(ctx, pv, 7))
	require.NotNil(t, created)
	assert.False(t, created.Participated)
	assert.Equal(t, "assignments", created.AssetCategory)
	assert.Equal(t, 7200, created.InteractionSeconds, "time on page is capped")
	m.pageViews.AssertCalled(t, "TouchEnrollmentActivity", mock.Anything, uint(4), uint(30))
}

// G154: participations are recorded for students only, once per URL per
// window.
func TestRecordParticipation_StudentsOnlyAndDeduplicated(t *testing.T) {
	m := newAnalyticsMocks()
	ctx := context.Background()

	m.enrollments.On("FindByUserAndCourse", mock.Anything, uint(4), uint(30), uint(7)).Return(&models.Enrollment{UserID: 4, CourseID: 30, Type: "StudentEnrollment"}, nil)
	m.enrollments.On("FindByUserAndCourse", mock.Anything, uint(9), uint(30), uint(7)).Return(&models.Enrollment{UserID: 9, CourseID: 30, Type: "TeacherEnrollment"}, nil)
	var rows []*models.PageView
	m.pageViews.On("Create", mock.Anything, mock.AnythingOfType("*models.PageView")).Run(func(args mock.Arguments) {
		rows = append(rows, args.Get(1).(*models.PageView))
	}).Return(nil)
	m.pageViews.On("TouchEnrollmentActivity", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	route := "/api/v1/courses/:course_id/discussion_topics/:topic_id/entries"
	m.svc.RecordParticipation(ctx, 4, 30, 7, "POST", "/api/v1/courses/30/discussion_topics/2/entries", route)
	m.svc.RecordParticipation(ctx, 4, 30, 7, "POST", "/api/v1/courses/30/discussion_topics/2/entries", route)
	m.svc.RecordParticipation(ctx, 9, 30, 7, "POST", "/api/v1/courses/30/discussion_topics/2/entries", route)

	require.Len(t, rows, 1, "repeat within the window and teacher writes are not participations")
	assert.True(t, rows[0].Participated)
	assert.Equal(t, "discussions", rows[0].AssetCategory)
	assert.Equal(t, uint(4), rows[0].UserID)
}

// G154: the drill-down is only for students of the course.
func TestGetStudentActivity_NonStudentIsNotFound(t *testing.T) {
	m := newAnalyticsMocks()
	m.enrollments.On("ListByCourseID", mock.Anything, uint(30), uint(7), mock.Anything).Return(&repository.PaginatedResult[models.Enrollment]{
		Items:      []models.Enrollment{{UserID: 9, CourseID: 30, Type: "TeacherEnrollment"}},
		TotalCount: 1,
	}, nil)
	_, err := m.svc.GetStudentActivity(context.Background(), 30, 9, 7)
	assert.ErrorIs(t, err, service.ErrAnalyticsStudentNotFound)
	_, err = m.svc.GetStudentAssignments(context.Background(), 30, 1234, 7)
	assert.ErrorIs(t, err, service.ErrAnalyticsStudentNotFound)
}

func TestGetCourseAssignmentStats_QuartilesAndBreakdown(t *testing.T) {
	m := newAnalyticsMocks()
	past := time.Now().Add(-48 * time.Hour)
	onTime := time.Now().Add(-72 * time.Hour)
	posted := time.Now()
	m.assignments.On("ListByCourseID", mock.Anything, uint(30), mock.Anything).Return(&repository.PaginatedResult[models.Assignment]{
		Items: []models.Assignment{{ID: 1, CourseID: 30, Name: "Essay", WorkflowState: models.AssignmentPublished, DueAt: &past, SubmissionTypes: "online_upload"}},
	}, nil)
	students := []models.Enrollment{}
	for i := uint(1); i <= 5; i++ {
		students = append(students, models.Enrollment{UserID: i, CourseID: 30, Type: "StudentEnrollment"})
	}
	m.enrollments.On("ListByCourseID", mock.Anything, uint(30), uint(0), mock.Anything).Return(&repository.PaginatedResult[models.Enrollment]{Items: students, TotalCount: 5}, nil)
	subs := []models.Submission{}
	for i, score := range []float64{2, 4, 6, 8} {
		subs = append(subs, models.Submission{AssignmentID: 1, UserID: uint(i + 1), SubmittedAt: &onTime, Score: f64(score), PostedAt: &posted})
	}
	m.submissions.On("BulkListByCourse", mock.Anything, uint(30), mock.Anything).Return(&repository.PaginatedResult[models.Submission]{Items: subs}, nil)

	stats, err := m.svc.GetCourseAssignmentStats(context.Background(), 30, 0)
	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, 5.0, stats[0]["median"])
	assert.Equal(t, 3.5, stats[0]["first_quartile"])
	assert.Equal(t, 6.5, stats[0]["third_quartile"])
	b := stats[0]["tardiness"].(service.TardinessBreakdown)
	assert.Equal(t, 4, b.OnTime)
	assert.Equal(t, 1, b.Missing)
}
