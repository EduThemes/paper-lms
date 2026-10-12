-- G111: SIS import options (batch mode, diffing, change threshold, async
-- processing) and the uploaded CSVs of each batch.
ALTER TABLE sis_batches ADD COLUMN IF NOT EXISTS user_id bigint NOT NULL DEFAULT 0;
ALTER TABLE sis_batches ADD COLUMN IF NOT EXISTS batch_mode boolean NOT NULL DEFAULT false;
ALTER TABLE sis_batches ADD COLUMN IF NOT EXISTS batch_mode_term_id bigint;
ALTER TABLE sis_batches ADD COLUMN IF NOT EXISTS diffing_data_set_identifier text;
ALTER TABLE sis_batches ADD COLUMN IF NOT EXISTS diffed_against_batch_id bigint;
ALTER TABLE sis_batches ADD COLUMN IF NOT EXISTS change_threshold bigint;
ALTER TABLE sis_batches ADD COLUMN IF NOT EXISTS started_at timestamptz;
ALTER TABLE sis_batches ADD COLUMN IF NOT EXISTS ended_at timestamptz;

CREATE INDEX IF NOT EXISTS idx_sis_batches_diffing_data_set_identifier
    ON public.sis_batches USING btree (diffing_data_set_identifier);

CREATE TABLE IF NOT EXISTS sis_batch_files (
    id bigserial,
    sis_batch_id bigint NOT NULL REFERENCES sis_batches(id) ON DELETE CASCADE,
    file_name text NOT NULL,
    import_type text NOT NULL,
    content bytea NOT NULL,
    created_at timestamptz,
    PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS idx_sis_batch_files_sis_batch_id ON public.sis_batch_files USING btree (sis_batch_id);

-- groups.csv / group_categories.csv SIS ids.
ALTER TABLE groups ADD COLUMN IF NOT EXISTS sis_source_id text;
ALTER TABLE group_categories ADD COLUMN IF NOT EXISTS sis_source_id text;
CREATE INDEX IF NOT EXISTS idx_groups_sis_source_id ON public.groups USING btree (sis_source_id);
CREATE INDEX IF NOT EXISTS idx_group_categories_sis_source_id ON public.group_categories USING btree (sis_source_id);
