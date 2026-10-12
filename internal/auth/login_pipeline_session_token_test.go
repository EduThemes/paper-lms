package auth

import (
	"context"
	"errors"
	"testing"
)

// G136: a /login/session_token redemption funnels through the pipeline
// with ProviderType "session_token". Like a passkey it names the user
// directly and skips the OTP step (the URL was minted from a full-access
// API token for that user), but suspension still refuses the session and
// a tenant MFA enrollment requirement still flags MustEnroll.
func TestPipeline_SessionToken_MintsSessionWithoutMFAStep(t *testing.T) {
	h := newHarness(t, "required_all")
	enrolled := enrolledUser(4, "eve.user@paper.test", "user")
	h.users.put(enrolled)
	u := unenrolledUser(5, "pat.user@paper.test", "user")
	h.users.put(u)

	res, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: ProviderTypeSessionToken, ExternalSubject: "4", EmailVerified: true,
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Token == "" || res.PendingToken != "" || res.MustEnroll {
		t.Fatalf("an enrolled user gets a plain session token (no OTP step), got %+v", res)
	}

	res, err = h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: ProviderTypeSessionToken, ExternalSubject: "5", EmailVerified: true,
	}, RequestMeta{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Token == "" || res.PendingToken != "" || !res.MustEnroll {
		t.Fatalf("an unenrolled user under required_all must still be sent to enroll, got %+v", res)
	}
	if res.User == nil || res.User.ID != 5 {
		t.Fatalf("wrong user resolved: %+v", res.User)
	}

	// A passkey is itself the second factor: no enrollment flag.
	res, err = h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: "passkey", ExternalSubject: "5", EmailVerified: true,
	}, RequestMeta{})
	if err != nil || res.MustEnroll {
		t.Fatalf("passkey login must not flag MustEnroll: err=%v res=%+v", err, res)
	}
}

func TestPipeline_SessionToken_SuspendedRefused(t *testing.T) {
	h := newHarness(t, "off")
	u := enrolledUser(6, "gone@paper.test", "user")
	u.Suspended = true
	h.users.put(u)

	res, err := h.pipeline.Execute(context.Background(), SSOOutcome{
		ProviderType: ProviderTypeSessionToken, ExternalSubject: "6",
	}, RequestMeta{})
	if !errors.Is(err, ErrAccountSuspended) || res != nil {
		t.Fatalf("expected ErrAccountSuspended, got err=%v res=%+v", err, res)
	}
}

func TestPipeline_SessionToken_MissingUserIDErrors(t *testing.T) {
	h := newHarness(t, "off")
	if _, err := h.pipeline.Execute(context.Background(), SSOOutcome{ProviderType: ProviderTypeSessionToken}, RequestMeta{}); err == nil {
		t.Fatal("expected an error for a session token outcome with no user id")
	}
}
