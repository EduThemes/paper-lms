-- Paper LMS: Initial schema migration
-- This migration establishes the baseline schema.
--
-- For NEW databases: This creates all tables from scratch.
-- For EXISTING databases (created via AutoMigrate): Run `make migrate-baseline`
--   to mark this migration as already applied without executing it.
--
-- To regenerate this file from your current GORM schema:
--   DATABASE_URL=postgres://... go run ./cmd/genschema > internal/db/migrations/000001_init.up.sql

-- ============================================================================
-- Core tables
-- ============================================================================

CREATE TABLE IF NOT EXISTS accounts (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    name text NOT NULL,
    parent_account_id bigint,
    root_account_id bigint,
    sis_account_id text,
    workflow_state text NOT NULL DEFAULT 'active'
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_sis_account_id ON accounts(sis_account_id);

CREATE TABLE IF NOT EXISTS users (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    name text NOT NULL,
    sortable_name text,
    short_name text,
    login_id text NOT NULL,
    sis_user_id text,
    email text NOT NULL,
    password_hash text NOT NULL,
    avatar_url text,
    role text NOT NULL DEFAULT 'user',
    locale text DEFAULT 'en',
    time_zone text DEFAULT 'America/New_York',
    reset_token text,
    reset_token_expires_at timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_login_id ON users(login_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_sis_user_id ON users(sis_user_id);

CREATE TABLE IF NOT EXISTS enrollment_terms (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    name text NOT NULL,
    start_at timestamptz,
    end_at timestamptz,
    workflow_state text DEFAULT 'active',
    sis_term_id text,
    grading_period_group_id bigint
);

CREATE TABLE IF NOT EXISTS courses (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    name text,
    course_code text,
    workflow_state text DEFAULT 'unpublished',
    account_id bigint,
    enrollment_term_id bigint,
    start_at timestamptz,
    end_at timestamptz,
    is_public boolean DEFAULT false,
    description text,
    syllabus_body text,
    storage_quota bigint,
    sis_course_id text,
    default_view text DEFAULT 'modules',
    grading_standard_enabled boolean DEFAULT false,
    grading_standard_id bigint,
    apply_assignment_group_weights boolean DEFAULT false,
    home_page_type text DEFAULT 'modules',
    ui_mode text DEFAULT 'standard'
);

CREATE TABLE IF NOT EXISTS course_sections (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    course_id bigint,
    name text,
    sis_section_id text,
    start_at timestamptz,
    end_at timestamptz,
    workflow_state text DEFAULT 'active'
);

CREATE TABLE IF NOT EXISTS enrollments (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    user_id bigint,
    course_id bigint,
    course_section_id bigint,
    enrollment_type text,
    workflow_state text DEFAULT 'active',
    role text,
    sis_import_id bigint,
    limit_privileges_to_course_section boolean DEFAULT false
);

-- ============================================================================
-- Modules & Content Tags
-- ============================================================================

CREATE TABLE IF NOT EXISTS context_modules (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    course_id bigint,
    name text,
    position bigint,
    unlock_at timestamptz,
    end_at timestamptz,
    workflow_state text DEFAULT 'active',
    require_sequential_progress boolean DEFAULT false,
    prerequisite_module_ids text
);

CREATE TABLE IF NOT EXISTS content_tags (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    context_module_id bigint,
    content_type text,
    content_id bigint,
    position bigint,
    title text,
    url text,
    indent bigint DEFAULT 0,
    workflow_state text DEFAULT 'active'
);

-- ============================================================================
-- Pages
-- ============================================================================

CREATE TABLE IF NOT EXISTS wiki_pages (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    course_id bigint,
    title text,
    body text,
    workflow_state text DEFAULT 'unpublished',
    editing_roles text DEFAULT 'teachers',
    url text,
    front_page boolean DEFAULT false,
    published boolean DEFAULT false,
    lock_at timestamptz,
    unlock_at timestamptz
);

-- ============================================================================
-- Assignments & Grading
-- ============================================================================

CREATE TABLE IF NOT EXISTS assignment_groups (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    course_id bigint,
    name text,
    position bigint,
    group_weight double precision DEFAULT 0,
    workflow_state text DEFAULT 'active'
);

CREATE TABLE IF NOT EXISTS assignments (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    course_id bigint,
    assignment_group_id bigint,
    name text,
    description text,
    due_at timestamptz,
    unlock_at timestamptz,
    lock_at timestamptz,
    points_possible double precision,
    grading_type text DEFAULT 'points',
    submission_types text DEFAULT 'none',
    published boolean DEFAULT false,
    position bigint,
    workflow_state text DEFAULT 'unpublished',
    peer_reviews boolean DEFAULT false,
    automatic_peer_reviews boolean DEFAULT false,
    grade_group_students_individually boolean DEFAULT false,
    allowed_extensions text,
    turnitin_enabled boolean DEFAULT false,
    muted boolean DEFAULT false,
    omit_from_final_grade boolean DEFAULT false,
    only_visible_to_overrides boolean DEFAULT false,
    post_to_sis boolean DEFAULT false
);

CREATE TABLE IF NOT EXISTS submissions (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    assignment_id bigint,
    user_id bigint,
    submission_type text,
    body text,
    url text,
    score double precision,
    grade text,
    graded_at timestamptz,
    grader_id bigint,
    submitted_at timestamptz,
    attempt bigint DEFAULT 1,
    late boolean DEFAULT false,
    missing boolean DEFAULT false,
    excused boolean DEFAULT false,
    workflow_state text DEFAULT 'unsubmitted',
    attachments text
);

CREATE TABLE IF NOT EXISTS submission_comments (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    submission_id bigint,
    author_id bigint,
    comment text,
    draft boolean DEFAULT false,
    hidden boolean DEFAULT false,
    group_comment_id text
);

CREATE TABLE IF NOT EXISTS grading_standards (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    title text,
    context_type text,
    context_id bigint,
    data text,
    workflow_state text DEFAULT 'active'
);

CREATE TABLE IF NOT EXISTS late_policies (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    course_id bigint UNIQUE,
    late_submission_deduction_enabled boolean DEFAULT false,
    late_submission_deduction double precision DEFAULT 0,
    late_submission_interval text DEFAULT 'day',
    late_submission_minimum_percent_enabled boolean DEFAULT false,
    late_submission_minimum_percent double precision DEFAULT 0,
    missing_submission_deduction_enabled boolean DEFAULT false,
    missing_submission_deduction double precision DEFAULT 0
);

-- ============================================================================
-- Grading Periods
-- ============================================================================

CREATE TABLE IF NOT EXISTS grading_period_groups (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    account_id bigint,
    course_id bigint,
    title text,
    weighted boolean DEFAULT false,
    workflow_state text DEFAULT 'active',
    display_totals_for_all_grading_periods boolean DEFAULT false
);

CREATE TABLE IF NOT EXISTS grading_periods (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    grading_period_group_id bigint,
    title text,
    start_date timestamptz,
    end_date timestamptz,
    close_date timestamptz,
    weight double precision DEFAULT 0,
    is_closed boolean DEFAULT false
);

-- ============================================================================
-- Assignment Overrides
-- ============================================================================

CREATE TABLE IF NOT EXISTS assignment_overrides (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    assignment_id bigint,
    title text,
    set_type text,
    set_id bigint,
    due_at timestamptz,
    unlock_at timestamptz,
    lock_at timestamptz,
    all_day boolean DEFAULT false,
    all_day_date timestamptz
);

CREATE TABLE IF NOT EXISTS assignment_override_students (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    assignment_override_id bigint,
    user_id bigint
);

-- ============================================================================
-- OAuth2 / Developer Keys / Access Tokens
-- ============================================================================

CREATE TABLE IF NOT EXISTS developer_keys (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    name text,
    api_key text,
    redirect_uri text,
    icon_url text,
    notes text,
    email text,
    workflow_state text DEFAULT 'active',
    account_id bigint,
    client_id text,
    client_secret text
);

CREATE TABLE IF NOT EXISTS access_tokens (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    developer_key_id bigint,
    user_id bigint,
    token text,
    refresh_token text,
    purpose text,
    scopes text,
    last_used_at timestamptz,
    expires_at timestamptz,
    code text,
    code_expires_at timestamptz
);

-- ============================================================================
-- LTI 1.3
-- ============================================================================

CREATE TABLE IF NOT EXISTS lti_tool_configurations (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    developer_key_id bigint,
    settings text,
    disabled boolean DEFAULT false,
    target_link_uri text,
    oidc_initiation_url text,
    public_jwk_url text,
    public_jwk text,
    custom_fields text,
    domain text,
    description text,
    privacy_level text DEFAULT 'anonymous',
    placements text
);

CREATE TABLE IF NOT EXISTS context_external_tools (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    name text,
    url text,
    domain text,
    consumer_key text,
    shared_secret text,
    description text,
    context_type text,
    context_id bigint,
    workflow_state text DEFAULT 'active',
    developer_key_id bigint,
    settings text
);

CREATE TABLE IF NOT EXISTS lti_resource_links (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    context_external_tool_id bigint,
    resource_link_id text,
    context_type text,
    context_id bigint,
    custom text,
    title text,
    description text,
    url text
);

CREATE TABLE IF NOT EXISTS lti_line_items (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    context_external_tool_id bigint,
    course_id bigint,
    assignment_id bigint,
    label text,
    score_maximum double precision,
    tag text,
    resource_id text,
    resource_link_id bigint
);

CREATE TABLE IF NOT EXISTS lti_results (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    line_item_id bigint,
    user_id bigint,
    result_score double precision,
    result_maximum double precision,
    comment text,
    activity_progress text,
    grading_progress text
);

CREATE TABLE IF NOT EXISTS nonces (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    nonce text,
    expires_at timestamptz
);

-- ============================================================================
-- Discussions
-- ============================================================================

CREATE TABLE IF NOT EXISTS discussion_topics (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    course_id bigint,
    title text,
    message text,
    discussion_type text DEFAULT 'side_comment',
    user_id bigint,
    workflow_state text DEFAULT 'active',
    published boolean DEFAULT true,
    position bigint,
    pinned boolean DEFAULT false,
    locked boolean DEFAULT false,
    allow_rating boolean DEFAULT false,
    only_graders_can_rate boolean DEFAULT false,
    require_initial_post boolean DEFAULT false,
    delayed_post_at timestamptz,
    lock_at timestamptz
);

CREATE TABLE IF NOT EXISTS discussion_entries (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    discussion_topic_id bigint,
    user_id bigint,
    parent_id bigint,
    message text,
    workflow_state text DEFAULT 'active',
    depth bigint DEFAULT 0,
    rating_count bigint DEFAULT 0,
    rating_sum bigint DEFAULT 0
);

CREATE TABLE IF NOT EXISTS discussion_entry_ratings (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    discussion_entry_id bigint,
    user_id bigint,
    rating bigint DEFAULT 0
);

CREATE TABLE IF NOT EXISTS discussion_entry_participants (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    discussion_entry_id bigint,
    user_id bigint,
    read boolean DEFAULT false,
    forced_read_state boolean DEFAULT false
);

CREATE TABLE IF NOT EXISTS discussion_topic_participants (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    discussion_topic_id bigint,
    user_id bigint,
    subscribed boolean DEFAULT true,
    unread_entry_count bigint DEFAULT 0
);

CREATE TABLE IF NOT EXISTS discussion_entry_versions (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    discussion_entry_id bigint,
    user_id bigint,
    message text,
    version bigint DEFAULT 1
);

-- ============================================================================
-- Files & Folders
-- ============================================================================

CREATE TABLE IF NOT EXISTS folders (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    context_type text,
    context_id bigint,
    name text,
    full_name text,
    parent_folder_id bigint,
    workflow_state text DEFAULT 'visible',
    position bigint,
    locked boolean DEFAULT false,
    lock_at timestamptz,
    unlock_at timestamptz,
    hidden boolean DEFAULT false
);

CREATE TABLE IF NOT EXISTS attachments (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    folder_id bigint,
    user_id bigint,
    display_name text,
    filename text,
    content_type text,
    size bigint,
    url text,
    workflow_state text DEFAULT 'active',
    locked boolean DEFAULT false,
    lock_at timestamptz,
    unlock_at timestamptz,
    hidden boolean DEFAULT false,
    context_type text,
    context_id bigint
);

-- ============================================================================
-- SIS Import
-- ============================================================================

CREATE TABLE IF NOT EXISTS sis_batches (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    account_id bigint,
    workflow_state text DEFAULT 'created',
    progress bigint DEFAULT 0,
    data text,
    batch_mode boolean DEFAULT false,
    started_at timestamptz,
    ended_at timestamptz,
    diffing_data_set_identifier text,
    created_count bigint DEFAULT 0,
    updated_count bigint DEFAULT 0,
    deleted_count bigint DEFAULT 0
);

CREATE TABLE IF NOT EXISTS sis_batch_errors (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    sis_batch_id bigint,
    row_number bigint,
    message text,
    file_name text
);

-- ============================================================================
-- Quizzes
-- ============================================================================

CREATE TABLE IF NOT EXISTS quizzes (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    course_id bigint,
    title text,
    description text,
    quiz_type text DEFAULT 'assignment',
    assignment_group_id bigint,
    time_limit bigint,
    shuffle_answers boolean DEFAULT false,
    show_correct_answers boolean DEFAULT true,
    show_correct_answers_at timestamptz,
    hide_correct_answers_at timestamptz,
    allowed_attempts bigint DEFAULT 1,
    scoring_policy text DEFAULT 'keep_highest',
    one_question_at_a_time boolean DEFAULT false,
    cant_go_back boolean DEFAULT false,
    access_code text,
    ip_filter text,
    due_at timestamptz,
    lock_at timestamptz,
    unlock_at timestamptz,
    published boolean DEFAULT false,
    points_possible double precision DEFAULT 0,
    workflow_state text DEFAULT 'unpublished',
    assignment_id bigint
);

CREATE TABLE IF NOT EXISTS quiz_questions (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    quiz_id bigint,
    position bigint,
    question_type text DEFAULT 'multiple_choice_question',
    question_text text,
    points_possible double precision DEFAULT 1,
    answers text,
    correct_comments text,
    incorrect_comments text,
    neutral_comments text,
    question_name text
);

CREATE TABLE IF NOT EXISTS quiz_submissions (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    quiz_id bigint,
    user_id bigint,
    submission_id bigint,
    started_at timestamptz,
    finished_at timestamptz,
    end_at timestamptz,
    attempt bigint DEFAULT 1,
    score double precision,
    kept_score double precision,
    fudge_points double precision DEFAULT 0,
    workflow_state text DEFAULT 'untaken',
    time_spent bigint DEFAULT 0,
    extra_attempts bigint DEFAULT 0,
    extra_time bigint DEFAULT 0,
    manually_unlocked boolean DEFAULT false
);

CREATE TABLE IF NOT EXISTS quiz_submission_answers (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    quiz_submission_id bigint,
    quiz_question_id bigint,
    answer text,
    correct boolean DEFAULT false,
    points double precision DEFAULT 0
);

-- ============================================================================
-- Rubrics
-- ============================================================================

CREATE TABLE IF NOT EXISTS rubrics (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    title text,
    context_type text,
    context_id bigint,
    points_possible double precision,
    data text,
    workflow_state text DEFAULT 'active',
    free_form_criterion_comments boolean DEFAULT false,
    hide_score_total boolean DEFAULT false
);

CREATE TABLE IF NOT EXISTS rubric_associations (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    rubric_id bigint,
    association_type text,
    association_id bigint,
    use_for_grading boolean DEFAULT false,
    hide_score_total boolean DEFAULT false,
    hide_points boolean DEFAULT false,
    hide_outcome_results boolean DEFAULT false,
    purpose text DEFAULT 'grading'
);

CREATE TABLE IF NOT EXISTS rubric_assessments (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    rubric_id bigint,
    rubric_association_id bigint,
    user_id bigint,
    assessor_id bigint,
    artifact_type text,
    artifact_id bigint,
    assessment_type text DEFAULT 'grading',
    data text,
    score double precision,
    comments text
);

-- ============================================================================
-- Calendar & Events
-- ============================================================================

CREATE TABLE IF NOT EXISTS calendar_events (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    title text,
    description text,
    context_type text,
    context_id bigint,
    start_at timestamptz,
    end_at timestamptz,
    all_day boolean DEFAULT false,
    all_day_date timestamptz,
    workflow_state text DEFAULT 'active',
    location_name text,
    location_address text,
    user_id bigint
);

-- ============================================================================
-- Conversations / Inbox
-- ============================================================================

CREATE TABLE IF NOT EXISTS conversations (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    subject text,
    workflow_state text DEFAULT 'active',
    context_type text,
    context_id bigint,
    last_message_at timestamptz,
    message_count bigint DEFAULT 0
);

CREATE TABLE IF NOT EXISTS conversation_participants (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    conversation_id bigint,
    user_id bigint,
    workflow_state text DEFAULT 'active',
    last_message_at timestamptz
);

CREATE TABLE IF NOT EXISTS conversation_messages (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    conversation_id bigint,
    author_id bigint,
    body text,
    generated boolean DEFAULT false,
    forwarded_message_ids text
);

-- ============================================================================
-- Notifications
-- ============================================================================

CREATE TABLE IF NOT EXISTS notification_preferences (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    user_id bigint,
    notification_category text,
    frequency text DEFAULT 'immediately'
);

CREATE TABLE IF NOT EXISTS notifications (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    user_id bigint,
    notification_type text,
    category text,
    subject text,
    message text,
    url text,
    context_type text,
    context_id bigint,
    read boolean DEFAULT false,
    sent_at timestamptz
);

CREATE TABLE IF NOT EXISTS communication_channels (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    user_id bigint,
    path text,
    path_type text DEFAULT 'email',
    position bigint DEFAULT 0,
    workflow_state text DEFAULT 'active',
    confirmation_code text,
    bounce_count bigint DEFAULT 0,
    last_bounce_at timestamptz,
    last_suppression_bounce_at timestamptz
);

CREATE TABLE IF NOT EXISTS notification_deliveries (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    notification_id bigint,
    communication_channel_id bigint,
    user_id bigint,
    delivery_type text DEFAULT 'email',
    workflow_state text DEFAULT 'pending',
    sent_at timestamptz,
    error_message text,
    retry_count bigint DEFAULT 0,
    next_retry_at timestamptz,
    digest_batch_id text
);

-- ============================================================================
-- Content Migration
-- ============================================================================

CREATE TABLE IF NOT EXISTS content_migrations (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    course_id bigint,
    user_id bigint,
    migration_type text,
    workflow_state text DEFAULT 'created',
    progress bigint DEFAULT 0,
    migration_settings text,
    started_at timestamptz,
    finished_at timestamptz,
    source_course_id bigint,
    error_message text
);

-- ============================================================================
-- Learning Outcomes
-- ============================================================================

CREATE TABLE IF NOT EXISTS learning_outcome_groups (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    context_type text,
    context_id bigint,
    title text,
    description text,
    parent_outcome_group_id bigint,
    workflow_state text DEFAULT 'active',
    vendor_guid text
);

CREATE TABLE IF NOT EXISTS learning_outcomes (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    context_type text,
    context_id bigint,
    learning_outcome_group_id bigint,
    title text,
    description text,
    vendor_guid text,
    display_name text,
    calculation_method text DEFAULT 'decaying_average',
    calculation_int bigint DEFAULT 65,
    mastery_points double precision DEFAULT 3,
    points_possible double precision DEFAULT 5,
    ratings text,
    workflow_state text DEFAULT 'active'
);

CREATE TABLE IF NOT EXISTS learning_outcome_results (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    learning_outcome_id bigint,
    user_id bigint,
    context_type text,
    context_id bigint,
    artifact_type text,
    artifact_id bigint,
    associated_asset_type text,
    associated_asset_id bigint,
    score double precision,
    possible double precision,
    mastery boolean DEFAULT false,
    percent double precision,
    attempt bigint DEFAULT 1,
    assessed_at timestamptz,
    title text
);

CREATE TABLE IF NOT EXISTS outcome_alignments (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    learning_outcome_id bigint NOT NULL,
    content_type text NOT NULL,
    content_id bigint NOT NULL,
    context_type text,
    context_id bigint
);

-- ============================================================================
-- Groups
-- ============================================================================

CREATE TABLE IF NOT EXISTS group_categories (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    context_type text,
    context_id bigint,
    name text,
    self_signup text,
    auto_leader text,
    group_limit bigint,
    workflow_state text DEFAULT 'active'
);

CREATE TABLE IF NOT EXISTS groups (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    group_category_id bigint,
    name text,
    description text,
    max_membership bigint,
    is_public boolean DEFAULT false,
    join_level text DEFAULT 'invitation_only',
    workflow_state text DEFAULT 'available',
    context_type text,
    context_id bigint
);

CREATE TABLE IF NOT EXISTS group_memberships (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    group_id bigint,
    user_id bigint,
    workflow_state text DEFAULT 'accepted',
    moderator boolean DEFAULT false
);

-- ============================================================================
-- Blueprint Courses
-- ============================================================================

CREATE TABLE IF NOT EXISTS blueprint_templates (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    course_id bigint,
    default_restrictions text,
    use_default_restrictions_by_type boolean DEFAULT false,
    restrictions_by_type text,
    workflow_state text DEFAULT 'active'
);

CREATE TABLE IF NOT EXISTS blueprint_subscriptions (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    blueprint_template_id bigint,
    child_course_id bigint,
    workflow_state text DEFAULT 'active'
);

CREATE TABLE IF NOT EXISTS blueprint_migrations (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    blueprint_template_id bigint,
    user_id bigint,
    workflow_state text DEFAULT 'queued',
    comment text,
    export_settings text,
    imports_status text,
    started_at timestamptz,
    completed_at timestamptz
);

-- ============================================================================
-- Course Pacing
-- ============================================================================

CREATE TABLE IF NOT EXISTS course_paces (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    course_id bigint,
    course_section_id bigint,
    user_id bigint,
    workflow_state text DEFAULT 'unpublished',
    exclude_weekends boolean DEFAULT true,
    hard_end_dates boolean DEFAULT false,
    end_date timestamptz,
    published_at timestamptz
);

CREATE TABLE IF NOT EXISTS course_pace_module_items (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    course_pace_id bigint,
    module_item_id bigint,
    duration bigint DEFAULT 0
);

-- ============================================================================
-- Collaborations & Conferences
-- ============================================================================

CREATE TABLE IF NOT EXISTS collaborations (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    collaboration_type text,
    document_id text,
    title text,
    description text,
    url text,
    context_type text,
    context_id bigint,
    user_id bigint,
    workflow_state text DEFAULT 'active'
);

CREATE TABLE IF NOT EXISTS conferences (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    title text,
    description text,
    conference_type text,
    conference_key text,
    context_type text,
    context_id bigint,
    user_id bigint,
    duration bigint,
    started_at timestamptz,
    ended_at timestamptz,
    join_url text,
    recording_url text,
    workflow_state text DEFAULT 'active',
    long_running boolean DEFAULT false
);

CREATE TABLE IF NOT EXISTS conference_participants (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    conference_id bigint,
    user_id bigint,
    participation_type text DEFAULT 'attendee'
);

-- ============================================================================
-- Analytics
-- ============================================================================

CREATE TABLE IF NOT EXISTS page_views (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    user_id bigint,
    context_type text,
    context_id bigint,
    url text,
    user_agent text,
    http_method text,
    remote_ip text,
    interaction_seconds double precision DEFAULT 0,
    participated boolean DEFAULT false,
    asset_type text,
    asset_id bigint
);

-- ============================================================================
-- Authentication Providers
-- ============================================================================

CREATE TABLE IF NOT EXISTS authentication_providers (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    account_id bigint,
    auth_type text NOT NULL,
    position bigint DEFAULT 0,
    settings text,
    workflow_state text DEFAULT 'active',
    idp_entity_id text,
    log_in_url text,
    log_out_url text,
    certificate_fingerprint text,
    requested_authn_context text,
    metadata_uri text,
    auth_host text,
    auth_port bigint,
    auth_base text,
    auth_filter text,
    auth_username text,
    auth_password text,
    auth_over_tls text DEFAULT 'simple_tls',
    identifier_format text DEFAULT 'urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress',
    jit_provisioning boolean DEFAULT false
);

-- ============================================================================
-- Announcements
-- ============================================================================

CREATE TABLE IF NOT EXISTS announcements (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    context_type text DEFAULT 'Course',
    context_id bigint,
    title text NOT NULL,
    message text,
    user_id bigint,
    position bigint,
    is_section_specific boolean DEFAULT false,
    workflow_state text DEFAULT 'active',
    posted_at timestamptz,
    delayed_post_at timestamptz,
    locked boolean DEFAULT false,
    require_acknowledgement boolean DEFAULT false
);

CREATE TABLE IF NOT EXISTS announcement_read_receipts (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    announcement_id bigint,
    user_id bigint,
    read boolean DEFAULT false,
    read_at timestamptz,
    acknowledged boolean DEFAULT false,
    acknowledged_at timestamptz
);

-- ============================================================================
-- Audit Logs
-- ============================================================================

CREATE TABLE IF NOT EXISTS audit_logs (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    event_type text NOT NULL,
    user_id bigint,
    course_id bigint,
    context_type text,
    context_id bigint,
    data text,
    ip_address text
);

CREATE TABLE IF NOT EXISTS grade_change_logs (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    course_id bigint NOT NULL,
    assignment_id bigint NOT NULL,
    student_id bigint NOT NULL,
    grader_id bigint NOT NULL,
    old_grade text,
    new_grade text,
    old_score double precision,
    new_score double precision,
    excused_before boolean DEFAULT false,
    excused_after boolean DEFAULT false,
    graded_anonymously boolean DEFAULT false
);

-- ============================================================================
-- Custom Roles & Permissions
-- ============================================================================

CREATE TABLE IF NOT EXISTS custom_roles (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    name text NOT NULL,
    base_role_type text NOT NULL,
    account_id bigint,
    workflow_state text DEFAULT 'active',
    label text
);

CREATE TABLE IF NOT EXISTS role_overrides (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    custom_role_id bigint NOT NULL,
    permission text NOT NULL,
    enabled boolean DEFAULT false,
    locked boolean DEFAULT false
);

-- ============================================================================
-- OneRoster
-- ============================================================================

CREATE TABLE IF NOT EXISTS one_roster_connections (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    account_id bigint,
    name text NOT NULL,
    base_url text NOT NULL,
    client_id text NOT NULL,
    client_secret text NOT NULL,
    token_url text,
    workflow_state text DEFAULT 'active',
    last_sync_at timestamptz,
    sync_status text DEFAULT 'never'
);

CREATE TABLE IF NOT EXISTS one_roster_sync_logs (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    connection_id bigint NOT NULL,
    sync_type text,
    workflow_state text DEFAULT 'running',
    started_at timestamptz,
    finished_at timestamptz,
    created_count bigint DEFAULT 0,
    updated_count bigint DEFAULT 0,
    error_count bigint DEFAULT 0,
    errors text
);

-- ============================================================================
-- Document Annotations
-- ============================================================================

CREATE TABLE IF NOT EXISTS document_annotations (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    submission_id bigint NOT NULL,
    attachment_id bigint,
    user_id bigint NOT NULL,
    annotation_type text NOT NULL,
    page bigint DEFAULT 1,
    x double precision DEFAULT 0,
    y double precision DEFAULT 0,
    width double precision DEFAULT 0,
    height double precision DEFAULT 0,
    color text DEFAULT '#FFFF00',
    content text,
    parent_id bigint,
    resolved boolean DEFAULT false,
    resolved_by bigint,
    resolved_at timestamptz,
    stroke_data text
);

-- ============================================================================
-- COPPA / FERPA Compliance
-- ============================================================================

CREATE TABLE IF NOT EXISTS parental_consents (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    student_id bigint NOT NULL,
    parent_email text NOT NULL,
    consent_type text NOT NULL,
    workflow_state text DEFAULT 'pending',
    granted_at timestamptz,
    revoked_at timestamptz,
    ip_address text,
    verification_code text,
    verification_expires_at timestamptz
);

CREATE TABLE IF NOT EXISTS data_processing_agreements (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    account_id bigint NOT NULL,
    vendor_name text NOT NULL,
    vendor_contact text,
    purpose text,
    data_categories text,
    workflow_state text DEFAULT 'active',
    effective_date timestamptz,
    expiration_date timestamptz,
    signed_by bigint,
    signed_at timestamptz,
    document_url text
);

CREATE TABLE IF NOT EXISTS age_verifications (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    user_id bigint NOT NULL,
    date_of_birth timestamptz,
    is_under_13 boolean DEFAULT false,
    verified_at timestamptz,
    verification_method text,
    ip_address text
);

CREATE TABLE IF NOT EXISTS data_retention_policies (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    account_id bigint NOT NULL,
    data_type text NOT NULL,
    retention_period_days bigint NOT NULL,
    action_after_retention text DEFAULT 'anonymize',
    workflow_state text DEFAULT 'active',
    last_applied_at timestamptz
);

CREATE TABLE IF NOT EXISTS data_deletion_requests (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    user_id bigint NOT NULL,
    requested_by bigint NOT NULL,
    reason text,
    workflow_state text DEFAULT 'pending',
    completed_at timestamptz,
    data_types text
);

CREATE TABLE IF NOT EXISTS data_export_requests (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    user_id bigint NOT NULL,
    requested_by bigint NOT NULL,
    export_format text DEFAULT 'json',
    workflow_state text DEFAULT 'pending',
    completed_at timestamptz,
    download_url text,
    expires_at timestamptz
);

CREATE TABLE IF NOT EXISTS pii_access_logs (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    user_id bigint NOT NULL,
    accessed_by bigint NOT NULL,
    access_type text NOT NULL,
    data_accessed text,
    purpose text,
    ip_address text
);

-- ============================================================================
-- Student Accommodations
-- ============================================================================

CREATE TABLE IF NOT EXISTS student_accommodations (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    user_id bigint NOT NULL,
    course_id bigint,
    accommodation_type text NOT NULL,
    details text,
    extended_time_multiplier double precision DEFAULT 1.0,
    workflow_state text DEFAULT 'active',
    approved_by bigint,
    approved_at timestamptz,
    expires_at timestamptz,
    notes text
);

CREATE TABLE IF NOT EXISTS accommodation_applications (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    student_accommodation_id bigint NOT NULL,
    assignment_id bigint,
    quiz_id bigint,
    applied boolean DEFAULT false,
    applied_at timestamptz,
    original_due_at timestamptz,
    extended_due_at timestamptz,
    original_time_limit bigint,
    extended_time_limit bigint
);

-- ============================================================================
-- Attendance
-- ============================================================================

CREATE TABLE IF NOT EXISTS attendance_records (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    course_id bigint NOT NULL,
    user_id bigint NOT NULL,
    date timestamptz NOT NULL,
    status text NOT NULL DEFAULT 'present',
    notes text,
    recorded_by bigint
);

-- ============================================================================
-- Portfolios
-- ============================================================================

CREATE TABLE IF NOT EXISTS portfolios (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    user_id bigint NOT NULL,
    title text NOT NULL,
    description text,
    workflow_state text DEFAULT 'active',
    visibility text DEFAULT 'private',
    template_id bigint,
    published_at timestamptz,
    share_token text
);

CREATE TABLE IF NOT EXISTS portfolio_sections (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    portfolio_id bigint NOT NULL,
    title text NOT NULL,
    description text,
    position bigint DEFAULT 0,
    section_type text DEFAULT 'custom',
    layout text DEFAULT 'list'
);

CREATE TABLE IF NOT EXISTS portfolio_artifacts (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    portfolio_section_id bigint NOT NULL,
    artifact_type text NOT NULL,
    title text,
    description text,
    content text,
    position bigint DEFAULT 0,
    submission_id bigint,
    attachment_id bigint,
    url text,
    metadata text
);

CREATE TABLE IF NOT EXISTS portfolio_reflections (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    portfolio_artifact_id bigint NOT NULL,
    user_id bigint NOT NULL,
    content text NOT NULL,
    reflection_type text DEFAULT 'text',
    metadata text
);

CREATE TABLE IF NOT EXISTS portfolio_templates (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    title text NOT NULL,
    description text,
    structure text,
    account_id bigint,
    workflow_state text DEFAULT 'active'
);

CREATE TABLE IF NOT EXISTS portfolio_comments (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    portfolio_id bigint NOT NULL,
    user_id bigint NOT NULL,
    comment text NOT NULL,
    parent_id bigint
);

-- ============================================================================
-- Course Home Engine
-- ============================================================================

CREATE TABLE IF NOT EXISTS course_home_buttons (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    course_id bigint NOT NULL,
    label text NOT NULL,
    icon text DEFAULT 'book-open',
    color text DEFAULT '#3B82F6',
    link_type text DEFAULT 'page',
    link_target text,
    position bigint DEFAULT 0,
    visible boolean DEFAULT true
);

CREATE TABLE IF NOT EXISTS todays_lesson_overrides (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    course_id bigint NOT NULL,
    date date NOT NULL,
    module_id bigint NOT NULL
);

CREATE TABLE IF NOT EXISTS course_visits (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    user_id bigint NOT NULL,
    course_id bigint NOT NULL,
    last_module_id bigint,
    last_module_item_id bigint,
    last_page_url text,
    last_visited_at timestamptz
);

-- ============================================================================
-- Indexes for frequently queried columns
-- ============================================================================

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_login_id ON users(login_id);
CREATE INDEX IF NOT EXISTS idx_users_reset_token ON users(reset_token);
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);

CREATE INDEX IF NOT EXISTS idx_courses_account_id ON courses(account_id);
CREATE INDEX IF NOT EXISTS idx_courses_workflow_state ON courses(workflow_state);
CREATE INDEX IF NOT EXISTS idx_courses_deleted_at ON courses(deleted_at);

CREATE INDEX IF NOT EXISTS idx_enrollments_user_id ON enrollments(user_id);
CREATE INDEX IF NOT EXISTS idx_enrollments_course_id ON enrollments(course_id);
CREATE INDEX IF NOT EXISTS idx_enrollments_workflow_state ON enrollments(workflow_state);
CREATE INDEX IF NOT EXISTS idx_enrollments_deleted_at ON enrollments(deleted_at);

CREATE INDEX IF NOT EXISTS idx_assignments_course_id ON assignments(course_id);
CREATE INDEX IF NOT EXISTS idx_assignments_assignment_group_id ON assignments(assignment_group_id);
CREATE INDEX IF NOT EXISTS idx_assignments_workflow_state ON assignments(workflow_state);
CREATE INDEX IF NOT EXISTS idx_assignments_deleted_at ON assignments(deleted_at);

CREATE INDEX IF NOT EXISTS idx_submissions_assignment_id ON submissions(assignment_id);
CREATE INDEX IF NOT EXISTS idx_submissions_user_id ON submissions(user_id);
CREATE INDEX IF NOT EXISTS idx_submissions_workflow_state ON submissions(workflow_state);
CREATE INDEX IF NOT EXISTS idx_submissions_deleted_at ON submissions(deleted_at);

CREATE INDEX IF NOT EXISTS idx_submission_comments_submission_id ON submission_comments(submission_id);

CREATE INDEX IF NOT EXISTS idx_discussion_topics_course_id ON discussion_topics(course_id);
CREATE INDEX IF NOT EXISTS idx_discussion_entries_topic_id ON discussion_entries(discussion_topic_id);
CREATE INDEX IF NOT EXISTS idx_discussion_entries_user_id ON discussion_entries(user_id);
CREATE INDEX IF NOT EXISTS idx_discussion_entries_workflow_state ON discussion_entries(workflow_state);

CREATE INDEX IF NOT EXISTS idx_wiki_pages_course_id ON wiki_pages(course_id);
CREATE INDEX IF NOT EXISTS idx_wiki_pages_workflow_state ON wiki_pages(workflow_state);

CREATE INDEX IF NOT EXISTS idx_modules_course_id ON context_modules(course_id);
CREATE INDEX IF NOT EXISTS idx_content_tags_module_id ON content_tags(context_module_id);

CREATE INDEX IF NOT EXISTS idx_quizzes_course_id ON quizzes(course_id);
CREATE INDEX IF NOT EXISTS idx_quiz_questions_quiz_id ON quiz_questions(quiz_id);
CREATE INDEX IF NOT EXISTS idx_quiz_submissions_quiz_id ON quiz_submissions(quiz_id);
CREATE INDEX IF NOT EXISTS idx_quiz_submissions_user_id ON quiz_submissions(user_id);
CREATE INDEX IF NOT EXISTS idx_quiz_submissions_workflow_state ON quiz_submissions(workflow_state);
CREATE INDEX IF NOT EXISTS idx_quiz_submission_answers_submission_id ON quiz_submission_answers(quiz_submission_id);

CREATE INDEX IF NOT EXISTS idx_folders_context ON folders(context_type, context_id);
CREATE INDEX IF NOT EXISTS idx_attachments_folder_id ON attachments(folder_id);

CREATE INDEX IF NOT EXISTS idx_calendar_events_context ON calendar_events(context_type, context_id);
CREATE INDEX IF NOT EXISTS idx_calendar_events_user_id ON calendar_events(user_id);

CREATE INDEX IF NOT EXISTS idx_conversations_last_message ON conversations(last_message_at);
CREATE INDEX IF NOT EXISTS idx_conversation_participants_user_id ON conversation_participants(user_id);
CREATE INDEX IF NOT EXISTS idx_conversation_participants_convo_id ON conversation_participants(conversation_id);
CREATE INDEX IF NOT EXISTS idx_conversation_messages_convo_id ON conversation_messages(conversation_id);

CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notification_preferences_user_id ON notification_preferences(user_id);
CREATE INDEX IF NOT EXISTS idx_notification_deliveries_user_id ON notification_deliveries(user_id);

CREATE INDEX IF NOT EXISTS idx_page_views_user_id ON page_views(user_id);
CREATE INDEX IF NOT EXISTS idx_page_views_context ON page_views(context_type, context_id);

CREATE INDEX IF NOT EXISTS idx_audit_logs_course_id ON audit_logs(course_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_grade_change_logs_course_id ON grade_change_logs(course_id);
CREATE INDEX IF NOT EXISTS idx_grade_change_logs_student_id ON grade_change_logs(student_id);

CREATE INDEX IF NOT EXISTS idx_announcement_receipts_announcement_id ON announcement_read_receipts(announcement_id);
CREATE INDEX IF NOT EXISTS idx_announcement_receipts_user_id ON announcement_read_receipts(user_id);

CREATE INDEX IF NOT EXISTS idx_attendance_course_date ON attendance_records(course_id, date);
CREATE INDEX IF NOT EXISTS idx_attendance_user_id ON attendance_records(user_id);

CREATE INDEX IF NOT EXISTS idx_course_visits_user_course ON course_visits(user_id, course_id);
CREATE INDEX IF NOT EXISTS idx_course_home_buttons_course_id ON course_home_buttons(course_id);
CREATE INDEX IF NOT EXISTS idx_todays_lesson_course_date ON todays_lesson_overrides(course_id, date);

CREATE INDEX IF NOT EXISTS idx_outcome_alignments_outcome ON outcome_alignments(learning_outcome_id);
CREATE INDEX IF NOT EXISTS idx_outcome_alignments_content ON outcome_alignments(content_type, content_id);

CREATE INDEX IF NOT EXISTS idx_portfolios_user_id ON portfolios(user_id);
CREATE INDEX IF NOT EXISTS idx_portfolio_sections_portfolio_id ON portfolio_sections(portfolio_id);
CREATE INDEX IF NOT EXISTS idx_portfolio_artifacts_section_id ON portfolio_artifacts(portfolio_section_id);
