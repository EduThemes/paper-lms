package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MasteryPathRepository persists Mastery Paths (Conditional Release) entities.
// We expose a single repo for the whole aggregate because rules, ranges, sets,
// and associations are always fetched together.
type MasteryPathRepository struct {
	db *gorm.DB
}

func NewMasteryPathRepository(db *gorm.DB) *MasteryPathRepository {
	return &MasteryPathRepository{db: db}
}

// ListRulesByCourse returns all active rules in a course with full nested
// scoring ranges, assignment sets, and associations preloaded.
func (r *MasteryPathRepository) ListRulesByCourse(ctx context.Context, courseID uint) ([]models.ConditionalReleaseRule, error) {
	var rules []models.ConditionalReleaseRule
	err := r.db.WithContext(ctx).
		Preload("ScoringRanges", "workflow_state = ?", "active").
		Preload("ScoringRanges.AssignmentSets", "workflow_state = ?", "active").
		Preload("ScoringRanges.AssignmentSets.Associations", "workflow_state = ?", "active").
		Where("course_id = ? AND workflow_state = ?", courseID, "active").
		Order("id ASC").
		Find(&rules).Error
	return rules, err
}

// FindRuleByID loads a rule with its nested children.
func (r *MasteryPathRepository) FindRuleByID(ctx context.Context, id uint) (*models.ConditionalReleaseRule, error) {
	var rule models.ConditionalReleaseRule
	err := r.db.WithContext(ctx).
		Preload("ScoringRanges", "workflow_state = ?", "active").
		Preload("ScoringRanges.AssignmentSets", "workflow_state = ?", "active").
		Preload("ScoringRanges.AssignmentSets.Associations", "workflow_state = ?", "active").
		Where("id = ? AND workflow_state = ?", id, "active").
		First(&rule).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// FindRuleByTriggerAssignment is the primary lookup at submission grading time.
func (r *MasteryPathRepository) FindRuleByTriggerAssignment(ctx context.Context, courseID, triggerAssignmentID uint) (*models.ConditionalReleaseRule, error) {
	var rule models.ConditionalReleaseRule
	err := r.db.WithContext(ctx).
		Preload("ScoringRanges", "workflow_state = ?", "active").
		Preload("ScoringRanges.AssignmentSets", "workflow_state = ?", "active").
		Preload("ScoringRanges.AssignmentSets.Associations", "workflow_state = ?", "active").
		Where("course_id = ? AND trigger_assignment_id = ? AND workflow_state = ?", courseID, triggerAssignmentID, "active").
		First(&rule).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &rule, nil
}

// ReplaceRule fully replaces a rule's children. If `rule.ID == 0` a new rule is
// created. All existing scoring ranges (and their nested children) for the rule
// are soft-deleted, then the supplied ranges/sets/associations are inserted.
// Wrapped in a transaction so callers always observe a consistent rule.
func (r *MasteryPathRepository) ReplaceRule(ctx context.Context, rule *models.ConditionalReleaseRule) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Upsert the rule row.
		if rule.ID == 0 {
			if err := tx.Create(rule).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Model(&models.ConditionalReleaseRule{}).
				Where("id = ?", rule.ID).
				Updates(map[string]interface{}{
					"course_id":             rule.CourseID,
					"trigger_assignment_id": rule.TriggerAssignmentID,
					"workflow_state":        "active",
					"updated_at":            time.Now(),
				}).Error; err != nil {
				return err
			}
		}

		// Soft-delete existing scoring ranges (and their descendents) for this rule.
		var existingRangeIDs []uint
		if err := tx.Model(&models.ConditionalReleaseScoringRange{}).
			Where("rule_id = ?", rule.ID).
			Pluck("id", &existingRangeIDs).Error; err != nil {
			return err
		}
		if len(existingRangeIDs) > 0 {
			if err := tx.Model(&models.ConditionalReleaseScoringRange{}).
				Where("rule_id = ?", rule.ID).
				Update("workflow_state", "deleted").Error; err != nil {
				return err
			}
			var existingSetIDs []uint
			if err := tx.Model(&models.ConditionalReleaseAssignmentSet{}).
				Where("scoring_range_id IN ?", existingRangeIDs).
				Pluck("id", &existingSetIDs).Error; err != nil {
				return err
			}
			if len(existingSetIDs) > 0 {
				if err := tx.Model(&models.ConditionalReleaseAssignmentSet{}).
					Where("scoring_range_id IN ?", existingRangeIDs).
					Update("workflow_state", "deleted").Error; err != nil {
					return err
				}
				if err := tx.Model(&models.ConditionalReleaseAssignmentSetAssociation{}).
					Where("set_id IN ?", existingSetIDs).
					Update("workflow_state", "deleted").Error; err != nil {
					return err
				}
			}
		}

		// Insert new ranges/sets/associations from the supplied tree.
		for ri := range rule.ScoringRanges {
			sr := &rule.ScoringRanges[ri]
			sr.ID = 0
			sr.RuleID = rule.ID
			sr.WorkflowState = "active"
			if sr.Position == 0 {
				sr.Position = ri + 1
			}
			if err := tx.Create(sr).Error; err != nil {
				return err
			}
			for si := range sr.AssignmentSets {
				set := &sr.AssignmentSets[si]
				set.ID = 0
				set.ScoringRangeID = sr.ID
				set.WorkflowState = "active"
				if set.Position == 0 {
					set.Position = si + 1
				}
				if err := tx.Create(set).Error; err != nil {
					return err
				}
				for ai := range set.Associations {
					assoc := &set.Associations[ai]
					assoc.ID = 0
					assoc.SetID = set.ID
					assoc.WorkflowState = "active"
					if assoc.Position == 0 {
						assoc.Position = ai + 1
					}
					if err := tx.Create(assoc).Error; err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
}

// DeleteRule soft-deletes a rule and all descendants.
func (r *MasteryPathRepository) DeleteRule(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.ConditionalReleaseRule{}).
			Where("id = ?", id).
			Update("workflow_state", "deleted").Error; err != nil {
			return err
		}
		var rangeIDs []uint
		if err := tx.Model(&models.ConditionalReleaseScoringRange{}).
			Where("rule_id = ?", id).
			Pluck("id", &rangeIDs).Error; err != nil {
			return err
		}
		if len(rangeIDs) == 0 {
			return nil
		}
		if err := tx.Model(&models.ConditionalReleaseScoringRange{}).
			Where("rule_id = ?", id).
			Update("workflow_state", "deleted").Error; err != nil {
			return err
		}
		var setIDs []uint
		if err := tx.Model(&models.ConditionalReleaseAssignmentSet{}).
			Where("scoring_range_id IN ?", rangeIDs).
			Pluck("id", &setIDs).Error; err != nil {
			return err
		}
		if len(setIDs) == 0 {
			return nil
		}
		if err := tx.Model(&models.ConditionalReleaseAssignmentSet{}).
			Where("scoring_range_id IN ?", rangeIDs).
			Update("workflow_state", "deleted").Error; err != nil {
			return err
		}
		return tx.Model(&models.ConditionalReleaseAssignmentSetAssociation{}).
			Where("set_id IN ?", setIDs).
			Update("workflow_state", "deleted").Error
	})
}

// UpsertActions records an action row for each set/student pair, idempotently.
// Uses ON CONFLICT DO NOTHING on (set_id, student_id, action_type).
func (r *MasteryPathRepository) UpsertActions(ctx context.Context, actions []models.ConditionalReleaseAssignmentSetAction) error {
	if len(actions) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&actions).Error
}

// ListAssignedSetIDsForStudent returns the IDs of every set that has been
// assigned to (and not subsequently unassigned from) the student in the course.
func (r *MasteryPathRepository) ListAssignedSetIDsForStudent(ctx context.Context, courseID, studentID uint) ([]uint, error) {
	// Pull every "assigned" action for the student on sets in this course.
	type row struct {
		SetID uint
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Table("conditional_release_assignment_set_actions AS act").
		Select("DISTINCT act.set_id AS set_id").
		Joins("JOIN conditional_release_assignment_sets AS s ON s.id = act.set_id").
		Joins("JOIN conditional_release_scoring_ranges AS r ON r.id = s.scoring_range_id").
		Joins("JOIN conditional_release_rules AS rl ON rl.id = r.rule_id").
		Where("rl.course_id = ? AND act.student_id = ? AND act.action_type = ?", courseID, studentID, "assigned").
		Where("NOT EXISTS (SELECT 1 FROM conditional_release_assignment_set_actions a2 WHERE a2.set_id = act.set_id AND a2.student_id = act.student_id AND a2.action_type = 'unassigned' AND a2.acted_at > act.acted_at)").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.SetID)
	}
	return ids, nil
}

// ListAssignmentIDsForSets returns the assignment IDs across the given sets.
func (r *MasteryPathRepository) ListAssignmentIDsForSets(ctx context.Context, setIDs []uint) ([]uint, error) {
	if len(setIDs) == 0 {
		return nil, nil
	}
	var ids []uint
	err := r.db.WithContext(ctx).
		Model(&models.ConditionalReleaseAssignmentSetAssociation{}).
		Where("set_id IN ? AND workflow_state = ?", setIDs, "active").
		Distinct().
		Pluck("assignment_id", &ids).Error
	return ids, err
}

// ListGatedAssignmentIDs returns every assignment a live rule of the
// course releases through Mastery Paths: assignments in an active
// association of an active set of an active range of an active rule.
// Such assignments are hidden from a student until a set containing
// them is assigned to that student.
func (r *MasteryPathRepository) ListGatedAssignmentIDs(ctx context.Context, courseID uint) ([]uint, error) {
	var ids []uint
	err := r.db.WithContext(ctx).
		Table("conditional_release_assignment_set_associations AS a").
		Joins("JOIN conditional_release_assignment_sets AS s ON s.id = a.set_id AND s.workflow_state = 'active'").
		Joins("JOIN conditional_release_scoring_ranges AS sr ON sr.id = s.scoring_range_id AND sr.workflow_state = 'active'").
		Joins("JOIN conditional_release_rules AS rl ON rl.id = sr.rule_id AND rl.workflow_state = 'active'").
		Where("rl.course_id = ? AND a.workflow_state = 'active'", courseID).
		Distinct().
		Pluck("a.assignment_id", &ids).Error
	return ids, err
}

// ReplaceStudentAssignment makes chosenSetIDs the student's released
// sets for the rule: 'assigned' rows on the rule's other sets are
// removed (a regrade into another band moves the student to that band's
// assignments, as Canvas does), and the chosen sets are assigned.
func (r *MasteryPathRepository) ReplaceStudentAssignment(ctx context.Context, ruleID, studentID uint, chosenSetIDs []uint, actedAt time.Time, source string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ruleSets := tx.Session(&gorm.Session{NewDB: true}).Table("conditional_release_assignment_sets AS s").
			Select("s.id").
			Joins("JOIN conditional_release_scoring_ranges AS sr ON sr.id = s.scoring_range_id").
			Where("sr.rule_id = ?", ruleID)
		del := tx.Where("student_id = ? AND set_id IN (?)", studentID, ruleSets)
		if len(chosenSetIDs) > 0 {
			del = del.Where("set_id NOT IN ?", chosenSetIDs)
		}
		if err := del.Delete(&models.ConditionalReleaseAssignmentSetAction{}).Error; err != nil {
			return err
		}
		if len(chosenSetIDs) == 0 {
			return nil
		}
		// A stale 'unassigned' row would mask the new assignment.
		if err := tx.Where("student_id = ? AND set_id IN ? AND action_type = ?", studentID, chosenSetIDs, "unassigned").
			Delete(&models.ConditionalReleaseAssignmentSetAction{}).Error; err != nil {
			return err
		}
		actions := make([]models.ConditionalReleaseAssignmentSetAction, 0, len(chosenSetIDs))
		for _, id := range chosenSetIDs {
			actions = append(actions, models.ConditionalReleaseAssignmentSetAction{
				SetID: id, StudentID: studentID, ActionType: "assigned", ActedAt: actedAt, Source: source,
			})
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&actions).Error
	})
}
