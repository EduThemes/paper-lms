package graphql_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/graphql"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// B086: allCourses must not list every course in the tenant (with rosters
// and unpublished assignments) to a non-admin.
func TestAllCourses_StudentSeesOnlyEnrolledCoursesAndPublishedAssignments(t *testing.T) {
	courseRepo := new(mocks.MockCourseRepository)
	userRepo := new(mocks.MockUserRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)
	assignmentRepo := new(mocks.MockAssignmentRepository)

	userRepo.On("FindByID", mock.Anything, uint(7), uint(1)).Return(&models.User{ID: 7, AccountID: 1, Role: "user"}, nil)
	courseRepo.On("ListByUserID", mock.Anything, uint(7), uint(0), mock.Anything).Return(&repository.PaginatedResult[models.Course]{
		Items: []models.Course{{ID: 10, AccountID: 1, Name: "Biology"}}, TotalCount: 1, Page: 1, PerPage: 10,
	}, nil)
	enrollmentRepo.On("FindByUserAndCourse", mock.Anything, uint(7), uint(10), uint(1)).Return(&models.Enrollment{
		UserID: 7, CourseID: 10, Type: "StudentEnrollment", WorkflowState: "active",
	}, nil)
	assignmentRepo.On("ListByCourseID", mock.Anything, uint(10), mock.Anything).Return(&repository.PaginatedResult[models.Assignment]{
		Items: []models.Assignment{
			{ID: 1, CourseID: 10, Name: "Published Lab", Published: true},
			{ID: 2, CourseID: 10, Name: "Draft Exam", Published: false},
		},
		TotalCount: 2, Page: 1, PerPage: 100,
	}, nil)

	r := graphql.NewResolver(
		service.NewCourseService(courseRepo, enrollmentRepo, nil),
		service.NewAssignmentService(assignmentRepo),
		service.NewUserService(userRepo),
		service.NewEnrollmentService(enrollmentRepo),
		nil, nil,
	)

	resp := r.Resolve(context.Background(), 7, 1, `{ allCourses(perPage: 100) { id name assignments { name } } }`, nil)
	require.Empty(t, resp.Errors)
	data := resp.Data.(map[string]interface{})
	courses := data["allCourses"].([]interface{})
	require.Len(t, courses, 1)
	course := courses[0].(map[string]interface{})
	assert.Equal(t, uint(10), course["id"])
	assignments := course["assignments"].([]interface{})
	require.Len(t, assignments, 1)
	assert.Equal(t, "Published Lab", assignments[0].(map[string]interface{})["name"])

	courseRepo.AssertNotCalled(t, "List", mock.Anything, mock.Anything, mock.Anything)
}
