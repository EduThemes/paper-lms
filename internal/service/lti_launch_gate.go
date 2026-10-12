package service

import (
	"context"
	"time"
)

// LTILaunchGate decides whether a non-staff caller may launch a tool link
// that sits in a module item (contentTagID) and/or is an assignment's
// tool (assignmentID); 0 means "none". The module item's own launch-params
// route refuses locked items, and the course tool launch route must not be
// a way around that: the launch_path points straight at it.
type LTILaunchGate interface {
	LearnerMayLaunch(ctx context.Context, courseID, userID, accountID, contentTagID, assignmentID uint) (bool, error)
}

// SetLaunchGate wires the module-lock / availability gate (nil-safe).
func (s *LTIService) SetLaunchGate(g LTILaunchGate) { s.launchGate = g }

// learnerMayLaunch runs the gate for a non-staff launch. A gate failure
// refuses the launch (fail closed).
func (s *LTIService) learnerMayLaunch(ctx context.Context, req LTILaunchRequest, contentTagID, assignmentID uint) bool {
	if req.IsStaff || s.launchGate == nil {
		return true
	}
	ok, err := s.launchGate.LearnerMayLaunch(ctx, req.CourseID, req.UserID, req.AccountID, contentTagID, assignmentID)
	return err == nil && ok
}

// AssignmentDatesFunc returns a student's effective dates for an
// assignment and whether it is assigned to them at all.
type AssignmentDatesFunc func(ctx context.Context, courseID, userID, accountID, assignmentID uint) (*EffectiveDueDate, bool, error)

// ModuleLaunchGate is the LTILaunchGate over the module progression
// engine: a module item must be unlocked for the student (unlock date,
// prerequisites, sequential progress); an assignment must not be hidden
// (Mastery Paths) or module-locked, must be assigned to the student, and
// must be inside its (override-aware) unlock / lock window. Callers who
// are not learners of the course (observers, designers) pass, as with
// the content gate.
type ModuleLaunchGate struct {
	progression *ModuleProgressionService
	dates       AssignmentDatesFunc
	now         func() time.Time
}

// NewModuleLaunchGate builds the gate; dates may be nil (no date window).
func NewModuleLaunchGate(progression *ModuleProgressionService, dates AssignmentDatesFunc) *ModuleLaunchGate {
	return &ModuleLaunchGate{progression: progression, dates: dates, now: time.Now}
}

// LearnerMayLaunch implements LTILaunchGate.
func (g *ModuleLaunchGate) LearnerMayLaunch(ctx context.Context, courseID, userID, accountID, contentTagID, assignmentID uint) (bool, error) {
	if g == nil || g.progression == nil {
		return true, nil
	}
	learner, err := g.progression.IsLearner(ctx, userID, courseID)
	if err != nil {
		return false, err
	}
	if !learner {
		return true, nil
	}
	if contentTagID != 0 {
		progress, err := g.progression.Evaluate(ctx, courseID, userID)
		if err != nil {
			return false, err
		}
		if _, ip := progress.Item(contentTagID); ip == nil || ip.Locked {
			return false, nil
		}
	}
	if assignmentID != 0 {
		access, err := g.progression.CheckContentAccess(ctx, courseID, userID, GatedAssignment, assignmentID)
		if err != nil {
			return false, err
		}
		if access.Hidden || access.Locked {
			return false, nil
		}
		if g.dates != nil {
			d, assigned, err := g.dates(ctx, courseID, userID, accountID, assignmentID)
			if err != nil {
				return false, err
			}
			if !assigned || d == nil {
				return false, nil
			}
			now := g.now()
			if (d.UnlockAt != nil && now.Before(*d.UnlockAt)) || (d.LockAt != nil && now.After(*d.LockAt)) {
				return false, nil
			}
		}
	}
	return true, nil
}
