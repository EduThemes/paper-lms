package gamification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/datatypes"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service/gamification/effects"
	"github.com/EduThemes/paper-lms/internal/service/gamification/predicates"
)

// ErrRuleNotFound is returned when a rule lookup misses.
var ErrRuleNotFound = errors.New("rule not found")

// ErrRuleOutOfScope is the defense-in-depth scope guard.
var ErrRuleOutOfScope = errors.New("rule is not in the requested scope")

// ErrInvalidRuleName is returned when name is empty or too long.
var ErrInvalidRuleName = errors.New("name is required, max 200 chars")

// ErrInvalidRuleDescription is returned when description exceeds 2000 chars.
var ErrInvalidRuleDescription = errors.New("description max 2000 chars")

// RuleService orchestrates recipe-builder (W2-E.1) rule CRUD + vocabulary
// validation around the GamificationRuleRepository.
type RuleService struct {
	repo repository.GamificationRuleRepository
}

// NewRuleService wires the service.
func NewRuleService(repo repository.GamificationRuleRepository) *RuleService {
	return &RuleService{repo: repo}
}

// RuleCreateInput is the parsed POST body.
type RuleCreateInput struct {
	Name            string
	Description     string
	AudienceLevel   string
	Enabled         bool
	TriggerEvent    json.RawMessage
	ConditionSet    json.RawMessage
	Effects         json.RawMessage
	CooldownSeconds *int
	MaxPerWindow    json.RawMessage
}

// RulePatchInput is the parsed PATCH body. Pointers / RawMessage emptiness
// distinguish "omitted" from "explicit zero".
type RulePatchInput struct {
	Name              *string
	Description       *string
	Enabled           *bool
	TriggerEvent      json.RawMessage
	ConditionSet      json.RawMessage
	Effects           json.RawMessage
	CooldownSeconds   *int
	MaxPerWindow      json.RawMessage
	ClearCooldown     bool
	ClearMaxPerWindow bool
}

// ValidateAudience confirms the audience_level is one of the canonical set.
func ValidateAudience(level string) error {
	for _, ok := range AudienceLevels {
		if level == ok {
			return nil
		}
	}
	return fmt.Errorf("audience_level must be one of %v", AudienceLevels)
}

// enumContains is a small utility to check membership in a string slice.
func enumContains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

// ValidateTriggerEvent decodes and validates the trigger_event JSON blob.
func ValidateTriggerEvent(raw json.RawMessage) error {
	if len(raw) == 0 {
		return errors.New("trigger_event is required")
	}
	var head struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return fmt.Errorf("trigger_event: %w", err)
	}
	switch head.Kind {
	case "OnEvent":
		var p struct {
			Verb       string `json:"verb"`
			ObjectType string `json:"object_type"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return fmt.Errorf("trigger_event OnEvent: %w", err)
		}
		if !enumContains(VerbCatalog, p.Verb) {
			return fmt.Errorf("trigger_event OnEvent.verb must be one of %v (got %q)", VerbCatalog, p.Verb)
		}
		if !enumContains(ObjectCatalog, p.ObjectType) {
			return fmt.Errorf("trigger_event OnEvent.object_type must be one of %v (got %q)", ObjectCatalog, p.ObjectType)
		}
	case "OnSchedule":
		var p struct {
			Cron string `json:"cron"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return fmt.Errorf("trigger_event OnSchedule: %w", err)
		}
		if strings.TrimSpace(p.Cron) == "" {
			return errors.New("trigger_event OnSchedule.cron must be non-empty")
		}
	case "OnManualTrigger":
		var p struct {
			Handle string `json:"handle"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return fmt.Errorf("trigger_event OnManualTrigger: %w", err)
		}
		if strings.TrimSpace(p.Handle) == "" {
			return errors.New("trigger_event OnManualTrigger.handle must be non-empty")
		}
	case "":
		return errors.New("trigger_event missing required field \"kind\"")
	default:
		return fmt.Errorf("unknown trigger_event kind: %q", head.Kind)
	}
	return nil
}

// ValidateMaxPerWindow validates the optional rate-limit shape.
func ValidateMaxPerWindow(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var p struct {
		Window string `json:"window"`
		Count  int    `json:"count"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("max_per_window: %w", err)
	}
	if !enumContains(WindowKinds, p.Window) {
		return fmt.Errorf("max_per_window.window must be one of %v (got %q)", WindowKinds, p.Window)
	}
	if p.Count <= 0 {
		return fmt.Errorf("max_per_window.count must be > 0 (got %d)", p.Count)
	}
	return nil
}

// ValidateRuleBody runs the full validator on the merged Create / Patch
// payload. Invoked AFTER the Patch's pointer-merge so a partial update that
// yields an invalid rule is rejected before persistence.
func ValidateRuleBody(audience string, trigger, condition, effectsRaw, maxPerWindow json.RawMessage, cooldown *int) error {
	if err := ValidateAudience(audience); err != nil {
		return err
	}
	if err := ValidateTriggerEvent(trigger); err != nil {
		return err
	}
	if len(condition) == 0 {
		return errors.New("condition_set is required")
	}
	if _, err := predicates.DecodePredicate(condition); err != nil {
		return fmt.Errorf("condition_set: %w", err)
	}
	if len(effectsRaw) == 0 {
		return errors.New("effects is required (use [] for no-effect rules)")
	}
	if _, err := effects.DecodeEffects(effectsRaw); err != nil {
		return fmt.Errorf("effects: %w", err)
	}
	if cooldown != nil && *cooldown <= 0 {
		return fmt.Errorf("cooldown_seconds must be > 0 when set (got %d)", *cooldown)
	}
	if err := ValidateMaxPerWindow(maxPerWindow); err != nil {
		return err
	}
	return nil
}

// List returns rules at the exact (tenant, scope_type, scope_id).
func (s *RuleService) List(ctx context.Context, tenantID uint, scopeType models.GamificationScopeType, scopeID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GamificationRule], error) {
	return s.repo.ListByScope(ctx, tenantID, scopeType, scopeID, params)
}

// Get returns the rule or ErrRuleNotFound / ErrRuleOutOfScope.
func (s *RuleService) Get(ctx context.Context, id, tenantID uint, scopeType models.GamificationScopeType, scopeID uint) (*models.GamificationRule, error) {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, ErrRuleNotFound
	}
	if !ruleMatchesScope(row, tenantID, scopeType, scopeID) {
		return nil, ErrRuleOutOfScope
	}
	return row, nil
}

// Create validates and persists a new rule. Returns the persisted row.
func (s *RuleService) Create(ctx context.Context, tenantID uint, scopeType models.GamificationScopeType, scopeID uint, creatorID uint, in RuleCreateInput) (*models.GamificationRule, error) {
	if strings.TrimSpace(in.Name) == "" || len(in.Name) > 200 {
		return nil, ErrInvalidRuleName
	}
	if len(in.Description) > 2000 {
		return nil, ErrInvalidRuleDescription
	}
	if err := ValidateRuleBody(in.AudienceLevel, in.TriggerEvent, in.ConditionSet, in.Effects, in.MaxPerWindow, in.CooldownSeconds); err != nil {
		return nil, err
	}

	row := &models.GamificationRule{
		TenantID:        tenantID,
		ScopeType:       scopeType,
		ScopeID:         scopeID,
		AudienceLevel:   models.GamificationAudience(in.AudienceLevel),
		Name:            strings.TrimSpace(in.Name),
		Description:     strings.TrimSpace(in.Description),
		Enabled:         in.Enabled,
		TriggerEvent:    datatypes.JSON(in.TriggerEvent),
		ConditionSet:    datatypes.JSON(in.ConditionSet),
		Effects:         datatypes.JSON(in.Effects),
		CooldownSeconds: in.CooldownSeconds,
		MaxPerWindow:    datatypes.JSON(in.MaxPerWindow),
	}
	if creatorID > 0 {
		c := creatorID
		row.CreatedBy = &c
	}
	if err := s.repo.Create(ctx, row); err != nil {
		return nil, err
	}
	return row, nil
}

// Patch applies the patch, scope-asserts, validates the merged result, persists.
func (s *RuleService) Patch(ctx context.Context, id, tenantID uint, scopeType models.GamificationScopeType, scopeID uint, in RulePatchInput) (*models.GamificationRule, error) {
	row, err := s.Get(ctx, id, tenantID, scopeType, scopeID)
	if err != nil {
		return nil, err
	}

	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if n == "" || len(n) > 200 {
			return nil, ErrInvalidRuleName
		}
		row.Name = n
	}
	if in.Description != nil {
		if len(*in.Description) > 2000 {
			return nil, ErrInvalidRuleDescription
		}
		row.Description = strings.TrimSpace(*in.Description)
	}
	if in.Enabled != nil {
		row.Enabled = *in.Enabled
	}
	if len(in.TriggerEvent) > 0 {
		row.TriggerEvent = datatypes.JSON(in.TriggerEvent)
	}
	if len(in.ConditionSet) > 0 {
		row.ConditionSet = datatypes.JSON(in.ConditionSet)
	}
	if len(in.Effects) > 0 {
		row.Effects = datatypes.JSON(in.Effects)
	}
	if in.ClearCooldown {
		row.CooldownSeconds = nil
	} else if in.CooldownSeconds != nil {
		row.CooldownSeconds = in.CooldownSeconds
	}
	if in.ClearMaxPerWindow {
		row.MaxPerWindow = nil
	} else if len(in.MaxPerWindow) > 0 {
		row.MaxPerWindow = datatypes.JSON(in.MaxPerWindow)
	}

	if err := ValidateRuleBody(
		string(row.AudienceLevel),
		json.RawMessage(row.TriggerEvent),
		json.RawMessage(row.ConditionSet),
		json.RawMessage(row.Effects),
		json.RawMessage(row.MaxPerWindow),
		row.CooldownSeconds,
	); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, row); err != nil {
		return nil, err
	}
	return row, nil
}

// Delete removes a rule after scope + tenant guards.
func (s *RuleService) Delete(ctx context.Context, id, tenantID uint, scopeType models.GamificationScopeType, scopeID uint) error {
	row, err := s.Get(ctx, id, tenantID, scopeType, scopeID)
	if err != nil {
		return err
	}
	return s.repo.Delete(ctx, row.ID)
}

func ruleMatchesScope(row *models.GamificationRule, tenantID uint, scopeType models.GamificationScopeType, scopeID uint) bool {
	return row.TenantID == tenantID && row.ScopeType == scopeType && row.ScopeID == scopeID
}
