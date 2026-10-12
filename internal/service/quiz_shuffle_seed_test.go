package service

import "testing"

// The seed must not be computable from the ids alone: with a different key
// the same ids give a different seed, and with the same key it is stable.
func TestQuizShuffleSeed_KeyedAndStable(t *testing.T) {
	t.Cleanup(func() { SetQuizShuffleKey(nil) })

	SetQuizShuffleKey([]byte("key-one"))
	a1 := QuizShuffleSeed(12, 34)
	a2 := QuizShuffleSeed(12, 34)
	if a1 != a2 {
		t.Fatalf("seed not stable for the same key: %d vs %d", a1, a2)
	}
	if a1 < 0 {
		t.Fatalf("seed must be non-negative, got %d", a1)
	}
	if QuizShuffleSeed(12, 35) == a1 {
		t.Fatalf("different ids gave the same seed")
	}

	SetQuizShuffleKey([]byte("key-two"))
	if b := QuizShuffleSeed(12, 34); b == a1 {
		t.Fatalf("seed did not depend on the key")
	}

	// Unconfigured: a random per-process key, still stable within the process.
	SetQuizShuffleKey(nil)
	c1, c2 := QuizShuffleSeed(12, 34), QuizShuffleSeed(12, 34)
	if c1 != c2 {
		t.Fatalf("fallback key not stable: %d vs %d", c1, c2)
	}
}
