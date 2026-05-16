package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
)

// MasteryPathService implements Canvas-compatible Conditional Release.
//
// At grade time we look up the rule for the trigger assignment, locate the
// student's score band, and persist `assigned` action rows for every set in
// that band. Idempotent — calling EvaluateForStudent twice with the same
// submission produces the same DB state.
type MasteryPathService struct {
	masteryRepo    *postgres.MasteryPathRepository
	submissionRepo repository.SubmissionRepository
	assignmentRepo repository.AssignmentRepository
}

func NewMasteryPathService(
	masteryRepo *postgres.MasteryPathRepository,
	submissionRepo repository.SubmissionRepository,
	assignmentRepo repository.AssignmentRepository,
) *MasteryPathService {
	return &MasteryPathService{
		masteryRepo:    masteryRepo,
		submissionRepo: submissionRepo,
		assignmentRepo: assignmentRepo,
	}
}

// RangeInput is the JSON-friendly shape teachers submit when creating/replacing
// a rule. Bounds are accepted as 0-100 percentages OR 0.0-1.0 fractions —
// whichever the frontend prefers; we normalize.
type RangeInput struct {
	LowerBound    float64  `json:"lower_bound"`
	UpperBound    float64  `json:"upper_bound"`
	Position      int      `json:"position"`
	AssignmentIDs []uint   `json:"assignment_ids"` // single-set shortcut
	Sets          [][]uint `json:"sets,omitempty"` // optional multi-set form
}

// ListRules returns every rule in a course.
func (s *MasteryPathService) ListRules(ctx context.Context, courseID uint) ([]models.ConditionalReleaseRule, error) {
	return s.masteryRepo.ListRulesByCourse(ctx, courseID)
}

// GetRuleForAssignment returns the rule whose trigger is the given assignment,
// or nil if none exists.
func (s *MasteryPathService) GetRuleForAssignment(ctx context.Context, courseID, assignmentID uint) (*models.ConditionalReleaseRule, error) {
	return s.masteryRepo.FindRuleByTriggerAssignment(ctx, courseID, assignmentID)
}

// CreateRule fully replaces the rule's structure. If a rule already exists for
// (courseID, triggerAssignmentID) it is updated in place.
func (s *MasteryPathService) CreateRule(ctx context.Context, courseID, triggerAssignmentID uint, ranges []RangeInput) (*models.ConditionalReleaseRule, error) {
	if len(ranges) < 2 || len(ranges) > 3 {
		return nil, errors.New("a Mastery Paths rule must have 2 or 3 scoring ranges")
	}
	// Normalize bounds: if any value > 1, assume the whole set is 0-100.
	scale := 1.0
	for _, r := range ranges {
		if r.LowerBound > 1.0 || r.UpperBound > 1.0 {
			scale = 100.0
			break
		}
	}
	for i := range ranges {
		ranges[i].LowerBound /= scale
		ranges[i].UpperBound /= scale
		if ranges[i].LowerBound < 0 {
			ranges[i].LowerBound = 0
		}
		if ranges[i].UpperBound > 1.0 {
			ranges[i].UpperBound = 1.0
		}
		if ranges[i].LowerBound >= ranges[i].UpperBound {
			return nil, fmt.Errorf("range %d: lower_bound must be < upper_bound", i+1)
		}
	}

	// Re-use existing rule row if one already exists for this trigger.
	existing, err := s.masteryRepo.FindRuleByTriggerAssignment(ctx, courseID, triggerAssignmentID)
	if err != nil {
		return nil, err
	}
	rule := &models.ConditionalReleaseRule{
		CourseID:            courseID,
		TriggerAssignmentID: triggerAssignmentID,
		WorkflowState:       "active",
	}
	if existing != nil {
		rule.ID = existing.ID
	}

	// Build the nested tree.
	rule.ScoringRanges = make([]models.ConditionalReleaseScoringRange, len(ranges))
	for i, r := range ranges {
		sets := r.Sets
		if len(sets) == 0 {
			sets = [][]uint{r.AssignmentIDs}
		}
		assignmentSets := make([]models.ConditionalReleaseAssignmentSet, 0, len(sets))
		for sidx, ids := range sets {
			assocs := make([]models.ConditionalReleaseAssignmentSetAssociation, len(ids))
			for ai, aid := range ids {
				assocs[ai] = models.ConditionalReleaseAssignmentSetAssociation{
					AssignmentID:  aid,
					Position:      ai + 1,
					WorkflowState: "active",
				}
			}
			assignmentSets = append(assignmentSets, models.ConditionalReleaseAssignmentSet{
				Position:      sidx + 1,
				WorkflowState: "active",
				Associations:  assocs,
			})
		}
		rule.ScoringRanges[i] = models.ConditionalReleaseScoringRange{
			LowerBound:     r.LowerBound,
			UpperBound:     r.UpperBound,
			Position:       i + 1,
			WorkflowState:  "active",
			AssignmentSets: assignmentSets,
		}
	}

	if err := s.masteryRepo.ReplaceRule(ctx, rule); err != nil {
		return nil, err
	}
	return s.masteryRepo.FindRuleByID(ctx, rule.ID)
}

// ReplaceRule replaces an existing rule's structure (used by PUT).
func (s *MasteryPathService) ReplaceRule(ctx context.Context, ruleID uint, ranges []RangeInput) (*models.ConditionalReleaseRule, error) {
	existing, err := s.masteryRepo.FindRuleByID(ctx, ruleID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("rule not found")
	}
	return s.CreateRule(ctx, existing.CourseID, existing.TriggerAssignmentID, ranges)
}

// DeleteRule soft-deletes a rule.
func (s *MasteryPathService) DeleteRule(ctx context.Context, id uint) error {
	return s.masteryRepo.DeleteRule(ctx, id)
}

// EvaluateForStudent — given a graded submission — picks the matching scoring
// range and writes `assigned` action rows for every set in that range.
//
// Idempotent: replaying for the same submission inserts no new rows because of
// the (set_id, student_id, action_type) unique index.
func (s *MasteryPathService) EvaluateForStudent(ctx context.Context, submissionID uint) error {
	sub, err := s.submissionRepo.FindByID(ctx, submissionID, 0)
	if err != nil {
		return err
	}
	if sub == nil || sub.Score == nil {
		return nil // not yet graded — nothing to do
	}

	assignment, err := s.assignmentRepo.FindByID(ctx, sub.AssignmentID, 0)
	if err != nil {
		return err
	}
	if assignment == nil || assignment.PointsPossible == nil || *assignment.PointsPossible <= 0 {
		return nil
	}

	rule, err := s.masteryRepo.FindRuleByTriggerAssignment(ctx, assignment.CourseID, assignment.ID)
	if err != nil {
		return err
	}
	if rule == nil {
		return nil
	}

	pct := *sub.Score / *assignment.PointsPossible
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}

	now := time.Now()
	source := fmt.Sprintf("auto:submission:%d", submissionID)

	var actions []models.ConditionalReleaseAssignmentSetAction
	for _, sr := range rule.ScoringRanges {
		if !sr.ContainsScore(pct) {
			continue
		}
		for _, set := range sr.AssignmentSets {
			actions = append(actions, models.ConditionalReleaseAssignmentSetAction{
				SetID:      set.ID,
				StudentID:  sub.UserID,
				ActionType: "assigned",
				ActedAt:    now,
				Source:     source,
			})
		}
	}
	return s.masteryRepo.UpsertActions(ctx, actions)
}

// GetEligibleAssignmentsForStudent returns the union of assignment IDs the
// student qualifies for via assigned-but-not-unassigned action rows.
func (s *MasteryPathService) GetEligibleAssignmentsForStudent(ctx context.Context, courseID, studentID uint) ([]uint, error) {
	setIDs, err := s.masteryRepo.ListAssignedSetIDsForStudent(ctx, courseID, studentID)
	if err != nil {
		return nil, err
	}
	return s.masteryRepo.ListAssignmentIDsForSets(ctx, setIDs)
}
