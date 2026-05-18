# Architecture Decision Records

This directory captures the load-bearing architectural decisions for
Paper LMS. Each ADR is a single page, focused on one decision, and
links to the code paths a reader can use to verify it.

ADRs describe **decisions that have already shipped**. If you're
proposing a new direction, open a discussion or a draft PR — don't
add an ADR until the call is locked.

## Format

Every ADR follows a four-section structure:

- **Status** — `Proposed`, `Accepted`, `Deprecated`, or `Superseded by ###`.
- **Context** — what forces drove the decision; what the codebase looked
  like before.
- **Decision** — the call itself, in declarative voice.
- **Consequences** — what changes downstream, what's now harder, what
  invariants other code must hold up.

Each ADR cites at least one file path so a reader can verify the claim
against live code.

## Index

| #   | Title | Status |
|-----|-------|--------|
| [0001](./0001-canvas-compatible-api-shape.md) | Canvas-compatible API shape | Accepted |
| [0002](./0002-auto-migrate-policy.md) | `AUTO_MIGRATE=false` in prod / CI; SQL chain as source of truth | Accepted |
| [0003](./0003-secretbox-encryption-at-rest.md) | `secretbox` envelope (AES-256-GCM + versioned key_id) for at-rest secrets | Accepted |
| [0004](./0004-login-pipeline-mfa-gate-placement.md) | `LoginPipeline.Execute` as the single MFA-gate convergence | Accepted |
| [0005](./0005-render-policy-single-source-of-truth.md) | `RenderPolicyFor` as the only place leaderboard visibility is decided | Accepted |

## Adding a new ADR

1. Copy [`template.md`](./template.md) to `NNNN-short-kebab-title.md`
   using the next available number.
2. Fill in the four sections; reference at least one file path.
3. Add the row to the index above.
4. Open a PR. Discuss the decision; merge once it's locked.

ADRs are immutable after merge. If a decision changes, write a new ADR
that supersedes the old one and update the old one's status.
