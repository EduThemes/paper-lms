package models_test

import (
	"errors"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// PENTEST F-018 regression suite. The state-machine helpers in
// internal/domain/models/state_transitions.go enforce two contracts:
// reachability (the edge exists) and role authz (the caller is
// allowed). These tests lock the lowest-trust attacker shapes:
//
//   - student trying to unilaterally promote a course state
//   - admin trying a non-edge (e.g., deleted → unpublished)
//   - submitting role with no auth context
//
// Adding a new model: copy a TestXxxTransitions block.

// TestTransitionCourseWorkflowState_StudentCannotTransition — a
// student is the canonical low-trust caller. A request body with
// workflow_state="published" on a course where they're enrolled must
// be rejected by the state machine. Pre-F-018 this would have
// silently flipped the row.
func TestTransitionCourseWorkflowState_StudentCannotTransition(t *testing.T) {
	err := models.TransitionCourseWorkflowState(models.CourseUnpublished, models.CourseAvailable, models.RoleStudent)
	if err == nil {
		t.Fatal("expected student to be denied unpublished→available")
	}
	if !errors.Is(err, models.ErrInvalidStateTransition) {
		t.Errorf("error should be ErrInvalidStateTransition, got %v", err)
	}
}

// TestTransitionCourseWorkflowState_TeacherCanPublish — happy path.
// Teacher promoting unpublished→available is the canonical action;
// must succeed without role downgrade.
func TestTransitionCourseWorkflowState_TeacherCanPublish(t *testing.T) {
	if err := models.TransitionCourseWorkflowState(models.CourseUnpublished, models.CourseAvailable, models.RoleTeacher); err != nil {
		t.Errorf("teacher should be allowed to publish: %v", err)
	}
}

// TestTransitionCourseWorkflowState_TeacherCannotUndelete — once a
// course is deleted only super_admin can resurrect it. A teacher
// flipping deleted→available is the canonical "covertly undo an
// admin destructive action" attack the F-018 fix exists to block.
func TestTransitionCourseWorkflowState_TeacherCannotUndelete(t *testing.T) {
	err := models.TransitionCourseWorkflowState(models.CourseDeleted, models.CourseAvailable, models.RoleTeacher)
	if err == nil {
		t.Fatal("expected teacher to be denied deleted→available")
	}
}

// TestTransitionCourseWorkflowState_SuperAdminCanUndelete — the
// recovery path: super_admin restoring a deleted course.
func TestTransitionCourseWorkflowState_SuperAdminCanUndelete(t *testing.T) {
	if err := models.TransitionCourseWorkflowState(models.CourseDeleted, models.CourseAvailable, models.RoleSuperUser); err != nil {
		t.Errorf("super_admin should be allowed to undelete: %v", err)
	}
}

// TestTransitionCourseWorkflowState_NoOpAllowed — assigning the same
// state to the same state must always succeed regardless of role
// (handlers typically read-modify-write the whole struct; we don't
// want to penalize a no-op write).
func TestTransitionCourseWorkflowState_NoOpAllowed(t *testing.T) {
	if err := models.TransitionCourseWorkflowState(models.CourseAvailable, models.CourseAvailable, models.RoleStudent); err != nil {
		t.Errorf("no-op transition should always succeed: %v", err)
	}
}

// TestTransitionCourseWorkflowState_UnknownFromStateRejected — an
// attacker who can corrupt the DB row to an unknown workflow_state
// (defense-in-depth, not a real threat path today) should not be
// able to transition out of it.
func TestTransitionCourseWorkflowState_UnknownFromStateRejected(t *testing.T) {
	err := models.TransitionCourseWorkflowState(models.CourseWorkflow("nonexistent_state"), models.CourseAvailable, models.RoleAdmin)
	if err == nil {
		t.Fatal("expected unknown from-state to be rejected")
	}
}

// TestTransitionSubmissionWorkflowState_StudentCannotUngrade — the
// F-051 / F-018 chain. A student resubmitting after a grade has
// landed must not be able to flip the state back to "submitted"
// (which would clear the grade context and bump attempts++ in the
// pre-fix submission service). Only teacher+ can reopen.
func TestTransitionSubmissionWorkflowState_StudentCannotUngrade(t *testing.T) {
	err := models.TransitionSubmissionWorkflowState(models.SubmissionGraded, models.SubmissionPendingReview, models.RoleStudent)
	if err == nil {
		t.Fatal("expected student to be denied graded→pending_review")
	}
}

// TestTransitionSubmissionWorkflowState_TeacherCanReopen — happy
// path: teacher reopens a graded submission for re-review.
func TestTransitionSubmissionWorkflowState_TeacherCanReopen(t *testing.T) {
	if err := models.TransitionSubmissionWorkflowState(models.SubmissionGraded, models.SubmissionPendingReview, models.RoleTeacher); err != nil {
		t.Errorf("teacher should be allowed to reopen for review: %v", err)
	}
}

// TestTransitionDiscussionTopicWorkflowState_DeletedTerminal — once
// a topic is deleted there's no transition out; only super_admin
// would be able to restore (and we deliberately don't list that
// edge, treating delete as terminal at the application level).
func TestTransitionDiscussionTopicWorkflowState_DeletedTerminal(t *testing.T) {
	err := models.TransitionDiscussionTopicWorkflowState(models.DiscussionTopicDeleted, models.DiscussionTopicActive, models.RoleSuperUser)
	if err == nil {
		t.Fatal("expected deleted→active to be rejected (terminal state)")
	}
}
