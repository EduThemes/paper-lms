package service

// Quiz → gradebook bridge (G041). SubmissionService implements
// QuizGradebook so a quiz's kept score lands on its online_quiz
// assignment through the same grading path as any other score: the
// course late policy is applied to the entered score, the assignment's
// post policy decides whether the student sees it, and the change is
// written to the grade-change log.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// QuizScorePosted reports whether a student may see their quiz grade: an
// automatically-posted assignment always shows it; a manual one only once
// the submission is posted.
func (s *SubmissionService) QuizScorePosted(ctx context.Context, assignment *models.Assignment, userID uint) bool {
	if assignment == nil || !assignment.UsesManualPostPolicy() {
		return true
	}
	// accountID=0: internal caller (assignment resolved from a
	// tenant-scoped quiz row).
	sub, err := s.submissionRepo.FindByAssignmentAndUser(ctx, assignment.ID, userID, 0)
	return err == nil && sub != nil && sub.PostedAt != nil
}

// quizSubmissionType is the submission_type of a quiz-backed submission.
const quizSubmissionType = "online_quiz"

// ErrQuizScoreNotStudent is returned when the quiz taker is course staff
// (a preview), whose score never goes to the gradebook.
var ErrQuizScoreNotStudent = errors.New("quiz taker is not a student of the course")

// RecordQuizScore writes a quiz attempt's kept score onto the student's
// submission for the quiz's assignment, creating the submission when it
// does not exist. The submission is stamped with the attempt's finish
// time (late is judged against the student's effective due date) and,
// while an essay / file-upload answer still needs grading, is left in
// pending_review with the partial score (Canvas behavior).
//
// An excused submission keeps its excusal and is not regraded. A closed
// grading period refuses the write.
func (s *SubmissionService) RecordQuizScore(ctx context.Context, assignment *models.Assignment, rec QuizScoreRecord) (*models.Submission, error) {
	if assignment == nil {
		return nil, ErrAssignmentUnavailable
	}
	// Staff who take (preview) a quiz get no gradebook row: their highest
	// active enrollment in the course is not a student one. A student
	// whose enrollment has since concluded still has their score kept.
	// accountID=0: internal caller (course id comes from the quiz row).
	if s.enrollmentRepo != nil {
		if e, err := s.enrollmentRepo.FindByUserAndCourse(ctx, rec.UserID, assignment.CourseID, 0); err == nil && e != nil && e.Type != "StudentEnrollment" {
			return nil, ErrQuizScoreNotStudent
		}
	}
	if closed, periodTitle := s.isGradingPeriodClosed(ctx, assignment); closed {
		return nil, fmt.Errorf("grading period %q is closed — grades cannot be modified", periodTitle)
	}

	// accountID=0: internal caller — the assignment was resolved from a
	// quiz row the caller already loaded under its tenant, and
	// (assignment_id, user_id) identifies the row.
	sub, findErr := s.submissionRepo.FindByAssignmentAndUser(ctx, assignment.ID, rec.UserID, 0)
	isNew := findErr != nil || sub == nil
	if isNew {
		sub = &models.Submission{AssignmentID: assignment.ID, UserID: rec.UserID}
	}
	oldGrade, oldScore := sub.Grade, sub.Score

	subType := quizSubmissionType
	submittedAt := rec.SubmittedAt
	if submittedAt.IsZero() {
		submittedAt = time.Now()
	}
	sub.SubmissionType = &subType
	sub.SubmittedAt = &submittedAt
	if rec.Attempt > 0 {
		sub.Attempt = rec.Attempt
	}
	dueAt, _, _ := s.EffectiveDates(ctx, assignment, rec.UserID)
	sub.Late = dueAt != nil && submittedAt.After(*dueAt)
	sub.Missing = false

	if sub.Excused {
		// Keep the excusal; record only that the quiz was taken.
		if sub.WorkflowState == "" || sub.WorkflowState == models.SubmissionUnsubmitted {
			sub.WorkflowState = models.SubmissionSubmitted
		}
	} else {
		s.applyGrade(ctx, assignment, sub, rec.Score, rec.GraderID, time.Now())
		if rec.GraderID == 0 {
			sub.GraderID = nil // auto-graded by the quiz engine
		}
		if rec.PendingReview {
			sub.WorkflowState = models.SubmissionPendingReview
		}
	}

	if isNew {
		if err := s.submissionRepo.Create(ctx, sub); err != nil {
			return nil, err
		}
	} else if err := s.submissionRepo.Update(ctx, sub); err != nil {
		return nil, err
	}
	if !sub.Excused {
		s.logGradeChange(ctx, assignment, sub, rec.GraderID, oldGrade, oldScore)
		s.fireIfPosted(sub)
	}
	return sub, nil
}
