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
