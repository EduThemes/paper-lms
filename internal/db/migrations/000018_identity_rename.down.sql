-- Reverse the data copies from 000018_identity_rename.up.sql.
-- Each statement back-populates the legacy column from the new one when the
-- legacy column is at its zero/NULL value, mirroring the up direction.
--
-- This reversal is safe to run on a fresh DB (all guards are no-ops) and
-- idempotent on a partially-migrated one.

BEGIN;

-- ============ nonces ============
-- value → nonce.
UPDATE nonces
SET nonce = value
WHERE (nonce IS NULL OR nonce = '')
  AND value IS NOT NULL
  AND value <> '';

-- ============ developer_keys ============
-- icon → icon_url.
UPDATE developer_keys
SET icon_url = icon
WHERE icon_url IS NULL
  AND icon IS NOT NULL
  AND icon <> '';

-- ============ communication_channels ============
-- 3. confirm_code → confirmation_code.
UPDATE communication_channels
SET confirmation_code = confirm_code
WHERE confirmation_code IS NULL
  AND confirm_code IS NOT NULL
  AND confirm_code <> '';

-- 2. channel_type → path_type.
UPDATE communication_channels
SET path_type = channel_type
WHERE path_type IS NULL
  AND channel_type IS NOT NULL
  AND channel_type <> '';

-- 1. address → path.
UPDATE communication_channels
SET path = address
WHERE path IS NULL
  AND address IS NOT NULL
  AND address <> '';

-- ============ authentication_providers ============
-- id_p_entity_id → idp_entity_id.
UPDATE authentication_providers
SET idp_entity_id = id_p_entity_id
WHERE idp_entity_id IS NULL
  AND id_p_entity_id IS NOT NULL
  AND id_p_entity_id <> '';

COMMIT;
