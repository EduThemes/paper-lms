-- 000163_planner_and_course_preferences.up.sql
--
-- Learner dashboard lane (G144, G145).
--
-- planner_notes / planner_overrides back the student To-do list. The
-- models existed but the tables were never in the SQL chain, so a
-- production database (AUTO_MIGRATE=false) had no planner storage. IF NOT
-- EXISTS keeps a dev database that AutoMigrate already built working.
--
-- user_course_preferences holds the per-user dashboard customization of a
-- course: favorite, nickname, card color and card position.

CREATE TABLE IF NOT EXISTS planner_notes (
    id bigserial,
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title varchar(255) NOT NULL,
    details text,
    todo_date timestamptz,
    course_id bigint,
    workflow_state varchar(50) DEFAULT 'active'::character varying,
    created_at timestamptz,
    updated_at timestamptz,
    PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS planner_overrides (
    id bigserial,
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plannable_type varchar(50) NOT NULL,
    plannable_id bigint NOT NULL,
    marked_complete boolean DEFAULT false,
    dismissed boolean DEFAULT false,
    created_at timestamptz,
    updated_at timestamptz,
    PRIMARY KEY (id)
);

-- A dev database built by AutoMigrate may hold duplicate overrides for one
-- plannable (the old service raced on find-then-create); keep the newest.
DELETE FROM planner_overrides a
 USING planner_overrides b
 WHERE a.user_id = b.user_id
   AND a.plannable_type = b.plannable_type
   AND a.plannable_id = b.plannable_id
   AND a.id < b.id;

CREATE INDEX IF NOT EXISTS idx_planner_notes_course_id ON public.planner_notes USING btree (course_id);
CREATE INDEX IF NOT EXISTS idx_planner_notes_todo_date ON public.planner_notes USING btree (todo_date);
CREATE INDEX IF NOT EXISTS idx_planner_notes_user_id ON public.planner_notes USING btree (user_id);
CREATE INDEX IF NOT EXISTS idx_planner_overrides_plannable_id ON public.planner_overrides USING btree (plannable_id);
CREATE INDEX IF NOT EXISTS idx_planner_overrides_plannable_type ON public.planner_overrides USING btree (plannable_type);
CREATE INDEX IF NOT EXISTS idx_planner_overrides_user_id ON public.planner_overrides USING btree (user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_planner_overrides_user_plannable ON public.planner_overrides USING btree (user_id, plannable_type, plannable_id);

CREATE TABLE IF NOT EXISTS user_course_preferences (
    id bigserial,
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    course_id bigint NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    favorite boolean NOT NULL DEFAULT false,
    nickname varchar(60),
    color varchar(7),
    position bigint,
    created_at timestamptz,
    updated_at timestamptz,
    PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS idx_user_course_preferences_course_id ON public.user_course_preferences USING btree (course_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_course_prefs_user_course ON public.user_course_preferences USING btree (user_id, course_id);
