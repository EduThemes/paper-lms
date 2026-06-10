package service_test

// Integration tests for the OneRoster deprovision pass (migration
// 000064). Runs against real Postgres via the PARITY_DB_URL pattern
// (freshDB lives in sis_import_initial_password_test.go); skipped when
// no database is reachable.
//
// Contracts locked here, in guardrail order:
//   1. Managed-only — manually-created users (sis_user_id IS NULL) and
//      users from other SIS sources never appear in either set.
//   2. Set math — absent managed users land in ToSuspend, reappearing
//      suspended users in ToReactivate; preview (apply=false) changes
//      nothing.
//   3. Apply — exactly the computed sets are suspended/reactivated,
//      tenant-bounded.
//   4. Circuit breaker — an implausibly large suspension delta aborts
//      the pass and suspends nobody.
//   5. tobedeleted roster users are treated as absent.

import (
	"context"
	"fmt"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"gorm.io/gorm"
)

func newDeprovisionFixture(t *testing.T) (*service.OneRosterService, *gorm.DB, func()) {
	t.Helper()
	g, cleanup := freshDB(t)

	for _, id := range []uint{1, 2} {
		if err := g.Exec(
			`INSERT INTO accounts (id, name, workflow_state, mfa_policy, default_locale, tenant_mode, max_upload_size_mb)
			 VALUES (?, ?, 'active', 'off', 'en', 'higher_ed', 500)
			 ON CONFLICT (id) DO NOTHING`,
			id, fmt.Sprintf("Account %d", id),
		).Error; err != nil {
			cleanup()
			t.Fatalf("seed account %d: %v", id, err)
		}
	}

	svc := service.NewOneRosterService(
		postgres.NewOneRosterConnectionRepository(g),
		postgres.NewOneRosterSyncLogRepository(g),
		postgres.NewUserRepository(g),
		postgres.NewCourseRepository(g),
		postgres.NewSectionRepository(g),
		postgres.NewEnrollmentRepository(g),
		postgres.NewAccountRepository(g),
		g,
	)
	return svc, g, cleanup
}

// seedDeprovisionUser inserts a user directly (the OneRoster sync path
// that normally creates these rows is exercised elsewhere). sisID nil
// models a manually-created account. suspendedBySIS=false with
// suspended=true models a manual/disciplinary admin suspension.
func seedDeprovisionUser(t *testing.T, g *gorm.DB, accountID uint, login string, sisID *string, suspended, suspendedBySIS bool) uint {
	t.Helper()
	u := &models.User{
		AccountID: accountID,
		Name:      login,
		LoginID:   login,
		Email:     login + "@example.com",
		Role:      "user",
		SISUserID: sisID,
	}
	if err := g.Create(u).Error; err != nil {
		t.Fatalf("seed user %s: %v", login, err)
	}
	if suspended || suspendedBySIS {
		if err := g.Model(&models.User{}).Where("id = ?", u.ID).
			Updates(map[string]interface{}{"suspended": suspended, "suspended_by_sis": suspendedBySIS}).Error; err != nil {
			t.Fatalf("set suspension flags for seed user %s: %v", login, err)
		}
	}
	return u.ID
}

func userSuspensionFlags(t *testing.T, g *gorm.DB, id uint) (suspended, bySIS bool) {
	t.Helper()
	var row struct {
		Suspended      bool
		SuspendedBySIS bool `gorm:"column:suspended_by_sis"`
	}
	if err := g.Model(&models.User{}).Select("suspended", "suspended_by_sis").Where("id = ?", id).Scan(&row).Error; err != nil {
		t.Fatalf("read suspension flags for user %d: %v", id, err)
	}
	return row.Suspended, row.SuspendedBySIS
}

func userSuspended(t *testing.T, g *gorm.DB, id uint) bool {
	t.Helper()
	suspended, _ := userSuspensionFlags(t, g, id)
	return suspended
}

func TestOneRosterDeprovision_SetMathAndApply(t *testing.T) {
	svc, g, cleanup := newDeprovisionFixture(t)
	defer cleanup()
	ctx := context.Background()

	// Account 1 population:
	present := seedDeprovisionUser(t, g, 1, "present.student", strPtr("oneroster:s1"), false, false)
	absent1 := seedDeprovisionUser(t, g, 1, "left.student", strPtr("oneroster:s2"), false, false)
	absent2 := seedDeprovisionUser(t, g, 1, "left.teacher", strPtr("oneroster:s3"), false, false)
	// SIS-suspended by a previous pass, now back on the roster.
	returned := seedDeprovisionUser(t, g, 1, "returned.student", strPtr("oneroster:s4"), true, true)
	// Manually suspended by an admin while still rostered — provenance
	// guard: the sync must NOT auto-reactivate a disciplinary suspension.
	disciplined := seedDeprovisionUser(t, g, 1, "disciplined.student", strPtr("oneroster:s5"), true, false)
	manualAdmin := seedDeprovisionUser(t, g, 1, "manual.admin", nil, false, false)
	csvUser := seedDeprovisionUser(t, g, 1, "csv.user", strPtr("sis-csv-001"), false, false)
	// A managed user in account 2, absent from account 1's roster —
	// the tenant boundary must keep it out of the sets and untouched.
	// (sis_user_id is globally unique, so it needs its own sourcedID.)
	otherTenant := seedDeprovisionUser(t, g, 2, "other.tenant", strPtr("oneroster:t2-gone"), false, false)

	conn := &models.OneRosterConnection{AccountID: 1}
	roster := map[string]bool{"oneroster:s1": true, "oneroster:s4": true, "oneroster:s5": true}

	// Preview: correct sets, no writes.
	result, err := svc.ReconcileDeprovision(ctx, conn, roster, false)
	if err != nil {
		t.Fatalf("preview reconcile: %v", err)
	}
	if result.ManagedTotal != 5 {
		t.Errorf("ManagedTotal = %d, want 5 (s1-s5; manual + csv + other-tenant excluded)", result.ManagedTotal)
	}
	wantSuspend := map[uint]bool{absent1: true, absent2: true}
	if len(result.ToSuspend) != 2 {
		t.Fatalf("ToSuspend = %+v, want exactly absent1+absent2", result.ToSuspend)
	}
	for _, u := range result.ToSuspend {
		if !wantSuspend[u.ID] {
			t.Errorf("ToSuspend contains unexpected user %d (%s)", u.ID, u.LoginID)
		}
	}
	if len(result.ToReactivate) != 1 || result.ToReactivate[0].ID != returned {
		t.Fatalf("ToReactivate = %+v, want exactly the returned SIS-suspended user (manual suspension must stand)", result.ToReactivate)
	}
	if result.Aborted || result.Applied {
		t.Fatalf("preview must be aborted=false applied=false, got %+v", result)
	}
	if userSuspended(t, g, absent1) || !userSuspended(t, g, returned) {
		t.Fatal("preview (apply=false) must not change any suspension state")
	}

	// Apply: exactly the computed sets flip; nobody else does.
	result, err = svc.ReconcileDeprovision(ctx, conn, roster, true)
	if err != nil {
		t.Fatalf("apply reconcile: %v", err)
	}
	if !result.Applied || result.Aborted {
		t.Fatalf("apply must report applied=true aborted=false, got %+v", result)
	}
	for name, want := range map[uint]bool{
		present:     false,
		absent1:     true,
		absent2:     true,
		returned:    false,
		disciplined: true, // manual suspension survives the sync
		manualAdmin: false,
		csvUser:     false,
		otherTenant: false,
	} {
		if got := userSuspended(t, g, name); got != want {
			t.Errorf("user %d suspended = %v, want %v", name, got, want)
		}
	}

	// Provenance flags after apply: sync suspensions are attributable,
	// reactivation clears the flag, manual suspension keeps flag false.
	if _, bySIS := userSuspensionFlags(t, g, absent1); !bySIS {
		t.Error("sync-suspended user must carry suspended_by_sis=true")
	}
	if _, bySIS := userSuspensionFlags(t, g, returned); bySIS {
		t.Error("reactivated user must have suspended_by_sis cleared")
	}
	if _, bySIS := userSuspensionFlags(t, g, disciplined); bySIS {
		t.Error("manually suspended user must keep suspended_by_sis=false")
	}
}

func TestOneRosterDeprovision_EmptyRosterAborts(t *testing.T) {
	svc, g, cleanup := newDeprovisionFixture(t)
	defer cleanup()
	ctx := context.Background()

	// Small tenant: 3 managed users is under the breaker floor of 10,
	// so only the empty-roster hard abort protects them from an SIS
	// that returns 200 + an empty user list.
	var ids []uint
	for i := 0; i < 3; i++ {
		login := fmt.Sprintf("small.user%d", i)
		ids = append(ids, seedDeprovisionUser(t, g, 1, login, strPtr(fmt.Sprintf("oneroster:small%d", i)), false, false))
	}

	conn := &models.OneRosterConnection{AccountID: 1}
	result, err := svc.ReconcileDeprovision(ctx, conn, map[string]bool{}, true)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if !result.Aborted || result.Applied {
		t.Fatalf("empty roster must hard-abort below the breaker floor, got %+v", result)
	}
	for _, id := range ids {
		if userSuspended(t, g, id) {
			t.Fatalf("empty-roster abort suspended user %d anyway", id)
		}
	}
}

func TestOneRosterDeprovision_CircuitBreaker(t *testing.T) {
	svc, g, cleanup := newDeprovisionFixture(t)
	defer cleanup()
	ctx := context.Background()

	// 20 managed users, roster keeps only 1 → 19 to suspend >
	// max(10, 25%·20=5) → threshold abort. (Roster is non-empty on
	// purpose; the empty-roster hard abort has its own test.)
	var ids []uint
	for i := 0; i < 20; i++ {
		login := fmt.Sprintf("bulk.user%02d", i)
		ids = append(ids, seedDeprovisionUser(t, g, 1, login, strPtr(fmt.Sprintf("oneroster:bulk%02d", i)), false, false))
	}

	conn := &models.OneRosterConnection{AccountID: 1}
	result, err := svc.ReconcileDeprovision(ctx, conn, map[string]bool{"oneroster:bulk19": true}, true)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if !result.Aborted || result.Applied {
		t.Fatalf("circuit breaker must abort (aborted=true applied=false), got %+v", result)
	}
	if result.AbortReason == "" {
		t.Error("abort must record a reason for the sync log")
	}
	for _, id := range ids {
		if userSuspended(t, g, id) {
			t.Fatalf("circuit breaker tripped but user %d was suspended anyway", id)
		}
	}

	// A plausible delta on the same population still flows: suspend 8
	// of 20 (8 ≤ max(10, 5)).
	roster := map[string]bool{}
	for i := 8; i < 20; i++ {
		roster[fmt.Sprintf("oneroster:bulk%02d", i)] = true
	}
	result, err = svc.ReconcileDeprovision(ctx, conn, roster, true)
	if err != nil {
		t.Fatalf("reconcile with plausible delta: %v", err)
	}
	if result.Aborted || !result.Applied || len(result.ToSuspend) != 8 {
		t.Fatalf("plausible delta must apply (8 suspensions), got %+v", result)
	}
	if !userSuspended(t, g, ids[0]) || userSuspended(t, g, ids[9]) {
		t.Fatal("apply after plausible delta suspended the wrong users")
	}
}

func TestOneRosterDeprovision_PresentSetTreatsToBeDeletedAsAbsent(t *testing.T) {
	set := service.OneRosterPresentSet([]struct{ SourcedID, Status string }{
		{SourcedID: "s1", Status: "active"},
		{SourcedID: "s2", Status: "tobedeleted"},
	})
	if !set["oneroster:s1"] {
		t.Error("active roster user must be present")
	}
	if set["oneroster:s2"] {
		t.Error("tobedeleted roster user must be treated as absent (deprovisioned)")
	}
}
