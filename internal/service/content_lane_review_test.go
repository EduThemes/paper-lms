package service_test

// Content-lane review follow-ups (service layer).

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// B145 follow-up: publishing a module never snapshots content that was
// deleted, even when a live module item still points at it.
func TestCommonsPublishModule_SkipsDeletedContent(t *testing.T) {
	ctx := context.Background()
	shared := new(mocks.MockSharedContentRepository)
	courses := new(mocks.MockCourseRepository)
	assignments := new(mocks.MockAssignmentRepository)
	pages := new(mocks.MockPageRepository)
	quizzes := new(mocks.MockQuizRepository)
	modules := new(mocks.MockModuleRepository)
	discussions := new(mocks.MockDiscussionTopicRepository)

	live, gone := uint(5), uint(6)
	courses.On("FindByID", mock.Anything, uint(10), uint(1)).Return(&models.Course{ID: 10, AccountID: 1, Name: "Bio"}, nil)
	modules.On("FindByID", mock.Anything, uint(40), uint(1)).Return(&models.ContextModule{
		ID: 40, CourseID: 10, Name: "Unit 1", WorkflowState: "active",
		Items: []models.ContentTag{
			{ID: 1, ContentType: "Assignment", ContentID: &live, Title: "Essay", WorkflowState: models.ContentTagActive},
			{ID: 2, ContentType: "Assignment", ContentID: &gone, Title: "Old quiz key", WorkflowState: models.ContentTagActive},
		},
	}, nil)
	assignments.On("FindByID", mock.Anything, live, uint(1)).Return(&models.Assignment{ID: live, CourseID: 10, Name: "Essay", WorkflowState: models.AssignmentPublished}, nil)
	assignments.On("FindByID", mock.Anything, gone, uint(1)).Return(&models.Assignment{ID: gone, CourseID: 10, Name: "Deleted answer key", WorkflowState: models.AssignmentDeleted}, nil)
	var published *models.SharedContent
	shared.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		published = args.Get(1).(*models.SharedContent)
	}).Return(nil)

	svc := service.NewCommonsService(shared, courses, assignments, pages, quizzes, modules, discussions)
	_, err := svc.Publish(ctx, 7, 10, 1, service.CommonsPublishOptions{ResourceType: "module", ResourceID: 40, Title: "Unit 1"})
	require.NoError(t, err)
	require.NotNil(t, published)
	assert.Contains(t, published.ContentSnapshot, "Essay")
	assert.NotContains(t, published.ContentSnapshot, "Deleted answer key")
}

type contentLaneEmptyAnnouncements struct{}

func (contentLaneEmptyAnnouncements) ListByCourseID(_ context.Context, _ uint, _ repository.PaginationParams) (*repository.PaginatedResult[models.Announcement], error) {
	return &repository.PaginatedResult[models.Announcement]{}, nil
}

// B045 (observer path): the observer overview's activity feed lists
// only published pages; the unfiltered list leaked draft page titles.
func TestObserverOverview_ActivityListsOnlyPublishedPages(t *testing.T) {
	ctx := context.Background()
	enrollments := new(mocks.MockEnrollmentRepository)
	courses := new(mocks.MockCourseRepository)
	assignments := new(mocks.MockAssignmentRepository)
	submissions := new(mocks.MockSubmissionRepository)
	quizzes := new(mocks.MockQuizRepository)
	pages := new(mocks.MockPageRepository)

	childID := uint(20)
	enrollments.On("ListByUserID", mock.Anything, uint(10), uint(0)).Return([]models.Enrollment{
		{Type: "ObserverEnrollment", AssociatedUserID: &childID, WorkflowState: "active", CourseID: 5},
	}, nil)
	enrollments.On("ListByUserID", mock.Anything, childID, uint(0)).Return([]models.Enrollment{
		{Type: "StudentEnrollment", WorkflowState: "active", CourseID: 5, UserID: childID},
	}, nil)
	courses.On("FindByID", mock.Anything, uint(5), uint(0)).Return(&models.Course{ID: 5, Name: "Biology"}, nil)
	assignments.On("ListByCourseID", mock.Anything, uint(5), mock.Anything).Return(&repository.PaginatedResult[models.Assignment]{}, nil)
	quizzes.On("ListByCourseID", mock.Anything, uint(5), mock.Anything).Return(&repository.PaginatedResult[models.Quiz]{}, nil)
	submissions.On("ListByUserAndCourse", mock.Anything, childID, uint(5)).Return([]models.Submission{}, nil)
	pages.On("ListPublishedByCourseID", mock.Anything, uint(5), mock.Anything).Return(&repository.PaginatedResult[models.WikiPage]{
		Items: []models.WikiPage{{ID: 1, CourseID: 5, Title: "Welcome", WorkflowState: "active"}},
	}, nil)
	pages.On("ListByCourseID", mock.Anything, uint(5), mock.Anything).Return(&repository.PaginatedResult[models.WikiPage]{
		Items: []models.WikiPage{
			{ID: 1, CourseID: 5, Title: "Welcome", WorkflowState: "active"},
			{ID: 2, CourseID: 5, Title: "Unit 3 Test Answers", WorkflowState: "unpublished"},
		},
	}, nil).Maybe()

	svc := service.NewObserverService(enrollments, courses, nil)
	svc.SetOverviewDeps(assignments, submissions, quizzes, contentLaneEmptyAnnouncements{}, pages)

	overview, err := svc.GetChildOverview(ctx, 10, childID)
	require.NoError(t, err)
	var titles []string
	for _, a := range overview.RecentActivity {
		if a.Type == "page" {
			titles = append(titles, a.Title)
		}
	}
	assert.Equal(t, []string{"Welcome"}, titles)
	pages.AssertNotCalled(t, "ListByCourseID", mock.Anything, mock.Anything, mock.Anything)
}
