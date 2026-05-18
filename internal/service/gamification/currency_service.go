package gamification

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// CurrencyCodeRE pins the user-defined-currency code shape: lowercase, must
// start with a letter, then [a-z0-9_], 2–32 chars total. Matches the
// predicate engine's resolution scheme (rules reference currencies by code).
var CurrencyCodeRE = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}$`)

// ColorRE accepts a 6-digit hex color or the empty string.
var ColorRE = regexp.MustCompile(`^(#[0-9A-Fa-f]{6})?$`)

// ErrCurrencyNotFound is returned when a lookup misses.
var ErrCurrencyNotFound = errors.New("currency not found")

// ErrCurrencyOutOfScope is returned when a write targets a row outside the
// route-derived (tenant, scope_type, scope_id).
var ErrCurrencyOutOfScope = errors.New("currency is not in the requested scope")

// ErrSystemCurrencyImmutable is returned when a delete targets system_owned.
var ErrSystemCurrencyImmutable = errors.New("system currencies cannot be deleted")

// ErrInvalidCurrencyCode is returned when the code fails regex validation.
var ErrInvalidCurrencyCode = errors.New("code must match ^[a-z][a-z0-9_]{1,31}$ (lowercase, starts with a letter, 2–32 chars)")

// ErrInvalidColor is returned when the color fails hex validation.
var ErrInvalidColor = errors.New("color must be a 6-digit hex like #A855F7, or empty")

// ErrInvalidLabel is returned when display_label is empty or too long.
var ErrInvalidLabel = errors.New("display_label is required, max 64 chars")

// ErrInvalidDescription is returned when description exceeds the cap.
var ErrInvalidDescription = errors.New("description max 500 chars")

// CurrencyService orchestrates currency-type CRUD + validation +
// scope/system-owned guards on top of the GamificationCurrencyTypeRepository.
type CurrencyService struct {
	repo repository.GamificationCurrencyTypeRepository
}

// NewCurrencyService wires the service.
func NewCurrencyService(repo repository.GamificationCurrencyTypeRepository) *CurrencyService {
	return &CurrencyService{repo: repo}
}

// CurrencyCreateInput is the parsed/sanitized create payload.
type CurrencyCreateInput struct {
	Code               string
	DisplayLabel       string
	DisplayLabelPlural string
	Icon               string
	Color              string
	DisplayOrder       int
	Spendable          bool
	Monotonic          bool
	VisibleToStudent   bool
	VisibleInTopbar    bool
	Description        string
}

// CurrencyPatchInput is the parsed PATCH payload. Pointers distinguish
// "field omitted" from "field set to zero value".
type CurrencyPatchInput struct {
	DisplayLabel       *string
	DisplayLabelPlural *string
	Icon               *string
	Color              *string
	DisplayOrder       *int
	Spendable          *bool
	Monotonic          *bool
	VisibleToStudent   *bool
	VisibleInTopbar    *bool
	Description        *string
}

// ValidateCurrencyCommonFields enforces display_label / color / description
// invariants shared by Create and Patch.
func ValidateCurrencyCommonFields(label, color, description string) error {
	if l := strings.TrimSpace(label); l == "" || len(l) > 64 {
		return ErrInvalidLabel
	}
	if !ColorRE.MatchString(color) {
		return ErrInvalidColor
	}
	if len(description) > 500 {
		return ErrInvalidDescription
	}
	return nil
}

// List returns all currencies for a tenant. Use topbarOnly=true for the
// topbar-pill view.
func (s *CurrencyService) List(ctx context.Context, tenantID uint, topbarOnly bool) ([]models.GamificationCurrencyType, error) {
	if topbarOnly {
		return s.repo.ListInTopbar(ctx, tenantID)
	}
	return s.repo.ListByTenant(ctx, tenantID)
}

// FindByID returns the row or ErrCurrencyNotFound.
func (s *CurrencyService) FindByID(ctx context.Context, id uint) (*models.GamificationCurrencyType, error) {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, ErrCurrencyNotFound
	}
	return row, nil
}

// Create validates + persists a new tenant currency. system_owned is forced
// to false. Returns repository.ErrCurrencyDuplicate on (tenant, scope, code)
// collisions (atomic via ON CONFLICT DO NOTHING).
func (s *CurrencyService) Create(ctx context.Context, tenantID uint, scopeType models.GamificationScopeType, scopeID uint, in CurrencyCreateInput) (*models.GamificationCurrencyType, error) {
	in.Code = strings.TrimSpace(in.Code)
	if !CurrencyCodeRE.MatchString(in.Code) {
		return nil, ErrInvalidCurrencyCode
	}
	if err := ValidateCurrencyCommonFields(in.DisplayLabel, in.Color, in.Description); err != nil {
		return nil, err
	}
	row := &models.GamificationCurrencyType{
		TenantID:            tenantID,
		ScopeType:           scopeType,
		ScopeID:             scopeID,
		Code:                in.Code,
		DisplayLabel:        strings.TrimSpace(in.DisplayLabel),
		DisplayLabelPlural:  strings.TrimSpace(in.DisplayLabelPlural),
		Icon:                strings.TrimSpace(in.Icon),
		Color:               strings.TrimSpace(in.Color),
		DisplayOrder:        in.DisplayOrder,
		Spendable:           in.Spendable,
		Monotonic:           in.Monotonic,
		FerpaClassification: "non_PII",
		VisibleToStudent:    in.VisibleToStudent,
		VisibleInTopbar:     in.VisibleInTopbar,
		SystemOwned:         false,
		Description:         strings.TrimSpace(in.Description),
	}
	if err := s.repo.Create(ctx, row); err != nil {
		return nil, err
	}
	return row, nil
}

// Update applies the patch after asserting scope ownership.
func (s *CurrencyService) Update(ctx context.Context, id, tenantID uint, scopeType models.GamificationScopeType, scopeID uint, in CurrencyPatchInput) (*models.GamificationCurrencyType, error) {
	row, err := s.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if row.TenantID != tenantID || row.ScopeType != scopeType || row.ScopeID != scopeID {
		return nil, ErrCurrencyOutOfScope
	}

	if in.DisplayLabel != nil {
		row.DisplayLabel = strings.TrimSpace(*in.DisplayLabel)
	}
	if in.DisplayLabelPlural != nil {
		row.DisplayLabelPlural = strings.TrimSpace(*in.DisplayLabelPlural)
	}
	if in.Icon != nil {
		row.Icon = strings.TrimSpace(*in.Icon)
	}
	if in.Color != nil {
		row.Color = strings.TrimSpace(*in.Color)
	}
	if in.DisplayOrder != nil {
		row.DisplayOrder = *in.DisplayOrder
	}
	if in.Spendable != nil {
		row.Spendable = *in.Spendable
	}
	if in.Monotonic != nil {
		row.Monotonic = *in.Monotonic
	}
	if in.VisibleToStudent != nil {
		row.VisibleToStudent = *in.VisibleToStudent
	}
	if in.VisibleInTopbar != nil {
		row.VisibleInTopbar = *in.VisibleInTopbar
	}
	if in.Description != nil {
		row.Description = strings.TrimSpace(*in.Description)
	}

	if err := ValidateCurrencyCommonFields(row.DisplayLabel, row.Color, row.Description); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, row); err != nil {
		return nil, err
	}
	return row, nil
}

// Delete removes a currency after scope + system_owned guards.
func (s *CurrencyService) Delete(ctx context.Context, id, tenantID uint, scopeType models.GamificationScopeType, scopeID uint) error {
	row, err := s.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if row.TenantID != tenantID || row.ScopeType != scopeType || row.ScopeID != scopeID {
		return ErrCurrencyOutOfScope
	}
	if row.SystemOwned {
		return ErrSystemCurrencyImmutable
	}
	return s.repo.Delete(ctx, row.ID)
}
