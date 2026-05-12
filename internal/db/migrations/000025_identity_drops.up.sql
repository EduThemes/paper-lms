-- Wave 2c: drop legacy columns on the identity domain. DATA-DESTRUCTIVE.
--
-- Deprecation window: added 2026-05-11. Operators with production data
-- must have run Wave 2b (000018) first. Once this runs, dropped columns
-- are gone — the .down.sql can recreate the shape but not the data.
--
-- Per-table summary:
--   users:                    1 dropped — deleted_at (SOFT_DELETE_LEFTOVER)
--   accounts:                 no stale columns; skip
--   developer_keys:           3 dropped — deleted_at (SOFT_DELETE), api_key
--                             (UNKNOWN, no Go-source refs), icon_url (Wave 2b
--                             source → icon)
--   access_tokens:            3 dropped — deleted_at (SOFT_DELETE), code
--                             (UNKNOWN, auth codes stored in memory not DB),
--                             code_expires_at (UNKNOWN, zero Go-source refs)
--   authentication_providers: 12 dropped — deleted_at (SOFT_DELETE),
--                             idp_entity_id (Wave 2b source → id_p_entity_id),
--                             auth_base, auth_filter, auth_host, auth_over_tls,
--                             auth_password, auth_port, auth_username,
--                             identifier_format, metadata_uri,
--                             requested_authn_context (all UNKNOWN, zero
--                             Go-source refs), settings (UNKNOWN, zero
--                             auth-provider-specific Go-source refs)
--   nonces:                   3 dropped — deleted_at (SOFT_DELETE), nonce
--                             (Wave 2b source → value), updated_at (UNKNOWN,
--                             no model field, zero nonce-specific Go refs)
--   communication_channels:   7 dropped — deleted_at (SOFT_DELETE), path
--                             (Wave 2b source → address), path_type (Wave 2b
--                             source → channel_type), confirmation_code
--                             (Wave 2b source → confirm_code), bounce_count,
--                             last_bounce_at, last_suppression_bounce_at
--                             (all UNKNOWN, zero Go-source refs)
--   pairing_codes:            no stale columns; skip
--
-- KEPT (no drop, has Go-source references — Wave 2d audit needed):
--   (none — all retained UNKNOWN columns in this domain resolved to zero
--    table-specific references after targeted grep)

BEGIN;

-- ============================================================================
-- users
-- ============================================================================
ALTER TABLE users DROP COLUMN IF EXISTS deleted_at;

-- ============================================================================
-- developer_keys
-- ============================================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE developer_keys DROP COLUMN IF EXISTS deleted_at;

-- Wave 2b source: data now lives in icon (added by migration 000016)
ALTER TABLE developer_keys DROP COLUMN IF EXISTS icon_url;

-- UNKNOWN: no Go-source refs; api_key is a JSON alias for client_id in the
-- API layer (handlers/developer_keys.go maps key.ClientID → "api_key" in
-- a response map), not a live DB column used via GORM
ALTER TABLE developer_keys DROP COLUMN IF EXISTS api_key; -- no Go-source refs found

-- ============================================================================
-- access_tokens
-- ============================================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE access_tokens DROP COLUMN IF EXISTS deleted_at;

-- UNKNOWN: OAuth2 authorization codes are stored in memory (sync.Map in
-- OAuth2Service), not persisted to the DB; no Go-source refs to this column
ALTER TABLE access_tokens DROP COLUMN IF EXISTS code; -- no Go-source refs found

-- UNKNOWN: zero Go-source refs found
ALTER TABLE access_tokens DROP COLUMN IF EXISTS code_expires_at; -- no Go-source refs found

-- ============================================================================
-- authentication_providers
-- ============================================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE authentication_providers DROP COLUMN IF EXISTS deleted_at;

-- Wave 2b source: data now lives in id_p_entity_id (added by migration 000016)
ALTER TABLE authentication_providers DROP COLUMN IF EXISTS idp_entity_id;

-- UNKNOWN columns — zero Go-source refs found in non-migration, non-test files
ALTER TABLE authentication_providers DROP COLUMN IF EXISTS auth_base;           -- no Go-source refs found
ALTER TABLE authentication_providers DROP COLUMN IF EXISTS auth_filter;         -- no Go-source refs found
ALTER TABLE authentication_providers DROP COLUMN IF EXISTS auth_host;           -- no Go-source refs found
ALTER TABLE authentication_providers DROP COLUMN IF EXISTS auth_over_tls;       -- no Go-source refs found
ALTER TABLE authentication_providers DROP COLUMN IF EXISTS auth_password;       -- no Go-source refs found
ALTER TABLE authentication_providers DROP COLUMN IF EXISTS auth_port;           -- no Go-source refs found
ALTER TABLE authentication_providers DROP COLUMN IF EXISTS auth_username;       -- no Go-source refs found
ALTER TABLE authentication_providers DROP COLUMN IF EXISTS identifier_format;   -- no Go-source refs found
ALTER TABLE authentication_providers DROP COLUMN IF EXISTS metadata_uri;        -- no Go-source refs found
ALTER TABLE authentication_providers DROP COLUMN IF EXISTS requested_authn_context; -- no Go-source refs found
-- UNKNOWN: STALE_COLUMNS.md lists generic 'settings' refs (accounts.go:62
-- etc.) that match other tables; no authentication_provider-specific Go usage
ALTER TABLE authentication_providers DROP COLUMN IF EXISTS settings;            -- no Go-source refs found

-- ============================================================================
-- nonces
-- ============================================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE nonces DROP COLUMN IF EXISTS deleted_at;

-- Wave 2b source: data now lives in value (added NOT NULL by migration 000016)
ALTER TABLE nonces DROP COLUMN IF EXISTS nonce;

-- UNKNOWN: nonce model has no UpdatedAt field; STALE_COLUMNS refs are generic
-- updated_at pattern matches against other tables
ALTER TABLE nonces DROP COLUMN IF EXISTS updated_at; -- no Go-source refs found

-- ============================================================================
-- communication_channels
-- ============================================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE communication_channels DROP COLUMN IF EXISTS deleted_at;

-- Wave 2b sources: data now lives in address / channel_type / confirm_code
-- (all three added by migration 000016)
ALTER TABLE communication_channels DROP COLUMN IF EXISTS path;
ALTER TABLE communication_channels DROP COLUMN IF EXISTS path_type;
ALTER TABLE communication_channels DROP COLUMN IF EXISTS confirmation_code;

-- UNKNOWN columns — zero Go-source refs found in non-migration, non-test files
ALTER TABLE communication_channels DROP COLUMN IF EXISTS bounce_count;              -- no Go-source refs found
ALTER TABLE communication_channels DROP COLUMN IF EXISTS last_bounce_at;            -- no Go-source refs found
ALTER TABLE communication_channels DROP COLUMN IF EXISTS last_suppression_bounce_at; -- no Go-source refs found

COMMIT;
