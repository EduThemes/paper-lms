package auth

import (
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

// TestVerifyTOTPWithReuseGuard_AcceptsFreshCodeAdvancesWindow verifies
// the happy path: a code from a window > lastUsedWindow is accepted,
// and the returned newLastUsedWindow is the current window.
func TestVerifyTOTPWithReuseGuard_AcceptsFreshCodeAdvancesWindow(t *testing.T) {
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "Paper LMS", AccountName: "test@paper.test"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	current := CurrentTOTPWindow()
	code, err := totp.GenerateCode(key.Secret(), time.Now())
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}

	newWindow, ok, err := VerifyTOTPWithReuseGuard(key.Secret(), code, current-2)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !ok {
		t.Fatal("expected fresh code to verify")
	}
	if newWindow < current {
		t.Errorf("newLastUsedWindow should be >= current; got %d vs current %d", newWindow, current)
	}
}

// TestVerifyTOTPWithReuseGuard_RejectsReplayInSameWindow is the
// load-bearing test for this sprint. A code that's cryptographically
// valid but whose window has already been consumed must be rejected
// with the replay sentinel.
func TestVerifyTOTPWithReuseGuard_RejectsReplayInSameWindow(t *testing.T) {
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "Paper LMS", AccountName: "test@paper.test"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	current := CurrentTOTPWindow()
	code, err := totp.GenerateCode(key.Secret(), time.Now())
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}

	// First use: succeeds, advances the last-used window to current.
	newWindow, ok, err := VerifyTOTPWithReuseGuard(key.Secret(), code, 0)
	if err != nil || !ok {
		t.Fatalf("first use should succeed; got ok=%v err=%v", ok, err)
	}
	if newWindow < current {
		t.Fatalf("newWindow should be >= current, got %d", newWindow)
	}

	// Second use of the SAME code, with lastUsedWindow now=newWindow,
	// must reject as replay.
	_, ok2, err2 := VerifyTOTPWithReuseGuard(key.Secret(), code, newWindow)
	if ok2 {
		t.Error("replay should not verify")
	}
	if !IsTOTPReplay(err2) {
		t.Errorf("expected replay sentinel error, got %v", err2)
	}
}

// TestVerifyTOTPWithReuseGuard_WrongCodeNotReplay confirms the error
// returned for a bad code is distinguishable from the replay sentinel.
// Frontend uses IsTOTPReplay to decide whether to show "code already
// used" vs "wrong code"; conflating the two would surface the wrong
// message.
func TestVerifyTOTPWithReuseGuard_WrongCodeNotReplay(t *testing.T) {
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "Paper LMS", AccountName: "test@paper.test"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	_, ok, err := VerifyTOTPWithReuseGuard(key.Secret(), "000000", 0)
	if ok {
		t.Error("000000 should not match")
	}
	if IsTOTPReplay(err) {
		t.Errorf("wrong code should NOT be classified as replay; got err=%v", err)
	}
}

