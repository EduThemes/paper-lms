package models

import "time"

// Usage-rights justifications (Canvas UsageRights::JUSTIFICATIONS).
const (
	UsageRightsOwnCopyright     = "own_copyright"
	UsageRightsUsedByPermission = "used_by_permission"
	UsageRightsPublicDomain     = "public_domain"
	UsageRightsFairUse          = "fair_use"
	UsageRightsCreativeCommons  = "creative_commons"
)

// ValidUsageRightsJustification reports whether j is a known
// justification. The empty string (rights not set) is valid.
func ValidUsageRightsJustification(j string) bool {
	switch j {
	case "", UsageRightsOwnCopyright, UsageRightsUsedByPermission, UsageRightsPublicDomain,
		UsageRightsFairUse, UsageRightsCreativeCommons:
		return true
	}
	return false
}

// LockedForLearnerAt reports whether learners are kept out of the file at
// time now: it is unpublished, or now falls outside its
// unlock_at..lock_at availability window.
func (a *Attachment) LockedForLearnerAt(now time.Time) bool {
	if a.Locked {
		return true
	}
	if a.UnlockAt != nil && now.Before(*a.UnlockAt) {
		return true
	}
	if a.LockAt != nil && !now.Before(*a.LockAt) {
		return true
	}
	return false
}
