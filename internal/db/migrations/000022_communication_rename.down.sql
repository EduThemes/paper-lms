-- Reverse the data copies from .up.sql. Each statement back-populates the
-- legacy column from the new one when the legacy column is at its zero value.
--
-- Lossy by design:
--   - bool→timestamp (read_at → read): truthiness is recoverable (presence
--     of read_at ⇒ true), but the chosen seed timestamp is lost. Acceptable
--     because Wave 2b is additive; Wave 2c's drop migration removes the legacy
--     columns for good.
--   - Polymorphic→FK (context_id → user_id / asset_id): only rows where
--     context_type='User' (or the asset context) can be safely reversed.

BEGIN;

-- Reverse 5: (context_type, context_id) → (asset_type, asset_id)
--   Only reverse rows that were migrated from asset columns (asset_id absent).
UPDATE page_views
SET asset_type = context_type,
    asset_id   = context_id
WHERE asset_id IS NULL
  AND context_id > 0
  AND context_type IS NOT NULL
  AND context_type <> '';

-- Reverse 4: (context_type='User', context_id) → user_id
--   Only reverse rows whose context came from a user FK.
UPDATE calendar_events
SET user_id = context_id
WHERE user_id IS NULL
  AND context_type = 'User'
  AND context_id > 0;

-- Reverse 3: read_at → read (discussion_entry_participants)
UPDATE discussion_entry_participants
SET read = true
WHERE read = false
  AND read_at IS NOT NULL;

-- Reverse 2: read_at → read (announcement_read_receipts)
UPDATE announcement_read_receipts
SET read = true
WHERE read = false
  AND read_at IS NOT NULL;

-- Reverse 1: o_id_c_initiation_url → oidc_initiation_url
UPDATE lti_tool_configurations
SET oidc_initiation_url = o_id_c_initiation_url
WHERE (oidc_initiation_url IS NULL OR oidc_initiation_url = '')
  AND o_id_c_initiation_url IS NOT NULL
  AND o_id_c_initiation_url <> '';

COMMIT;
