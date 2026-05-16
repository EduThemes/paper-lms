package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
)

// SiteAdminContextID is the sentinel context_id we use to store the
// site-admin (root-of-the-world) flag. Real Account rows start at 1, so 0
// is reserved for SiteAdmin.
const SiteAdminContextID uint = 0

// EffectiveFlag is what the API/UI actually consumes: the resolved state of
// a feature for a context, plus all the metadata needed to render a settings
// row (locked? inherited from where? release stage?).
type EffectiveFlag struct {
	Feature      string `json:"feature"`
	State        string `json:"state"`
	DisplayName  string `json:"display_name"`
	Description  string `json:"description"`
	AppliesTo    string `json:"applies_to"`
	ReleaseStage string `json:"release_stage"`
	// ContextType/ContextID describe where the resolved state came from
	// (e.g. an account flag may govern a course flag). Empty if the resolved
	// state is the registry default.
	ParentContextType string `json:"parent_context_type,omitempty"`
	ParentContextID   uint   `json:"parent_context_id,omitempty"`
	// Locked = a higher context has set on/off, so this context cannot
	// override.
	Locked bool `json:"locked"`
	// Hidden = the feature is in `hidden` release stage and the caller
	// is not a site admin. The handler may filter these out.
	Hidden bool `json:"hidden"`
}

// FeatureFlagService resolves and updates feature flags with Canvas-style
// context inheritance: User has no chain; Course inherits from its Account;
// Account inherits from SiteAdmin.
type FeatureFlagService struct {
	flagRepo    postgres.FeatureFlagRepository
	courseRepo  repository.CourseRepository
	accountRepo repository.AccountRepository
	userRepo    repository.UserRepository
}

func NewFeatureFlagService(
	flagRepo postgres.FeatureFlagRepository,
	courseRepo repository.CourseRepository,
	accountRepo repository.AccountRepository,
	userRepo repository.UserRepository,
) *FeatureFlagService {
	return &FeatureFlagService{
		flagRepo:    flagRepo,
		courseRepo:  courseRepo,
		accountRepo: accountRepo,
		userRepo:    userRepo,
	}
}

// IsEnabled is the boolean fast-path used by feature-gated code.
func (s *FeatureFlagService) IsEnabled(ctx context.Context, feature, contextType string, contextID uint) bool {
	resolved := s.resolve(ctx, feature, contextType, contextID)
	return resolved.State == models.FeatureStateOn
}

// ListEffectiveFlags returns the resolved state of every feature that
// `applies_to` the given context (or any of its ancestors).
func (s *FeatureFlagService) ListEffectiveFlags(ctx context.Context, contextType string, contextID uint, isSiteAdmin bool) []EffectiveFlag {
	out := make([]EffectiveFlag, 0, len(models.FeatureDefinitions))
	for _, def := range models.FeatureDefinitions {
		if !appliesToContext(def, contextType) {
			continue
		}
		eff := s.resolve(ctx, def.Name, contextType, contextID)
		if eff.Hidden && !isSiteAdmin {
			continue
		}
		out = append(out, eff)
	}
	return out
}

// GetEffectiveFlag returns the resolved state for one feature.
func (s *FeatureFlagService) GetEffectiveFlag(ctx context.Context, feature, contextType string, contextID uint) (EffectiveFlag, error) {
	if _, ok := models.LookupDefinition(feature); !ok {
		return EffectiveFlag{}, fmt.Errorf("unknown feature: %s", feature)
	}
	return s.resolve(ctx, feature, contextType, contextID), nil
}

// SetState writes a flag override after RBAC checks. The caller must already
// have established that `currentUserIsAdmin` (account-level) or
// `currentUserIsTeacher` (for course flags). The service rejects writes that
// would override a higher-context lock.
func (s *FeatureFlagService) SetState(
	ctx context.Context,
	feature, contextType string,
	contextID uint,
	state string,
	currentUserIsAdmin bool,
	currentUserIsTeacher bool,
) error {
	def, ok := models.LookupDefinition(feature)
	if !ok {
		return fmt.Errorf("unknown feature: %s", feature)
	}
	if !validState(state) {
		return fmt.Errorf("invalid state: %s", state)
	}
	if !appliesToContext(def, contextType) {
		return fmt.Errorf("feature %q does not apply to %s context", feature, contextType)
	}

	// RBAC: only admins can edit account- or site-admin-level flags.
	// Teachers may toggle course-level flags.
	switch contextType {
	case models.FeatureContextSiteAdmin, models.FeatureContextAccount:
		if !currentUserIsAdmin {
			return errors.New("admin permission required")
		}
	case models.FeatureContextCourse:
		if !currentUserIsAdmin && !currentUserIsTeacher {
			return errors.New("teacher or admin permission required")
		}
	case models.FeatureContextUser:
		// Per-user flags: caller layer must enforce self-or-admin.
	default:
		return fmt.Errorf("invalid context type: %s", contextType)
	}

	// Honor parent locks: if a parent context is `on` or `off`, refuse.
	parent := s.resolveParent(ctx, feature, contextType, contextID)
	if parent != nil && (parent.State == models.FeatureStateOn || parent.State == models.FeatureStateOff) {
		return fmt.Errorf("feature is locked by %s context", parent.ContextType)
	}

	flag := &models.FeatureFlag{
		Feature:     feature,
		State:       state,
		ContextType: contextType,
		ContextID:   contextID,
	}
	return s.flagRepo.Upsert(ctx, flag)
}

// Reset deletes the flag at the given context, falling back to inherited.
func (s *FeatureFlagService) Reset(ctx context.Context, feature, contextType string, contextID uint) error {
	return s.flagRepo.DeleteByContext(ctx, contextType, contextID, feature)
}

// ----- internals ------------------------------------------------------------

// resolve walks the inheritance chain from most-specific to least-specific:
//
//	Course (specific) → Account → SiteAdmin (registry default)
//	Account → SiteAdmin
//	User → registry default (Users do not inherit from Accounts)
//
// On each step, an `on`/`off` flag at a higher level *locks* the value.
// `allowed` at a higher level means lower contexts may set their own state.
func (s *FeatureFlagService) resolve(ctx context.Context, feature, contextType string, contextID uint) EffectiveFlag {
	def, ok := models.LookupDefinition(feature)
	if !ok {
		return EffectiveFlag{Feature: feature, State: models.FeatureStateOff}
	}

	chain := s.contextChain(ctx, contextType, contextID)
	resolvedState := def.DefaultState
	var lockedBy *models.FeatureFlag
	var sourcedFrom *models.FeatureFlag

	// Walk most-general to most-specific so that lower contexts override higher
	// ones unless a higher context has locked the value.
	for _, c := range chain {
		flag, err := s.flagRepo.FindByContext(ctx, c.Type, c.ID, feature)
		if err != nil {
			continue
		}
		if lockedBy != nil {
			// Already locked by a higher context — child contexts cannot change.
			continue
		}
		switch flag.State {
		case models.FeatureStateOn, models.FeatureStateOff:
			// Hard set + lock for descendants (unless this IS the leaf).
			resolvedState = flag.State
			sourcedFrom = flag
			if c.Type != contextType || c.ID != contextID {
				lockedBy = flag
			}
		case models.FeatureStateAllowed:
			// Allowed means "no opinion, descendants may decide."
			sourcedFrom = flag
		case models.FeatureStateHidden:
			resolvedState = flag.State
			sourcedFrom = flag
		}
	}

	out := EffectiveFlag{
		Feature:      feature,
		State:        resolvedState,
		DisplayName:  def.DisplayName,
		Description:  def.Description,
		AppliesTo:    def.AppliesTo,
		ReleaseStage: def.ReleaseStage,
		Hidden:       def.ReleaseStage == models.FeatureStageHidden && resolvedState != models.FeatureStateOn,
	}
	if lockedBy != nil {
		out.Locked = true
		out.ParentContextType = lockedBy.ContextType
		out.ParentContextID = lockedBy.ContextID
	} else if sourcedFrom != nil && (sourcedFrom.ContextType != contextType || sourcedFrom.ContextID != contextID) {
		out.ParentContextType = sourcedFrom.ContextType
		out.ParentContextID = sourcedFrom.ContextID
	}
	return out
}

// resolveParent returns the most-specific *ancestor* flag (excluding the
// leaf), used for write-time lock enforcement.
func (s *FeatureFlagService) resolveParent(ctx context.Context, feature, contextType string, contextID uint) *models.FeatureFlag {
	chain := s.contextChain(ctx, contextType, contextID)
	// drop the last element (which is the leaf itself)
	if len(chain) <= 1 {
		return nil
	}
	for i := len(chain) - 2; i >= 0; i-- {
		c := chain[i]
		if flag, err := s.flagRepo.FindByContext(ctx, c.Type, c.ID, feature); err == nil {
			return flag
		}
	}
	return nil
}

type contextRef struct {
	Type string
	ID   uint
}

// contextChain returns ancestors-first, leaf-last.
func (s *FeatureFlagService) contextChain(ctx context.Context, contextType string, contextID uint) []contextRef {
	switch contextType {
	case models.FeatureContextUser:
		return []contextRef{{Type: contextType, ID: contextID}}
	case models.FeatureContextSiteAdmin:
		return []contextRef{{Type: models.FeatureContextSiteAdmin, ID: SiteAdminContextID}}
	case models.FeatureContextAccount:
		return []contextRef{
			{Type: models.FeatureContextSiteAdmin, ID: SiteAdminContextID},
			{Type: models.FeatureContextAccount, ID: contextID},
		}
	case models.FeatureContextCourse:
		// Look up the course to find its account.
		chain := []contextRef{{Type: models.FeatureContextSiteAdmin, ID: SiteAdminContextID}}
		if course, err := s.courseRepo.FindByID(ctx, contextID, 0); err == nil {
			chain = append(chain, contextRef{Type: models.FeatureContextAccount, ID: course.AccountID})
		}
		chain = append(chain, contextRef{Type: models.FeatureContextCourse, ID: contextID})
		return chain
	}
	return nil
}

func appliesToContext(def models.FeatureDefinition, contextType string) bool {
	// A feature defined for an Account also applies to its Courses (so a
	// teacher sees account-level flags as locked rows). User flags only show
	// on the User page.
	switch def.AppliesTo {
	case models.FeatureContextSiteAdmin:
		return true
	case models.FeatureContextAccount:
		return contextType == models.FeatureContextSiteAdmin ||
			contextType == models.FeatureContextAccount ||
			contextType == models.FeatureContextCourse
	case models.FeatureContextCourse:
		return contextType == models.FeatureContextSiteAdmin ||
			contextType == models.FeatureContextAccount ||
			contextType == models.FeatureContextCourse
	case models.FeatureContextUser:
		return contextType == models.FeatureContextUser
	}
	return false
}

func validState(s string) bool {
	switch s {
	case models.FeatureStateAllowed, models.FeatureStateOn,
		models.FeatureStateOff, models.FeatureStateHidden:
		return true
	}
	return false
}
