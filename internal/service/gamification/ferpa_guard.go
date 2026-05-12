package gamification

// FERPA guard for gamification.Emit (Wave 1, Sprint C).
//
// Every gamification event field can be tagged in the
// gamification_ferpa_field_tags lookup with one of:
//
//	directory_information | education_record | non_PII | instructor_metadata
//
// The data-access layer downstream of the event store trusts the
// event's PolicyFlags to decide what's safe to surface on a public
// leaderboard vs. what stays gated behind an instructor view. That
// trust only holds if the flags are set correctly at ingest — hence
// this pre-flight check.
//
// Wave 1 only gate-checks education_record. The other classifications
// are advisory: the guard records no violation for them. When a
// downstream surface needs the directory/instructor distinction
// enforced, extend this guard rather than scattering the rule across
// callers.

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// FerpaViolation describes a single mis-classified field detected on
// an event. The guard returns a slice — one Emit can carry multiple
// violations, and the user-facing error includes all of them for
// faster diagnosis.
type FerpaViolation struct {
	ObjectType     string
	FieldPath      string   // dot-path like "result.score" or "context.course_id"
	Classification string   // what the tag says the field should be
	Missing        []string // which policy_flags are missing
}

// requiredEducationRecordFlags is the policy_flags set an event must
// carry when any of its fields is tagged education_record. Both must
// be present; either missing produces a violation.
var requiredEducationRecordFlags = []string{"ferpa_protected", "education_record"}

// CheckFerpa scans the event's Result and Context JSONB against the
// FERPA field-tag lookup for the event's ObjectType. Returns the
// slice of violations; empty means the event is policy-compliant.
//
// Wave 1 simplification: only top-level JSON keys are checked (no
// recursion into nested objects). FieldPath values in tags are
// expressed as "result.<key>" / "context.<key>" — a literal
// "<bucket>.<key>" pair. Nested-object support lands when a real
// rule needs it.
//
// Returns an error (not a violation) only when the event itself is
// structurally broken — malformed Result or Context JSON. A broken
// event isn't a policy question; it's a programmer bug at the call
// site that should surface before any policy check matters.
func CheckFerpa(ctx context.Context, repo repository.GamificationFerpaFieldTagRepository, event *models.GamificationEvent) ([]FerpaViolation, error) {
	tags, err := repo.ListByObjectType(ctx, event.ObjectType)
	if err != nil {
		return nil, err
	}
	if len(tags) == 0 {
		// No policy declared for this object_type → no policy to
		// violate. Wave 1 trust posture: absence of tag = no
		// enforcement, on the theory that tags ship alongside the
		// rules that need them.
		return nil, nil
	}

	resultMap, err := decodeJSONBucket(event.Result)
	if err != nil {
		return nil, err
	}
	contextMap, err := decodeJSONBucket(event.Context)
	if err != nil {
		return nil, err
	}

	flagSet := make(map[string]struct{}, len(event.PolicyFlags))
	for _, f := range event.PolicyFlags {
		flagSet[f] = struct{}{}
	}

	var violations []FerpaViolation
	for _, tag := range tags {
		if tag.Classification != "education_record" {
			// directory_information / instructor_metadata / non_PII
			// are advisory in Wave 1. Skip without recording.
			continue
		}

		bucket, key, ok := splitFieldPath(tag.FieldPath)
		if !ok {
			// Unknown bucket prefix — silently ignore. Field paths
			// outside result/context aren't checkable in Wave 1.
			continue
		}

		var present bool
		switch bucket {
		case "result":
			_, present = resultMap[key]
		case "context":
			_, present = contextMap[key]
		default:
			continue
		}
		if !present {
			// The field isn't on the event, so nothing to leak.
			continue
		}

		missing := make([]string, 0, len(requiredEducationRecordFlags))
		for _, required := range requiredEducationRecordFlags {
			if _, ok := flagSet[required]; !ok {
				missing = append(missing, required)
			}
		}
		if len(missing) == 0 {
			continue
		}
		violations = append(violations, FerpaViolation{
			ObjectType:     tag.ObjectType,
			FieldPath:      tag.FieldPath,
			Classification: tag.Classification,
			Missing:        missing,
		})
	}

	return violations, nil
}

// decodeJSONBucket parses a JSONB payload as a top-level object. nil
// or empty input becomes an empty map; anything that doesn't parse
// returns an error.
func decodeJSONBucket(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// splitFieldPath breaks a dotted path like "result.score" into its
// bucket and key. Returns ok=false when the path doesn't have exactly
// two segments — Wave 1 doesn't traverse deeper.
func splitFieldPath(path string) (bucket, key string, ok bool) {
	idx := strings.IndexByte(path, '.')
	if idx <= 0 || idx == len(path)-1 {
		return "", "", false
	}
	bucket = path[:idx]
	key = path[idx+1:]
	if strings.ContainsRune(key, '.') {
		// Nested path — not supported in Wave 1.
		return "", "", false
	}
	return bucket, key, true
}
