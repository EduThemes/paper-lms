package models

import (
	"time"

	"gorm.io/gorm"
)

// ModuleProgressionState is the evaluated state of one module for one
// student (Canvas ContextModuleProgression#workflow_state).
type ModuleProgressionState string

const (
	// ModuleProgressionLocked: the module's unlock_at is in the future
	// or a prerequisite module is not completed yet.
	ModuleProgressionLocked ModuleProgressionState = "locked"
	// ModuleProgressionUnlocked: open, no requirement met yet.
	ModuleProgressionUnlocked ModuleProgressionState = "unlocked"
	// ModuleProgressionStarted: open, at least one requirement met.
	ModuleProgressionStarted ModuleProgressionState = "started"
	// ModuleProgressionCompleted: every requirement met (or any one when
	// the module's requirement_count is 1, or the module has none).
	ModuleProgressionCompleted ModuleProgressionState = "completed"
)

// ContextModuleProgression is a student's evaluated progress through one
// module (migration 000092). It is a cache of the evaluation, rewritten
// each time the student's progress is computed; the facts it is computed
// from live in submissions, quiz submissions, discussion entries and
// ModuleItemUserAction.
type ContextModuleProgression struct {
	ID              uint                   `json:"id" gorm:"column:id;primaryKey"`
	ContextModuleID uint                   `json:"context_module_id" gorm:"not null;uniqueIndex:idx_module_progressions_module_user,priority:1"`
	UserID          uint                   `json:"user_id" gorm:"not null;uniqueIndex:idx_module_progressions_module_user,priority:2;index:idx_module_progressions_user_id"`
	WorkflowState   ModuleProgressionState `json:"workflow_state" gorm:"type:text;not null;default:'locked'"`
	// RequirementsMet is a JSON array of the module item ids whose
	// completion requirement the student has met.
	RequirementsMet string     `json:"requirements_met" gorm:"type:text;not null;default:'[]'"`
	CurrentPosition *int       `json:"current_position"`
	CompletedAt     *time.Time `json:"completed_at"`
	EvaluatedAt     *time.Time `json:"evaluated_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (ContextModuleProgression) TableName() string { return "context_module_progressions" }

// BeforeCreate mirrors the SQL defaults: GORM writes an empty string for
// zero-value strings, which the workflow_state CHECK and the JSON column
// reject.
func (p *ContextModuleProgression) BeforeCreate(_ *gorm.DB) error {
	if p.WorkflowState == "" {
		p.WorkflowState = ModuleProgressionLocked
	}
	if p.RequirementsMet == "" {
		p.RequirementsMet = "[]"
	}
	return nil
}

// ModuleItemUserAction records what a student did with a module item
// that no other table captures: viewing it (must_view) and marking it
// done (must_mark_done). One row per (item, user).
type ModuleItemUserAction struct {
	ID           uint       `json:"id" gorm:"column:id;primaryKey"`
	ContentTagID uint       `json:"content_tag_id" gorm:"not null;uniqueIndex:idx_module_item_user_actions_tag_user,priority:1"`
	UserID       uint       `json:"user_id" gorm:"not null;uniqueIndex:idx_module_item_user_actions_tag_user,priority:2;index:idx_module_item_user_actions_user_id"`
	ViewedAt     *time.Time `json:"viewed_at"`
	MarkedDoneAt *time.Time `json:"marked_done_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (ModuleItemUserAction) TableName() string { return "module_item_user_actions" }
