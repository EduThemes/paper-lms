package models

import (
	"errors"
	"fmt"
)

// ErrInvalidStateTransition is returned when a handler attempts to move
// a row from one workflow_state to another that isn't allowed by the
// model's state machine, or when the caller's role isn't authorized to
// make the requested transition.
//
// PENTEST F-018 closure. Pre-fix, most Update handlers accepted the
// JSON body's `workflow_state` field verbatim and assigned it onto the
// loaded model. That let an authenticated user mark another row
// `deleted`, flip `unpublished → published` without a permission
// check, or stage other state machine violations purely by editing
// the request body.
//
// The TransitionXxx helpers in this file enforce two contracts:
//
//   1. **Reachability**: the requested target state is one of the
//      explicitly-allowed transitions from the current state. Unknown
//      states or backwards transitions are rejected.
//
//   2. **Role authorization**: the calling role appears in the
//      allow-list for that specific edge. Even an admin shouldn't be
//      able to magically promote `deleted → published` if the data
//      model says that's not a thing.
//
// Adding a new model: define the transitions map below, expose a
// `TransitionXxxWorkflowState(from, to, role)` thin wrapper, and call
// it from the handler before assigning the new state.
var ErrInvalidStateTransition = errors.New("invalid workflow_state transition")

// Role names recognized by the state-machine helpers. These mirror the
// strings used by the auth middleware to label callers — keeping the
// constants here means callers don't pass string literals into the
// transition helper.
const (
	RoleStudent   = "student"
	RoleTA        = "ta"
	RoleTeacher   = "teacher"
	RoleAdmin     = "admin"
	RoleObserver  = "observer"
	RoleSuperUser = "super_admin"
)

// transitionMap is the generic shape every model's state machine
// implements: from-state → to-state → set of allowed roles. Using
// generic CourseWorkflow / AssignmentWorkflow etc. as keys keeps the
// table self-documenting at the call site.
type transitionMap[S ~string] map[S]map[S][]string

// canTransition is the shared evaluator. Returns nil iff the from→to
// edge exists AND role is in the edge's allow-list.
func canTransition[S ~string](m transitionMap[S], from, to S, role string) error {
	if from == to {
		return nil
	}
	allowed, ok := m[from]
	if !ok {
		return fmt.Errorf("%w: %q has no outgoing transitions", ErrInvalidStateTransition, from)
	}
	roles, ok := allowed[to]
	if !ok {
		return fmt.Errorf("%w: %q → %q not allowed", ErrInvalidStateTransition, from, to)
	}
	for _, r := range roles {
		if r == role || r == "*" {
			return nil
		}
	}
	return fmt.Errorf("%w: role %q cannot transition %q → %q", ErrInvalidStateTransition, role, from, to)
}

// --- Course state machine ---

var courseTransitions = transitionMap[CourseWorkflow]{
	CourseClaimed:     {CourseCreated: {RoleTeacher, RoleAdmin, RoleSuperUser}, CourseDeleted: {RoleAdmin, RoleSuperUser}},
	CourseCreated:     {CourseUnpublished: {RoleTeacher, RoleAdmin, RoleSuperUser}, CourseDeleted: {RoleAdmin, RoleSuperUser}},
	CourseUnpublished: {CourseAvailable: {RoleTeacher, RoleAdmin, RoleSuperUser}, CourseDeleted: {RoleAdmin, RoleSuperUser}},
	CourseAvailable:   {CourseUnpublished: {RoleTeacher, RoleAdmin, RoleSuperUser}, CourseCompleted: {RoleTeacher, RoleAdmin, RoleSuperUser}, CourseDeleted: {RoleAdmin, RoleSuperUser}},
	CourseCompleted:   {CourseAvailable: {RoleAdmin, RoleSuperUser}, CourseDeleted: {RoleAdmin, RoleSuperUser}},
	CourseDeleted:     {CourseAvailable: {RoleSuperUser}},
}

// TransitionCourseWorkflowState validates an attempted Course
// workflow_state change.
func TransitionCourseWorkflowState(from, to CourseWorkflow, role string) error {
	return canTransition(courseTransitions, from, to, role)
}

// --- DiscussionTopic state machine ---

var discussionTopicTransitions = transitionMap[DiscussionTopicWorkflow]{
	DiscussionTopicUnpublished: {DiscussionTopicActive: {RoleTeacher, RoleAdmin, RoleSuperUser}, DiscussionTopicDeleted: {RoleTeacher, RoleAdmin, RoleSuperUser}},
	DiscussionTopicActive:      {DiscussionTopicUnpublished: {RoleTeacher, RoleAdmin, RoleSuperUser}, DiscussionTopicLocked: {RoleTeacher, RoleAdmin, RoleSuperUser}, DiscussionTopicPostDelayed: {RoleTeacher, RoleAdmin, RoleSuperUser}, DiscussionTopicDeleted: {RoleTeacher, RoleAdmin, RoleSuperUser}},
	DiscussionTopicLocked:      {DiscussionTopicActive: {RoleTeacher, RoleAdmin, RoleSuperUser}, DiscussionTopicDeleted: {RoleTeacher, RoleAdmin, RoleSuperUser}},
	DiscussionTopicPostDelayed: {DiscussionTopicActive: {RoleTeacher, RoleAdmin, RoleSuperUser}, DiscussionTopicDeleted: {RoleTeacher, RoleAdmin, RoleSuperUser}},
	DiscussionTopicDeleted:     {},
}

// TransitionDiscussionTopicWorkflowState validates an attempted
// DiscussionTopic workflow_state change.
func TransitionDiscussionTopicWorkflowState(from, to DiscussionTopicWorkflow, role string) error {
	return canTransition(discussionTopicTransitions, from, to, role)
}

// --- Assignment state machine ---

var assignmentTransitions = transitionMap[AssignmentWorkflow]{
	AssignmentUnpublished: {AssignmentPublished: {RoleTeacher, RoleAdmin, RoleSuperUser}, AssignmentDeleted: {RoleTeacher, RoleAdmin, RoleSuperUser}},
	AssignmentPublished:   {AssignmentUnpublished: {RoleTeacher, RoleAdmin, RoleSuperUser}, AssignmentDeleted: {RoleTeacher, RoleAdmin, RoleSuperUser}},
	AssignmentDeleted:     {},
}

// TransitionAssignmentWorkflowState validates an attempted Assignment
// workflow_state change.
func TransitionAssignmentWorkflowState(from, to AssignmentWorkflow, role string) error {
	return canTransition(assignmentTransitions, from, to, role)
}

// --- Submission state machine ---

// SubmissionWorkflow transitions are stricter than the others:
// `graded → submitted` would be a covert reset of grading state and
// must not be reachable from a student request. Reopening a graded
// submission is a deliberate teacher-driven flow (no handler today;
// chain with F-051 follow-up).
var submissionTransitions = transitionMap[SubmissionWorkflow]{
	SubmissionUnsubmitted: {SubmissionSubmitted: {RoleStudent, RoleTeacher, RoleAdmin, RoleSuperUser}, SubmissionPendingReview: {RoleTeacher, RoleAdmin, RoleSuperUser}, SubmissionGraded: {RoleTeacher, RoleAdmin, RoleSuperUser}},
	SubmissionSubmitted:   {SubmissionGraded: {RoleTeacher, RoleAdmin, RoleSuperUser}, SubmissionPendingReview: {RoleTeacher, RoleAdmin, RoleSuperUser}},
	SubmissionPendingReview: {SubmissionGraded: {RoleTeacher, RoleAdmin, RoleSuperUser}, SubmissionSubmitted: {RoleTeacher, RoleAdmin, RoleSuperUser}},
	SubmissionGraded:        {SubmissionPendingReview: {RoleTeacher, RoleAdmin, RoleSuperUser}}, // explicit teacher-driven reopen only
}

// TransitionSubmissionWorkflowState validates an attempted Submission
// workflow_state change.
func TransitionSubmissionWorkflowState(from, to SubmissionWorkflow, role string) error {
	return canTransition(submissionTransitions, from, to, role)
}
