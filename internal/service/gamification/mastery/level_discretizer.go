package mastery

// Level mirrors Khan-style mastery buckets. The string constants match the
// values used in the OutcomeMastery predicate's MinLevel parameter so a rule
// authored against "proficient" reads cleanly end-to-end.
const (
	LevelNovice     = "novice"
	LevelFamiliar   = "familiar"
	LevelProficient = "proficient"
	LevelMastered   = "mastered"
)

// LevelOrdinal returns the position of a level in the progression so callers
// can compare "is at least proficient" without case-splitting on strings.
// Unknown levels return -1.
func LevelOrdinal(level string) int {
	switch level {
	case LevelNovice:
		return 0
	case LevelFamiliar:
		return 1
	case LevelProficient:
		return 2
	case LevelMastered:
		return 3
	}
	return -1
}

// LevelFor discretizes a continuous mastery value in [0, 1] into one of the
// four Khan-style buckets. All six calc methods funnel through this helper so
// the bucketing stays consistent regardless of which method computed the
// value.
//
// Boundaries:
//
//	[0.00, 0.40) → novice
//	[0.40, 0.65) → familiar
//	[0.65, 0.85) → proficient
//	[0.85, 1.00] → mastered
//
// Values outside [0, 1] are clamped; callers should normalize before calling
// but the clamp keeps downstream rendering safe.
func LevelFor(value float64) string {
	switch {
	case value < 0.40:
		return LevelNovice
	case value < 0.65:
		return LevelFamiliar
	case value < 0.85:
		return LevelProficient
	default:
		return LevelMastered
	}
}
