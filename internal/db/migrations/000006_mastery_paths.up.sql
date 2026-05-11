-- Mastery Paths (Conditional Release) — Canvas-compatible schema.
-- See canvas-lms-master/app/models/conditional_release/ for the reference.

CREATE TABLE IF NOT EXISTS conditional_release_rules (
    id                    bigserial PRIMARY KEY,
    course_id             bigint NOT NULL,
    trigger_assignment_id bigint NOT NULL,
    workflow_state        text   NOT NULL DEFAULT 'active',
    created_at            timestamptz,
    updated_at            timestamptz
);
CREATE INDEX IF NOT EXISTS idx_cr_rules_course_id      ON conditional_release_rules(course_id);
CREATE INDEX IF NOT EXISTS idx_cr_rules_workflow_state ON conditional_release_rules(workflow_state);
CREATE UNIQUE INDEX IF NOT EXISTS idx_cr_rule_trigger
    ON conditional_release_rules(trigger_assignment_id)
    WHERE workflow_state = 'active';

CREATE TABLE IF NOT EXISTS conditional_release_scoring_ranges (
    id              bigserial PRIMARY KEY,
    rule_id         bigint NOT NULL REFERENCES conditional_release_rules(id) ON DELETE CASCADE,
    lower_bound     double precision NOT NULL DEFAULT 0,
    upper_bound     double precision NOT NULL DEFAULT 1,
    position        integer NOT NULL DEFAULT 0,
    workflow_state  text NOT NULL DEFAULT 'active',
    created_at      timestamptz,
    updated_at      timestamptz
);
CREATE INDEX IF NOT EXISTS idx_cr_sr_rule_id ON conditional_release_scoring_ranges(rule_id);

CREATE TABLE IF NOT EXISTS conditional_release_assignment_sets (
    id               bigserial PRIMARY KEY,
    scoring_range_id bigint NOT NULL REFERENCES conditional_release_scoring_ranges(id) ON DELETE CASCADE,
    position         integer NOT NULL DEFAULT 0,
    workflow_state   text NOT NULL DEFAULT 'active',
    created_at       timestamptz,
    updated_at       timestamptz
);
CREATE INDEX IF NOT EXISTS idx_cr_set_range_id ON conditional_release_assignment_sets(scoring_range_id);

CREATE TABLE IF NOT EXISTS conditional_release_assignment_set_associations (
    id              bigserial PRIMARY KEY,
    set_id          bigint NOT NULL REFERENCES conditional_release_assignment_sets(id) ON DELETE CASCADE,
    assignment_id   bigint NOT NULL,
    position        integer NOT NULL DEFAULT 0,
    workflow_state  text NOT NULL DEFAULT 'active',
    created_at      timestamptz,
    updated_at      timestamptz
);
CREATE INDEX IF NOT EXISTS idx_cr_assoc_set_id        ON conditional_release_assignment_set_associations(set_id);
CREATE INDEX IF NOT EXISTS idx_cr_assoc_assignment_id ON conditional_release_assignment_set_associations(assignment_id);

CREATE TABLE IF NOT EXISTS conditional_release_assignment_set_actions (
    id           bigserial PRIMARY KEY,
    set_id       bigint NOT NULL REFERENCES conditional_release_assignment_sets(id) ON DELETE CASCADE,
    student_id   bigint NOT NULL,
    action_type  text   NOT NULL DEFAULT 'assigned',
    acted_at     timestamptz NOT NULL DEFAULT NOW(),
    source       text,
    created_at   timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_cr_action_unique
    ON conditional_release_assignment_set_actions(set_id, student_id, action_type);
CREATE INDEX IF NOT EXISTS idx_cr_action_student      ON conditional_release_assignment_set_actions(student_id);
