package gamification

// vocabulary.go — the canonical xAPI verb + object_type constants used by
// every internal emit call-site, **plus** the structured catalog the
// recipe-builder write API (Sprint W2-E.1) validates incoming rule
// payloads against and the vocabulary discovery endpoint serializes for
// the recipe-builder UI (W2-E.2/E.3).
//
// Rules reference verbs and object types directly in their
// `trigger_event.verb` / `trigger_event.object_type` JSONB fields, so one
// drift between an emit call-site and a rule = silent rule miss. Drift
// between the catalog and the runtime decoder is guarded by
// vocabulary_test.go: every catalog entry must round-trip a synthesised
// minimum-valid JSON instance through `predicates.DecodePredicate` /
// `effects.DecodeEffects`.

// Verb values mirror the xAPI predicate-form vocabulary. Lower-case,
// past-tense, no spaces. Match the SYNTHESIS.md trigger inventory.
const (
	VerbSubmitted  = "submitted"
	VerbGraded     = "graded"
	VerbCompleted  = "completed"
	VerbViewed     = "viewed"
	VerbEnrolled   = "enrolled"
	VerbMastered   = "mastered"
	VerbProgressed = "progressed"
	VerbPosted     = "posted"
	VerbAssessed   = "assessed"

	// VerbEarned is emitted by the AwardBadge effect on first-time badge
	// award (W2-E.1). Lets rule authors chain reactions: "WHEN you earn
	// this badge THEN AwardCurrency(xp, 100)". Dedup'd second awards do
	// not re-emit (the badge-award INSERT … ON CONFLICT DO NOTHING
	// returns `created=false` and the effect skips the chain emit).
	VerbEarned = "earned"
)

// Object type values are the canonical Go model type names (singular,
// PascalCase) so that the rules engine's `object_type` matches what a
// human authoring a rule would call the entity.
const (
	ObjectAssignment      = "Assignment"
	ObjectSubmission      = "Submission"
	ObjectQuiz            = "Quiz"
	ObjectPage            = "Page"
	ObjectModuleItem      = "ModuleItem"
	ObjectModule          = "Module"
	ObjectCourse          = "Course"
	ObjectOutcome         = "Outcome"
	ObjectDiscussionEntry = "DiscussionEntry"
	ObjectRubric          = "Rubric"

	// ObjectBadge is paired with VerbEarned for the badge-award chain
	// trigger introduced in W2-E.1.
	ObjectBadge = "Badge"
)

// EmitterSource is the canonical source string for events emitted by
// internal Paper LMS services (vs. external "lti", "webhook", or
// "migration_import" sources from the GamificationEvent.Source enum).
const EmitterSource = "internal"

// ----------------------------------------------------------------------
// Catalog: the declarative schema the recipe-builder write API and
// vocabulary discovery endpoint share. Keep aligned with the runtime
// decoders in service/gamification/{predicates,effects}/factory.go —
// vocabulary_test.go fails CI on drift.
// ----------------------------------------------------------------------

// ParamType enumerates the JSON shapes the recipe-builder UI knows how to
// render an inline editor for. Add new types here only when the UI gains
// a matching editor; otherwise reuse an existing one.
type ParamType string

const (
	ParamTypeInt    ParamType = "int"
	ParamTypeFloat  ParamType = "float"
	ParamTypeBool   ParamType = "bool"
	ParamTypeString ParamType = "string"
	// ParamTypeEnum carries a closed enumeration; the UI renders a select.
	ParamTypeEnum ParamType = "enum"
	// ParamTypeRef points at an entity the UI looks up via a picker.
	// Ref is one of: "assignment" | "quiz" | "content" | "outcome" |
	// "currency_code" | "badge" | "badge_code".
	ParamTypeRef ParamType = "ref"
)

// ParamSpec describes one parameter on a trigger / predicate / effect.
// Optional bounds are pointer-typed so the JSON serialization omits them
// when unset (the frontend treats absent as "no constraint").
type ParamSpec struct {
	Name        string    `json:"name"`
	Type        ParamType `json:"type"`
	Required    bool      `json:"required,omitempty"`
	Description string    `json:"description,omitempty"`

	// Enum is populated when Type == ParamTypeEnum.
	Enum []string `json:"enum,omitempty"`

	// Ref is populated when Type == ParamTypeRef. Names the entity kind
	// the UI picker should resolve against.
	Ref string `json:"ref,omitempty"`

	// Min / Max numeric bounds. Pointer-typed so a Min of zero is
	// distinguishable from "no minimum."
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
}

// KindSpec is one entry in the catalog: a discriminator string plus the
// list of parameters the runtime decoder expects on that kind.
type KindSpec struct {
	Kind        string      `json:"kind"`
	Description string      `json:"description,omitempty"`
	Params      []ParamSpec `json:"params"`
}

func intMin(v float64) *float64 { return &v }

// VerbCatalog is the ordered list of verb strings the OnEvent trigger
// validator accepts. Keep in sync with the Verb* consts above.
var VerbCatalog = []string{
	VerbSubmitted,
	VerbGraded,
	VerbCompleted,
	VerbViewed,
	VerbEnrolled,
	VerbMastered,
	VerbProgressed,
	VerbPosted,
	VerbAssessed,
	VerbEarned,
}

// ObjectCatalog is the ordered list of object_type strings the OnEvent
// trigger validator accepts. Keep in sync with the Object* consts above.
var ObjectCatalog = []string{
	ObjectAssignment,
	ObjectSubmission,
	ObjectQuiz,
	ObjectPage,
	ObjectModuleItem,
	ObjectModule,
	ObjectCourse,
	ObjectOutcome,
	ObjectDiscussionEntry,
	ObjectRubric,
	ObjectBadge,
}

// AudienceLevels mirrors the GamificationAudience enum. Used to validate
// the recipe builder's "this rule applies to" picker.
var AudienceLevels = []string{
	"k5", "m68", "h912", "higher_ed", "corp", "pro",
}

// ScopeTypes mirrors the gamification_scope_type Postgres enum. Used by
// the recipe builder's header chip; the validator infers scope from
// route, not from the body.
var ScopeTypes = []string{
	"site", "district", "school", "course", "section",
}

// SetOps lists the ConditionSet operator strings the predicate decoder
// accepts. N_OF_M additionally requires a positive threshold.
var SetOps = []string{"AND", "OR", "N_OF_M"}

// WindowKinds enumerates valid `max_per_window.window` strings.
var WindowKinds = []string{"day", "week", "lifetime"}

// MasteryLevels mirrors the four Khan-style mastery buckets used by
// OutcomeMastery predicates. Order is ordinal (novice < … < mastered).
var MasteryLevels = []string{"novice", "familiar", "proficient", "mastered"}

// TriggerCatalog is the set of trigger discriminator strings the rule
// index recognises (see rule_index.go). UI renders one inline editor per
// kind.
var TriggerCatalog = []KindSpec{
	{
		Kind:        "OnEvent",
		Description: "Fires when an actor performs the (verb, object_type) pair.",
		Params: []ParamSpec{
			{Name: "verb", Type: ParamTypeEnum, Required: true, Enum: VerbCatalog},
			{Name: "object_type", Type: ParamTypeEnum, Required: true, Enum: ObjectCatalog},
		},
	},
	{
		Kind:        "OnSchedule",
		Description: "Fires on a cron schedule. Cron worker lands in a later sprint; rules can be authored and stored now.",
		Params: []ParamSpec{
			{Name: "cron", Type: ParamTypeString, Required: true, Description: "Standard 5-field cron expression."},
		},
	},
	{
		Kind:        "OnManualTrigger",
		Description: "Fires when an admin or instructor manually invokes the named handle.",
		Params: []ParamSpec{
			{Name: "handle", Type: ParamTypeString, Required: true},
		},
	},
}

// PredicateCatalog is the set of atomic predicate kinds the runtime
// decoder accepts (predicates/factory.go). ConditionSet is the recursive
// AND/OR/N_OF_M wrapper — it's described separately on the response so
// the UI can wrap arbitrary subtrees.
var PredicateCatalog = []KindSpec{
	{
		Kind:        "SubmittedAssignment",
		Description: "True if the actor has a submission for the given assignment, optionally bounded by score.",
		Params: []ParamSpec{
			{Name: "assignment_id", Type: ParamTypeRef, Ref: "assignment", Required: true},
			{Name: "min_score", Type: ParamTypeFloat},
			{Name: "max_score", Type: ParamTypeFloat},
			{Name: "require_on_time", Type: ParamTypeBool},
		},
	},
	{
		Kind:        "SubmittedQuiz",
		Description: "True if the actor has a quiz submission for the given quiz, optionally bounded by score.",
		Params: []ParamSpec{
			{Name: "quiz_id", Type: ParamTypeRef, Ref: "quiz", Required: true},
			{Name: "min_score", Type: ParamTypeFloat},
			{Name: "max_score", Type: ParamTypeFloat},
		},
	},
	{
		Kind:        "ViewedContent",
		Description: "True if the actor has viewed the given content at least min_views times (default 1) with at least min_seconds_viewed cumulative seconds.",
		Params: []ParamSpec{
			{Name: "content_id", Type: ParamTypeRef, Ref: "content", Required: true},
			{Name: "min_views", Type: ParamTypeInt, Min: intMin(0)},
			{Name: "min_seconds_viewed", Type: ParamTypeInt, Min: intMin(0)},
		},
	},
	{
		Kind:        "OutcomeMastery",
		Description: "True if the actor has reached the given mastery level on the outcome (calc_method override optional).",
		Params: []ParamSpec{
			{Name: "outcome_id", Type: ParamTypeRef, Ref: "outcome", Required: true},
			{Name: "min_level", Type: ParamTypeEnum, Required: true, Enum: MasteryLevels},
			{Name: "calc_method", Type: ParamTypeString},
		},
	},
	{
		Kind:        "CurrencyThreshold",
		Description: "True if the actor's balance for the named currency is at least min_amount.",
		Params: []ParamSpec{
			{Name: "code", Type: ParamTypeRef, Ref: "currency_code", Required: true},
			{Name: "min_amount", Type: ParamTypeInt, Required: true, Min: intMin(0)},
		},
	},
	{
		Kind:        "EarnedBadge",
		Description: "True if the actor has been awarded the named badge.",
		Params: []ParamSpec{
			{Name: "badge_id", Type: ParamTypeRef, Ref: "badge", Required: true},
		},
	},
	{
		Kind:        "ReputationThreshold",
		Description: "True if the actor's reputation (the system 'reputation' currency) is at least min_amount.",
		Params: []ParamSpec{
			{Name: "min_amount", Type: ParamTypeInt, Required: true, Min: intMin(0)},
		},
	},
}

// EffectCatalog is the set of effect kinds the runtime decoder accepts
// (effects/factory.go). Effects run in the order they appear in the
// rule's `effects` array; the UI's drag-to-reorder palette writes the
// final order back to JSON before POST.
var EffectCatalog = []KindSpec{
	{
		Kind:        "AwardCurrency",
		Description: "Ledger a positive delta to the actor's wallet for the named currency. Optional multiplier scales Amount; values ≤ 0 are treated as 1.0.",
		Params: []ParamSpec{
			{Name: "code", Type: ParamTypeRef, Ref: "currency_code", Required: true},
			{Name: "amount", Type: ParamTypeInt, Required: true, Min: intMin(1)},
			{Name: "multiplier", Type: ParamTypeFloat},
		},
	},
	{
		Kind:        "AwardBadge",
		Description: "Issue the named badge to the actor. Idempotent: a second fire for the same (user, badge) is deduplicated and does not re-emit badge.earned.",
		Params: []ParamSpec{
			{Name: "code", Type: ParamTypeRef, Ref: "badge_code", Required: true},
			{Name: "evidence", Type: ParamTypeString, Description: "Free-form audit note surfaced in the rule_evaluation detail."},
		},
	},
}
