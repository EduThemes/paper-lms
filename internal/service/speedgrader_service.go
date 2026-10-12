package service

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// SpeedGraderData holds the complete data set needed for the SpeedGrader view.
type SpeedGraderData struct {
	Assignment  *models.Assignment   `json:"assignment"`
	Students    []SpeedGraderStudent `json:"students"`
	UserNameMap map[uint]string      `json:"-"` // all enrolled user names (teachers, TAs, students)
	Anonymized  bool                 `json:"-"` // student identities hidden from graders
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
	Assignment *models.Assignment         `json:"-"`
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
func (s *SpeedGraderService) GetSpeedGraderData(ctx context.Context, courseID, assignmentID, accountID uint) (*SpeedGraderData, error) {
	return s.GetSpeedGraderDataAnonymized(ctx, courseID, assignmentID, accountID, nil)
}

// GetSpeedGraderDataAnonymized is GetSpeedGraderData with the anonymity
// decision supplied by the caller: anonymize reports, given the course's
// student ids, whether identities are hidden (Canvas lifts anonymity once
// grades are posted). nil means "whenever anonymous grading is on".
func (s *SpeedGraderService) GetSpeedGraderDataAnonymized(ctx context.Context, courseID, assignmentID, accountID uint, anonymize func(*models.Assignment, []uint) bool) (*SpeedGraderData, error) {
	// Fetch assignment under the caller's tenant and tie it to the course.
	assignment, err := s.assignmentRepo.FindByID(ctx, assignmentID, accountID)
	if err != nil {
		return nil, ErrSpeedGraderAssignmentNotFound
	}
	if assignment.CourseID != courseID || assignment.WorkflowState == models.AssignmentDeleted {
		return nil, ErrSpeedGraderAssignmentNotFound
	}

	// Fetch all enrollments for the course to get students
	params := repository.PaginationParams{Page: 1, PerPage: 1000}
	enrollments, err := s.enrollmentRepo.ListByCourseID(ctx, courseID, 0, params)
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

	// Build student list from enrollments (only StudentEnrollment types).
	// For anonymous grading the label order must not follow enrollment
	// order (that alone can de-anonymize), so students are shuffled
	// deterministically per assignment before numbering.
	// A student enrolled in two sections holds two StudentEnrollment
	// rows; grade them once.
	studentEnrollments := make([]models.Enrollment, 0, len(enrollments.Items))
	seenStudent := map[uint]bool{}
	for _, enrollment := range enrollments.Items {
		if enrollment.Type == "StudentEnrollment" && !seenStudent[enrollment.UserID] {
			seenStudent[enrollment.UserID] = true
			studentEnrollments = append(studentEnrollments, enrollment)
		}
	}
	hideNames := assignment.AnonymousGrading
	if anonymize != nil {
		ids := make([]uint, 0, len(studentEnrollments))
		for _, e := range studentEnrollments {
			ids = append(ids, e.UserID)
		}
		hideNames = anonymize(assignment, ids)
	}
	if hideNames {
		sort.SliceStable(studentEnrollments, func(i, j int) bool {
			return anonymousSortKey(assignment.ID, studentEnrollments[i].UserID) < anonymousSortKey(assignment.ID, studentEnrollments[j].UserID)
		})
	}

	students := make([]SpeedGraderStudent, 0)
	anonymousIndex := 0
	for _, enrollment := range studentEnrollments {
		anonymousIndex++
		userName := ""
		if hideNames {
			userName = fmt.Sprintf("Student %d", anonymousIndex)
			// B119: a student's own comments must carry the anonymous
			// label, never the real name from the course roster.
			userNameMap[enrollment.UserID] = userName
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
			comments, err := s.submissionCommentRepo.ListBySubmissionID(ctx, sub.ID, accountID)
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
		Anonymized:  hideNames,
	}, nil
}

// ErrSpeedGraderAssignmentNotFound is returned when the assignment is not
// in the caller's tenant or not in the URL's course. Handlers map it to 404.
var ErrSpeedGraderAssignmentNotFound = errors.New("assignment not found")

// GetStudentSubmission returns a single student's submission with comments
// for the given assignment. The assignment is loaded under accountID and
// must belong to courseID (B010: the handler used to discard course_id
// and every read here ran with accountID=0, so any instructor could read
// any tenant's submission and comments).
func (s *SpeedGraderService) GetStudentSubmission(ctx context.Context, courseID, assignmentID, userID, accountID uint) (*SpeedGraderStudentSubmission, error) {
	assignment, err := s.assignmentRepo.FindByID(ctx, assignmentID, accountID)
	if err != nil || assignment.CourseID != courseID || assignment.WorkflowState == models.AssignmentDeleted {
		return nil, ErrSpeedGraderAssignmentNotFound
	}

	submission, err := s.submissionRepo.FindByAssignmentAndUser(ctx, assignmentID, userID, accountID)
	if err != nil {
		return &SpeedGraderStudentSubmission{
			Assignment: assignment,
			Submission: nil,
			Comments:   make([]models.SubmissionComment, 0),
		}, nil
	}

	comments, err := s.submissionCommentRepo.ListBySubmissionID(ctx, submission.ID, accountID)
	if err != nil {
		comments = make([]models.SubmissionComment, 0)
	}

	return &SpeedGraderStudentSubmission{
		Assignment: assignment,
		Submission: submission,
		Comments:   comments,
	}, nil
}

// anonymousSortKey gives a stable, per-assignment pseudo-random order for
// anonymous grading so "Student N" numbering doesn't mirror enrollment
// order.
func anonymousSortKey(assignmentID, userID uint) uint64 {
	var buf [16]byte
	binary.BigEndian.PutUint64(buf[:8], uint64(assignmentID))
	binary.BigEndian.PutUint64(buf[8:], uint64(userID))
	sum := sha256.Sum256(buf[:])
	return binary.BigEndian.Uint64(sum[:8])
}
