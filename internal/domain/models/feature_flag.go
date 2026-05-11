package models

import "time"

// Feature flag state values mirror Canvas's FeatureFlag::STATES.
//
//	allowed - off by default, but flag may be enabled by lower context
//	on      - enabled (and locked, lower contexts cannot override)
//	off     - disabled (and locked, lower contexts cannot override)
//	hidden  - not visible to non-site-admins (used for unreleased features)
const (
	FeatureStateAllowed = "allowed"
	FeatureStateOn      = "on"
	FeatureStateOff     = "off"
	FeatureStateHidden  = "hidden"
)

// Context types — must match Canvas's polymorphic context naming.
const (
	FeatureContextSiteAdmin = "SiteAdmin"
	FeatureContextAccount   = "Account"
	FeatureContextCourse    = "Course"
	FeatureContextUser      = "User"
)

// Release stages communicate maturity to the UI.
const (
	FeatureStageHidden   = "hidden"
	FeatureStageBeta     = "beta"
	FeatureStageReleased = "released"
)

// FeatureFlag is the persisted override of a feature for a particular context.
// Mirrors Canvas's `feature_flags` table layout.
type FeatureFlag struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Feature     string    `json:"feature" gorm:"not null;index;size:255"`
	State       string    `json:"state" gorm:"not null;default:'allowed';size:32"`
	ContextType string    `json:"context_type" gorm:"not null;index;size:32"`
	ContextID   uint      `json:"context_id" gorm:"not null;index"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName pins the table name (avoid GORM pluralization quirks).
func (FeatureFlag) TableName() string { return "feature_flags" }

// FeatureDefinition describes a single available feature. Definitions are
// hard-coded (rather than stored in DB) — same pattern Canvas uses with
// `Feature.register`.
type FeatureDefinition struct {
	Name         string `json:"feature"`
	DisplayName  string `json:"display_name"`
	Description  string `json:"description"`
	AppliesTo    string `json:"applies_to"`    // SiteAdmin | Account | Course | User
	DefaultState string `json:"default_state"` // allowed | on | off | hidden
	ReleaseStage string `json:"release_stage"` // hidden | beta | released
}

// FeatureDefinitions is the canonical registry of features Paper LMS knows
// about. Adding a new flag is as simple as appending here. Front-end and
// back-end both read from this list (via the API).
var FeatureDefinitions = map[string]FeatureDefinition{
	"new_quizzes": {
		Name:         "new_quizzes",
		DisplayName:  "New Quizzes",
		Description:  "Modern quiz engine with item banks, advanced question types, and richer analytics.",
		AppliesTo:    FeatureContextCourse,
		DefaultState: FeatureStateAllowed,
		ReleaseStage: FeatureStageBeta,
	},
	"mastery_paths": {
		Name:         "mastery_paths",
		DisplayName:  "Mastery Paths",
		Description:  "Conditional release of content based on student performance.",
		AppliesTo:    FeatureContextCourse,
		DefaultState: FeatureStateAllowed,
		ReleaseStage: FeatureStageReleased,
	},
	"k2_mode": {
		Name:         "k2_mode",
		DisplayName:  "K-2 Simplified UI",
		Description:  "Younger-learner UI with larger touch targets and simplified navigation.",
		AppliesTo:    FeatureContextCourse,
		DefaultState: FeatureStateAllowed,
		ReleaseStage: FeatureStageReleased,
	},
	"tiptap_rce": {
		Name:         "tiptap_rce",
		DisplayName:  "Tiptap Rich Content Editor",
		Description:  "Next-generation editor replacing the legacy RCE. Better paste handling and accessibility.",
		AppliesTo:    FeatureContextAccount,
		DefaultState: FeatureStateOn,
		ReleaseStage: FeatureStageReleased,
	},
	"appointment_groups": {
		Name:         "appointment_groups",
		DisplayName:  "Appointment Groups",
		Description:  "Allow teachers to create scheduled appointment slots students can sign up for.",
		AppliesTo:    FeatureContextAccount,
		DefaultState: FeatureStateAllowed,
		ReleaseStage: FeatureStageBeta,
	},
	"discussion_checkpoints": {
		Name:         "discussion_checkpoints",
		DisplayName:  "Discussion Checkpoints",
		Description:  "Split discussion grades into a 'reply to topic' and 'reply to peers' checkpoint.",
		AppliesTo:    FeatureContextCourse,
		DefaultState: FeatureStateAllowed,
		ReleaseStage: FeatureStageBeta,
	},
	"course_paces": {
		Name:         "course_paces",
		DisplayName:  "Course Pacing",
		Description:  "Per-student pacing plans that auto-adjust assignment due dates.",
		AppliesTo:    FeatureContextCourse,
		DefaultState: FeatureStateAllowed,
		ReleaseStage: FeatureStageReleased,
	},
	"blueprint_courses": {
		Name:         "blueprint_courses",
		DisplayName:  "Blueprint Courses",
		Description:  "Sync course content from a blueprint template to associated courses.",
		AppliesTo:    FeatureContextAccount,
		DefaultState: FeatureStateAllowed,
		ReleaseStage: FeatureStageReleased,
	},
	"speedgrader_v2": {
		Name:         "speedgrader_v2",
		DisplayName:  "SpeedGrader v2",
		Description:  "Redesigned grading workspace with keyboard-first navigation.",
		AppliesTo:    FeatureContextCourse,
		DefaultState: FeatureStateAllowed,
		ReleaseStage: FeatureStageHidden,
	},
	"high_contrast_ui": {
		Name:         "high_contrast_ui",
		DisplayName:  "High Contrast UI",
		Description:  "Per-user accessibility theme with WCAG AAA contrast ratios.",
		AppliesTo:    FeatureContextUser,
		DefaultState: FeatureStateAllowed,
		ReleaseStage: FeatureStageReleased,
	},
}

// LookupDefinition fetches a definition by name, ok=false if unknown.
func LookupDefinition(name string) (FeatureDefinition, bool) {
	def, ok := FeatureDefinitions[name]
	return def, ok
}
