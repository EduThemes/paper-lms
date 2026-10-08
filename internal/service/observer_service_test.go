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

type fakeObserverAnnouncements struct{}

func (fakeObserverAnnouncements) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Announcement], error) {
	return &repository.PaginatedResult[models.Announcement]{}, nil
}

// Fixture: parent 100 is linked to child 200 through course 1. The child is
// also (later) enrolled in course 2, with no observer enrollment there.
func newObserverFixture(t *testing.T) (*service.ObserverService, *mocks.MockEnrollmentRepository, *mocks.MockSubmissionRepository, *mocks.MockPageRepository) {
	t.Helper()
	child := uint(200)
	enrollments := new(mocks.MockEnrollmentRepository)
	enrollments.On("ListByUserID", mock.Anything, uint(100), uint(0)).Return([]models.Enrollment{
		{ID: 1, UserID: 100, CourseID: 1, Type: "ObserverEnrollment", WorkflowState: models.EnrollmentActive, AssociatedUserID: &child},
	}, nil)
	enrollments.On("ListByUserID", mock.Anything, uint(200), uint(0)).Return([]models.Enrollment{
		{ID: 2, UserID: 200, CourseID: 1, Type: "StudentEnrollment", WorkflowState: models.EnrollmentActive},
		{ID: 3, UserID: 200, CourseID: 2, Type: "StudentEnrollment", WorkflowState: models.EnrollmentActive},
	}, nil)

	courses := new(mocks.MockCourseRepository)
	courses.On("FindByID", mock.Anything, uint(1), uint(0)).Return(&models.Course{ID: 1, Name: "Math"}, nil)
	courses.On("FindByID", mock.Anything, uint(2), uint(0)).Return(&models.Course{ID: 2, Name: "Art"}, nil)

	users := new(mocks.MockUserRepository)
	users.On("FindByIDs", mock.Anything, []uint{200}, uint(0)).Return([]models.User{{ID: 200, Name: "Jane Doe", SortableName: "Doe, Jane"}}, nil)

	svc := service.NewObserverService(enrollments, courses, users)
	submissions := new(mocks.MockSubmissionRepository)
	pages := new(mocks.MockPageRepository)
	assignments := new(mocks.MockAssignmentRepository)
	quizzes := new(mocks.MockQuizRepository)
	svc.SetOverviewDeps(assignments, submissions, quizzes, fakeObserverAnnouncements{}, pages)

	points := 10.0
	assignments.On("ListByCourseID", mock.Anything, mock.Anything, mock.Anything).Return(&repository.PaginatedResult[models.Assignment]{
		Items: []models.Assignment{{ID: 50, CourseID: 1, Name: "Quiz 1", Published: true, PointsPossible: &points}},
	}, nil)
	quizzes.On("ListByCourseID", mock.Anything, mock.Anything, mock.Anything).Return(&repository.PaginatedResult[models.Quiz]{}, nil)
	return svc, enrollments, submissions, pages
}

// B203: unposted grades and unpublished pages must not reach the parent.
func TestObserverOverview_HidesUnpostedGradesAndDraftPages(t *testing.T) {
	svc, enrollments, submissions, pages := newObserverFixture(t)
	enrollments.On("Create", mock.Anything, mock.Anything).Return(nil)

	graded := time.Now().Add(-time.Hour)
	score := 7.0
	submissions.On("ListByUserAndCourse", mock.Anything, uint(200), uint(1)).Return([]models.Submission{
		{ID: 9, UserID: 200, AssignmentID: 50, Score: &score, GradedAt: &graded}, // graded, NOT posted
	}, nil)
	submissions.On("ListByUserAndCourse", mock.Anything, uint(200), uint(2)).Return([]models.Submission{}, nil)
	pages.On("ListPublishedByCourseID", mock.Anything, mock.Anything, mock.Anything).Return(&repository.PaginatedResult[models.WikiPage]{
		Items: []models.WikiPage{
			{ID: 1, CourseID: 1, Title: "Secret draft", WorkflowState: "unpublished", UpdatedAt: time.Now()},
			{ID: 2, CourseID: 1, Title: "Syllabus", WorkflowState: "active", UpdatedAt: time.Now()},
		},
	}, nil)

	overview, err := svc.GetChildOverview(context.Background(), 100, 200)
	require.NoError(t, err)
	assert.Empty(t, overview.RecentGrades, "an unposted grade must not appear")
	var titles []string
	for _, a := range overview.RecentActivity {
		titles = append(titles, a.Title)
	}
	assert.Contains(t, titles, "Syllabus")
	assert.NotContains(t, titles, "Secret draft")
}

// B204: a course the child joined after pairing gets an observer enrollment
// for the parent, so the course page authorizes.
func TestObserverOverview_SyncsObserverEnrollmentForNewCourse(t *testing.T) {
	svc, enrollments, submissions, pages := newObserverFixture(t)
	var created []*models.Enrollment
	enrollments.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		created = append(created, args.Get(1).(*models.Enrollment))
	}).Return(nil)
	submissions.On("ListByUserAndCourse", mock.Anything, mock.Anything, mock.Anything).Return([]models.Submission{}, nil)
	pages.On("ListPublishedByCourseID", mock.Anything, mock.Anything, mock.Anything).Return(&repository.PaginatedResult[models.WikiPage]{}, nil)

	_, err := svc.GetChildOverview(context.Background(), 100, 200)
	require.NoError(t, err)
	require.Len(t, created, 1, "only course 2 lacks an observer enrollment")
	assert.Equal(t, uint(2), created[0].CourseID)
	assert.Equal(t, uint(100), created[0].UserID)
	assert.Equal(t, "ObserverEnrollment", created[0].Type)
	require.NotNil(t, created[0].AssociatedUserID)
	assert.Equal(t, uint(200), *created[0].AssociatedUserID)
}

// B202: the child picker gets names, not just ids.
func TestListObservedStudentProfiles_IncludesNames(t *testing.T) {
	svc, _, _, _ := newObserverFixture(t)
	students, err := svc.ListObservedStudentProfiles(context.Background(), 100)
	require.NoError(t, err)
	require.Len(t, students, 1)
	assert.Equal(t, uint(200), students[0].ID)
	assert.Equal(t, "Jane Doe", students[0].Name)
}

// F25: the observer dashboard's course grade uses the same calculator as
// the Grades page: assignment-group weights apply, and a hidden (unposted)
// grade counts as not graded yet.
func TestObserverOverview_CourseGradeAppliesGroupWeights(t *testing.T) {
	child := uint(200)
	enrollments := new(mocks.MockEnrollmentRepository)
	enrollments.On("ListByUserID", mock.Anything, uint(100), uint(0)).Return([]models.Enrollment{
		{ID: 1, UserID: 100, CourseID: 1, Type: "ObserverEnrollment", WorkflowState: models.EnrollmentActive, AssociatedUserID: &child},
	}, nil)
	enrollments.On("ListByUserID", mock.Anything, uint(200), uint(0)).Return([]models.Enrollment{
		{ID: 2, UserID: 200, CourseID: 1, Type: "StudentEnrollment", WorkflowState: models.EnrollmentActive},
	}, nil)
	enrollments.On("Create", mock.Anything, mock.Anything).Return(nil).Maybe()
	courses := new(mocks.MockCourseRepository)
	courses.On("FindByID", mock.Anything, uint(1), uint(0)).Return(&models.Course{ID: 1, Name: "Math", ApplyGroupWeights: true}, nil)

	svc := service.NewObserverService(enrollments, courses, new(mocks.MockUserRepository))
	submissions := new(mocks.MockSubmissionRepository)
	pages := new(mocks.MockPageRepository)
	assignments := new(mocks.MockAssignmentRepository)
	quizzes := new(mocks.MockQuizRepository)
	groups := new(mocks.MockAssignmentGroupRepository)
	svc.SetOverviewDeps(assignments, submissions, quizzes, fakeObserverAnnouncements{}, pages)
	svc.SetAssignmentGroupRepo(groups)

	hundred := 100.0
	exams, homework := uint(1), uint(2)
	assignments.On("ListByCourseID", mock.Anything, uint(1), mock.Anything).Return(&repository.PaginatedResult[models.Assignment]{
		Items: []models.Assignment{
			{ID: 50, CourseID: 1, Name: "Exam", WorkflowState: models.AssignmentPublished, Published: true, PointsPossible: &hundred, AssignmentGroupID: &exams},
			{ID: 51, CourseID: 1, Name: "Homework", WorkflowState: models.AssignmentPublished, Published: true, PointsPossible: &hundred, AssignmentGroupID: &homework},
			{ID: 52, CourseID: 1, Name: "Hidden quiz", WorkflowState: models.AssignmentPublished, Published: true, PointsPossible: &hundred, AssignmentGroupID: &homework, PostPolicy: "manual"},
		},
	}, nil)
	groups.On("ListByCourseID", mock.Anything, uint(1), mock.Anything).Return(&repository.PaginatedResult[models.AssignmentGroup]{
		Items: []models.AssignmentGroup{{ID: 1, CourseID: 1, Name: "Exams", GroupWeight: 80}, {ID: 2, CourseID: 1, Name: "Homework", GroupWeight: 20}},
	}, nil)
	quizzes.On("ListByCourseID", mock.Anything, mock.Anything, mock.Anything).Return(&repository.PaginatedResult[models.Quiz]{}, nil)
	pages.On("ListPublishedByCourseID", mock.Anything, mock.Anything, mock.Anything).Return(&repository.PaginatedResult[models.WikiPage]{}, nil)

	posted := time.Now().Add(-time.Hour)
	fifty, full, zero := 50.0, 100.0, 0.0
	submissions.On("ListByUserAndCourse", mock.Anything, uint(200), uint(1)).Return([]models.Submission{
		{ID: 9, UserID: 200, AssignmentID: 50, Score: &fifty, GradedAt: &posted, PostedAt: &posted, WorkflowState: models.SubmissionGraded},
		{ID: 10, UserID: 200, AssignmentID: 51, Score: &full, GradedAt: &posted, PostedAt: &posted, WorkflowState: models.SubmissionGraded},
		{ID: 11, UserID: 200, AssignmentID: 52, Score: &zero, GradedAt: &posted, WorkflowState: models.SubmissionGraded}, // hidden
	}, nil)

	overview, err := svc.GetChildOverview(context.Background(), 100, 200)
	require.NoError(t, err)
	require.Len(t, overview.Courses, 1)
	require.NotNil(t, overview.Courses[0].CurrentGrade)
	// Exams 50% x 80 + Homework 100% x 20 = 60%, not the 75% point average,
	// and the hidden 0 does not drag homework down.
	assert.InDelta(t, 60.0, *overview.Courses[0].CurrentGrade, 1e-9)
}
