package gamification

// vocabulary.go — the canonical xAPI verb + object_type constants used by
// every internal emit call-site. Rules reference these strings directly
// in their `trigger_event.verb` / `trigger_event.object_type` JSONB
// fields, so one drift between an emit call-site and a rule = silent
// rule miss.
//
// Wave 1 vocabulary is intentionally small (20 verbs, 7 object types).
// New verbs/objects land here first, then in the emit call-site, then
// in any rule-authoring documentation. Drift caught at compile time
// beats drift caught in production by a teacher whose rule mysteriously
// stops firing.

// Verb values mirror the xAPI predicate-form vocabulary. Lower-case,
// past-tense, no spaces. Match the SYNTHESIS.md trigger inventory.
const (
	// Submission / grading verbs.
	VerbSubmitted = "submitted"
	VerbGraded    = "graded"

	// Quiz / assessment verbs.
	VerbCompleted = "completed"

	// Engagement verbs.
	VerbViewed = "viewed"

	// Enrollment verbs.
	VerbEnrolled = "enrolled"

	// Mastery / outcome verbs.
	VerbMastered    = "mastered"
	VerbProgressed  = "progressed"

	// Discussion / contribution verbs.
	VerbPosted = "posted"

	// Rubric / peer-review verbs.
	VerbAssessed = "assessed"
)

// Object type values are the canonical Go model type names (singular,
// PascalCase) so that the rules engine's `object_type` matches what a
// human authoring a rule would call the entity.
const (
	ObjectAssignment = "Assignment"
	ObjectSubmission = "Submission"
	ObjectQuiz       = "Quiz"
	ObjectPage       = "Page"
	ObjectModuleItem = "ModuleItem"
	ObjectModule     = "Module"
	ObjectCourse     = "Course"
	ObjectOutcome    = "Outcome"

	// Discussion contribution objects.
	ObjectDiscussionEntry = "DiscussionEntry"

	// Rubric assessment objects.
	ObjectRubric = "Rubric"
)

// EmitterSource is the canonical source string for events emitted by
// internal Paper LMS services (vs. external "lti", "webhook", or
// "migration_import" sources from the GamificationEvent.Source enum).
const EmitterSource = "internal"
