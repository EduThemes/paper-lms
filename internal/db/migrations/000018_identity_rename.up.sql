-- Wave 2b: data migration for the identity domain.
--
-- Covers tables: users, accounts, developer_keys, access_tokens,
-- authentication_providers, nonces, communication_channels, pairing_codes.
--
-- The original SQL schema used several different column names that the GORM
-- model later standardised. Wave 1 (000016) added the new GORM-named columns;
-- this migration back-fills them from the legacy columns so both sets of
-- callers see consistent data until Wave 2c drops the old columns.
--
-- Tables with NO COPY work (all stale cols are SOFT_DELETE_LEFTOVER or dead
-- UNKNOWN with no matching GORM field):
--   users           — only stale col is deleted_at (SOFT_DELETE_LEFTOVER)
--   accounts        — no stale columns at all
--   pairing_codes   — no stale columns at all
--   access_tokens   — code / code_expires_at have no GORM target (dead OAuth
--                     authorisation-code columns); deleted_at is SOFT_DELETE
--
-- Tables with COPY work authored below:
--   authentication_providers  idp_entity_id → id_p_entity_id
--   communication_channels    path → address, path_type → channel_type,
--                             confirmation_code → confirm_code
--   developer_keys            icon_url → icon
--   nonces                    nonce → value
--
-- All statements are idempotent (nullable target: WHERE new IS NULL AND old IS NOT NULL).

BEGIN;

-- ============ authentication_providers ============
-- RENAME_CANDIDATE: idp_entity_id (legacy) → id_p_entity_id (GORM).
-- GORM derives "id_p_entity_id" from the struct field IDPEntityID via its
-- default NamingStrategy. Migration 000016 added id_p_entity_id; copy forward.
UPDATE authentication_providers
SET id_p_entity_id = idp_entity_id
WHERE id_p_entity_id IS NULL
  AND idp_entity_id IS NOT NULL
  AND idp_entity_id <> '';

-- ============ communication_channels ============
-- 1. path → address (old column name for the channel's contact endpoint).
UPDATE communication_channels
SET address = path
WHERE address IS NULL
  AND path IS NOT NULL
  AND path <> '';

-- 2. path_type → channel_type (e.g. 'email', 'sms', 'push').
UPDATE communication_channels
SET channel_type = path_type
WHERE channel_type IS NULL
  AND path_type IS NOT NULL
  AND path_type <> '';

-- 3. confirmation_code → confirm_code (GORM field ConfirmCode → confirm_code).
UPDATE communication_channels
SET confirm_code = confirmation_code
WHERE confirm_code IS NULL
  AND confirmation_code IS NOT NULL
  AND confirmation_code <> '';

-- ============ developer_keys ============
-- icon_url → icon (GORM field Icon → icon; Wave 1 added the icon column).
UPDATE developer_keys
SET icon = icon_url
WHERE icon IS NULL
  AND icon_url IS NOT NULL
  AND icon_url <> '';

-- ============ nonces ============
-- nonce → value (GORM field Value → value; Wave 1 added value NOT NULL).
-- The target was added as NOT NULL in 000016; rows inserted post-Wave-1 already
-- have value populated. Only pre-Wave-1 rows (if any) need back-filling.
UPDATE nonces
SET value = nonce
WHERE (value IS NULL OR value = '')
  AND nonce IS NOT NULL
  AND nonce <> '';

COMMIT;
