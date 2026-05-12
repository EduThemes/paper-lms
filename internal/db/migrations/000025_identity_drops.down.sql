-- Reverse of 000025: re-add the dropped identity-domain columns with their
-- original types from migrations 000001 (init) and 000009 (pairing_codes).
-- Structural only — the row data is LOST.
--
-- IMPORTANT: rows that existed when 000025 ran no longer have legacy-column
-- values. After this down runs, all legacy columns are NULL/default. If you
-- need the legacy data back, you must restore from a backup taken before
-- 000025 ran; the .up.sql is data-destructive by design.
--
-- Columns that were NOT NULL in the original schema are re-added as nullable
-- here: recreating NOT NULL without a default would fail on populated tables.
-- Operators rolling back (who are presumably also restoring row data from a
-- backup) can re-tighten constraints separately once data is restored.

BEGIN;

-- ============================================================================
-- users
-- ============================================================================
ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============================================================================
-- developer_keys
-- ============================================================================
ALTER TABLE developer_keys ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE developer_keys ADD COLUMN IF NOT EXISTS icon_url text;
ALTER TABLE developer_keys ADD COLUMN IF NOT EXISTS api_key text;

-- ============================================================================
-- access_tokens
-- ============================================================================
ALTER TABLE access_tokens ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE access_tokens ADD COLUMN IF NOT EXISTS code text;
ALTER TABLE access_tokens ADD COLUMN IF NOT EXISTS code_expires_at timestamptz;

-- ============================================================================
-- authentication_providers
-- ============================================================================
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS idp_entity_id text;
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS auth_base text;
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS auth_filter text;
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS auth_host text;
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS auth_over_tls text DEFAULT 'simple_tls';
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS auth_password text;
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS auth_port bigint;
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS auth_username text;
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS identifier_format text DEFAULT 'urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress';
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS metadata_uri text;
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS requested_authn_context text;
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS settings text;

-- ============================================================================
-- nonces
-- ============================================================================
ALTER TABLE nonces ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE nonces ADD COLUMN IF NOT EXISTS nonce text;
ALTER TABLE nonces ADD COLUMN IF NOT EXISTS updated_at timestamptz;

-- ============================================================================
-- communication_channels
-- ============================================================================
ALTER TABLE communication_channels ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE communication_channels ADD COLUMN IF NOT EXISTS path text;
ALTER TABLE communication_channels ADD COLUMN IF NOT EXISTS path_type text DEFAULT 'email';
ALTER TABLE communication_channels ADD COLUMN IF NOT EXISTS confirmation_code text;
ALTER TABLE communication_channels ADD COLUMN IF NOT EXISTS bounce_count bigint DEFAULT 0;
ALTER TABLE communication_channels ADD COLUMN IF NOT EXISTS last_bounce_at timestamptz;
ALTER TABLE communication_channels ADD COLUMN IF NOT EXISTS last_suppression_bounce_at timestamptz;

COMMIT;
