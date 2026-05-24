package auth

import (
	"testing"
	"time"
)

func TestSAMLReplayCache_FirstUseAccepted(t *testing.T) {
	c := NewSAMLReplayCache()
	if !c.CheckAndStore("assertion-1", time.Now().Add(5*time.Minute)) {
		t.Fatalf("first use of fresh assertion-id should return true")
	}
}

func TestSAMLReplayCache_SecondUseRejected(t *testing.T) {
	c := NewSAMLReplayCache()
	c.CheckAndStore("assertion-1", time.Now().Add(5*time.Minute))

	if c.CheckAndStore("assertion-1", time.Now().Add(5*time.Minute)) {
		t.Fatalf("second use of same assertion-id should return false (replay detected)")
	}
}

func TestSAMLReplayCache_AfterExpiryAccepted(t *testing.T) {
	c := NewSAMLReplayCache()
	// Past expiry — entry is treated as evictable.
	c.CheckAndStore("assertion-1", time.Now().Add(-1*time.Minute))

	if !c.CheckAndStore("assertion-1", time.Now().Add(5*time.Minute)) {
		t.Fatalf("re-use of expired assertion-id should be accepted (entry would be evicted by purge loop)")
	}
}

func TestSAMLReplayCache_EmptyIDRejected(t *testing.T) {
	c := NewSAMLReplayCache()
	if c.CheckAndStore("", time.Now().Add(5*time.Minute)) {
		t.Fatalf("empty assertion-id MUST be treated as a replay (refuse)")
	}
}

func TestSAMLReplayCache_DistinctIDsIndependent(t *testing.T) {
	c := NewSAMLReplayCache()
	if !c.CheckAndStore("a", time.Now().Add(5*time.Minute)) {
		t.Fatalf("first use of a should be accepted")
	}
	if !c.CheckAndStore("b", time.Now().Add(5*time.Minute)) {
		t.Fatalf("first use of b should be accepted (different id)")
	}
}
