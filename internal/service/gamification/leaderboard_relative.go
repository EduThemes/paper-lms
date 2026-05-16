package gamification

import (
	"fmt"
	"hash/fnv"

	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service/gamification/pseudonym"
)

// RelativeRow is a single row in a relative-window response. Fillers
// (synthetic motivational entries that pad the bottom of the window so
// no real learner sees themselves dead last) share the same shape as
// real rows so a curious learner can't distinguish them in the
// network tab. IsFiller is exposed to the handler only — the JSON
// serializer strips it before sending to the client.
//
// Per the user's W3-C requirements (2026-05-14): the viewer should
// always appear in the middle of the window unless they're in the
// top 5. A learner in last place sees 2 real peers above + their own
// row + 2 fillers below (geometric falloff). Goal: avoid demotivation,
// increase motivation; fillers are close enough to feel beatable.
type RelativeRow struct {
	UserID         uint
	Rank           int   // 0 for fillers — they're not "rank 17" in any real sense
	LifetimeEarned int64
	IsViewer       bool
	IsFiller       bool
	// Pseudonym carries the rendered name when the policy is anonymizing;
	// the handler picks between Pseudonym and the real user's legal name
	// based on RenderPolicy.UsePseudonyms.
	Pseudonym string
}

// RelativeWindow bundles the composed window plus next-to-beat metadata.
type RelativeWindow struct {
	Rows        []RelativeRow
	NextToBeat  *NextToBeat
}

// NextToBeat is the "if you earn N more XP you pass …" callout payload.
// Nil when the viewer is already rank 1 or isn't ranked at all.
type NextToBeat struct {
	UserID         uint
	Pseudonym      string
	LegalName      string
	LifetimeEarned int64
	Gap            int64
}

// RelativeWindowSize is the canonical W3-C window length (5 rows).
// Tunable later if cohort sizes shift.
const RelativeWindowSize = 5

// fillerDecay controls how steeply filler scores fall below the lowest
// real row. 0.85 is the user's specified geometric falloff — close
// enough that progress moves the viewer past a filler quickly.
const fillerDecay = 0.85

// ComposeRelativeWindow assembles the W3-C window for a single viewer.
// Inputs:
//   - ranked: the full sorted list of real candidates (post opt-out).
//   - viewerID: 0 if the viewer isn't a ranked candidate (teacher, opted
//     out, etc.) — the window then centers on the top of the ranked list.
//   - pool: pseudonym pool for the filler names (same pool the viewer's
//     own enrollment uses, so fillers blend in).
//   - viewerEnrollmentID: seed input for stable filler identity per
//     (viewer, course).
//
// Output: a window of length RelativeWindowSize (or fewer if `ranked`
// is empty AND the viewer has no rank), with the viewer placed at
// index 2 when possible. Fillers fill any leading or trailing gap.
func ComposeRelativeWindow(ranked []repository.RankRow, viewerID uint, pool pseudonym.Pool, viewerEnrollmentID uint) RelativeWindow {
	viewerIdx := -1
	for i, r := range ranked {
		if r.UserID == viewerID {
			viewerIdx = i
			break
		}
	}

	// Slice the real-row window first, then pad with fillers as needed
	// so the viewer lands at index 2 (the middle of a 5-row window).
	const wantBefore = 2
	const wantAfter = 2

	var realSlice []repository.RankRow
	switch {
	case viewerIdx < 0:
		// Viewer not ranked. Show the top of the board (with the lowest
		// available rank truncation). The bottom of the window will be
		// fillers if the cohort is tiny.
		end := RelativeWindowSize
		if end > len(ranked) {
			end = len(ranked)
		}
		realSlice = ranked[:end]
	default:
		start := viewerIdx - wantBefore
		if start < 0 {
			start = 0
		}
		end := viewerIdx + wantAfter + 1
		if end > len(ranked) {
			end = len(ranked)
		}
		realSlice = ranked[start:end]
	}

	rows := make([]RelativeRow, 0, RelativeWindowSize)
	for _, r := range realSlice {
		rows = append(rows, RelativeRow{
			UserID:         r.UserID,
			Rank:           r.Rank,
			LifetimeEarned: r.LifetimeEarned,
			IsViewer:       r.UserID == viewerID,
		})
	}

	// Filler padding. We pad the BOTTOM first because the user's
	// requirement is "never see myself dead last". Top padding only
	// kicks in when the cohort itself is smaller than 5 — and even
	// then the viewer is usually rank 1 there, so we pad below them
	// to fill the window.
	//
	// Decay anchor: captured ONCE before the loop so each filler
	// decays from the last real row's score, not from the prior
	// filler's score. This matches the user's "always close and
	// motivating" intent — a 40 XP viewer sees fillers at 34/29/25
	// rather than the cumulative 20/8/3 of the original implementation.
	var fillerAnchor int64
	switch {
	case len(rows) > 0:
		fillerAnchor = rows[len(rows)-1].LifetimeEarned
	case viewerIdx >= 0:
		fillerAnchor = ranked[viewerIdx].LifetimeEarned
	}
	for fillerOffset := 0; len(rows) < RelativeWindowSize; fillerOffset++ {
		fillerName := fillerNameAt(pool, viewerEnrollmentID, len(rows))
		fillerScore := decayScore(fillerAnchor, fillerOffset)
		rows = append(rows, RelativeRow{
			IsFiller:       true,
			Pseudonym:      fillerName,
			LifetimeEarned: fillerScore,
		})
	}

	// Next-to-beat. The row directly above the viewer in the *real*
	// ranking (not the window slice — which may have been padded).
	// Returns nil when the viewer is rank 1 or unranked.
	var ntb *NextToBeat
	if viewerIdx > 0 {
		above := ranked[viewerIdx-1]
		viewer := ranked[viewerIdx]
		ntb = &NextToBeat{
			UserID:         above.UserID,
			LifetimeEarned: above.LifetimeEarned,
			Gap:            (above.LifetimeEarned - viewer.LifetimeEarned) + 1,
		}
	}

	return RelativeWindow{Rows: rows, NextToBeat: ntb}
}

// fillerNameAt deterministically rolls a pseudonym for a filler slot.
// Stable per (viewerEnrollmentID, slotIndex) so the same kid sees the
// same fillers across refreshes within a window. Different from the
// real-row pseudonym generator's seed shape so fillers don't shadow
// real enrollments in the same pool's combinatorial space.
func fillerNameAt(pool pseudonym.Pool, viewerEnrollmentID uint, slotIndex int) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(fmt.Sprintf("paper-lms.filler.v1|%s|%d|%d", pool.Code, viewerEnrollmentID, slotIndex)))
	seed := h.Sum64()
	if len(pool.Adjectives) == 0 || len(pool.Nouns) == 0 {
		return "Anonymous Wanderer"
	}
	adj := pool.Adjectives[seed%uint64(len(pool.Adjectives))]
	noun := pool.Nouns[(seed/uint64(len(pool.Adjectives)))%uint64(len(pool.Nouns))]
	return adj + " " + noun
}

// decayScore returns `anchor * fillerDecay^(fillerOffset+1)`, the score
// for a filler at position `fillerOffset` below the anchor (offset 0
// is the first filler below the lowest real row). Geometric falloff
// from a fixed anchor — not cumulative across previous fillers, which
// would compound too aggressively (40 → 20 → 8 instead of 34 → 29 → 25).
//
// Minimum floor of 0 — we never go negative, that'd be confusing UX.
// When anchor <= 0 (viewer at rank 1 of a single-student cohort with no
// XP), every filler renders at 0; the window is motivationally honest
// — nobody has earned anything yet.
func decayScore(anchor int64, fillerOffset int) int64 {
	if anchor <= 0 {
		return 0
	}
	score := float64(anchor)
	for i := 0; i <= fillerOffset; i++ {
		score *= fillerDecay
	}
	if score < 1 {
		score = 0
	}
	return int64(score)
}
