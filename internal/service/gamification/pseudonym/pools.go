// Package pseudonym wires the per-enrollment leaderboard alias scheme
// added in Wave 3 Sprint W3-B.
//
// Mechanic
//
// When a learner views a peer's row on a course leaderboard, they see a
// stable whimsical alias (e.g. "Wandering Otter") instead of the peer's
// legal name. The mapping lives on `enrollments.pseudonym_name` so it
// stays stable for the duration of the term but varies between courses.
//
// Per the user's design constraint, learners may switch their *own*
// pseudonym to a different curated pool (animals, superheroes,
// explorers) or to first-name mode where the tenant policy allows it.
// They cannot free-text — every accepted name must be reachable from
// a server-side pool's adjective × noun combinatorial space. This
// rules out "butthead mcnastyface" and friends.
//
// The catalog is intentionally code-resident (not DB-resident): adding
// or curating a pool is a reviewable PR, not a runtime config flip,
// and the entire vocabulary is auditable at a glance.
package pseudonym

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
)

// PoolCode names a vocabulary pool. The string values are stored
// verbatim in enrollments.pseudonym_pool_code; renaming a pool is a
// breaking change for anyone whose enrollment row already references
// the old code.
type PoolCode string

const (
	PoolAnimals     PoolCode = "animals_v1"
	PoolSuperheroes PoolCode = "superheroes_v1"
	PoolExplorers   PoolCode = "explorers_v1"
	// PoolFirstName is a special pool that does NOT generate combinatorial
	// names from a word list; instead the renderer pulls the first
	// whitespace-delimited token of `users.name`. Stored as the pool
	// code on the enrollment, with `pseudonym_name` left NULL.
	PoolFirstName PoolCode = "first_name"
)

// Pool is a generative vocabulary. Adjectives × Nouns gives the
// combinatorial space the deterministic generator picks from.
type Pool struct {
	Code        PoolCode
	Label       string   // human-readable label for the picker UI
	Description string
	Adjectives  []string // curated, classroom-safe; ~40 entries
	Nouns       []string // curated, classroom-safe; ~60 entries
}

// Catalog enumerates the pools available for selection. Order is
// stable for the discovery endpoint so the UI can present them in a
// predictable sequence; insert new pools at the end of the slice to
// preserve that ordering for existing clients.
func Catalog() []Pool {
	return []Pool{
		animalsPool,
		superheroesPool,
		explorersPool,
	}
}

// PoolByCode returns a *Pool by string code. The special PoolFirstName
// code returns nil with no error — the caller resolves the first-name
// path separately because it doesn't consult a Pool's word lists.
func PoolByCode(code PoolCode) (*Pool, error) {
	if code == PoolFirstName {
		return nil, nil
	}
	for i := range catalogIndex {
		p := &catalogIndex[i]
		if p.Code == code {
			return p, nil
		}
	}
	return nil, fmt.Errorf("unknown pseudonym pool code: %q", code)
}

// catalogIndex backs PoolByCode without re-allocating Catalog() per
// call. Kept in package state because adding a pool is a code change,
// not runtime config.
var catalogIndex = []Pool{animalsPool, superheroesPool, explorersPool}

// GenerateForEnrollment returns a stable pseudonym for (enrollmentID,
// attempt) within the given pool. Stability invariant: same
// (enrollmentID, attempt, pool.Code) always returns the same string.
//
// Attempt is the re-roll counter the caller advances when a UNIQUE
// constraint trips on insert. 16 attempts cover the realistic
// per-course collision space — with ~2400 combinations and ≤30 peers
// per cohort the birthday-paradox probability of needing more than
// a handful of re-rolls is vanishingly small.
func GenerateForEnrollment(pool Pool, enrollmentID uint, attempt int) string {
	if len(pool.Adjectives) == 0 || len(pool.Nouns) == 0 {
		// Defensive: an empty pool would loop or divide-by-zero.
		// Return a deterministic-but-marked placeholder so the value
		// is obviously wrong in QA rather than silently zero-indexed.
		return "Anonymous Learner"
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(fmt.Sprintf("paper-lms.pseudonym.v1|%s|%d|%d", pool.Code, enrollmentID, attempt)))
	seed := h.Sum64()
	adj := pool.Adjectives[seed%uint64(len(pool.Adjectives))]
	noun := pool.Nouns[(seed/uint64(len(pool.Adjectives)))%uint64(len(pool.Nouns))]
	return adj + " " + noun
}

// Validate returns nil iff `name` is a producible value from `pool`
// (i.e. its " "-split tokens are an (adjective, noun) pair that both
// exist in the pool's word lists). Used to reject learner-supplied
// names that aren't drawn from the curated vocabulary, even if those
// names look superficially safe ("Cool Frog" might not be in the
// pool — accept only what the pool actually generates).
func Validate(pool Pool, name string) error {
	parts := strings.SplitN(strings.TrimSpace(name), " ", 2)
	if len(parts) != 2 {
		return errors.New("pseudonym must be a two-word adjective + noun")
	}
	adj, noun := parts[0], parts[1]
	if !containsFold(pool.Adjectives, adj) {
		return fmt.Errorf("adjective %q is not in pool %s", adj, pool.Code)
	}
	if !containsFold(pool.Nouns, noun) {
		return fmt.Errorf("noun %q is not in pool %s", noun, pool.Code)
	}
	return nil
}

func containsFold(haystack []string, needle string) bool {
	for _, h := range haystack {
		if strings.EqualFold(h, needle) {
			return true
		}
	}
	return false
}

// CandidateCount returns the total combinatorial size of the pool.
// Useful for monitoring (alerting if a course is large enough that
// pseudonym collisions are likely).
func CandidateCount(pool Pool) int {
	return len(pool.Adjectives) * len(pool.Nouns)
}

// FirstNameOf splits `legalName` on whitespace and returns the first
// non-empty token. Empty input returns the empty string; the caller
// should fall back to a default (e.g. "Learner") when that happens.
//
// Kept in this package so the W3-B `RenderPolicyFor` and the
// /pseudonym handler share one implementation; if the splitting rule
// later grows (e.g. honors mononymic learners), one edit covers both
// surfaces.
func FirstNameOf(legalName string) string {
	for _, tok := range strings.Fields(legalName) {
		return tok
	}
	return ""
}

// Generator wraps a small interface so handlers that need to roll a
// pseudonym in the request path can be tested without a real DB —
// the caller passes a function that loads an enrollment's current
// (pool, attempt) state and persists the next one.
type Generator interface {
	// Generate rolls a pseudonym for the given enrollment under the
	// supplied pool, retrying up to 16 attempts on uniqueness conflict.
	// `tryInsert` returns (created, error): true if the row was
	// successfully written, false on UNIQUE collision, any other
	// error halts the loop.
	Generate(ctx context.Context, pool Pool, enrollmentID uint, tryInsert func(name string, attempt int) (bool, error)) (string, error)
}

type generator struct{}

// NewGenerator returns the default deterministic generator.
func NewGenerator() Generator { return &generator{} }

func (g *generator) Generate(ctx context.Context, pool Pool, enrollmentID uint, tryInsert func(name string, attempt int) (bool, error)) (string, error) {
	const maxAttempts = 16
	for attempt := 0; attempt < maxAttempts; attempt++ {
		name := GenerateForEnrollment(pool, enrollmentID, attempt)
		created, err := tryInsert(name, attempt)
		if err != nil {
			return "", fmt.Errorf("pseudonym insert (attempt %d): %w", attempt, err)
		}
		if created {
			return name, nil
		}
	}
	return "", fmt.Errorf("could not allocate unique pseudonym in pool %s after %d attempts", pool.Code, maxAttempts)
}
