-- 000092_module_progressions.up.sql
--
-- Module progression engine (Canvas ContextModuleProgression).
--
--   * context_modules.requirement_count: 0 = the student must meet every
--     completion requirement in the module, 1 = any one of them (Canvas
--     "complete all" / "complete one").
--   * context_module_progressions: the per-student evaluated state of a
--     module (locked / unlocked / started / completed), which
--     requirements are met, the sequential-progress position and the
--     completion time. Rows are rewritten every time a student's
--     progress is evaluated, so teachers can read them back.
--   * module_item_user_actions: the facts a progression cannot derive
--     from other tables: when a student viewed a module item (must_view)
--     and whether they marked it done (must_mark_done).
--
-- workflow_state and requirements_met are NOT NULL with SQL defaults;
-- the GORM model mirrors them in BeforeCreate.

BEGIN;

ALTER TABLE context_modules
    ADD COLUMN IF NOT EXISTS requirement_count bigint NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS context_module_progressions (
    id                bigserial PRIMARY KEY,
    context_module_id bigint NOT NULL REFERENCES context_modules(id) ON DELETE CASCADE,
    user_id           bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workflow_state    text   NOT NULL DEFAULT 'locked',
    requirements_met  text   NOT NULL DEFAULT '[]',
    current_position  bigint,
    completed_at      timestamptz,
    evaluated_at      timestamptz,
    created_at        timestamptz,
    updated_at        timestamptz,
    CONSTRAINT context_module_progressions_workflow_state_check
        CHECK (workflow_state IN ('locked', 'unlocked', 'started', 'completed'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_module_progressions_module_user
    ON context_module_progressions (context_module_id, user_id);
CREATE INDEX IF NOT EXISTS idx_module_progressions_user_id
    ON context_module_progressions (user_id);

CREATE TABLE IF NOT EXISTS module_item_user_actions (
    id             bigserial PRIMARY KEY,
    content_tag_id bigint NOT NULL REFERENCES content_tags(id) ON DELETE CASCADE,
    user_id        bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    viewed_at      timestamptz,
    marked_done_at timestamptz,
    created_at     timestamptz,
    updated_at     timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_module_item_user_actions_tag_user
    ON module_item_user_actions (content_tag_id, user_id);
CREATE INDEX IF NOT EXISTS idx_module_item_user_actions_user_id
    ON module_item_user_actions (user_id);

COMMIT;
