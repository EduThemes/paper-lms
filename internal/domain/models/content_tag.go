package models

import "time"

type ContentTag struct {
	ID              uint               `json:"id" gorm:"column:id;primaryKey"`
	ContextModuleID uint               `json:"context_module_id" gorm:"not null;index"`
	ContentType     string             `json:"content_type" gorm:"not null"` // Assignment, WikiPage, ExternalUrl, ContextModuleSubHeader
	ContentID       *uint              `json:"content_id"`
	Title           string             `json:"title" gorm:"not null"`
	Position        int                `json:"position"`
	URL             string             `json:"url" gorm:"column:url"`
	Indent          int                `json:"indent" gorm:"default:0"`
	NewTab          bool               `json:"new_tab" gorm:"default:false"`
	WorkflowState   ContentTagWorkflow `json:"workflow_state" gorm:"type:text;not null;default:'active'"`
	// Completion requirement (Canvas completion_requirements). Empty =
	// none; otherwise one of the ModuleCompletion* values. MinScore is
	// the threshold for ModuleCompletionMinScore.
	CompletionType string    `json:"completion_type" gorm:"column:completion_type;type:text;not null;default:''"`
	MinScore       *float64  `json:"min_score" gorm:"column:min_score"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
}

func (ContentTag) TableName() string {
	return "content_tags"
}

// Module item completion requirement types (Canvas parity).
const (
	ModuleCompletionMustView       = "must_view"
	ModuleCompletionMustSubmit     = "must_submit"
	ModuleCompletionMustContribute = "must_contribute"
	ModuleCompletionMinScore       = "min_score"
	ModuleCompletionMustMarkDone   = "must_mark_done"
)

// ValidCompletionTypesFor returns the requirement types Canvas allows on
// a module item of the given content_type ("" — no requirement — is
// always allowed and not listed).
func ValidCompletionTypesFor(contentType string) []string {
	switch contentType {
	case "Assignment", "Quizzes::Quiz", "Quiz":
		return []string{ModuleCompletionMustView, ModuleCompletionMustSubmit, ModuleCompletionMinScore, ModuleCompletionMustMarkDone}
	case "DiscussionTopic":
		return []string{ModuleCompletionMustView, ModuleCompletionMustContribute, ModuleCompletionMustSubmit, ModuleCompletionMinScore, ModuleCompletionMustMarkDone}
	case "ContextModuleSubHeader":
		return nil
	default: // WikiPage, ExternalUrl, Attachment, ContextExternalTool, …
		return []string{ModuleCompletionMustView, ModuleCompletionMustMarkDone}
	}
}
