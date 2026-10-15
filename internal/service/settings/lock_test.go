package settings

import (
	"context"
	"errors"
	"testing"
)

// pages.custom_css_enabled is an OffLocksBelow key: an explicit false at
// the instance or at any account in the chain wins over every true below
// it. Chain used throughout: district (1) → school (10) → classroom (100).

const cssKey = PagesCustomCSSEnabled

func lockFixture(t *testing.T) *Service {
	t.Helper()
	svc, _, _ := newServiceWithAudit(t, map[uint]uint{1: 0, 10: 1, 100: 10})
	return svc
}

func mustSet(t *testing.T, svc *Service, scope ScopeType, id uint, v string) {
	t.Helper()
	if err := svc.Set(context.Background(), scope, id, cssKey, v, 1); err != nil {
		t.Fatalf("set %s/%d=%s: %v", scope, id, v, err)
	}
}

func getCSS(t *testing.T, svc *Service, accountID uint) EffectiveValue {
	t.Helper()
	ev, err := svc.Get(context.Background(), cssKey, ScopeHints{AccountID: accountID})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	return ev
}

func TestCatalog_CustomCSSIsAccountAdminEditableAndLocks(t *testing.T) {
	def, ok := Find(cssKey)
	if !ok || !def.OffLocksBelow || !def.AccountAdminEditable || def.ValueType != TypeBool {
		t.Fatalf("catalog entry: %+v", def)
	}
	keys := AccountAdminKeys()
	if len(keys) != 1 || keys[0].Key != cssKey {
		t.Fatalf("AccountAdminKeys = %+v, want only %s", keys, cssKey)
	}
	for _, d := range Catalog {
		if d.AccountAdminEditable && !d.AllowsScope(ScopeAccount) {
			t.Errorf("%s is account-admin editable but not account-scoped", d.Key)
		}
		if d.AccountAdminEditable && d.IsSecret() {
			t.Errorf("%s is a secret; tenant admins must not read or write it", d.Key)
		}
		if d.OffLocksBelow && d.ValueType != TypeBool {
			t.Errorf("%s locks when off but is not a bool", d.Key)
		}
	}
}

func TestOffLocks_NothingSetIsDefaultOn(t *testing.T) {
	svc := lockFixture(t)
	ev := getCSS(t, svc, 100)
	if ev.Value != "true" || ev.Source != SourceDefault {
		t.Fatalf("got %+v, want default true", ev)
	}
}

func TestOffLocks_InstanceOffBeatsAccountOn(t *testing.T) {
	svc := lockFixture(t)
	mustSet(t, svc, ScopeInstance, 0, "false")
	mustSet(t, svc, ScopeAccount, 100, "true")
	mustSet(t, svc, ScopeAccount, 1, "true")
	ev := getCSS(t, svc, 100)
	if ev.Value != "false" || ev.Source != SourceInstance {
		t.Fatalf("got %+v, want instance false", ev)
	}
}

func TestOffLocks_ParentOffChildOnIsOffLockedByParent(t *testing.T) {
	svc := lockFixture(t)
	mustSet(t, svc, ScopeAccount, 10, "false")
	mustSet(t, svc, ScopeAccount, 100, "true")
	ev := getCSS(t, svc, 100)
	if ev.Value != "false" || ev.Source != SourceAccount || ev.ScopeID != 10 {
		t.Fatalf("got %+v, want false from account 10", ev)
	}
	view, err := svc.ViewForAccount(context.Background(), cssKey, 100)
	if err != nil {
		t.Fatal(err)
	}
	if view.Own == nil || view.Own.Value != "true" {
		t.Fatalf("own = %+v, want explicit true", view.Own)
	}
	if view.LockedBy == nil || view.LockedBy.Scope != ScopeAccount || view.LockedBy.ScopeID != 10 {
		t.Fatalf("locked by = %+v, want account 10", view.LockedBy)
	}
	// The parent itself is off but not locked (nothing above it is off).
	pv, _ := svc.ViewForAccount(context.Background(), cssKey, 10)
	if pv.LockedBy != nil || pv.Effective.Value != "false" {
		t.Fatalf("parent view = %+v", pv)
	}
}

func TestOffLocks_HighestOffIsTheLock(t *testing.T) {
	svc := lockFixture(t)
	mustSet(t, svc, ScopeAccount, 1, "false")
	mustSet(t, svc, ScopeAccount, 10, "false")
	view, _ := svc.ViewForAccount(context.Background(), cssKey, 100)
	if view.LockedBy == nil || view.LockedBy.ScopeID != 1 {
		t.Fatalf("locked by = %+v, want the district (1)", view.LockedBy)
	}
	mustSet(t, svc, ScopeInstance, 0, "false")
	view, _ = svc.ViewForAccount(context.Background(), cssKey, 100)
	if view.LockedBy == nil || view.LockedBy.Scope != ScopeInstance {
		t.Fatalf("locked by = %+v, want the instance", view.LockedBy)
	}
}

func TestOffLocks_ChildOffParentOnIsOffNotLocked(t *testing.T) {
	svc := lockFixture(t)
	mustSet(t, svc, ScopeInstance, 0, "true")
	mustSet(t, svc, ScopeAccount, 1, "true")
	mustSet(t, svc, ScopeAccount, 100, "false")
	if ev := getCSS(t, svc, 100); ev.Value != "false" || ev.ScopeID != 100 {
		t.Fatalf("child: got %+v, want its own false", ev)
	}
	if ev := getCSS(t, svc, 10); ev.Value != "true" || ev.ScopeID != 1 {
		t.Fatalf("sibling path: got %+v, want district true", ev)
	}
	view, _ := svc.ViewForAccount(context.Background(), cssKey, 100)
	if view.LockedBy != nil {
		t.Fatalf("own false must not count as a lock: %+v", view.LockedBy)
	}
}

func TestOffLocks_NearestTrueWhenNothingOff(t *testing.T) {
	svc := lockFixture(t)
	mustSet(t, svc, ScopeInstance, 0, "true")
	mustSet(t, svc, ScopeAccount, 10, "TRUE")
	ev := getCSS(t, svc, 100)
	if ev.Source != SourceAccount || ev.ScopeID != 10 {
		t.Fatalf("got %+v, want nearest explicit (account 10)", ev)
	}
}

func TestChain_NearestFirstThenInstance(t *testing.T) {
	svc := lockFixture(t)
	mustSet(t, svc, ScopeInstance, 0, "true")
	mustSet(t, svc, ScopeAccount, 1, "false")
	mustSet(t, svc, ScopeAccount, 100, "true")
	levels, err := svc.Chain(context.Background(), cssKey, 100)
	if err != nil {
		t.Fatal(err)
	}
	want := []Level{{Scope: ScopeAccount, ScopeID: 100}, {Scope: ScopeAccount, ScopeID: 1}, {Scope: ScopeInstance}}
	if len(levels) != len(want) {
		t.Fatalf("levels = %+v", levels)
	}
	for i := range want {
		if levels[i].Scope != want[i].Scope || levels[i].ScopeID != want[i].ScopeID {
			t.Fatalf("level %d = %+v, want %+v", i, levels[i], want[i])
		}
	}
	if _, err := svc.Chain(context.Background(), "nope.nope", 1); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("unknown key: %v", err)
	}
}

func TestSetForAccount_RefusesOnUnderLockAllowsOffAndClear(t *testing.T) {
	svc, repo, audit := newServiceWithAudit(t, map[uint]uint{1: 0, 10: 1})
	ctx := context.Background()
	on, off := "true", "false"

	if err := svc.SetForAccount(ctx, 10, cssKey, &on, 7); err != nil {
		t.Fatalf("on with nothing above: %v", err)
	}
	mustSet(t, svc, ScopeAccount, 1, "false")

	err := svc.SetForAccount(ctx, 10, cssKey, &on, 7)
	var locked *LockedError
	if !errors.Is(err, ErrLocked) || !errors.As(err, &locked) || locked.By.ScopeID != 1 {
		t.Fatalf("on under a parent's off: err=%v", err)
	}
	if err := svc.SetForAccount(ctx, 10, cssKey, &off, 7); err != nil {
		t.Fatalf("off is always allowed: %v", err)
	}
	if err := svc.SetForAccount(ctx, 10, cssKey, nil, 7); err != nil {
		t.Fatalf("clear is always allowed: %v", err)
	}
	if _, ok := repo.rows[keyOf("account", 10, cssKey)]; ok {
		t.Fatal("clear left the row")
	}
	// set on, set off, clear → three audit rows by user 7.
	n := 0
	for _, e := range audit.events {
		if e.EventType == "setting_change" && e.UserID == 7 {
			n++
		}
	}
	if n != 3 {
		t.Fatalf("audit rows by user 7 = %d, want 3", n)
	}
}

func TestGet_NearestWinsUnchangedForOtherKeys(t *testing.T) {
	// A plain key still resolves nearest-first: child beats parent.
	svc := lockFixture(t)
	_ = svc.Set(context.Background(), ScopeAccount, 1, "quotas.max_upload_size_mb", "100", 1)
	_ = svc.Set(context.Background(), ScopeAccount, 100, "quotas.max_upload_size_mb", "200", 1)
	ev, _ := svc.Get(context.Background(), "quotas.max_upload_size_mb", ScopeHints{AccountID: 100})
	if ev.Value != "200" {
		t.Fatalf("got %+v", ev)
	}
}
