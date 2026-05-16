-- 000052_users_account_id.up.sql
--
-- Phase 13 Item 13.1.A — multi-tenancy correctness, schema stage.
--
-- The audit found `users.account_id` doesn't exist (despite gamification
-- and authentication_providers already keying on it). Every multi-tenant
-- check in the codebase currently falls back to the hardcoded `1` in
-- `handlers/commons.go:callerAccountID`. The Kansas K-12 contract is
-- non-viable until users carry a tenant column.
--
-- Three-step add to avoid a write lock spike:
--   1. ADD COLUMN nullable, default NULL (no rewrite — Postgres
--      stores NULL in the row header).
--   2. Backfill from each user's primary StudentEnrollment → Course →
--      Account. Users with no enrollment fall back to account_id = 1
--      (the legacy default which is what every current handler
--      assumes today).
--   3. SET NOT NULL + add index. The NOT NULL conversion does a full
--      scan but no rewrite once the backfill is verified complete.
--
-- 13.1.B (JWT) adds an account_id claim. 13.1.C (callerAccountID
-- becomes authoritative) removes the hardcoded `1`. 13.1.D plumbs
-- account_id through the repo layer. 13.1.E asserts at handler scope.
-- 13.1.F makes RequireAdmin tenant-aware.

BEGIN;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS account_id bigint REFERENCES accounts(id) ON DELETE CASCADE;

-- Backfill: prefer the account of each user's primary StudentEnrollment,
-- fall back to TeacherEnrollment/TaEnrollment/DesignerEnrollment, then
-- to ObserverEnrollment (parents follow the child's tenant), then to
-- account 1 for users with no enrollment at all (the legacy default).
--
-- Done in a single SQL pass via DISTINCT ON ordering — for tens of
-- thousands of rows this completes well under a second on dev hardware,
-- and the audit's deployment scale is hundreds-of-thousands tops.
UPDATE users u
SET account_id = sub.account_id
FROM (
    SELECT DISTINCT ON (e.user_id) e.user_id,
        c.account_id
    FROM enrollments e
    JOIN courses c ON c.id = e.course_id
    WHERE e.workflow_state = 'active'
    ORDER BY e.user_id,
        CASE e.type
            WHEN 'StudentEnrollment'  THEN 1
            WHEN 'TeacherEnrollment'  THEN 2
            WHEN 'TaEnrollment'       THEN 3
            WHEN 'DesignerEnrollment' THEN 4
            WHEN 'ObserverEnrollment' THEN 5
            ELSE 6
        END,
        e.created_at ASC
) AS sub
WHERE u.id = sub.user_id
  AND u.account_id IS NULL;

-- Users with zero active enrollments (system admins, freshly-provisioned
-- accounts, orphan rows from legacy imports) take the root tenant.
UPDATE users
   SET account_id = 1
 WHERE account_id IS NULL;

-- Lock down: every user has a tenant now, and the column gains a
-- covering index for tenant-scoped reads.
ALTER TABLE users
    ALTER COLUMN account_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_users_account_id
    ON users (account_id);

COMMIT;
