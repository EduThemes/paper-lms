package pseudonym

import (
	"context"
	"errors"
	"testing"
)

func TestGenerateForEnrollment_Deterministic(t *testing.T) {
	pool := animalsPool
	a := GenerateForEnrollment(pool, 42, 0)
	b := GenerateForEnrollment(pool, 42, 0)
	if a != b {
		t.Errorf("same (enrollmentID, attempt) should produce same name; got %q vs %q", a, b)
	}
	c := GenerateForEnrollment(pool, 42, 1)
	if a == c {
		t.Errorf("different attempt should produce different name; both %q", a)
	}
}

func TestGenerateForEnrollment_DifferentEnrollmentsDifferentNames(t *testing.T) {
	pool := animalsPool
	// Sample a few — with ~2400 combos and only 2 attempts the chance
	// of a collision between two specific IDs is ~1/2400. We check
	// that at least one of three distinct ids produces a unique name.
	seen := map[string]bool{}
	for id := uint(1); id <= 4; id++ {
		seen[GenerateForEnrollment(pool, id, 0)] = true
	}
	if len(seen) < 2 {
		t.Errorf("expected at least 2 distinct names across 4 enrollment IDs, got %d: %v", len(seen), seen)
	}
}

func TestValidate_RejectsOutOfPool(t *testing.T) {
	pool := animalsPool
	if err := Validate(pool, "Butthead McNastyface"); err == nil {
		t.Error("expected rejection of out-of-pool name")
	}
	if err := Validate(pool, "Brave"); err == nil {
		t.Error("expected rejection of single-word name")
	}
	// Pick a real generable name and re-validate.
	good := GenerateForEnrollment(pool, 1, 0)
	if err := Validate(pool, good); err != nil {
		t.Errorf("Validate rejected a generated name %q: %v", good, err)
	}
}

func TestPoolByCode_FirstNameReturnsNil(t *testing.T) {
	p, err := PoolByCode(PoolFirstName)
	if err != nil {
		t.Fatalf("first_name lookup should not error: %v", err)
	}
	if p != nil {
		t.Errorf("first_name should return nil pool (special case), got %+v", p)
	}
}

func TestGenerator_RetriesOnCollision(t *testing.T) {
	pool := animalsPool
	gen := NewGenerator()
	calls := 0
	name, err := gen.Generate(context.Background(), pool, 7, func(name string, attempt int) (bool, error) {
		calls++
		// Pretend the first 3 attempts collided.
		if attempt < 3 {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 4 {
		t.Errorf("expected 4 callback invocations (3 collisions + 1 success), got %d", calls)
	}
	if name == "" {
		t.Error("expected a non-empty pseudonym after retries")
	}
}

func TestGenerator_GivesUpAfterMaxAttempts(t *testing.T) {
	pool := animalsPool
	gen := NewGenerator()
	_, err := gen.Generate(context.Background(), pool, 7, func(name string, attempt int) (bool, error) {
		return false, nil // always collide
	})
	if err == nil {
		t.Fatal("expected an error after the max-attempt loop exits")
	}
}

func TestGenerator_PropagatesUnderlyingError(t *testing.T) {
	pool := animalsPool
	gen := NewGenerator()
	sentinel := errors.New("boom")
	_, err := gen.Generate(context.Background(), pool, 7, func(name string, attempt int) (bool, error) {
		return false, sentinel
	})
	if err == nil || !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel error, got %v", err)
	}
}
