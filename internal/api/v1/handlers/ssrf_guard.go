package handlers

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/security"
)

// ErrSSRFBlocked is the handler-package re-export of
// security.ErrSSRFBlocked. The guard was moved into
// internal/security so non-handler callers (OIDC discovery, CAS
// validation, OneRoster, webhook delivery) can wear the same shield
// without a circular dependency.
var ErrSSRFBlocked = security.ErrSSRFBlocked

// validateExternalURL forwards to security.ValidateExternalURL.
// Existing handler code keeps its same call site; new callers
// outside the handler package should import internal/security
// directly.
func validateExternalURL(ctx context.Context, rawURL string) error {
	return security.ValidateExternalURL(ctx, rawURL)
}
