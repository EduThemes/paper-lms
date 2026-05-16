package service

import (
	"context"
	"fmt"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// SpeedGraderData holds the complete data set needed for the SpeedGrader view.
type SpeedGraderData struct {
	Assignment  *models.Assignment   `json:"assignment"`
	Students    []SpeedGraderStudent `json:"students"`
	UserNameMap map[uint]string      `json:"-"` // all enrolled user names (teachers, TAs, students)
}

// SpeedGraderStudent represents a single student's submission data in SpeedGrader.
type SpeedGraderStudent struct {
	UserID     uint                       `json:"user_id"`
	UserName   string                     `json:"user_name"`
	Submission *models.Submission         `json:"submission"`
	Comments   []models.SubmissionComment `json:"comments"`
}

// SpeedGraderStudentSubmission holds a single student's submission with comments.
type SpeedGraderStudentSubmission struct {
	Submission *models.Submission         `json:"submission"`
	Comments   []models.SubmissionComment `json:"comments"`
}

// SpeedGraderService provides data aggregation methods for the SpeedGrader UI.
type SpeedGraderService struct {
	submissionRepo        repository.SubmissionRepository
	submissionCommentRepo repository.SubmissionCommentRepository
	assignmentRepo        repository.AssignmentRepository
	enrollmentRepo        repository.EnrollmentRepository
	rubricAssessRepo      repository.RubricAssessmentRepository
}

// NewSpeedGraderService creates a new SpeedGraderService with the required repositories.
func NewSpeedGraderService(
	submissionRepo repository.SubmissionRepository,
	submissionCommentRepo repository.SubmissionCommentRepository,
	assignmentRepo repository.AssignmentRepository,
	enrollmentRepo repository.EnrollmentRepository,
	rubricAssessRepo repository.RubricAssessmentRepository,
) *SpeedGraderService {
	return &SpeedGraderService{
		submissionRepo:        submissionRepo,
		submissionCommentRepo: submissionCommentRepo,
		assignmentRepo:        assignmentRepo,
		enrollmentRepo:        enrollmentRepo,
		rubricAssessRepo:      rubricAssessRepo,
	}
}

// GetSpeedGraderData returns the complete SpeedGrader data for an assignment,
// including the assignment details and all enrolled students with their
// submissions and comments.
func (s *SpeedGraderService) GetSpeedGraderData(ctx context.Context, courseID, assignmentID uint) (*SpeedGraderData, error) {
	// Fetch assignment
	assignment, err := s.assignmentRepo.FindByID(ctx, assignmentID, 0)
	if err != nil {
		return nil, fmt.Errorf("assignment not found: %w", err)
	}
	if assignment.CourseID != courseID {
		return nil, fmt.Errorf("assignment does not belong to this course")
	}

	// Fetch all enrollments for the course to get students
	params := repository.PaginationParams{Page: 1, PerPage: 1000}
	enrollments, err := s.enrollmentRepo.ListByCourseID(ctx, courseID, params)
	if err != nil {
		return nil, fmt.Errorf("could not fetch enrollments: %w", err)
	}

	// Fetch all submissions for this assignment
	submissionResult, err := s.submissionRepo.ListByAssignmentID(ctx, assignmentID, repository.PaginationParams{Page: 1, PerPage: 1000})
	if err != nil {
		return nil, fmt.Errorf("could not fetch submissions: %w", err)
	}

	// Build submission lookup by user ID
	submissionsByUser := make(map[uint]*models.Submission)
	for i, sub := range submissionResult.Items {
		submissionsByUser[sub.UserID] = &submissionResult.Items[i]
	}

	// Build name lookup from ALL enrollments (teachers, TAs, students)
	userNameMap := make(map[uint]string)
	for _, enrollment := range enrollments.Items {
		if enrollment.User != nil && enrollment.User.Name != "" {
			userNameMap[enrollment.UserID] = enrollment.User.Name
		}
	}

	// Build student list from enrollments (only StudentEnrollment types)
	students := make([]SpeedGraderStudent, 0)
	anonymousIndex := 0
	for _, enrollment := range enrollments.Items {
		if enrollment.Type != "StudentEnrollment" {
			continue
		}

		anonymousIndex++
		userName := ""
		if assignment.AnonymousGrading {
			userName = fmt.Sprintf("Student %d", anonymousIndex)
		} else {
			if enrollment.User != nil {
				userName = enrollment.User.Name
			}
			if userName == "" {
				userName = fmt.Sprintf("User %d", enrollment.UserID)
			}
		}

		student := SpeedGraderStudent{
			UserID:   enrollment.UserID,
			UserName: userName,
			Comments: make([]models.SubmissionComment, 0),
		}

		// Attach submission if it exists
		if sub, ok := submissionsByUser[enrollment.UserID]; ok {
			student.Submission = sub

			// Fetch comments for this submission
			comments, err := s.submissionCommentRepo.ListBySubmissionID(ctx, sub.ID, 0)
			if err == nil && len(comments) > 0 {
				student.Comments = comments
			}
		}

		students = append(students, student)
	}

	return &SpeedGraderData{
		Assignment:  assignment,
		Students:    students,
		UserNameMap: userNameMap,
	}, nil
}

// GetStudentSubmission returns a single student's submission with comments
// for the given assignment.
func (s *SpeedGraderService) GetStudentSubmission(ctx context.Context, assignmentID, userID uint) (*SpeedGraderStudentSubmission, error) {
	// accountID=0: SpeedGrader's tenant scope is verified at the handler when
	// it loads the parent assignment; this read trusts that upstream check.
	submission, err := s.submissionRepo.FindByAssignmentAndUser(ctx, assignmentID, userID, 0)
	if err != nil {
		return &SpeedGraderStudentSubmission{
			Submission: nil,
			Comments:   make([]models.SubmissionComment, 0),
		}, nil
	}

	comments, err := s.submissionCommentRepo.ListBySubmissionID(ctx, submission.ID, 0)
	if err != nil {
		comments = make([]models.SubmissionComment, 0)
	}

	return &SpeedGraderStudentSubmission{
		Submission: submission,
		Comments:   comments,
	}, nil
}
