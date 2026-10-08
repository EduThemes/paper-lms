-- 000165_user_profile.up.sql
--
-- Learner dashboard lane (G161): user profile — pronouns, a short bio, an
-- uploaded avatar (storage key; avatar_url then points at the avatar
-- endpoint) and profile links.

ALTER TABLE users ADD COLUMN IF NOT EXISTS pronouns varchar(50) DEFAULT ''::character varying;
ALTER TABLE users ADD COLUMN IF NOT EXISTS bio text DEFAULT ''::text;
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_key text DEFAULT ''::text;

CREATE TABLE IF NOT EXISTS user_profile_links (
    id bigserial,
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title varchar(255) NOT NULL,
    url text NOT NULL,
    position bigint NOT NULL DEFAULT 0,
    created_at timestamptz,
    updated_at timestamptz,
    PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS idx_user_profile_links_user_id ON public.user_profile_links USING btree (user_id);
