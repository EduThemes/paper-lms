package main

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"gorm.io/gorm"
)

// gradingUIDeps are the already-built collaborators the grading-UI
// features hook into. Kept in one struct so main.go wires the lane with
// a single call.
type gradingUIDeps struct {
	submissionRepo      repository.SubmissionRepository
	assignmentRepo      repository.AssignmentRepository
	courseRepo          repository.CourseRepository
	groupMembershipRepo repository.GroupMembershipRepository
	peerReviewRepo      repository.PeerReviewRepository
	enrollmentRepo      repository.EnrollmentRepository
	attachmentRepo      repository.AttachmentRepository
	userRepo            repository.UserRepository

	submissionService *service.SubmissionService
	peerReviewService *service.PeerReviewService
	fileService       *service.FileService
	rubricService     *service.RubricService

	submissionHandler       *handlers.SubmissionHandler
	speedGraderHandler      *handlers.SpeedGraderHandler
	gradebookHandler        *handlers.GradebookHandler
	assignmentHandler       *handlers.AssignmentHandler
	peerReviewHandler       *handlers.PeerReviewHandler
	rubricAssessmentHandler *handlers.RubricAssessmentHandler
	gradingPolicyHandler    *handlers.GradingPolicyHandler
}

// wireGradingUI builds the GradingUIService / handler and attaches the
// optional grading-UI collaborators to the existing handlers: attempt
// history, anonymous grading, moderated grading, custom statuses, group
// comments, assignment setting validation and peer-review names.
func wireGradingUI(database *gorm.DB, d gradingUIDeps) *handlers.GradingUIHandler {
	repo := postgres.NewGradingUIRepository(database)
	svc := service.NewGradingUIService(repo, d.submissionRepo, d.assignmentRepo, d.courseRepo, d.groupMembershipRepo, d.peerReviewRepo, d.submissionService)

	d.submissionHandler.SetGradingUIService(svc)
	d.speedGraderHandler.SetGradingUIService(svc)
	d.gradebookHandler.SetGradingUIService(svc)
	d.assignmentHandler.SetGradingUIDeps(d.enrollmentRepo, d.attachmentRepo)
	d.assignmentHandler.SetGradesExistCheck(svc.GradesExist)
	d.peerReviewHandler.SetUserRepository(d.userRepo)
	anonymity := handlers.AnonymityCheck(func(ctx context.Context, courseID, assignmentID, accountID uint) bool {
		a, err := d.assignmentRepo.FindByID(ctx, assignmentID, accountID)
		if err != nil || a == nil || a.CourseID != courseID || !a.AnonymousGrading {
			return false
		}
		students, err := d.submissionService.CourseStudents(ctx, courseID, accountID)
		if err != nil {
			return true // fail closed: keep the student hidden
		}
		ids := make([]uint, 0, len(students))
		for _, s := range students {
			ids = append(ids, s.UserID)
		}
		return svc.AnonymizeStudents(ctx, a, ids)
	})
	if d.rubricAssessmentHandler != nil {
		d.rubricAssessmentHandler.SetAnonymityCheck(anonymity)
	}
	// The user-addressed SpeedGrader and history routes refuse graders
	// while students are anonymized.
	d.speedGraderHandler.SetAnonymityCheck(anonymity)
	if d.gradingPolicyHandler != nil {
		d.gradingPolicyHandler.SetAnonymityCheck(anonymity)
	}

	h := handlers.NewGradingUIHandler(svc, d.submissionHandler, d.userRepo, d.peerReviewService, d.fileService)
	h.SetRubricService(d.rubricService)
	return h
}
