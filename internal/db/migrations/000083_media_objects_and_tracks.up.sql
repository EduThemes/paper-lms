-- 000083_media_objects_and_tracks.up.sql
--
-- Audio/video uploaded or recorded from the rich content editor, and their
-- caption tracks (Canvas media_objects / media_tracks, G002). The bytes are
-- an attachment owned by the uploader; media_id is the unguessable id rich
-- content embeds (/api/v1/media_objects/:media_id/stream).

BEGIN;

CREATE TABLE IF NOT EXISTS media_objects (
    id bigserial PRIMARY KEY,
    account_id bigint NOT NULL,
    user_id bigint NOT NULL,
    attachment_id bigint NOT NULL,
    media_id text NOT NULL,
    context_type text NOT NULL,
    context_id bigint NOT NULL,
    media_type text NOT NULL,
    content_type text NOT NULL,
    title text NOT NULL DEFAULT '',
    workflow_state text NOT NULL DEFAULT 'active',
    created_at timestamptz,
    updated_at timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_media_objects_media_id ON media_objects USING btree (media_id);
CREATE INDEX IF NOT EXISTS idx_media_objects_account_id ON media_objects USING btree (account_id);
CREATE INDEX IF NOT EXISTS idx_media_objects_user_id ON media_objects USING btree (user_id);
CREATE INDEX IF NOT EXISTS idx_media_objects_attachment_id ON media_objects USING btree (attachment_id);

CREATE TABLE IF NOT EXISTS media_tracks (
    id bigserial PRIMARY KEY,
    media_object_id bigint NOT NULL,
    account_id bigint NOT NULL,
    user_id bigint NOT NULL,
    kind text NOT NULL DEFAULT 'subtitles',
    locale text NOT NULL,
    label text NOT NULL DEFAULT '',
    content text NOT NULL,
    webvtt_content text NOT NULL,
    created_at timestamptz,
    updated_at timestamptz,
    CONSTRAINT chk_media_tracks_kind CHECK (kind IN ('subtitles', 'captions', 'descriptions', 'chapters', 'metadata'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_media_tracks_object_locale ON media_tracks USING btree (media_object_id, locale);
CREATE INDEX IF NOT EXISTS idx_media_tracks_account_id ON media_tracks USING btree (account_id);

COMMIT;
