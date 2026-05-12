package gamification

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"gorm.io/datatypes"
)

// mustTrigger marshals an arbitrary value into a datatypes.JSON payload
// suitable for GamificationRule.TriggerEvent. Test-only; panics on error
// so the test source stays compact (json.Marshal on a literal map cannot
// reasonably fail at runtime).
func mustTrigger(t *testing.T, v any) datatypes.JSON {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal trigger: %v", err)
	}
	return datatypes.JSON(raw)
}

// rawTrigger builds a TriggerEvent from a literal byte string, used to
// exercise the malformed-JSON path.
func rawTrigger(s string) datatypes.JSON {
	return datatypes.JSON([]byte(s))
}

// makeRule is a fixture helper that fills in the boring fields and
// stamps the caller-supplied id + trigger.
func makeRule(id uint, trigger datatypes.JSON) models.GamificationRule {
	return models.GamificationRule{
		ID:            id,
		TenantID:      1,
		ScopeType:     models.ScopeSite,
		ScopeID:       1,
		AudienceLevel: models.AudienceK5,
		Name:          "test-rule",
		Enabled:       true,
		TriggerEvent:  trigger,
	}
}

func TestBuildRuleIndex_Empty(t *testing.T) {
	idx := BuildRuleIndex(nil)
	if idx == nil {
		t.Fatal("BuildRuleIndex(nil) returned nil index")
	}
	if got := idx.LookupOnEvent("completed", "Quiz"); len(got) != 0 {
		t.Errorf("LookupOnEvent on empty index: want 0, got %d", len(got))
	}
	if got := idx.ListOnSchedule(); len(got) != 0 {
		t.Errorf("ListOnSchedule on empty index: want 0, got %d", len(got))
	}
	if got := idx.LookupOnManual("award_xp"); len(got) != 0 {
		t.Errorf("LookupOnManual on empty index: want 0, got %d", len(got))
	}
	if got := idx.Skipped(); len(got) != 0 {
		t.Errorf("Skipped on empty index: want 0, got %d", len(got))
	}
}

func TestBuildRuleIndex_SingleOnEvent(t *testing.T) {
	rule := makeRule(42, mustTrigger(t, map[string]string{
		"kind":        "OnEvent",
		"verb":        "completed",
		"object_type": "Quiz",
	}))
	idx := BuildRuleIndex([]models.GamificationRule{rule})

	got := idx.LookupOnEvent("completed", "Quiz")
	if len(got) != 1 {
		t.Fatalf("want 1 rule, got %d", len(got))
	}
	if got[0].ID != 42 {
		t.Errorf("want rule ID 42, got %d", got[0].ID)
	}
	if len(idx.Skipped()) != 0 {
		t.Errorf("want 0 skipped, got %v", idx.Skipped())
	}
}

func TestBuildRuleIndex_MultipleOnEvent_SameKey_PreservesInputOrder(t *testing.T) {
	r1 := makeRule(1, mustTrigger(t, map[string]string{
		"kind": "OnEvent", "verb": "completed", "object_type": "Quiz",
	}))
	r2 := makeRule(2, mustTrigger(t, map[string]string{
		"kind": "OnEvent", "verb": "completed", "object_type": "Quiz",
	}))
	idx := BuildRuleIndex([]models.GamificationRule{r1, r2})

	got := idx.LookupOnEvent("completed", "Quiz")
	if len(got) != 2 {
		t.Fatalf("want 2 rules, got %d", len(got))
	}
	if got[0].ID != 1 || got[1].ID != 2 {
		t.Errorf("input order not preserved: got IDs %d,%d", got[0].ID, got[1].ID)
	}
}

func TestBuildRuleIndex_OnEvent_EmptyVerb_Skipped(t *testing.T) {
	rule := makeRule(7, mustTrigger(t, map[string]string{
		"kind":        "OnEvent",
		"verb":        "",
		"object_type": "Quiz",
	}))
	idx := BuildRuleIndex([]models.GamificationRule{rule})

	if got := idx.LookupOnEvent("", "Quiz"); len(got) != 0 {
		t.Errorf("empty-verb rule should not be indexed; got %d hits", len(got))
	}
	skipped := idx.Skipped()
	if len(skipped) != 1 {
		t.Fatalf("want 1 skipped, got %d", len(skipped))
	}
	if skipped[0].RuleID != 7 {
		t.Errorf("want skipped rule ID 7, got %d", skipped[0].RuleID)
	}
	if !strings.Contains(strings.ToLower(skipped[0].Reason), "verb") {
		t.Errorf("skip reason should mention 'verb': %q", skipped[0].Reason)
	}
}

func TestBuildRuleIndex_OnEvent_EmptyObjectType_Skipped(t *testing.T) {
	rule := makeRule(8, mustTrigger(t, map[string]string{
		"kind":        "OnEvent",
		"verb":        "completed",
		"object_type": "",
	}))
	idx := BuildRuleIndex([]models.GamificationRule{rule})

	skipped := idx.Skipped()
	if len(skipped) != 1 {
		t.Fatalf("want 1 skipped, got %d", len(skipped))
	}
	if !strings.Contains(skipped[0].Reason, "object_type") {
		t.Errorf("skip reason should mention 'object_type': %q", skipped[0].Reason)
	}
}

func TestBuildRuleIndex_OnSchedule(t *testing.T) {
	rule := makeRule(11, mustTrigger(t, map[string]string{
		"kind": "OnSchedule",
		"cron": "0 3 * * *",
	}))
	idx := BuildRuleIndex([]models.GamificationRule{rule})

	sched := idx.ListOnSchedule()
	if len(sched) != 1 {
		t.Fatalf("want 1 scheduled rule, got %d", len(sched))
	}
	if sched[0].ID != 11 {
		t.Errorf("want rule ID 11, got %d", sched[0].ID)
	}
	// Should NOT appear in any OnEvent bucket.
	if got := idx.LookupOnEvent("", ""); len(got) != 0 {
		t.Errorf("OnSchedule rule leaked into OnEvent bucket: %d hits", len(got))
	}
	if len(idx.Skipped()) != 0 {
		t.Errorf("want 0 skipped, got %v", idx.Skipped())
	}
}

func TestBuildRuleIndex_OnManualTrigger_MatchAndMiss(t *testing.T) {
	rule := makeRule(21, mustTrigger(t, map[string]string{
		"kind":   "OnManualTrigger",
		"handle": "award_xp",
	}))
	idx := BuildRuleIndex([]models.GamificationRule{rule})

	got := idx.LookupOnManual("award_xp")
	if len(got) != 1 {
		t.Fatalf("want 1 manual rule, got %d", len(got))
	}
	if got[0].ID != 21 {
		t.Errorf("want rule ID 21, got %d", got[0].ID)
	}
	if miss := idx.LookupOnManual("nonexistent_handle"); len(miss) != 0 {
		t.Errorf("mismatched handle should return empty; got %d", len(miss))
	}
}

func TestBuildRuleIndex_UnknownKind_Skipped(t *testing.T) {
	rule := makeRule(31, mustTrigger(t, map[string]string{
		"kind": "OnBananaPeel",
	}))
	idx := BuildRuleIndex([]models.GamificationRule{rule})

	skipped := idx.Skipped()
	if len(skipped) != 1 {
		t.Fatalf("want 1 skipped, got %d", len(skipped))
	}
	if skipped[0].RuleID != 31 {
		t.Errorf("want skipped rule ID 31, got %d", skipped[0].RuleID)
	}
	if !strings.Contains(strings.ToLower(skipped[0].Reason), "unknown") {
		t.Errorf("skip reason should mention 'unknown': %q", skipped[0].Reason)
	}
}

func TestBuildRuleIndex_MalformedJSON_Skipped_NoPanic(t *testing.T) {
	rule := makeRule(99, rawTrigger(`{"kind":"OnEvent",`)) // truncated
	idx := BuildRuleIndex([]models.GamificationRule{rule})

	skipped := idx.Skipped()
	if len(skipped) != 1 {
		t.Fatalf("want 1 skipped, got %d", len(skipped))
	}
	if skipped[0].RuleID != 99 {
		t.Errorf("want skipped rule ID 99, got %d", skipped[0].RuleID)
	}
	if !strings.Contains(strings.ToLower(skipped[0].Reason), "malformed") {
		t.Errorf("skip reason should mention 'malformed': %q", skipped[0].Reason)
	}
}

func TestBuildRuleIndex_DisabledRule_SilentlySkipped(t *testing.T) {
	rule := makeRule(50, mustTrigger(t, map[string]string{
		"kind": "OnEvent", "verb": "completed", "object_type": "Quiz",
	}))
	rule.Enabled = false
	idx := BuildRuleIndex([]models.GamificationRule{rule})

	if got := idx.LookupOnEvent("completed", "Quiz"); len(got) != 0 {
		t.Errorf("disabled rule should not be indexed; got %d hits", len(got))
	}
	if got := idx.Skipped(); len(got) != 0 {
		t.Errorf("disabled rule should be silently skipped (not in Skipped()); got %v", got)
	}
}

func TestBuildRuleIndex_MixedInput_EachInOwnBucket(t *testing.T) {
	rules := []models.GamificationRule{
		makeRule(1, mustTrigger(t, map[string]string{
			"kind": "OnEvent", "verb": "completed", "object_type": "Quiz",
		})),
		makeRule(2, mustTrigger(t, map[string]string{
			"kind": "OnEvent", "verb": "submitted", "object_type": "Assignment",
		})),
		makeRule(3, mustTrigger(t, map[string]string{
			"kind": "OnSchedule", "cron": "0 3 * * *",
		})),
		makeRule(4, mustTrigger(t, map[string]string{
			"kind": "OnManualTrigger", "handle": "award_xp",
		})),
		makeRule(5, rawTrigger(`{"not":"valid"`)), // malformed
	}
	idx := BuildRuleIndex(rules)

	if got := idx.LookupOnEvent("completed", "Quiz"); len(got) != 1 || got[0].ID != 1 {
		t.Errorf("OnEvent(completed,Quiz) bucket wrong: %+v", got)
	}
	if got := idx.LookupOnEvent("submitted", "Assignment"); len(got) != 1 || got[0].ID != 2 {
		t.Errorf("OnEvent(submitted,Assignment) bucket wrong: %+v", got)
	}
	if got := idx.ListOnSchedule(); len(got) != 1 || got[0].ID != 3 {
		t.Errorf("OnSchedule bucket wrong: %+v", got)
	}
	if got := idx.LookupOnManual("award_xp"); len(got) != 1 || got[0].ID != 4 {
		t.Errorf("OnManual(award_xp) bucket wrong: %+v", got)
	}
	if got := idx.Skipped(); len(got) != 1 || got[0].RuleID != 5 {
		t.Errorf("Skipped bucket wrong: %+v", got)
	}
	// Cross-bucket leakage check: the scheduled rule must not appear in
	// any OnEvent map slot via a zero-valued TriggerKey.
	if got := idx.LookupOnEvent("", ""); len(got) != 0 {
		t.Errorf("zero-key OnEvent lookup leaked rules: %+v", got)
	}
}

func TestRuleIndex_NilReceiver_Safe(t *testing.T) {
	// Defensive: a nil *RuleIndex should not panic when callers forget
	// to check the build result. All accessors return zero values.
	var idx *RuleIndex
	if got := idx.LookupOnEvent("a", "b"); got != nil {
		t.Errorf("nil receiver LookupOnEvent should return nil, got %v", got)
	}
	if got := idx.ListOnSchedule(); got != nil {
		t.Errorf("nil receiver ListOnSchedule should return nil, got %v", got)
	}
	if got := idx.LookupOnManual("x"); got != nil {
		t.Errorf("nil receiver LookupOnManual should return nil, got %v", got)
	}
	if got := idx.Skipped(); got != nil {
		t.Errorf("nil receiver Skipped should return nil, got %v", got)
	}
}
