# 0005. `RenderPolicyFor` as the only place leaderboard visibility is decided

## Status

Accepted

## Context

Phase 6 / Wave 3 lit up student-facing leaderboards on top of the
Wave 1-2 gamification engine. The leaderboard is the most visible
privacy surface the product has: a K-5 student's last name should
never appear next to a rank number on a peer's screen; a high-school
top-5 list is motivating but the rest of the cohort doesn't need to
see their own position in last place; a corporate or higher-ed
tenant probably *does* want real names.

Worked through with the user 2026-05-14, the tenant-mode × role
matrix shipped to look like this:

- **K-5 / M68 students** — pseudonyms always on; no top-N for any
  viewer; no learner-driven pool switching (teacher-controlled).
- **H912 students** — pseudonyms on; top-5 visible only to top-5
  themselves; learner may switch pool or pick first-name mode.
- **HigherEd / Corp / Pro students** — real names; top-5 visible to
  top-5.
- **Admin + course teachers / TAs** — always see real names + full list.

The danger pattern, very common in WordPress gamification plugins, is
to repeat this matrix at every render site: leaderboard list,
leaderboard widget, notification text, badge feed, profile widget,
emailed digest. Each site grows its own bugs.

## Decision

**`gamification.RenderPolicyFor(tenantMode, role, viewerRank)` is the
single source of truth.** Every caller that needs to decide whether
to show a name, a pseudonym, or a top-N list calls into it and
trusts the returned policy.

The function lives in
`internal/service/gamification/leaderboard_render_policy.go`. It
returns a `RenderPolicy` value with the booleans the caller needs:
`UsePseudonym`, `ShowTopN` (and `DefaultTopNSize` when relevant),
`LearnerCanSwitch`, `RevealOwnLegalName` (always true — the viewer
sees their own real name regardless of policy).

`GetCourseLeaderboard` is the canonical caller:

1. Resolve viewer role in course (`resolveViewerRoleInCourse`, the
   `(result, wrote, err)` helper that may emit its own 403).
2. Load tenant mode (`accountRepo.GetByID` — the eighth and final
   constructor parameter on `GamificationHandler`).
3. Call `RenderPolicyFor(tenantMode, role, viewerRank)`.
4. Apply the policy in order: substitute pseudonyms on peer rows;
   trim to top-N or compose a relative window with filler ghosts
   (`leaderboard_relative.go`) when `ShowTopN == false`; never alter
   the viewer's own row identity.

The pseudonym substrate is a frozen-in-code catalog
(`internal/service/gamification/pseudonym/`), not a DB table. Three
pools ship: `animals_v1`, `superheroes_v1`, `explorers_v1`. Generation
is FNV-64 deterministic so the same kid sees the same pseudonym for
the same enrollment. `Validate` rejects free-text names not in the
pool's combinatorial space — no "butthead mcnastyface" through the
self-update endpoint.

The relative-window mechanic (Sprint W3-C) means a learner outside
the policy-allowed top-N sees a 5-row window centered on themselves,
with filler "ghost" entries padding any gap below their rank.
**Filler privacy invariant**: response shape is byte-identical to a
real row — no `is_filler` flag, no marker, no special user_id.
Inspecting the network tab cannot distinguish a filler from a real
peer.

## Consequences

- **Adding a new leaderboard surface** (e.g., a profile widget, a
  weekly digest email) is "call `RenderPolicyFor`, render with the
  flags it returns." No new policy logic.
- **Changing the matrix** is one edit to one function and one test
  (`leaderboard_render_policy_test.go` covers all five tenant modes ×
  three roles).
- **FERPA cascade is enforced at two points**: snapshot-write time
  (the weekly snapshot CLI filters opt-out users when persisting) and
  snapshot-read time (the same filter runs again on read). A learner
  who opts out post-snapshot vanishes from peer views without a
  snapshot rewrite. Rank numbers are re-issued 1..N after the filter
  drops to avoid visible gaps.
- **Adding a new tenant mode** requires extending the matrix in
  `RenderPolicyFor` *and* its test. Out-of-band documentation that
  drifts from the function is the failure mode we're preventing.
- **Cross-tenant leakage is not the policy's job** — `account_id`
  filtering happens upstream in the repository layer (see Phase 13
  patterns + ADR-pending on multi-tenancy). `RenderPolicyFor` trusts
  that the candidate set it sees is already tenant-scoped.

## References

- `internal/service/gamification/leaderboard_render_policy.go` — the
  function
- `internal/service/gamification/leaderboard_relative.go` — relative
  window + filler ghosts
- `internal/service/gamification/pseudonym/` — curated word lists +
  generator
- `internal/service/gamification/leaderboard_relative_test.go` —
  matrix + filler-privacy tests
- `internal/api/v1/handlers/gamification_leaderboards.go` — canonical
  caller, including `resolveViewerRoleInCourse`
- `docs/audits/2026-05-15-gamification-audit.md` — Sprint 7-A audit
  that hardened the surfaces around this
