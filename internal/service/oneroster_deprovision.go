package service

// Roster-driven auto-deprovisioning (K-12 non-negotiable): a OneRoster
// FULL sync returns the complete roster, so any managed user absent
// from it has left the district and must lose access on this sync —
// not linger as an active orphaned account. Deprovisioning = suspend
// (the reversible kill-switch from migration 000063), never delete;
// users who reappear in the roster are auto-unsuspended.
//
// Guardrails (all load-bearing — see the deprovision test for locks):
//   - Scoped to oneroster-managed users only (sis_user_id LIKE
//     'oneroster:%'). Manually-created accounts and CSV-SIS users are
//     never touched.
//   - Opt-in per connection (deprovision_enabled, default false).
//   - Full-sync only: an incremental sync is a delta, so absence from
//     it means nothing. Gated on the sync log's SyncType, not on an
//     empty filter — a first-ever incremental sync also has no filter.
//   - Circuit breaker: an empty roster, or an implausibly large
//     suspension delta, aborts the pass and suspends nobody.
//   - Provenance: only suspensions this pass created (suspended_by_sis)
//     are ever auto-reactivated; manual admin suspensions stand.
//
// Known limitation (fast-follow): the managed population is account-
// scoped, not connection-scoped — sis_user_id carries no connection id.
// Two OneRoster connections on one account would each see the other's
// users as absent. Until sis IDs are namespaced per connection, run at
// most one deprovision-enabled connection per account.

import (
	"context"
	"fmt"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

const (
	// onerosterSISPrefix marks users provisioned by the OneRoster sync
	// (set in syncUsers); the LIKE pattern bounds deprovisioning to them.
	onerosterSISPrefix = "oneroster:"
	sisManagedPattern  = onerosterSISPrefix + "%"

	// Circuit-breaker threshold: abort if the pass would suspend more
	// than max(floor, pct·managed) users. The floor keeps small
	// legitimate deltas flowing in small districts; the percentage
	// bounds the blast radius in large ones. Per-connection overrides
	// are a planned fast-follow.
	deprovisionMinFloor = 10
	deprovisionMaxPct   = 0.25
)

// DeprovisionUser identifies one user a deprovision pass would touch —
// enough for an admin to recognize them in a preview.
type DeprovisionUser struct {
	ID      uint   `json:"id"`
	Name    string `json:"name"`
	LoginID string `json:"login_id"`
}

// DeprovisionResult is the outcome of one reconcile pass, either
// previewed (Applied=false) or executed during a full sync.
type DeprovisionResult struct {
	ManagedTotal int               `json:"managed_total"`
	ToSuspend    []DeprovisionUser `json:"to_suspend"`
	ToReactivate []DeprovisionUser `json:"to_reactivate"`
	Aborted      bool              `json:"aborted"`
	AbortReason  string            `json:"abort_reason"`
	Applied      bool              `json:"applied"`
}

// onerosterPresentSet maps "oneroster:<sourcedId>" → true for every
// roster user that should retain access. Status "tobedeleted" users are
// excluded on purpose: the SIS has marked them for removal, so they are
// treated as absent and deprovisioned.
func onerosterPresentSet(users []onerosterUser) map[string]bool {
	present := make(map[string]bool, len(users))
	for _, u := range users {
		if u.Status == "tobedeleted" {
			continue
		}
		present[onerosterSISPrefix+u.SourcedID] = true
	}
	return present
}

// reconcileDeprovision computes the suspend/reactivate sets for the
// connection's account against a full roster's present set, and — when
// apply is true and the circuit breaker doesn't trip — executes them.
func (s *OneRosterService) reconcileDeprovision(ctx context.Context, conn *models.OneRosterConnection, presentSISIDs map[string]bool, apply bool) (*DeprovisionResult, error) {
	managed, err := s.userRepo.ListSISManaged(ctx, conn.AccountID, sisManagedPattern)
	if err != nil {
		return nil, fmt.Errorf("listing SIS-managed users: %w", err)
	}

	result := &DeprovisionResult{ManagedTotal: len(managed)}
	for _, u := range managed {
		present := u.SISUserID != nil && presentSISIDs[*u.SISUserID]
		entry := DeprovisionUser{ID: u.ID, Name: u.Name, LoginID: u.LoginID}
		switch {
		case !present && !u.Suspended:
			result.ToSuspend = append(result.ToSuspend, entry)
		case present && u.Suspended && u.SuspendedBySIS:
			// Only reactivate suspensions this pass created (provenance
			// flag) — a manual/disciplinary admin suspension is never
			// silently undone because the user is still on the roster.
			result.ToReactivate = append(result.ToReactivate, entry)
		}
	}

	switch {
	case len(presentSISIDs) == 0 && result.ManagedTotal > 0:
		// An SIS returning 200 + an empty user list is a known failure
		// mode; never read it as "everyone left", whatever the threshold.
		result.Aborted = true
		result.AbortReason = fmt.Sprintf(
			"refusing to deprovision against an empty roster (%d managed users): the roster fetch likely failed upstream",
			result.ManagedTotal)
	case len(result.ToSuspend) > deprovisionThreshold(result.ManagedTotal):
		result.Aborted = true
		result.AbortReason = fmt.Sprintf(
			"refusing to suspend %d of %d managed users (threshold %d): the roster fetch may be incomplete",
			len(result.ToSuspend), result.ManagedTotal, deprovisionThreshold(result.ManagedTotal))
	}

	if !apply || result.Aborted {
		return result, nil
	}

	if err := s.userRepo.ApplySISDeprovision(ctx,
		deprovisionUserIDs(result.ToSuspend), deprovisionUserIDs(result.ToReactivate), conn.AccountID); err != nil {
		return result, fmt.Errorf("applying deprovision pass: %w", err)
	}
	result.Applied = true
	return result, nil
}

func deprovisionThreshold(managedTotal int) int {
	threshold := deprovisionMinFloor
	if pct := int(deprovisionMaxPct * float64(managedTotal)); pct > threshold {
		threshold = pct
	}
	return threshold
}

func deprovisionUserIDs(users []DeprovisionUser) []uint {
	ids := make([]uint, len(users))
	for i, u := range users {
		ids[i] = u.ID
	}
	return ids
}

// PreviewDeprovision fetches the live roster and reports exactly who a
// deprovision-enabled full sync would suspend/reactivate, without
// changing anything. Synchronous and read-only; admins use it before
// (and after) flipping deprovision_enabled on a connection.
func (s *OneRosterService) PreviewDeprovision(ctx context.Context, connectionID, accountID uint) (*DeprovisionResult, error) {
	conn, err := s.connRepo.FindByID(ctx, connectionID, accountID)
	if err != nil {
		return nil, fmt.Errorf("connection not found: %w", err)
	}

	token, err := s.fetchToken(conn)
	if err != nil {
		return nil, fmt.Errorf("OAuth2 token request failed: %w", err)
	}
	users, err := s.fetchUsers(conn.BaseURL, token, "")
	if err != nil {
		return nil, fmt.Errorf("roster fetch failed: %w", err)
	}

	return s.reconcileDeprovision(ctx, conn, onerosterPresentSet(users), false)
}
