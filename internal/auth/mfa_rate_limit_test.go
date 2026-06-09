package auth

import (
	"testing"
	"time"
)

func TestMFAAttemptTracker_AllowsUpToCapThenRejects(t *testing.T) {
	tr := NewMFAAttemptTracker(func() time.Time { return time.Unix(0, 0) })
	tok := "pending-jwt-fixture-1"

	for i := 0; i < MaxVerifyAttempts; i++ {
		if err := tr.CheckAndIncrementVerify(tok); err != nil {
			t.Fatalf("attempt %d should succeed (cap is %d); got %v", i+1, MaxVerifyAttempts, err)
		}
	}
	// Cap+1 — must fail.
	err := tr.CheckAndIncrementVerify(tok)
	if !IsTooManyMFAAttempts(err) {
		t.Errorf("expected ErrTooManyMFAAttempts on attempt %d, got %v", MaxVerifyAttempts+1, err)
	}
}

func TestMFAAttemptTracker_PerTokenIndependence(t *testing.T) {
	tr := NewMFAAttemptTracker(nil)
	a := "pending-jwt-fixture-A"
	b := "pending-jwt-fixture-B"

	// Burn token A to its cap.
	for i := 0; i < MaxVerifyAttempts; i++ {
		_ = tr.CheckAndIncrementVerify(a)
	}
	if !IsTooManyMFAAttempts(tr.CheckAndIncrementVerify(a)) {
		t.Fatal("A should be exhausted")
	}
	// Token B must still allow attempts.
	if err := tr.CheckAndIncrementVerify(b); err != nil {
		t.Errorf("B should be independent; got %v", err)
	}
}

func TestMFAAttemptTracker_RecoveryHasTighterCap(t *testing.T) {
	tr := NewMFAAttemptTracker(nil)
	tok := "pending-jwt-fixture-rec"

	for i := 0; i < MaxRecoveryAttempts; i++ {
		if err := tr.CheckAndIncrementRecovery(tok); err != nil {
			t.Fatalf("attempt %d should succeed (cap is %d); got %v", i+1, MaxRecoveryAttempts, err)
		}
	}
	if !IsTooManyMFAAttempts(tr.CheckAndIncrementRecovery(tok)) {
		t.Errorf("recovery should be rejected after %d attempts", MaxRecoveryAttempts)
	}
}

func TestMFAAttemptTracker_VerifyAndRecoveryCountsAreSeparate(t *testing.T) {
	// A user who exhausts verify attempts should NOT have their
	// recovery attempts pre-decremented — recovery codes are a
	// separate fallback channel.
	tr := NewMFAAttemptTracker(nil)
	tok := "pending-jwt-fixture-sep"

	for i := 0; i < MaxVerifyAttempts; i++ {
		_ = tr.CheckAndIncrementVerify(tok)
	}
	// Verify is now exhausted; recovery on the same token must still
	// be allowed for the full MaxRecoveryAttempts.
	for i := 0; i < MaxRecoveryAttempts; i++ {
		if err := tr.CheckAndIncrementRecovery(tok); err != nil {
			t.Fatalf("recovery attempt %d should succeed independent of verify cap; got %v", i+1, err)
		}
	}
}

func TestMFAAttemptTracker_Reset(t *testing.T) {
	tr := NewMFAAttemptTracker(nil)
	tok := "pending-jwt-fixture-reset"
	for i := 0; i < MaxVerifyAttempts; i++ {
		_ = tr.CheckAndIncrementVerify(tok)
	}
	if !IsTooManyMFAAttempts(tr.CheckAndIncrementVerify(tok)) {
		t.Fatal("expected exhausted")
	}
	tr.Reset(tok)
	if err := tr.CheckAndIncrementVerify(tok); err != nil {
		t.Errorf("after Reset, attempts should be allowed again; got %v", err)
	}
}

// The per-account cap is the defense against the per-token cap being
// defeated by re-minting: it's keyed on the user id, not the token, so a
// fresh pending token does not buy a fresh budget.
func TestMFAAttemptTracker_PerAccount_SurvivesTokenReMint(t *testing.T) {
	now := time.Unix(0, 0)
	tr := NewMFAAttemptTracker(func() time.Time { return now })
	const uid = uint(42)

	for i := 0; i < MaxVerifyAttemptsPerAccount; i++ {
		if err := tr.CheckAndIncrementVerifyForUser(uid); err != nil {
			t.Fatalf("account attempt %d should succeed (cap %d); got %v", i+1, MaxVerifyAttemptsPerAccount, err)
		}
	}
	if !IsTooManyMFAAttempts(tr.CheckAndIncrementVerifyForUser(uid)) {
		t.Fatalf("account budget must be exhausted after %d attempts regardless of token re-mints", MaxVerifyAttemptsPerAccount)
	}
}

func TestMFAAttemptTracker_PerAccount_WindowResets(t *testing.T) {
	now := time.Unix(0, 0)
	tr := NewMFAAttemptTracker(func() time.Time { return now })
	const uid = uint(7)

	for i := 0; i < MaxVerifyAttemptsPerAccount; i++ {
		_ = tr.CheckAndIncrementVerifyForUser(uid)
	}
	if !IsTooManyMFAAttempts(tr.CheckAndIncrementVerifyForUser(uid)) {
		t.Fatal("should be exhausted within the window")
	}
	// Advance past the window — the budget rolls over.
	now = now.Add(accountAttemptWindow + time.Second)
	if err := tr.CheckAndIncrementVerifyForUser(uid); err != nil {
		t.Errorf("after the window elapses, attempts should be allowed again; got %v", err)
	}
}

func TestMFAAttemptTracker_PerAccount_Independence(t *testing.T) {
	tr := NewMFAAttemptTracker(nil)
	for i := 0; i < MaxVerifyAttemptsPerAccount; i++ {
		_ = tr.CheckAndIncrementVerifyForUser(1)
	}
	if !IsTooManyMFAAttempts(tr.CheckAndIncrementVerifyForUser(1)) {
		t.Fatal("user 1 should be exhausted")
	}
	if err := tr.CheckAndIncrementVerifyForUser(2); err != nil {
		t.Errorf("user 2 must be independent; got %v", err)
	}
}
