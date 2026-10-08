package service_test

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

// B209: per-student page views in a course's analytics must count only that
// course's page views, not the student's site-wide total.
func TestGetStudentSummaries_PageViewsScopedToCourse(t *testing.T) {
	pageViews := new(mocks.MockPageViewRepository)
	submissions := new(mocks.MockSubmissionRepository)
	enrollments := new(mocks.MockEnrollmentRepository)

	enrollments.On("ListByCourseID", mock.Anything, uint(30), uint(0), mock.Anything).Return(&repository.PaginatedResult[models.Enrollment]{
		Items: []models.Enrollment{{UserID: 4, CourseID: 30, Type: "StudentEnrollment"}},
	}, nil)
	submissions.On("BulkListByCourse", mock.Anything, uint(30), mock.Anything).Return(&repository.PaginatedResult[models.Submission]{}, nil)
	pageViews.On("CountByUserAndContext", mock.Anything, uint(4), "Course", uint(30)).Return(int64(12), nil)
	pageViews.On("SumInteractionByUserAndContext", mock.Anything, uint(4), "Course", uint(30)).Return(int64(600), nil)

	svc := service.NewAnalyticsService(pageViews, submissions, enrollments, new(mocks.MockAssignmentRepository))
	summaries, err := svc.GetStudentSummaries(context.Background(), 30, repository.PaginationParams{Page: 1, PerPage: 50})
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, int64(12), summaries[0]["page_views"])
	pageViews.AssertNotCalled(t, "ListByUserID", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
