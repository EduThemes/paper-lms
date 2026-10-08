package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// LTILineItemFilter narrows an AGS line item listing (G128). Empty string
// fields and nil pointers do not filter.
type LTILineItemFilter struct {
	ResourceLinkID string // resource_link_id: the LTI resource link UUID
	ResourceID     string // resource_id: the tool's own resource identifier
	Tag            string // tag
	// DeveloperKeyID restricts the listing to line items owned by one tool
	// (a tool caller sees only its own line items).
	DeveloperKeyID *uint
}

// LTIAdvantageRepository holds the LTI 1.3 / Advantage queries that span
// several tables (resource links, line items, tool installs, tool
// configurations, module items). Every read takes the caller's tenant
// (accountID) as the trailing argument; accountID==0 skips the tenant
// filter and is reserved for auth-internal callers (tool-signed requests
// whose tenant is derived from the verified developer key).
type LTIAdvantageRepository interface {
	CreateResourceLink(ctx context.Context, link *models.LTIResourceLink) error
	UpdateResourceLink(ctx context.Context, link *models.LTIResourceLink) error
	// FindResourceLinkByUUID resolves a resource link by its LTI
	// resource_link_id under the tenant (via the link's course).
	FindResourceLinkByUUID(ctx context.Context, uuid string, accountID uint) (*models.LTIResourceLink, error)
	FindResourceLinkByAssignment(ctx context.Context, assignmentID, accountID uint) (*models.LTIResourceLink, error)
	FindResourceLinkByContentTag(ctx context.Context, contentTagID, accountID uint) (*models.LTIResourceLink, error)
	DeleteResourceLinksByAssignment(ctx context.Context, assignmentID, accountID uint) error
	// FindPlacementResourceLink finds the course-level resource link a
	// placement launch (e.g. course_navigation) of a tool reuses.
	FindPlacementResourceLink(ctx context.Context, toolID, courseID uint, placement string) (*models.LTIResourceLink, error)

	ListLineItems(ctx context.Context, courseID uint, filter LTILineItemFilter, params PaginationParams) (*PaginatedResult[models.LTILineItem], error)

	// ListActiveToolsForCourse returns the active tools installed in the
	// course plus those installed at the course's tenant account.
	ListActiveToolsForCourse(ctx context.Context, courseID, accountID uint) ([]models.ContextExternalTool, error)
	// FindActiveToolForCourse loads one active tool that is installed in
	// the course or at the course's tenant account.
	FindActiveToolForCourse(ctx context.Context, toolID, courseID, accountID uint) (*models.ContextExternalTool, error)
	// FindActiveToolByDeveloperKeyForCourse finds the active install of an
	// LTI developer key reachable from the course (course install first).
	FindActiveToolByDeveloperKeyForCourse(ctx context.Context, devKeyID, courseID, accountID uint) (*models.ContextExternalTool, error)

	// ListToolConfigurationsByAccount returns the LTI tool configurations
	// of the tenant's live LTI developer keys.
	ListToolConfigurationsByAccount(ctx context.Context, accountID uint) ([]models.LTIToolConfiguration, error)

	// FindContentTagInCourse loads a live module item whose module is in
	// the course (and the course in the tenant).
	FindContentTagInCourse(ctx context.Context, contentTagID, courseID, accountID uint) (*models.ContentTag, error)
	// NextModuleItemPosition returns 1 + the highest position in a module.
	NextModuleItemPosition(ctx context.Context, moduleID uint) (int, error)
}

// LTIPlatformKeyRotator is implemented by the Postgres platform-key store.
// It is kept separate from LTIPlatformKeyRepository so test fakes of the
// boot-time key loader need not implement rotation.
type LTIPlatformKeyRotator interface {
	// ListKeys returns every stored platform key (current "default",
	// upcoming "next", retired "previous").
	ListKeys(ctx context.Context) ([]models.LTIPlatformKey, error)
	// Rotate retires the current key to "previous" (dropping the old
	// previous), promotes "next" to current, and stores newNext as the
	// new "next" key, in one transaction. If there is no "next" key,
	// promoteFallback becomes current instead.
	Rotate(ctx context.Context, newNext, promoteFallback *models.LTIPlatformKey) error
}
