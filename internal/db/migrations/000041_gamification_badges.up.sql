-- 000041_gamification_badges.up.sql
--
-- Phase 6 Wave 2 Sprint W2-D — internal-only badges.
--
-- Two tables:
--   * gamification_badges        — the definition (admin/instructor authored)
--   * gamification_badge_awards  — the issuance (per-user, per-badge ledger)
--
-- Per SYNTHESIS §5, K-12 ships internal-only badges by default. No
-- production OB 3.0 wallet handles under-13 users, so the school is the
-- COPPA-consenting party and the badge artifact stays server-side. The
-- `internal_only` column stakes out the future OB 3.0 export pivot (W5):
-- flipping a per-badge `internal_only=false` toggle is the signal that a
-- tenant has admin/parent consent to export that badge to a 3rd-party
-- wallet. Schema is therefore ready for that pivot without churn.
--
-- Rules reference badges by `code` (e.g., 'first_quiz_passed') so a rule
-- template can refer to a badge that doesn't exist in this tenant yet —
-- the resolver returns nil and the rule is a no-op. Same pattern as
-- gamification_currency_types.

BEGIN;

CREATE TABLE IF NOT EXISTS gamification_badges (
    id                  bigserial PRIMARY KEY,
    tenant_id           bigint        NOT NULL,
    scope_type          text          NOT NULL,
    scope_id            bigint        NOT NULL,
    code                text          NOT NULL,
    name                text          NOT NULL,
    description         text          NOT NULL DEFAULT '',
    icon                text          NOT NULL DEFAULT '',
    image_url           text          NOT NULL DEFAULT '',
    color               text          NOT NULL DEFAULT '',
    internal_only       boolean       NOT NULL DEFAULT TRUE,
    system_owned        boolean       NOT NULL DEFAULT FALSE,
    audience_level      text          NOT NULL DEFAULT '',
    created_by          bigint,
    created_at          timestamptz   NOT NULL DEFAULT now(),
    updated_at          timestamptz   NOT NULL DEFAULT now(),

    CONSTRAINT uniq_gam_badge_scope_code UNIQUE (tenant_id, scope_type, scope_id, code)
);

CREATE INDEX IF NOT EXISTS idx_gam_badges_tenant ON gamification_badges (tenant_id);
CREATE INDEX IF NOT EXISTS idx_gam_badges_scope  ON gamification_badges (scope_type, scope_id);

CREATE TABLE IF NOT EXISTS gamification_badge_awards (
    id                  bigserial PRIMARY KEY,
    user_id             bigint        NOT NULL,
    badge_id            bigint        NOT NULL REFERENCES gamification_badges(id) ON DELETE CASCADE,
    awarded_at          timestamptz   NOT NULL DEFAULT now(),
    awarded_by          bigint,
    evidence_event_id   bigint,

    -- A user holds each badge at most once. Re-awarding is a no-op for
    -- the rule engine; the W2-D effect uses INSERT ... ON CONFLICT DO
    -- NOTHING against this constraint so idempotency is atomic.
    CONSTRAINT uniq_gam_badge_award UNIQUE (user_id, badge_id)
);

CREATE INDEX IF NOT EXISTS idx_gam_badge_awards_user  ON gamification_badge_awards (user_id);
CREATE INDEX IF NOT EXISTS idx_gam_badge_awards_badge ON gamification_badge_awards (badge_id);

COMMIT;
