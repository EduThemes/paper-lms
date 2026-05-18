package gamification

import (
	"context"
	"errors"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// ErrBadgeNotFound is returned when a badge lookup misses.
var ErrBadgeNotFound = errors.New("badge not found")

// ErrBadgeOutOfScope is returned when a write targets a badge outside the
// route-derived (tenant, scope_type, scope_id).
var ErrBadgeOutOfScope = errors.New("badge is not in the requested scope")

// ErrSystemBadgeImmutable is returned when a delete targets system_owned.
var ErrSystemBadgeImmutable = errors.New("system badges cannot be deleted")

// ErrBadgeWrongTenant is returned when an admin tries to award a badge from a
// different tenant.
var ErrBadgeWrongTenant = errors.New("badge is not in your tenant")

// ErrInvalidBadgeName is returned when the badge name fails validation.
var ErrInvalidBadgeName = errors.New("name is required, max 80 chars")

// BadgeService orchestrates badge CRUD + per-user awards.
type BadgeService struct {
	badgeRepo repository.GamificationBadgeRepository
	awardRepo repository.GamificationBadgeAwardRepository
}

// NewBadgeService wires the service.
func NewBadgeService(badgeRepo repository.GamificationBadgeRepository, awardRepo repository.GamificationBadgeAwardRepository) *BadgeService {
	return &BadgeService{badgeRepo: badgeRepo, awardRepo: awardRepo}
}

// BadgeCreateInput is the parsed POST payload.
type BadgeCreateInput struct {
	Code          string
	Name          string
	Description   string
	Icon          string
	ImageURL      string
	Color         string
	InternalOnly  bool
	AudienceLevel string
}

// BadgePatchInput is the parsed PATCH payload.
type BadgePatchInput struct {
	Name          *string
	Description   *string
	Icon          *string
	ImageURL      *string
	Color         *string
	InternalOnly  *bool
	AudienceLevel *string
}

// ParseAudienceLevel normalizes user input to a typed GamificationAudience.
// Empty / whitespace-only / unknown -> nil ("no audience set").
func ParseAudienceLevel(raw string) *models.GamificationAudience {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	a := models.GamificationAudience(trimmed)
	switch a {
	case models.AudienceK5,
		models.AudienceM68,
		models.AudienceH912,
		models.AudienceHigherEd,
		models.AudienceCorp,
		models.AudiencePro:
		return &a
	}
	return nil
}

// List returns all badges for a tenant.
func (s *BadgeService) List(ctx context.Context, tenantID uint) ([]models.GamificationBadge, error) {
	return s.badgeRepo.ListByTenant(ctx, tenantID)
}

// FindByID returns the row or ErrBadgeNotFound.
func (s *BadgeService) FindByID(ctx context.Context, id uint) (*models.GamificationBadge, error) {
	row, err := s.badgeRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, ErrBadgeNotFound
	}
	return row, nil
}

// Create validates + persists a new badge. system_owned is forced to false.
func (s *BadgeService) Create(ctx context.Context, tenantID uint, scopeType models.GamificationScopeType, scopeID uint, creatorID uint, in BadgeCreateInput) (*models.GamificationBadge, error) {
	in.Code = strings.TrimSpace(in.Code)
	if !CurrencyCodeRE.MatchString(in.Code) {
		return nil, ErrInvalidCurrencyCode
	}
	if l := strings.TrimSpace(in.Name); l == "" || len(l) > 80 {
		return nil, ErrInvalidBadgeName
	}
	if !ColorRE.MatchString(in.Color) {
		return nil, ErrInvalidColor
	}
	if len(in.Description) > 500 {
		return nil, ErrInvalidDescription
	}

	var createdBy *uint
	if creatorID > 0 {
		c := creatorID
		createdBy = &c
	}
	row := &models.GamificationBadge{
		TenantID:      tenantID,
		ScopeType:     scopeType,
		ScopeID:       scopeID,
		Code:          in.Code,
		Name:          strings.TrimSpace(in.Name),
		Description:   strings.TrimSpace(in.Description),
		Icon:          strings.TrimSpace(in.Icon),
		ImageURL:      strings.TrimSpace(in.ImageURL),
		Color:         strings.TrimSpace(in.Color),
		InternalOnly:  in.InternalOnly,
		SystemOwned:   false,
		AudienceLevel: ParseAudienceLevel(in.AudienceLevel),
		CreatedBy:     createdBy,
	}
	if err := s.badgeRepo.Create(ctx, row); err != nil {
		return nil, err
	}
	return row, nil
}

// Update applies the patch after asserting scope ownership.
func (s *BadgeService) Update(ctx context.Context, id, tenantID uint, scopeType models.GamificationScopeType, scopeID uint, in BadgePatchInput) (*models.GamificationBadge, error) {
	row, err := s.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if row.TenantID != tenantID || row.ScopeType != scopeType || row.ScopeID != scopeID {
		return nil, ErrBadgeOutOfScope
	}

	if in.Name != nil {
		row.Name = strings.TrimSpace(*in.Name)
	}
	if in.Description != nil {
		row.Description = strings.TrimSpace(*in.Description)
	}
	if in.Icon != nil {
		row.Icon = strings.TrimSpace(*in.Icon)
	}
	if in.ImageURL != nil {
		row.ImageURL = strings.TrimSpace(*in.ImageURL)
	}
	if in.Color != nil {
		row.Color = strings.TrimSpace(*in.Color)
	}
	if in.InternalOnly != nil {
		row.InternalOnly = *in.InternalOnly
	}
	if in.AudienceLevel != nil {
		row.AudienceLevel = ParseAudienceLevel(*in.AudienceLevel)
	}
	if row.Name == "" || len(row.Name) > 80 {
		return nil, ErrInvalidBadgeName
	}
	if !ColorRE.MatchString(row.Color) {
		return nil, ErrInvalidColor
	}
	if len(row.Description) > 500 {
		return nil, ErrInvalidDescription
	}
	if err := s.badgeRepo.Update(ctx, row); err != nil {
		return nil, err
	}
	return row, nil
}

// Delete removes a badge after scope + system_owned guards.
func (s *BadgeService) Delete(ctx context.Context, id, tenantID uint, scopeType models.GamificationScopeType, scopeID uint) error {
	row, err := s.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if row.TenantID != tenantID || row.ScopeType != scopeType || row.ScopeID != scopeID {
		return ErrBadgeOutOfScope
	}
	if row.SystemOwned {
		return ErrSystemBadgeImmutable
	}
	return s.badgeRepo.Delete(ctx, row.ID)
}

// ListAwardsForUser returns earned-badge awards for the user.
func (s *BadgeService) ListAwardsForUser(ctx context.Context, userID uint) ([]models.GamificationBadgeAward, error) {
	return s.awardRepo.ListForUser(ctx, userID)
}

// AwardOutcome is the result of an Award call.
type AwardOutcome struct {
	Award   *models.GamificationBadgeAward
	Badge   *models.GamificationBadge
	Created bool
}

// AwardToUser issues a badge to a user. Returns ErrBadgeNotFound if the badge
// id doesn't resolve, ErrBadgeWrongTenant if it isn't in the caller's tenant.
func (s *BadgeService) AwardToUser(ctx context.Context, targetUserID, badgeID, awarderID, callerTenantID uint) (*AwardOutcome, error) {
	badge, err := s.FindByID(ctx, badgeID)
	if err != nil {
		return nil, err
	}
	if badge.TenantID != callerTenantID {
		return nil, ErrBadgeWrongTenant
	}
	award := &models.GamificationBadgeAward{
		UserID:    targetUserID,
		BadgeID:   badge.ID,
		AwardedBy: &awarderID,
	}
	created, err := s.awardRepo.Award(ctx, award)
	if err != nil {
		return nil, err
	}
	return &AwardOutcome{Award: award, Badge: badge, Created: created}, nil
}

// Revoke removes a (user, badge) award. Idempotent.
func (s *BadgeService) Revoke(ctx context.Context, userID, badgeID uint) error {
	return s.awardRepo.Revoke(ctx, userID, badgeID)
}

// LookupBadgesByIDs fetches badges in bulk by ID, returning a map keyed by
// badge.ID. Missing IDs simply aren't in the map (used to surface flattened
// award rows where the badge has been deleted).
func (s *BadgeService) LookupBadgesByIDs(ctx context.Context, ids []uint) map[uint]*models.GamificationBadge {
	out := make(map[uint]*models.GamificationBadge, len(ids))
	for _, id := range ids {
		row, err := s.badgeRepo.FindByID(ctx, id)
		if err == nil && row != nil {
			out[id] = row
		}
	}
	return out
}
