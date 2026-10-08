package service_test

// A student may hold several StudentEnrollment rows in one course (one
// per section, G110). The gradebook and SpeedGrader list each student
// once. All data is made up.

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

func twoSectionEnrollments() *repository.PaginatedResult[models.Enrollment] {
	s1, s2 := uint(7), uint(8)
	return &repository.PaginatedResult[models.Enrollment]{
		Items: []models.Enrollment{
			{ID: 1, UserID: 10, CourseID: 1, CourseSectionID: &s1, Type: "StudentEnrollment", User: &models.User{ID: 10, Name: "Jane Doe"}},
			{ID: 2, UserID: 10, CourseID: 1, CourseSectionID: &s2, Type: "StudentEnrollment", User: &models.User{ID: 10, Name: "Jane Doe"}},
			{ID: 3, UserID: 11, CourseID: 1, CourseSectionID: &s2, Type: "StudentEnrollment", User: &models.User{ID: 11, Name: "John Roe"}},
		},
		TotalCount: 3, Page: 1, PerPage: 1000,
	}
}

func TestGetGradebook_StudentInTwoSectionsListedOnce(t *testing.T) {
	assignmentRepo := new(mocks.MockAssignmentRepository)
	submissionRepo := new(mocks.MockSubmissionRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)
	enrollmentRepo.On("ListByCourseID", mock.Anything, uint(1), uint(0), mock.AnythingOfType("repository.PaginationParams")).
		Return(twoSectionEnrollments(), nil)
	assignmentRepo.On("ListByCourseID", mock.Anything, uint(1), mock.AnythingOfType("repository.PaginationParams")).
		Return(makeAssignments(), nil)
	submissionRepo.On("BulkListByCourse", mock.Anything, uint(1), mock.AnythingOfType("repository.PaginationParams")).
		Return(&repository.PaginatedResult[models.Submission]{}, nil)

	svc := service.NewGradingService(submissionRepo, assignmentRepo, new(mocks.MockAssignmentGroupRepository), enrollmentRepo, unweightedCourseRepo(1), nil)
	gb, err := svc.GetGradebook(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, gb.Students, 2)
	assert.Equal(t, uint(10), gb.Students[0].ID)
	assert.Equal(t, uint(11), gb.Students[1].ID)
}

func TestSpeedGrader_StudentInTwoSectionsListedOnce(t *testing.T) {
	assignmentRepo := new(mocks.MockAssignmentRepository)
	submissionRepo := new(mocks.MockSubmissionRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)
	commentRepo := new(mocks.MockSubmissionCommentRepository)
	assignmentRepo.On("FindByID", mock.Anything, uint(5), uint(1)).
		Return(&models.Assignment{ID: 5, CourseID: 1, Name: "Essay", WorkflowState: models.AssignmentPublished}, nil)
	enrollmentRepo.On("ListByCourseID", mock.Anything, uint(1), uint(0), mock.AnythingOfType("repository.PaginationParams")).
		Return(twoSectionEnrollments(), nil)
	submissionRepo.On("ListByAssignmentID", mock.Anything, uint(5), mock.AnythingOfType("repository.PaginationParams")).
		Return(&repository.PaginatedResult[models.Submission]{
			Items: []models.Submission{{ID: 50, AssignmentID: 5, UserID: 10}},
		}, nil)
	commentRepo.On("ListBySubmissionID", mock.Anything, uint(50), uint(1)).Return([]models.SubmissionComment{}, nil)

	svc := service.NewSpeedGraderService(submissionRepo, commentRepo, assignmentRepo, enrollmentRepo, nil)
	data, err := svc.GetSpeedGraderData(context.Background(), 1, 5, 1)
	require.NoError(t, err)
	require.Len(t, data.Students, 2)
	assert.Equal(t, uint(10), data.Students[0].UserID)
	assert.NotNil(t, data.Students[0].Submission)
	assert.Equal(t, uint(11), data.Students[1].UserID)
}
