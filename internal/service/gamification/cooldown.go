// Package gamification provides the dispatcher-side guards and orchestration
// for the Phase 6 rules engine. cooldown.go implements the cooldown_seconds
// and max_per_window gates a rule must pass before its effects run.
package gamification

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// Gate identifies which guard blocked a rule fire. Empty string when
// the rule was allowed through. The dispatcher reads this directly
// rather than parsing the Reason string, which lets the human-readable
// reason evolve without breaking machine-readable audit tags.
type Gate string

const (
	GateNone          Gate = ""
	GateCooldown      Gate = "cooldown"
	GateMaxPerWindow  Gate = "max_per_window"
)

// CooldownCheckResult is what the dispatcher consumes before deciding
// whether to run a rule's effects. When Allowed is false, Gate names
// the guard that blocked (cooldown vs. max_per_window) and Reason
// carries the human-readable explanation for the audit row.
type CooldownCheckResult struct {
	Allowed bool
	Gate    Gate
	Reason  string
}

// maxPerWindowConfig mirrors the JSONB shape stored on
// gamification_rules.max_per_window:
//
//	{"window":"day"|"week"|"lifetime","count":N}
type maxPerWindowConfig struct {
	Window string `json:"window"`
	Count  int    `json:"count"`
}

// CheckCooldown evaluates both cooldown_seconds and max_per_window gates
// against the rule_evaluations history. Returns Allowed=true with an
// empty Reason when neither gate blocks; either gate failure short-
// circuits with Allowed=false and a human-readable Reason. The two
// gates are evaluated independently — passing one does not bypass the
// other.
//
// `now` is passed in rather than read from time.Now() so tests can drive
// the clock and so the dispatcher can pin the check to the triggering
// event's emitted_at timestamp.
func CheckCooldown(
	ctx context.Context,
	repo repository.GamificationRuleRepository,
	rule *models.GamificationRule,
	userID uint,
	now time.Time,
) (CooldownCheckResult, error) {
	// Gate 1: cooldown_seconds.
	if rule.CooldownSeconds != nil && *rule.CooldownSeconds > 0 {
		last, err := repo.LastFiringForUserRule(ctx, userID, rule.ID)
		if err != nil {
			return CooldownCheckResult{}, fmt.Errorf("cooldown: load last firing: %w", err)
		}
		if last != nil {
			cooldown := time.Duration(*rule.CooldownSeconds) * time.Second
			elapsed := now.Sub(last.EvaluatedAt)
			if elapsed < cooldown {
				remaining := int((cooldown - elapsed) / time.Second)
				// Always report at least 1s remaining when blocked so the
				// audit message never reads "0 seconds remaining".
				if remaining < 1 {
					remaining = 1
				}
				return CooldownCheckResult{
					Allowed: false,
					Gate:    GateCooldown,
					Reason:  fmt.Sprintf("cooldown active (%d seconds remaining)", remaining),
				}, nil
			}
		}
	}

	// Gate 2: max_per_window.
	if len(rule.MaxPerWindow) > 0 {
		var cfg maxPerWindowConfig
		if err := json.Unmarshal(rule.MaxPerWindow, &cfg); err != nil {
			return CooldownCheckResult{}, fmt.Errorf("max_per_window: invalid JSON: %w", err)
		}

		var windowStart time.Time
		switch cfg.Window {
		case "day":
			windowStart = now.Add(-24 * time.Hour)
		case "week":
			windowStart = now.Add(-7 * 24 * time.Hour)
		case "lifetime":
			windowStart = time.Time{}
		default:
			return CooldownCheckResult{}, fmt.Errorf("max_per_window: unknown window %q (expected day|week|lifetime)", cfg.Window)
		}

		count, err := repo.CountFiringsInWindow(ctx, userID, rule.ID, windowStart)
		if err != nil {
			return CooldownCheckResult{}, fmt.Errorf("max_per_window: count firings: %w", err)
		}
		if count >= int64(cfg.Count) {
			return CooldownCheckResult{
				Allowed: false,
				Gate:    GateMaxPerWindow,
				Reason:  fmt.Sprintf("max_per_window reached (%d in %s)", cfg.Count, cfg.Window),
			}, nil
		}
	}

	return CooldownCheckResult{Allowed: true}, nil
}
