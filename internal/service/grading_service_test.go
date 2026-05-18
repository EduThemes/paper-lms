package service_test

import (
	"context"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func makeAssignments(assignments ...models.Assignment) *repository.PaginatedResult[models.Assignment] {
	return &repository.PaginatedResult[models.Assignment]{
		Items:      assignments,
		TotalCount: int64(len(assignments)),
		Page:       1,
		PerPage:    1000,
	}
}

// unweightedCourseRepo returns a mock CourseRepository that returns a course with ApplyGroupWeights=false
func unweightedCourseRepo(courseID uint) *mocks.MockCourseRepository {
	courseRepo := new(mocks.MockCourseRepository)
	courseRepo.On("FindByID", mock.Anything, courseID, uint(0)).
		Return(&models.Course{ID: courseID, ApplyGroupWeights: false}, nil)
	return courseRepo
}

func TestGetStudentGrade_AllGraded(t *testing.T) {
	pts100 := 100.0
	score90 := 90.0
	score80 := 80.0

	assignmentRepo := new(mocks.MockAssignmentRepository)
	submissionRepo := new(mocks.MockSubmissionRepository)
	groupRepo := new(mocks.MockAssignmentGroupRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)

	assignmentRepo.On("ListByCourseID", mock.Anything, uint(1), mock.AnythingOfType("repository.PaginationParams")).
		Return(makeAssignments(
			models.Assignment{ID: 1, CourseID: 1, Name: "HW1", PointsPossible: &pts100},
			models.Assignment{ID: 2, CourseID: 1, Name: "HW2", PointsPossible: &pts100},
		), nil)
	submissionRepo.On("ListByUserAndCourse", mock.Anything, uint(10), uint(1)).
		Return([]models.Submission{
			{ID: 1, AssignmentID: 1, UserID: 10, Score: &score90, WorkflowState: "graded"},
			{ID: 2, AssignmentID: 2, UserID: 10, Score: &score80, WorkflowState: "graded"},
		}, nil)

	svc := service.NewGradingService(submissionRepo, assignmentRepo, groupRepo, enrollmentRepo, unweightedCourseRepo(1), nil)
	grade, err := svc.GetStudentGrade(context.Background(), 1, 10)

	assert.NoError(t, err)
	assert.NotNil(t, grade)
	// (90 + 80) / (100 + 100) = 170/200 = 85%
	assert.Equal(t, 85.0, grade.CurrentScore)
	assert.Equal(t, 85.0, grade.FinalScore)
	assert.Equal(t, "B", grade.CurrentGrade)
	assert.Equal(t, "B", grade.FinalGrade)
}

func TestGetStudentGrade_PartialGraded(t *testing.T) {
	pts100 := 100.0
	score90 := 90.0

	assignmentRepo := new(mocks.MockAssignmentRepository)
	submissionRepo := new(mocks.MockSubmissionRepository)
	groupRepo := new(mocks.MockAssignmentGroupRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)

	assignmentRepo.On("ListByCourseID", mock.Anything, uint(1), mock.AnythingOfType("repository.PaginationParams")).
		Return(makeAssignments(
			models.Assignment{ID: 1, CourseID: 1, Name: "HW1", PointsPossible: &pts100},
			models.Assignment{ID: 2, CourseID: 1, Name: "HW2", PointsPossible: &pts100},
		), nil)
	submissionRepo.On("ListByUserAndCourse", mock.Anything, uint(10), uint(1)).
		Return([]models.Submission{
			{ID: 1, AssignmentID: 1, UserID: 10, Score: &score90, WorkflowState: "graded"},
		}, nil)

	svc := service.NewGradingService(submissionRepo, assignmentRepo, groupRepo, enrollmentRepo, unweightedCourseRepo(1), nil)
	grade, err := svc.GetStudentGrade(context.Background(), 1, 10)

	assert.NoError(t, err)
	assert.NotNil(t, grade)
	// Current: only graded -> 90/100 = 90%
	assert.Equal(t, 90.0, grade.CurrentScore)
	// Final: all assignments -> 90/200 = 45%
	assert.Equal(t, 45.0, grade.FinalScore)
	assert.Equal(t, "A-", grade.CurrentGrade)
	assert.Equal(t, "F", grade.FinalGrade)
}

func TestGetStudentGrade_NoSubmissions(t *testing.T) {
	pts100 := 100.0

	assignmentRepo := new(mocks.MockAssignmentRepository)
	submissionRepo := new(mocks.MockSubmissionRepository)
	groupRepo := new(mocks.MockAssignmentGroupRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)

	assignmentRepo.On("ListByCourseID", mock.Anything, uint(1), mock.AnythingOfType("repository.PaginationParams")).
		Return(makeAssignments(
			models.Assignment{ID: 1, CourseID: 1, Name: "HW1", PointsPossible: &pts100},
			models.Assignment{ID: 2, CourseID: 1, Name: "HW2", PointsPossible: &pts100},
		), nil)
	submissionRepo.On("ListByUserAndCourse", mock.Anything, uint(10), uint(1)).
		Return([]models.Submission{}, nil)

	svc := service.NewGradingService(submissionRepo, assignmentRepo, groupRepo, enrollmentRepo, unweightedCourseRepo(1), nil)
	grade, err := svc.GetStudentGrade(context.Background(), 1, 10)

	assert.NoError(t, err)
	assert.NotNil(t, grade)
	assert.Equal(t, 0.0, grade.CurrentScore)
	assert.Equal(t, 0.0, grade.FinalScore)
}

func TestGetStudentGrade_ZeroPointsAssignment(t *testing.T) {
	pts100 := 100.0
	pts0 := 0.0
	score95 := 95.0

	assignmentRepo := new(mocks.MockAssignmentRepository)
	submissionRepo := new(mocks.MockSubmissionRepository)
	groupRepo := new(mocks.MockAssignmentGroupRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)

	assignmentRepo.On("ListByCourseID", mock.Anything, uint(1), mock.AnythingOfType("repository.PaginationParams")).
		Return(makeAssignments(
			models.Assignment{ID: 1, CourseID: 1, Name: "HW1", PointsPossible: &pts100},
			models.Assignment{ID: 2, CourseID: 1, Name: "Extra Credit", PointsPossible: &pts0},
		), nil)
	submissionRepo.On("ListByUserAndCourse", mock.Anything, uint(10), uint(1)).
		Return([]models.Submission{
			{ID: 1, AssignmentID: 1, UserID: 10, Score: &score95, WorkflowState: "graded"},
		}, nil)

	svc := service.NewGradingService(submissionRepo, assignmentRepo, groupRepo, enrollmentRepo, unweightedCourseRepo(1), nil)
	grade, err := svc.GetStudentGrade(context.Background(), 1, 10)

	assert.NoError(t, err)
	assert.NotNil(t, grade)
	// Zero-point assignment should be skipped, so 95/100 = 95%
	assert.Equal(t, 95.0, grade.CurrentScore)
	assert.Equal(t, 95.0, grade.FinalScore)
}

func TestScoreToLetterGrade_A(t *testing.T) {
	pts100 := 100.0
	score93 := 93.0

	assignmentRepo := new(mocks.MockAssignmentRepository)
	submissionRepo := new(mocks.MockSubmissionRepository)
	groupRepo := new(mocks.MockAssignmentGroupRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)

	assignmentRepo.On("ListByCourseID", mock.Anything, uint(1), mock.AnythingOfType("repository.PaginationParams")).
		Return(makeAssignments(
			models.Assignment{ID: 1, CourseID: 1, Name: "HW1", PointsPossible: &pts100},
		), nil)
	submissionRepo.On("ListByUserAndCourse", mock.Anything, uint(10), uint(1)).
		Return([]models.Submission{
			{ID: 1, AssignmentID: 1, UserID: 10, Score: &score93, WorkflowState: "graded"},
		}, nil)

	svc := service.NewGradingService(submissionRepo, assignmentRepo, groupRepo, enrollmentRepo, unweightedCourseRepo(1), nil)
	grade, err := svc.GetStudentGrade(context.Background(), 1, 10)

	assert.NoError(t, err)
	assert.Equal(t, "A", grade.CurrentGrade)
}

func TestScoreToLetterGrade_BPlus(t *testing.T) {
	pts100 := 100.0
	score87 := 87.0

	assignmentRepo := new(mocks.MockAssignmentRepository)
	submissionRepo := new(mocks.MockSubmissionRepository)
	groupRepo := new(mocks.MockAssignmentGroupRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)

	assignmentRepo.On("ListByCourseID", mock.Anything, uint(1), mock.AnythingOfType("repository.PaginationParams")).
		Return(makeAssignments(
			models.Assignment{ID: 1, CourseID: 1, Name: "HW1", PointsPossible: &pts100},
		), nil)
	submissionRepo.On("ListByUserAndCourse", mock.Anything, uint(10), uint(1)).
		Return([]models.Submission{
			{ID: 1, AssignmentID: 1, UserID: 10, Score: &score87, WorkflowState: "graded"},
		}, nil)

	svc := service.NewGradingService(submissionRepo, assignmentRepo, groupRepo, enrollmentRepo, unweightedCourseRepo(1), nil)
	grade, err := svc.GetStudentGrade(context.Background(), 1, 10)

	assert.NoError(t, err)
	assert.Equal(t, "B+", grade.CurrentGrade)
}

func TestScoreToLetterGrade_F(t *testing.T) {
	pts100 := 100.0
	score50 := 50.0

	assignmentRepo := new(mocks.MockAssignmentRepository)
	submissionRepo := new(mocks.MockSubmissionRepository)
	groupRepo := new(mocks.MockAssignmentGroupRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)

	assignmentRepo.On("ListByCourseID", mock.Anything, uint(1), mock.AnythingOfType("repository.PaginationParams")).
		Return(makeAssignments(
			models.Assignment{ID: 1, CourseID: 1, Name: "HW1", PointsPossible: &pts100},
		), nil)
	submissionRepo.On("ListByUserAndCourse", mock.Anything, uint(10), uint(1)).
		Return([]models.Submission{
			{ID: 1, AssignmentID: 1, UserID: 10, Score: &score50, WorkflowState: "graded"},
		}, nil)

	svc := service.NewGradingService(submissionRepo, assignmentRepo, groupRepo, enrollmentRepo, unweightedCourseRepo(1), nil)
	grade, err := svc.GetStudentGrade(context.Background(), 1, 10)

	assert.NoError(t, err)
	assert.Equal(t, "F", grade.CurrentGrade)
	assert.Equal(t, "F", grade.FinalGrade)
}

func TestScoreToLetterGrade_Boundaries(t *testing.T) {
	pts100 := 100.0

	tests := []struct {
		name     string
		score    float64
		expected string
	}{
		{"A at 93", 93, "A"},
		{"A at 100", 100, "A"},
		{"A- at 90", 90, "A-"},
		{"A- at 92.99", 92.99, "A-"},
		{"B+ at 87", 87, "B+"},
		{"B+ at 89.99", 89.99, "B+"},
		{"B at 83", 83, "B"},
		{"B at 86.99", 86.99, "B"},
		{"B- at 80", 80, "B-"},
		{"C+ at 77", 77, "C+"},
		{"C at 73", 73, "C"},
		{"C- at 70", 70, "C-"},
		{"D+ at 67", 67, "D+"},
		{"D at 63", 63, "D"},
		{"D- at 60", 60, "D-"},
		{"F at 59.99", 59.99, "F"},
		{"F at 0", 0, "F"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := tt.score

			assignmentRepo := new(mocks.MockAssignmentRepository)
			submissionRepo := new(mocks.MockSubmissionRepository)
			groupRepo := new(mocks.MockAssignmentGroupRepository)
			enrollmentRepo := new(mocks.MockEnrollmentRepository)

			assignmentRepo.On("ListByCourseID", mock.Anything, uint(1), mock.AnythingOfType("repository.PaginationParams")).
				Return(makeAssignments(
					models.Assignment{ID: 1, CourseID: 1, Name: "HW1", PointsPossible: &pts100},
				), nil)
			submissionRepo.On("ListByUserAndCourse", mock.Anything, uint(10), uint(1)).
				Return([]models.Submission{
					{ID: 1, AssignmentID: 1, UserID: 10, Score: &score, WorkflowState: "graded"},
				}, nil)

			svc := service.NewGradingService(submissionRepo, assignmentRepo, groupRepo, enrollmentRepo, unweightedCourseRepo(1), nil)
			grade, err := svc.GetStudentGrade(context.Background(), 1, 10)

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, grade.CurrentGrade, "score %.2f should map to %s", tt.score, tt.expected)
		})
	}
}

func TestGetGradebook_Success(t *testing.T) {
	pts100 := 100.0
	score90 := 90.0

	assignmentRepo := new(mocks.MockAssignmentRepository)
	submissionRepo := new(mocks.MockSubmissionRepository)
	groupRepo := new(mocks.MockAssignmentGroupRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)

	enrollmentRepo.On("ListByCourseID", mock.Anything, uint(1), uint(0), mock.AnythingOfType("repository.PaginationParams")).
		Return(&repository.PaginatedResult[models.Enrollment]{
			Items: []models.Enrollment{
				{ID: 1, UserID: 10, CourseID: 1, Type: "StudentEnrollment", User: &models.User{ID: 10, Name: "Alice"}},
				{ID: 2, UserID: 20, CourseID: 1, Type: "TeacherEnrollment", User: &models.User{ID: 20, Name: "Prof. Smith"}},
			},
			TotalCount: 2,
			Page:       1,
			PerPage:    1000,
		}, nil)
	assignmentRepo.On("ListByCourseID", mock.Anything, uint(1), mock.AnythingOfType("repository.PaginationParams")).
		Return(makeAssignments(
			models.Assignment{ID: 1, CourseID: 1, Name: "HW1", PointsPossible: &pts100},
		), nil)
	submissionRepo.On("BulkListByCourse", mock.Anything, uint(1), mock.AnythingOfType("repository.PaginationParams")).
		Return(&repository.PaginatedResult[models.Submission]{
			Items: []models.Submission{
				{ID: 1, AssignmentID: 1, UserID: 10, Score: &score90, WorkflowState: "graded"},
			},
			TotalCount: 1,
			Page:       1,
			PerPage:    10000,
		}, nil)

	svc := service.NewGradingService(submissionRepo, assignmentRepo, groupRepo, enrollmentRepo, unweightedCourseRepo(1), nil)
	gradebook, err := svc.GetGradebook(context.Background(), 1)

	assert.NoError(t, err)
	assert.NotNil(t, gradebook)
	// Only students should appear, not teachers
	assert.Len(t, gradebook.Students, 1)
	assert.Equal(t, uint(10), gradebook.Students[0].ID)
	assert.Equal(t, "Alice", gradebook.Students[0].Name)
	// Assignments
	assert.Len(t, gradebook.Assignments, 1)
	assert.Equal(t, "HW1", gradebook.Assignments[0].Name)
	// Submissions map
	assert.NotNil(t, gradebook.Submissions["10"])
	assert.NotNil(t, gradebook.Submissions["10"]["1"])
	assert.Equal(t, 90.0, *gradebook.Submissions["10"]["1"].Score)
}

func TestGetGradebook_OnlyStudents(t *testing.T) {
	assignmentRepo := new(mocks.MockAssignmentRepository)
	submissionRepo := new(mocks.MockSubmissionRepository)
	groupRepo := new(mocks.MockAssignmentGroupRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)

	enrollmentRepo.On("ListByCourseID", mock.Anything, uint(1), uint(0), mock.AnythingOfType("repository.PaginationParams")).
		Return(&repository.PaginatedResult[models.Enrollment]{
			Items: []models.Enrollment{
				{ID: 1, UserID: 10, CourseID: 1, Type: "TeacherEnrollment", User: &models.User{ID: 10, Name: "Prof. Smith"}},
				{ID: 2, UserID: 20, CourseID: 1, Type: "StudentEnrollment", User: &models.User{ID: 20, Name: "Bob"}},
				{ID: 3, UserID: 30, CourseID: 1, Type: "TaEnrollment", User: &models.User{ID: 30, Name: "TA Jane"}},
				{ID: 4, UserID: 40, CourseID: 1, Type: "ObserverEnrollment", User: &models.User{ID: 40, Name: "Parent"}},
				{ID: 5, UserID: 50, CourseID: 1, Type: "StudentEnrollment", User: &models.User{ID: 50, Name: "Carol"}},
			},
			TotalCount: 5,
			Page:       1,
			PerPage:    1000,
		}, nil)
	assignmentRepo.On("ListByCourseID", mock.Anything, uint(1), mock.AnythingOfType("repository.PaginationParams")).
		Return(makeAssignments(), nil)
	submissionRepo.On("BulkListByCourse", mock.Anything, uint(1), mock.AnythingOfType("repository.PaginationParams")).
		Return(&repository.PaginatedResult[models.Submission]{
			Items:      []models.Submission{},
			TotalCount: 0,
			Page:       1,
			PerPage:    10000,
		}, nil)

	svc := service.NewGradingService(submissionRepo, assignmentRepo, groupRepo, enrollmentRepo, unweightedCourseRepo(1), nil)
	gradebook, err := svc.GetGradebook(context.Background(), 1)

	assert.NoError(t, err)
	assert.NotNil(t, gradebook)
	// Only StudentEnrollment entries should be included
	assert.Len(t, gradebook.Students, 2)

	studentNames := make([]string, len(gradebook.Students))
	for i, s := range gradebook.Students {
		studentNames[i] = s.Name
	}
	assert.Contains(t, studentNames, "Bob")
	assert.Contains(t, studentNames, "Carol")
	assert.NotContains(t, studentNames, "Prof. Smith")
	assert.NotContains(t, studentNames, "TA Jane")
	assert.NotContains(t, studentNames, "Parent")
}
