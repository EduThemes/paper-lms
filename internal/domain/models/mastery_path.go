package models

import "time"

// ConditionalReleaseRule defines a Mastery Paths rule attached to a single
// "trigger" assignment. When that assignment is graded, the student's score
// is checked against the rule's scoring ranges and one assignment set is
// chosen to be released to the student.
//
// Mirrors Canvas `conditional_release_rules` table (see
// canvas-lms-master/app/models/conditional_release/rule.rb).
type ConditionalReleaseRule struct {
	ID                  uint                             `json:"id" gorm:"column:id;primaryKey"`
	CourseID            uint                             `json:"course_id" gorm:"not null;index"`
	TriggerAssignmentID uint                             `json:"trigger_assignment_id" gorm:"not null;uniqueIndex:idx_cr_rule_trigger"`
	WorkflowState       string                           `json:"workflow_state" gorm:"not null;default:'active';index"`
	CreatedAt           time.Time                        `json:"created_at"`
	UpdatedAt           time.Time                        `json:"updated_at"`
	ScoringRanges       []ConditionalReleaseScoringRange `json:"scoring_ranges,omitempty" gorm:"foreignKey:RuleID"`
}

// ConditionalReleaseScoringRange defines one band of scores (e.g. 70-89%).
// Bounds are stored as fractional 0.0-1.0 to match Canvas semantics
// (Canvas stores them as decimal fractions). LowerBound is inclusive,
// UpperBound is exclusive (with a special case for 1.0/100%).
type ConditionalReleaseScoringRange struct {
	ID             uint                              `json:"id" gorm:"column:id;primaryKey"`
	RuleID         uint                              `json:"rule_id" gorm:"not null;index"`
	LowerBound     float64                           `json:"lower_bound" gorm:"not null;default:0"`
	UpperBound     float64                           `json:"upper_bound" gorm:"not null;default:1"`
	Position       int                               `json:"position" gorm:"not null;default:0"`
	WorkflowState  string                            `json:"workflow_state" gorm:"not null;default:'active'"`
	CreatedAt      time.Time                         `json:"created_at"`
	UpdatedAt      time.Time                         `json:"updated_at"`
	AssignmentSets []ConditionalReleaseAssignmentSet `json:"assignment_sets,omitempty" gorm:"foreignKey:ScoringRangeID"`
}

// ConditionalReleaseAssignmentSet groups a list of assignments to release
// together. A scoring range may contain multiple sets (e.g. a "choose one"
// pathway), but the typical case is a single set per range.
type ConditionalReleaseAssignmentSet struct {
	ID             uint                                         `json:"id" gorm:"column:id;primaryKey"`
	ScoringRangeID uint                                         `json:"scoring_range_id" gorm:"not null;index"`
	Position       int                                          `json:"position" gorm:"not null;default:0"`
	WorkflowState  string                                       `json:"workflow_state" gorm:"not null;default:'active'"`
	CreatedAt      time.Time                                    `json:"created_at"`
	UpdatedAt      time.Time                                    `json:"updated_at"`
	Associations   []ConditionalReleaseAssignmentSetAssociation `json:"assignment_set_associations,omitempty" gorm:"foreignKey:SetID"`
}

// ConditionalReleaseAssignmentSetAssociation links an assignment set to a
// concrete assignment that should be released when this set is chosen.
type ConditionalReleaseAssignmentSetAssociation struct {
	ID            uint      `json:"id" gorm:"column:id;primaryKey"`
	SetID         uint      `json:"assignment_set_id" gorm:"not null;index"`
	AssignmentID  uint      `json:"assignment_id" gorm:"not null;index"`
	Position      int       `json:"position" gorm:"not null;default:0"`
	WorkflowState string    `json:"workflow_state" gorm:"not null;default:'active'"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ConditionalReleaseAssignmentSetAction records when a set was assigned (or
// later unassigned) to a specific student. Idempotency is enforced via the
// (set_id, student_id, action_type) unique index.
type ConditionalReleaseAssignmentSetAction struct {
	ID         uint      `json:"id" gorm:"column:id;primaryKey"`
	SetID      uint      `json:"assignment_set_id" gorm:"not null;index;uniqueIndex:idx_cr_action_unique"`
	StudentID  uint      `json:"student_id" gorm:"not null;index;uniqueIndex:idx_cr_action_unique"`
	ActionType string    `json:"action_type" gorm:"not null;default:'assigned';uniqueIndex:idx_cr_action_unique"` // "assigned" | "unassigned"
	ActedAt    time.Time `json:"acted_at" gorm:"not null"`
	Source     string    `json:"source"` // free-form e.g. "auto:submission:<id>"
	CreatedAt  time.Time `json:"created_at"`
}

func (ConditionalReleaseRule) TableName() string {
	return "conditional_release_rules"
}
func (ConditionalReleaseScoringRange) TableName() string {
	return "conditional_release_scoring_ranges"
}
func (ConditionalReleaseAssignmentSet) TableName() string {
	return "conditional_release_assignment_sets"
}
func (ConditionalReleaseAssignmentSetAssociation) TableName() string {
	return "conditional_release_assignment_set_associations"
}
func (ConditionalReleaseAssignmentSetAction) TableName() string {
	return "conditional_release_assignment_set_actions"
}

// ContainsScore reports whether a fractional 0..1 score falls into this range.
// Matches Canvas semantics: lower-bound inclusive, upper-bound exclusive,
// except a perfect 1.0 belongs in any range whose upper_bound == 1.0.
func (r *ConditionalReleaseScoringRange) ContainsScore(score float64) bool {
	if score < r.LowerBound {
		return false
	}
	if score >= r.UpperBound {
		// Allow perfect score (1.0) to land in the top band.
		if !(score >= 1.0 && r.UpperBound >= 1.0) {
			return false
		}
	}
	return true
}
