-- 000016_backfill_missing_tables.up.sql
--
-- Schema parity backfill. Brings the SQL migration chain into full
-- agreement with GORM's AutoMigrate output so production deploys can run
-- with AUTO_MIGRATE=false. Before this migration, a fresh prod install
-- would crash on first boot when the seeder tried to INSERT columns
-- (e.g. accounts.max_upload_size_mb) that the SQL chain never created.
--
-- Contents:
--   * 5  CREATE TABLE   — tables AutoMigrate created that were never
--                         added to the migration chain (peer_reviews,
--                         module_prerequisites, question_banks,
--                         question_bank_entries, quiz_question_groups)
--   * 270 ALTER TABLE   — columns added to existing tables after their
--                         initial migration was authored. Across 78
--                         tables. Every column is added with IF NOT
--                         EXISTS so this is safe on databases that have
--                         been kept in sync via AutoMigrate.
--   * 174 CREATE INDEX  — GORM-emitted btree indexes on FK and common
--                         query columns. 133 were always safe to create;
--                         41 became safe once the columns above exist.
--
-- Re-runnable: every statement uses IF NOT EXISTS.
-- Regenerate with: DATABASE_URL=... go run ./cmd/schemadiff --emit-sql
--
-- Stale columns (345 across 97 tables) are *not* dropped here. Those are
-- usually leftovers from model refactors that bypassed migrations. They
-- may hold production data and need per-column human judgment to drop.
-- Run `make schema-diff` to see the full list. Cleanup is tracked
-- separately and is intentionally out of scope for this migration.

-- === Section: content ===

CREATE TABLE IF NOT EXISTS module_prerequisites (
    id bigserial,
    module_id bigint NOT NULL,
    prerequisite_module_id bigint NOT NULL,
    PRIMARY KEY (id)
);

-- === Section: assignments ===

CREATE TABLE IF NOT EXISTS peer_reviews (
    id bigserial,
    assignment_id bigint,
    submission_id bigint,
    reviewer_id bigint,
    reviewee_id bigint,
    workflow_state text DEFAULT 'assigned'::text,
    score numeric,
    comments text,
    created_at timestamptz,
    updated_at timestamptz,
    PRIMARY KEY (id)
);

-- === Section: quizzes ===

CREATE TABLE IF NOT EXISTS question_bank_entries (
    id bigserial,
    question_bank_id bigint,
    question_name text,
    question_type text,
    question_text text,
    points_possible numeric DEFAULT 1,
    answers text,
    feedback text,
    position bigint,
    created_at timestamptz,
    updated_at timestamptz,
    PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS question_banks (
    id bigserial,
    course_id bigint,
    title text,
    workflow_state text DEFAULT 'active'::text,
    created_at timestamptz,
    updated_at timestamptz,
    PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS quiz_question_groups (
    id bigserial,
    quiz_id bigint NOT NULL,
    name text,
    pick_count bigint NOT NULL DEFAULT 1,
    points_per_item numeric,
    question_bank_id bigint,
    position bigint,
    created_at timestamptz,
    updated_at timestamptz,
    PRIMARY KEY (id)
);

-- === Section: columns ===

ALTER TABLE access_tokens ADD COLUMN IF NOT EXISTS token_hint text;
ALTER TABLE access_tokens ADD COLUMN IF NOT EXISTS workflow_state text DEFAULT 'active'::text;
ALTER TABLE accommodation_applications ADD COLUMN IF NOT EXISTS accommodation_id bigint NOT NULL;
ALTER TABLE accommodation_applications ADD COLUMN IF NOT EXISTS resource_type text NOT NULL;
ALTER TABLE accommodation_applications ADD COLUMN IF NOT EXISTS resource_id bigint NOT NULL;
ALTER TABLE accommodation_applications ADD COLUMN IF NOT EXISTS user_id bigint NOT NULL;
ALTER TABLE accommodation_applications ADD COLUMN IF NOT EXISTS adjusted_due_at timestamptz;
ALTER TABLE accommodation_applications ADD COLUMN IF NOT EXISTS adjusted_time_limit bigint;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS max_upload_size_mb bigint NOT NULL DEFAULT 500;
ALTER TABLE age_verifications ADD COLUMN IF NOT EXISTS is_under13 boolean NOT NULL DEFAULT false;
ALTER TABLE age_verifications ADD COLUMN IF NOT EXISTS is_minor boolean NOT NULL DEFAULT true;
ALTER TABLE age_verifications ADD COLUMN IF NOT EXISTS verified_by text;
ALTER TABLE age_verifications ADD COLUMN IF NOT EXISTS requires_consent boolean NOT NULL DEFAULT true;
ALTER TABLE announcements ADD COLUMN IF NOT EXISTS course_id bigint;
ALTER TABLE announcements ADD COLUMN IF NOT EXISTS account_id bigint;
ALTER TABLE announcements ADD COLUMN IF NOT EXISTS priority text DEFAULT 'normal'::text;
ALTER TABLE announcements ADD COLUMN IF NOT EXISTS require_ack boolean DEFAULT false;
ALTER TABLE announcements ADD COLUMN IF NOT EXISTS target_audience text DEFAULT 'all'::text;
ALTER TABLE announcements ADD COLUMN IF NOT EXISTS allow_comments boolean DEFAULT false;
ALTER TABLE announcements ADD COLUMN IF NOT EXISTS is_global boolean DEFAULT false;
ALTER TABLE assignment_groups ADD COLUMN IF NOT EXISTS rules jsonb DEFAULT '{}'::jsonb;
ALTER TABLE assignment_override_students ADD COLUMN IF NOT EXISTS assignment_id bigint NOT NULL;
ALTER TABLE assignment_overrides ADD COLUMN IF NOT EXISTS course_section_id bigint;
ALTER TABLE assignment_overrides ADD COLUMN IF NOT EXISTS workflow_state text NOT NULL DEFAULT 'active'::text;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS anonymous_grading boolean DEFAULT false;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS post_policy text DEFAULT 'automatic'::text;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS peer_reviews_enabled boolean DEFAULT false;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS peer_review_count bigint DEFAULT 0;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS group_category_id bigint;
ALTER TABLE attachments ADD COLUMN IF NOT EXISTS md5 text;
ALTER TABLE attachments ADD COLUMN IF NOT EXISTS storage_path text NOT NULL;
ALTER TABLE attachments ADD COLUMN IF NOT EXISTS file_state text NOT NULL DEFAULT 'available'::text;
ALTER TABLE attachments ADD COLUMN IF NOT EXISTS upload_status text NOT NULL DEFAULT 'success'::text;
ALTER TABLE attendance_records ADD COLUMN IF NOT EXISTS section_id bigint;
ALTER TABLE attendance_records ADD COLUMN IF NOT EXISTS marked_by_id bigint NOT NULL;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS account_id bigint;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS action text;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS payload jsonb;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS user_agent text;
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS id_p_entity_id text;
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS id_p_certificate text;
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS ldap_host text;
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS ldap_port bigint;
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS ldap_base text;
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS ldap_filter text;
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS ldap_bind_dn text;
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS ldap_bind_password text;
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS ldap_use_tls boolean;
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS ldap_login_attribute text DEFAULT 'uid'::text;
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS cas_base_url text;
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS cas_login_url text;
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS cas_validate_url text;
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS cas_logout_url text;
ALTER TABLE authentication_providers ADD COLUMN IF NOT EXISTS federated_attributes text;
ALTER TABLE blueprint_templates ADD COLUMN IF NOT EXISTS use_default_restrictions boolean DEFAULT true;
ALTER TABLE calendar_events ADD COLUMN IF NOT EXISTS created_by_user_id bigint NOT NULL;
ALTER TABLE communication_channels ADD COLUMN IF NOT EXISTS channel_type text;
ALTER TABLE communication_channels ADD COLUMN IF NOT EXISTS address text;
ALTER TABLE communication_channels ADD COLUMN IF NOT EXISTS confirmed boolean DEFAULT false;
ALTER TABLE communication_channels ADD COLUMN IF NOT EXISTS confirm_code text;
ALTER TABLE communication_channels ADD COLUMN IF NOT EXISTS confirmed_at timestamptz;
ALTER TABLE conferences ADD COLUMN IF NOT EXISTS recordings jsonb DEFAULT '[]'::jsonb;
ALTER TABLE conferences ADD COLUMN IF NOT EXISTS settings jsonb DEFAULT '{}'::jsonb;
ALTER TABLE content_migrations ADD COLUMN IF NOT EXISTS attachment text;
ALTER TABLE content_tags ADD COLUMN IF NOT EXISTS new_tab boolean DEFAULT false;
ALTER TABLE context_external_tools ADD COLUMN IF NOT EXISTS custom_fields text;
ALTER TABLE conversation_messages ADD COLUMN IF NOT EXISTS user_id bigint NOT NULL;
ALTER TABLE conversation_messages ADD COLUMN IF NOT EXISTS workflow_state text DEFAULT 'active'::text;
ALTER TABLE conversation_participants ADD COLUMN IF NOT EXISTS last_read_at timestamptz;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS created_by_user_id bigint NOT NULL;
ALTER TABLE course_home_buttons ADD COLUMN IF NOT EXISTS button_type text NOT NULL;
ALTER TABLE course_home_buttons ADD COLUMN IF NOT EXISTS link_id bigint;
ALTER TABLE course_home_buttons ADD COLUMN IF NOT EXISTS link_url text;
ALTER TABLE course_visits ADD COLUMN IF NOT EXISTS last_url text;
ALTER TABLE course_visits ADD COLUMN IF NOT EXISTS last_title text;
ALTER TABLE courses ADD COLUMN IF NOT EXISTS license text DEFAULT 'private'::text;
ALTER TABLE courses ADD COLUMN IF NOT EXISTS apply_group_weights boolean DEFAULT false;
ALTER TABLE courses ADD COLUMN IF NOT EXISTS navigation_tabs text;
ALTER TABLE custom_roles ADD COLUMN IF NOT EXISTS permissions jsonb DEFAULT '{}'::jsonb;
ALTER TABLE custom_roles ADD COLUMN IF NOT EXISTS created_by_user_id bigint;
ALTER TABLE data_deletion_requests ADD COLUMN IF NOT EXISTS requested_by_id bigint NOT NULL;
ALTER TABLE data_deletion_requests ADD COLUMN IF NOT EXISTS request_type text NOT NULL;
ALTER TABLE data_deletion_requests ADD COLUMN IF NOT EXISTS data_scope text;
ALTER TABLE data_deletion_requests ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'pending'::text;
ALTER TABLE data_deletion_requests ADD COLUMN IF NOT EXISTS reviewed_by_id bigint;
ALTER TABLE data_deletion_requests ADD COLUMN IF NOT EXISTS reviewed_at timestamptz;
ALTER TABLE data_deletion_requests ADD COLUMN IF NOT EXISTS deletion_log text;
ALTER TABLE data_export_requests ADD COLUMN IF NOT EXISTS requested_by_id bigint NOT NULL;
ALTER TABLE data_export_requests ADD COLUMN IF NOT EXISTS data_scope text;
ALTER TABLE data_export_requests ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'pending'::text;
ALTER TABLE data_export_requests ADD COLUMN IF NOT EXISTS file_size_bytes bigint;
ALTER TABLE data_processing_agreements ADD COLUMN IF NOT EXISTS retention_period text;
ALTER TABLE data_processing_agreements ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'draft'::text;
ALTER TABLE data_processing_agreements ADD COLUMN IF NOT EXISTS expires_at timestamptz;
ALTER TABLE data_retention_policies ADD COLUMN IF NOT EXISTS data_category text NOT NULL;
ALTER TABLE data_retention_policies ADD COLUMN IF NOT EXISTS retention_period bigint;
ALTER TABLE data_retention_policies ADD COLUMN IF NOT EXISTS retention_action text NOT NULL DEFAULT 'anonymize'::text;
ALTER TABLE data_retention_policies ADD COLUMN IF NOT EXISTS auto_apply boolean DEFAULT false;
ALTER TABLE data_retention_policies ADD COLUMN IF NOT EXISTS description text;
ALTER TABLE developer_keys ADD COLUMN IF NOT EXISTS redirect_uris text;
ALTER TABLE developer_keys ADD COLUMN IF NOT EXISTS icon text;
ALTER TABLE developer_keys ADD COLUMN IF NOT EXISTS scopes text;
ALTER TABLE developer_keys ADD COLUMN IF NOT EXISTS require_scopes boolean DEFAULT false;
ALTER TABLE developer_keys ADD COLUMN IF NOT EXISTS is_lti_key boolean DEFAULT false;
ALTER TABLE discussion_entry_participants ADD COLUMN IF NOT EXISTS read_at timestamptz;
ALTER TABLE discussion_topic_participants ADD COLUMN IF NOT EXISTS forced_read_state text;
ALTER TABLE discussion_topic_participants ADD COLUMN IF NOT EXISTS last_read_at timestamptz;
ALTER TABLE discussion_topics ADD COLUMN IF NOT EXISTS posted_at timestamptz;
ALTER TABLE discussion_topics ADD COLUMN IF NOT EXISTS sort_by_rating boolean DEFAULT false;
ALTER TABLE discussion_topics ADD COLUMN IF NOT EXISTS assignment_id bigint;
ALTER TABLE document_annotations ADD COLUMN IF NOT EXISTS page_number bigint DEFAULT 1;
ALTER TABLE document_annotations ADD COLUMN IF NOT EXISTS selection_start bigint DEFAULT 0;
ALTER TABLE document_annotations ADD COLUMN IF NOT EXISTS selection_end bigint DEFAULT 0;
ALTER TABLE document_annotations ADD COLUMN IF NOT EXISTS path_data text;
ALTER TABLE document_annotations ADD COLUMN IF NOT EXISTS parent_annotation_id bigint;
ALTER TABLE document_annotations ADD COLUMN IF NOT EXISTS resolved_by_user_id bigint;
ALTER TABLE document_annotations ADD COLUMN IF NOT EXISTS workflow_state text NOT NULL DEFAULT 'active'::text;
ALTER TABLE enrollment_terms ADD COLUMN IF NOT EXISTS account_id bigint;
ALTER TABLE enrollments ADD COLUMN IF NOT EXISTS type text NOT NULL;
ALTER TABLE enrollments ADD COLUMN IF NOT EXISTS last_activity_at timestamptz;
ALTER TABLE enrollments ADD COLUMN IF NOT EXISTS associated_user_id bigint;
ALTER TABLE grade_change_logs ADD COLUMN IF NOT EXISTS submission_id bigint;
ALTER TABLE grade_change_logs ADD COLUMN IF NOT EXISTS excused boolean;
ALTER TABLE grade_change_logs ADD COLUMN IF NOT EXISTS grading_method text;
ALTER TABLE grading_period_groups ADD COLUMN IF NOT EXISTS display_totals boolean DEFAULT false;
ALTER TABLE grading_periods ADD COLUMN IF NOT EXISTS workflow_state text NOT NULL DEFAULT 'active'::text;
ALTER TABLE group_categories ADD COLUMN IF NOT EXISTS course_id bigint;
ALTER TABLE group_categories ADD COLUMN IF NOT EXISTS account_id bigint;
ALTER TABLE group_categories ADD COLUMN IF NOT EXISTS role text;
ALTER TABLE learning_outcome_groups ADD COLUMN IF NOT EXISTS parent_group_id bigint;
ALTER TABLE learning_outcome_results ADD COLUMN IF NOT EXISTS submitted_at timestamptz;
ALTER TABLE learning_outcomes ADD COLUMN IF NOT EXISTS outcome_group_id bigint NOT NULL;
ALTER TABLE learning_outcomes ADD COLUMN IF NOT EXISTS ratings_data jsonb;
ALTER TABLE lti_line_items ADD COLUMN IF NOT EXISTS resource_link_id_str text;
ALTER TABLE lti_line_items ADD COLUMN IF NOT EXISTS lti_submission_type text;
ALTER TABLE lti_resource_links ADD COLUMN IF NOT EXISTS custom_parameters text;
ALTER TABLE lti_resource_links ADD COLUMN IF NOT EXISTS lookup_uuid text;
ALTER TABLE lti_results ADD COLUMN IF NOT EXISTS timestamp timestamptz;
ALTER TABLE lti_tool_configurations ADD COLUMN IF NOT EXISTS title text NOT NULL;
ALTER TABLE lti_tool_configurations ADD COLUMN IF NOT EXISTS o_id_c_initiation_url text NOT NULL;
ALTER TABLE lti_tool_configurations ADD COLUMN IF NOT EXISTS tool_id text;
ALTER TABLE lti_tool_configurations ADD COLUMN IF NOT EXISTS scopes text;
ALTER TABLE nonces ADD COLUMN IF NOT EXISTS value text NOT NULL;
ALTER TABLE notification_deliveries ADD COLUMN IF NOT EXISTS channel_type text;
ALTER TABLE notification_deliveries ADD COLUMN IF NOT EXISTS address text;
ALTER TABLE notification_deliveries ADD COLUMN IF NOT EXISTS subject text;
ALTER TABLE notification_deliveries ADD COLUMN IF NOT EXISTS body text;
ALTER TABLE notification_deliveries ADD COLUMN IF NOT EXISTS delivery_status text DEFAULT 'pending'::text;
ALTER TABLE notification_deliveries ADD COLUMN IF NOT EXISTS digest_type text;
ALTER TABLE notification_deliveries ADD COLUMN IF NOT EXISTS max_retries bigint DEFAULT 3;
ALTER TABLE notification_deliveries ADD COLUMN IF NOT EXISTS last_error text;
ALTER TABLE notification_deliveries ADD COLUMN IF NOT EXISTS delivered_at timestamptz;
ALTER TABLE notification_deliveries ADD COLUMN IF NOT EXISTS scheduled_for timestamptz;
ALTER TABLE notification_preferences ADD COLUMN IF NOT EXISTS policy text DEFAULT 'daily'::text;
ALTER TABLE notification_preferences ADD COLUMN IF NOT EXISTS notify_new_message boolean DEFAULT true;
ALTER TABLE notification_preferences ADD COLUMN IF NOT EXISTS notify_event_start boolean DEFAULT false;
ALTER TABLE notification_preferences ADD COLUMN IF NOT EXISTS notify_submission_grade boolean DEFAULT true;
ALTER TABLE notification_preferences ADD COLUMN IF NOT EXISTS notify_new_announcement boolean DEFAULT true;
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS title text NOT NULL;
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS related_user_id bigint;
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS is_read boolean DEFAULT false;
ALTER TABLE one_roster_connections ADD COLUMN IF NOT EXISTS scope text DEFAULT 'https://purl.imsglobal.org/spec/or/v1p1/scope/roster-core.readonly'::text;
ALTER TABLE one_roster_connections ADD COLUMN IF NOT EXISTS last_sync_error text;
ALTER TABLE one_roster_connections ADD COLUMN IF NOT EXISTS sync_filter text;
ALTER TABLE one_roster_connections ADD COLUMN IF NOT EXISTS auto_sync boolean DEFAULT false;
ALTER TABLE one_roster_connections ADD COLUMN IF NOT EXISTS auto_sync_interval bigint DEFAULT 24;
ALTER TABLE one_roster_sync_logs ADD COLUMN IF NOT EXISTS status text NOT NULL;
ALTER TABLE one_roster_sync_logs ADD COLUMN IF NOT EXISTS orgs_created bigint DEFAULT 0;
ALTER TABLE one_roster_sync_logs ADD COLUMN IF NOT EXISTS orgs_updated bigint DEFAULT 0;
ALTER TABLE one_roster_sync_logs ADD COLUMN IF NOT EXISTS users_created bigint DEFAULT 0;
ALTER TABLE one_roster_sync_logs ADD COLUMN IF NOT EXISTS users_updated bigint DEFAULT 0;
ALTER TABLE one_roster_sync_logs ADD COLUMN IF NOT EXISTS classes_created bigint DEFAULT 0;
ALTER TABLE one_roster_sync_logs ADD COLUMN IF NOT EXISTS classes_updated bigint DEFAULT 0;
ALTER TABLE one_roster_sync_logs ADD COLUMN IF NOT EXISTS enrollments_created bigint DEFAULT 0;
ALTER TABLE one_roster_sync_logs ADD COLUMN IF NOT EXISTS enrollments_updated bigint DEFAULT 0;
ALTER TABLE one_roster_sync_logs ADD COLUMN IF NOT EXISTS completed_at timestamptz;
ALTER TABLE one_roster_sync_logs ADD COLUMN IF NOT EXISTS error_details text;
ALTER TABLE outcome_alignments ADD COLUMN IF NOT EXISTS assignment_id bigint NOT NULL;
ALTER TABLE outcome_alignments ADD COLUMN IF NOT EXISTS course_id bigint NOT NULL;
ALTER TABLE page_views ADD COLUMN IF NOT EXISTS action text;
ALTER TABLE parental_consents ADD COLUMN IF NOT EXISTS parent_user_id bigint;
ALTER TABLE parental_consents ADD COLUMN IF NOT EXISTS parent_name text NOT NULL;
ALTER TABLE parental_consents ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'pending'::text;
ALTER TABLE parental_consents ADD COLUMN IF NOT EXISTS consent_method text;
ALTER TABLE parental_consents ADD COLUMN IF NOT EXISTS verification_token text;
ALTER TABLE parental_consents ADD COLUMN IF NOT EXISTS consented_at timestamptz;
ALTER TABLE parental_consents ADD COLUMN IF NOT EXISTS expires_at timestamptz;
ALTER TABLE parental_consents ADD COLUMN IF NOT EXISTS user_agent text;
ALTER TABLE parental_consents ADD COLUMN IF NOT EXISTS notes text;
ALTER TABLE pii_access_logs ADD COLUMN IF NOT EXISTS accessor_id bigint NOT NULL;
ALTER TABLE pii_access_logs ADD COLUMN IF NOT EXISTS student_id bigint NOT NULL;
ALTER TABLE pii_access_logs ADD COLUMN IF NOT EXISTS data_field text NOT NULL;
ALTER TABLE pii_access_logs ADD COLUMN IF NOT EXISTS resource text;
ALTER TABLE pii_access_logs ADD COLUMN IF NOT EXISTS resource_id bigint;
ALTER TABLE pii_access_logs ADD COLUMN IF NOT EXISTS user_agent text;
ALTER TABLE pii_access_logs ADD COLUMN IF NOT EXISTS justification text;
ALTER TABLE portfolio_artifacts ADD COLUMN IF NOT EXISTS portfolio_id bigint NOT NULL;
ALTER TABLE portfolio_artifacts ADD COLUMN IF NOT EXISTS section_id bigint;
ALTER TABLE portfolio_artifacts ADD COLUMN IF NOT EXISTS content_url text;
ALTER TABLE portfolio_artifacts ADD COLUMN IF NOT EXISTS thumbnail_url text;
ALTER TABLE portfolio_artifacts ADD COLUMN IF NOT EXISTS source_type text;
ALTER TABLE portfolio_artifacts ADD COLUMN IF NOT EXISTS source_course_id bigint;
ALTER TABLE portfolio_artifacts ADD COLUMN IF NOT EXISTS source_id bigint;
ALTER TABLE portfolio_artifacts ADD COLUMN IF NOT EXISTS file_type text;
ALTER TABLE portfolio_artifacts ADD COLUMN IF NOT EXISTS file_size_bytes bigint;
ALTER TABLE portfolio_artifacts ADD COLUMN IF NOT EXISTS tags text;
ALTER TABLE portfolio_artifacts ADD COLUMN IF NOT EXISTS outcome_ids text;
ALTER TABLE portfolio_artifacts ADD COLUMN IF NOT EXISTS is_featured boolean DEFAULT false;
ALTER TABLE portfolio_comments ADD COLUMN IF NOT EXISTS section_id bigint;
ALTER TABLE portfolio_comments ADD COLUMN IF NOT EXISTS artifact_id bigint;
ALTER TABLE portfolio_comments ADD COLUMN IF NOT EXISTS content text NOT NULL;
ALTER TABLE portfolio_reflections ADD COLUMN IF NOT EXISTS artifact_id bigint NOT NULL;
ALTER TABLE portfolio_reflections ADD COLUMN IF NOT EXISTS prompt_text text;
ALTER TABLE portfolio_sections ADD COLUMN IF NOT EXISTS content text;
ALTER TABLE portfolio_sections ADD COLUMN IF NOT EXISTS is_visible boolean DEFAULT true;
ALTER TABLE portfolio_templates ADD COLUMN IF NOT EXISTS created_by_id bigint;
ALTER TABLE portfolio_templates ADD COLUMN IF NOT EXISTS name text NOT NULL;
ALTER TABLE portfolio_templates ADD COLUMN IF NOT EXISTS theme_id text NOT NULL;
ALTER TABLE portfolio_templates ADD COLUMN IF NOT EXISTS sections text;
ALTER TABLE portfolio_templates ADD COLUMN IF NOT EXISTS is_public boolean DEFAULT false;
ALTER TABLE portfolio_templates ADD COLUMN IF NOT EXISTS usage_count bigint DEFAULT 0;
ALTER TABLE portfolios ADD COLUMN IF NOT EXISTS slug text NOT NULL;
ALTER TABLE portfolios ADD COLUMN IF NOT EXISTS theme_id text NOT NULL DEFAULT 'clean-modern'::text;
ALTER TABLE portfolios ADD COLUMN IF NOT EXISTS custom_css text;
ALTER TABLE portfolios ADD COLUMN IF NOT EXISTS header_image_url text;
ALTER TABLE portfolios ADD COLUMN IF NOT EXISTS avatar_url text;
ALTER TABLE portfolios ADD COLUMN IF NOT EXISTS tagline text;
ALTER TABLE portfolios ADD COLUMN IF NOT EXISTS contact_email text;
ALTER TABLE portfolios ADD COLUMN IF NOT EXISTS linked_in_url text;
ALTER TABLE portfolios ADD COLUMN IF NOT EXISTS website_url text;
ALTER TABLE portfolios ADD COLUMN IF NOT EXISTS is_public boolean DEFAULT false;
ALTER TABLE portfolios ADD COLUMN IF NOT EXISTS public_url text;
ALTER TABLE portfolios ADD COLUMN IF NOT EXISTS custom_domain text;
ALTER TABLE portfolios ADD COLUMN IF NOT EXISTS view_count bigint DEFAULT 0;
ALTER TABLE portfolios ADD COLUMN IF NOT EXISTS last_exported_at timestamptz;
ALTER TABLE quiz_questions ADD COLUMN IF NOT EXISTS quiz_question_group_id bigint;
ALTER TABLE quiz_questions ADD COLUMN IF NOT EXISTS workflow_state text NOT NULL DEFAULT 'active'::text;
ALTER TABLE quiz_submission_answers ADD COLUMN IF NOT EXISTS question_id bigint NOT NULL;
ALTER TABLE quiz_submissions ADD COLUMN IF NOT EXISTS selected_questions text;
ALTER TABLE quiz_submissions ADD COLUMN IF NOT EXISTS validation_token text NOT NULL;
ALTER TABLE role_overrides ADD COLUMN IF NOT EXISTS account_id bigint NOT NULL;
ALTER TABLE role_overrides ADD COLUMN IF NOT EXISTS role_id bigint NOT NULL;
ALTER TABLE role_overrides ADD COLUMN IF NOT EXISTS context_type text NOT NULL DEFAULT 'Account'::text;
ALTER TABLE role_overrides ADD COLUMN IF NOT EXISTS context_id bigint NOT NULL DEFAULT 0;
ALTER TABLE rubric_assessments ADD COLUMN IF NOT EXISTS workflow_state text NOT NULL DEFAULT 'active'::text;
ALTER TABLE rubric_associations ADD COLUMN IF NOT EXISTS context_type text;
ALTER TABLE rubric_associations ADD COLUMN IF NOT EXISTS context_id bigint;
ALTER TABLE rubrics ADD COLUMN IF NOT EXISTS description text;
ALTER TABLE rubrics ADD COLUMN IF NOT EXISTS hide_points boolean DEFAULT false;
ALTER TABLE sis_batch_errors ADD COLUMN IF NOT EXISTS row bigint;
ALTER TABLE sis_batch_errors ADD COLUMN IF NOT EXISTS file text;
ALTER TABLE sis_batches ADD COLUMN IF NOT EXISTS total_rows bigint DEFAULT 0;
ALTER TABLE sis_batches ADD COLUMN IF NOT EXISTS processed_rows bigint DEFAULT 0;
ALTER TABLE student_accommodations ADD COLUMN IF NOT EXISTS description text;
ALTER TABLE student_accommodations ADD COLUMN IF NOT EXISTS time_multiplier numeric;
ALTER TABLE student_accommodations ADD COLUMN IF NOT EXISTS extra_days bigint;
ALTER TABLE student_accommodations ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'active'::text;
ALTER TABLE student_accommodations ADD COLUMN IF NOT EXISTS plan_type text;
ALTER TABLE student_accommodations ADD COLUMN IF NOT EXISTS plan_external_id text;
ALTER TABLE student_accommodations ADD COLUMN IF NOT EXISTS created_by_id bigint NOT NULL;
ALTER TABLE student_accommodations ADD COLUMN IF NOT EXISTS approved_by_id bigint;
ALTER TABLE student_accommodations ADD COLUMN IF NOT EXISTS effective_from timestamptz NOT NULL;
ALTER TABLE student_accommodations ADD COLUMN IF NOT EXISTS effective_until timestamptz;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS posted_at timestamptz;
ALTER TABLE todays_lesson_overrides ADD COLUMN IF NOT EXISTS link_type text NOT NULL;
ALTER TABLE todays_lesson_overrides ADD COLUMN IF NOT EXISTS link_id bigint;
ALTER TABLE todays_lesson_overrides ADD COLUMN IF NOT EXISTS link_url text;
ALTER TABLE todays_lesson_overrides ADD COLUMN IF NOT EXISTS label text;
ALTER TABLE wiki_pages ADD COLUMN IF NOT EXISTS public boolean DEFAULT false;
ALTER TABLE wiki_pages ADD COLUMN IF NOT EXISTS website_mode boolean DEFAULT false;

-- === Section: indexes ===

CREATE INDEX IF NOT EXISTS idx_access_tokens_developer_key_id ON public.access_tokens USING btree (developer_key_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_access_tokens_refresh_token ON public.access_tokens USING btree (refresh_token);
CREATE UNIQUE INDEX IF NOT EXISTS idx_access_tokens_token ON public.access_tokens USING btree (token);
CREATE INDEX IF NOT EXISTS idx_access_tokens_user_id ON public.access_tokens USING btree (user_id);
CREATE INDEX IF NOT EXISTS idx_accommodation_applications_accommodation_id ON public.accommodation_applications USING btree (accommodation_id);
CREATE INDEX IF NOT EXISTS idx_accommodation_applications_resource_id ON public.accommodation_applications USING btree (resource_id);
CREATE INDEX IF NOT EXISTS idx_accommodation_applications_user_id ON public.accommodation_applications USING btree (user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_age_verifications_user_id ON public.age_verifications USING btree (user_id);
CREATE INDEX IF NOT EXISTS idx_annotation_submission_page ON public.document_annotations USING btree (submission_id, page_number);
CREATE UNIQUE INDEX IF NOT EXISTS idx_announcement_user ON public.announcement_read_receipts USING btree (announcement_id, user_id);
CREATE INDEX IF NOT EXISTS idx_announcements_account_id ON public.announcements USING btree (account_id);
CREATE INDEX IF NOT EXISTS idx_announcements_course_id ON public.announcements USING btree (course_id);
CREATE INDEX IF NOT EXISTS idx_appointment_reservations_group_id ON public.appointment_reservations USING btree (group_id);
CREATE INDEX IF NOT EXISTS idx_appointment_slots_start_at ON public.appointment_slots USING btree (start_at);
CREATE INDEX IF NOT EXISTS idx_assignment_override_students_assignment_id ON public.assignment_override_students USING btree (assignment_id);
CREATE INDEX IF NOT EXISTS idx_assignment_override_students_assignment_override_id ON public.assignment_override_students USING btree (assignment_override_id);
CREATE INDEX IF NOT EXISTS idx_assignment_overrides_assignment_id ON public.assignment_overrides USING btree (assignment_id);
CREATE INDEX IF NOT EXISTS idx_assignments_group_category_id ON public.assignments USING btree (group_category_id);
CREATE INDEX IF NOT EXISTS idx_attendance_records_course_id ON public.attendance_records USING btree (course_id);
CREATE INDEX IF NOT EXISTS idx_attendance_records_date ON public.attendance_records USING btree (date);
CREATE INDEX IF NOT EXISTS idx_attendance_records_section_id ON public.attendance_records USING btree (section_id);
CREATE INDEX IF NOT EXISTS idx_attendance_records_user_id ON public.attendance_records USING btree (user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_account_id ON public.audit_logs USING btree (account_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON public.audit_logs USING btree (created_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_event_type ON public.audit_logs USING btree (event_type);
CREATE INDEX IF NOT EXISTS idx_authentication_providers_account_id ON public.authentication_providers USING btree (account_id);
CREATE INDEX IF NOT EXISTS idx_blueprint_migrations_blueprint_template_id ON public.blueprint_migrations USING btree (blueprint_template_id);
CREATE INDEX IF NOT EXISTS idx_blueprint_templates_course_id ON public.blueprint_templates USING btree (course_id);
CREATE INDEX IF NOT EXISTS idx_cal_event_context ON public.calendar_events USING btree (context_type, context_id);
CREATE INDEX IF NOT EXISTS idx_calendar_events_start_at ON public.calendar_events USING btree (start_at);
CREATE INDEX IF NOT EXISTS idx_collab_context ON public.collaborations USING btree (context_type, context_id);
CREATE INDEX IF NOT EXISTS idx_communication_channels_user_id ON public.communication_channels USING btree (user_id);
CREATE INDEX IF NOT EXISTS idx_conditional_release_assignment_set_actions_set_id ON public.conditional_release_assignment_set_actions USING btree (set_id);
CREATE INDEX IF NOT EXISTS idx_conditional_release_assignment_set_actions_student_id ON public.conditional_release_assignment_set_actions USING btree (student_id);
CREATE INDEX IF NOT EXISTS idx_conditional_release_assignment_set_associations_ass26affdd9 ON public.conditional_release_assignment_set_associations USING btree (assignment_id);
CREATE INDEX IF NOT EXISTS idx_conditional_release_assignment_set_associations_set_id ON public.conditional_release_assignment_set_associations USING btree (set_id);
CREATE INDEX IF NOT EXISTS idx_conditional_release_assignment_sets_scoring_range_id ON public.conditional_release_assignment_sets USING btree (scoring_range_id);
CREATE INDEX IF NOT EXISTS idx_conditional_release_rules_course_id ON public.conditional_release_rules USING btree (course_id);
CREATE INDEX IF NOT EXISTS idx_conditional_release_rules_workflow_state ON public.conditional_release_rules USING btree (workflow_state);
CREATE INDEX IF NOT EXISTS idx_conditional_release_scoring_ranges_rule_id ON public.conditional_release_scoring_ranges USING btree (rule_id);
CREATE INDEX IF NOT EXISTS idx_conf_context ON public.conferences USING btree (context_type, context_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_conf_user ON public.conference_participants USING btree (conference_id, user_id);
CREATE INDEX IF NOT EXISTS idx_content_migrations_course_id ON public.content_migrations USING btree (course_id);
CREATE INDEX IF NOT EXISTS idx_content_tags_context_module_id ON public.content_tags USING btree (context_module_id);
CREATE INDEX IF NOT EXISTS idx_context_external_tools_context_id ON public.context_external_tools USING btree (context_id);
CREATE INDEX IF NOT EXISTS idx_context_external_tools_developer_key_id ON public.context_external_tools USING btree (developer_key_id);
CREATE INDEX IF NOT EXISTS idx_context_modules_course_id ON public.context_modules USING btree (course_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_conv_user ON public.conversation_participants USING btree (conversation_id, user_id);
CREATE INDEX IF NOT EXISTS idx_conversation_messages_conversation_id ON public.conversation_messages USING btree (conversation_id);
CREATE INDEX IF NOT EXISTS idx_conversations_last_message_at ON public.conversations USING btree (last_message_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_course_date ON public.todays_lesson_overrides USING btree (course_id, date);
CREATE INDEX IF NOT EXISTS idx_course_paces_course_id ON public.course_paces USING btree (course_id);
CREATE INDEX IF NOT EXISTS idx_course_paces_course_section_id ON public.course_paces USING btree (course_section_id);
CREATE INDEX IF NOT EXISTS idx_course_paces_user_id ON public.course_paces USING btree (user_id);
CREATE INDEX IF NOT EXISTS idx_course_sections_course_id ON public.course_sections USING btree (course_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_course_sections_sis_section_id ON public.course_sections USING btree (sis_section_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_courses_sis_course_id ON public.courses USING btree (sis_course_id);
CREATE INDEX IF NOT EXISTS idx_custom_gradebook_column_data_user_id ON public.custom_gradebook_column_data USING btree (user_id);
CREATE INDEX IF NOT EXISTS idx_custom_roles_account_id ON public.custom_roles USING btree (account_id);
CREATE INDEX IF NOT EXISTS idx_custom_roles_base_role_type ON public.custom_roles USING btree (base_role_type);
CREATE INDEX IF NOT EXISTS idx_custom_roles_created_by_user_id ON public.custom_roles USING btree (created_by_user_id);
CREATE INDEX IF NOT EXISTS idx_data_deletion_requests_requested_by_id ON public.data_deletion_requests USING btree (requested_by_id);
CREATE INDEX IF NOT EXISTS idx_data_deletion_requests_user_id ON public.data_deletion_requests USING btree (user_id);
CREATE INDEX IF NOT EXISTS idx_data_export_requests_requested_by_id ON public.data_export_requests USING btree (requested_by_id);
CREATE INDEX IF NOT EXISTS idx_data_export_requests_user_id ON public.data_export_requests USING btree (user_id);
CREATE INDEX IF NOT EXISTS idx_data_processing_agreements_account_id ON public.data_processing_agreements USING btree (account_id);
CREATE INDEX IF NOT EXISTS idx_data_retention_policies_account_id ON public.data_retention_policies USING btree (account_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_developer_keys_client_id ON public.developer_keys USING btree (client_id);
CREATE INDEX IF NOT EXISTS idx_discussion_checkpoint_submissions_discussion_checkpoint_id ON public.discussion_checkpoint_submissions USING btree (discussion_checkpoint_id);
CREATE INDEX IF NOT EXISTS idx_discussion_checkpoints_discussion_topic_id ON public.discussion_checkpoints USING btree (discussion_topic_id);
CREATE INDEX IF NOT EXISTS idx_discussion_entries_discussion_topic_id ON public.discussion_entries USING btree (discussion_topic_id);
CREATE INDEX IF NOT EXISTS idx_discussion_entries_parent_id ON public.discussion_entries USING btree (parent_id);
CREATE INDEX IF NOT EXISTS idx_discussion_entry_versions_discussion_entry_id ON public.discussion_entry_versions USING btree (discussion_entry_id);
CREATE INDEX IF NOT EXISTS idx_document_annotations_parent_annotation_id ON public.document_annotations USING btree (parent_annotation_id);
CREATE INDEX IF NOT EXISTS idx_document_annotations_submission_id ON public.document_annotations USING btree (submission_id);
CREATE INDEX IF NOT EXISTS idx_document_annotations_user_id ON public.document_annotations USING btree (user_id);
CREATE INDEX IF NOT EXISTS idx_enrollment_terms_account_id ON public.enrollment_terms USING btree (account_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_enrollment_terms_sis_term_id ON public.enrollment_terms USING btree (sis_term_id);
CREATE INDEX IF NOT EXISTS idx_enrollments_associated_user_id ON public.enrollments USING btree (associated_user_id);
CREATE INDEX IF NOT EXISTS idx_enrollments_course_section_id ON public.enrollments USING btree (course_section_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_entry_user ON public.discussion_entry_participants USING btree (discussion_entry_id, user_id);
CREATE INDEX IF NOT EXISTS idx_feature_flags_context_id ON public.feature_flags USING btree (context_id);
CREATE INDEX IF NOT EXISTS idx_feature_flags_context_type ON public.feature_flags USING btree (context_type);
CREATE INDEX IF NOT EXISTS idx_folders_parent_folder_id ON public.folders USING btree (parent_folder_id);
CREATE INDEX IF NOT EXISTS idx_grade_change_logs_assignment_id ON public.grade_change_logs USING btree (assignment_id);
CREATE INDEX IF NOT EXISTS idx_grade_change_logs_created_at ON public.grade_change_logs USING btree (created_at);
CREATE INDEX IF NOT EXISTS idx_grade_change_logs_grader_id ON public.grade_change_logs USING btree (grader_id);
CREATE INDEX IF NOT EXISTS idx_grading_period_groups_account_id ON public.grading_period_groups USING btree (account_id);
CREATE INDEX IF NOT EXISTS idx_grading_periods_grading_period_group_id ON public.grading_periods USING btree (grading_period_group_id);
CREATE INDEX IF NOT EXISTS idx_group_categories_account_id ON public.group_categories USING btree (account_id);
CREATE INDEX IF NOT EXISTS idx_group_categories_course_id ON public.group_categories USING btree (course_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_group_user ON public.group_memberships USING btree (group_id, user_id);
CREATE INDEX IF NOT EXISTS idx_groups_context_id ON public.groups USING btree (context_id);
CREATE INDEX IF NOT EXISTS idx_groups_group_category_id ON public.groups USING btree (group_category_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_late_policies_course_id ON public.late_policies USING btree (course_id);
CREATE INDEX IF NOT EXISTS idx_learning_outcome_groups_context_id ON public.learning_outcome_groups USING btree (context_id);
CREATE INDEX IF NOT EXISTS idx_learning_outcome_groups_parent_group_id ON public.learning_outcome_groups USING btree (parent_group_id);
CREATE INDEX IF NOT EXISTS idx_learning_outcome_results_learning_outcome_id ON public.learning_outcome_results USING btree (learning_outcome_id);
CREATE INDEX IF NOT EXISTS idx_learning_outcome_results_user_id ON public.learning_outcome_results USING btree (user_id);
CREATE INDEX IF NOT EXISTS idx_learning_outcomes_context_id ON public.learning_outcomes USING btree (context_id);
CREATE INDEX IF NOT EXISTS idx_learning_outcomes_outcome_group_id ON public.learning_outcomes USING btree (outcome_group_id);
CREATE INDEX IF NOT EXISTS idx_lti_line_items_assignment_id ON public.lti_line_items USING btree (assignment_id);
CREATE INDEX IF NOT EXISTS idx_lti_line_items_course_id ON public.lti_line_items USING btree (course_id);
CREATE INDEX IF NOT EXISTS idx_lti_line_items_resource_link_id ON public.lti_line_items USING btree (resource_link_id);
CREATE INDEX IF NOT EXISTS idx_lti_resource_links_context_external_tool_id ON public.lti_resource_links USING btree (context_external_tool_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_lti_resource_links_lookup_uuid ON public.lti_resource_links USING btree (lookup_uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_lti_resource_links_resource_link_id ON public.lti_resource_links USING btree (resource_link_id);
CREATE INDEX IF NOT EXISTS idx_lti_results_line_item_id ON public.lti_results USING btree (line_item_id);
CREATE INDEX IF NOT EXISTS idx_lti_results_user_id ON public.lti_results USING btree (user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_lti_tool_configurations_developer_key_id ON public.lti_tool_configurations USING btree (developer_key_id);
CREATE INDEX IF NOT EXISTS idx_module_prerequisites_module_id ON public.module_prerequisites USING btree (module_id);
CREATE INDEX IF NOT EXISTS idx_nonces_expires_at ON public.nonces USING btree (expires_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_nonces_value ON public.nonces USING btree (value);
CREATE INDEX IF NOT EXISTS idx_notification_deliveries_delivery_status ON public.notification_deliveries USING btree (delivery_status);
CREATE INDEX IF NOT EXISTS idx_notification_deliveries_notification_id ON public.notification_deliveries USING btree (notification_id);
CREATE INDEX IF NOT EXISTS idx_notification_deliveries_scheduled_for ON public.notification_deliveries USING btree (scheduled_for);
CREATE INDEX IF NOT EXISTS idx_notifications_is_read ON public.notifications USING btree (is_read);
CREATE INDEX IF NOT EXISTS idx_one_roster_connections_account_id ON public.one_roster_connections USING btree (account_id);
CREATE INDEX IF NOT EXISTS idx_one_roster_sync_logs_connection_id ON public.one_roster_sync_logs USING btree (connection_id);
CREATE INDEX IF NOT EXISTS idx_outcome_alignments_assignment_id ON public.outcome_alignments USING btree (assignment_id);
CREATE INDEX IF NOT EXISTS idx_outcome_alignments_course_id ON public.outcome_alignments USING btree (course_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_outcome_assignment ON public.outcome_alignments USING btree (learning_outcome_id, assignment_id);
CREATE INDEX IF NOT EXISTS idx_outcome_proficiency_ratings_outcome_proficiency_id ON public.outcome_proficiency_ratings USING btree (outcome_proficiency_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_override_user ON public.assignment_override_students USING btree (user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_pace_module_item ON public.course_pace_module_items USING btree (course_pace_id, module_item_id);
CREATE INDEX IF NOT EXISTS idx_page_views_context_id ON public.page_views USING btree (context_id);
CREATE INDEX IF NOT EXISTS idx_parental_consents_parent_user_id ON public.parental_consents USING btree (parent_user_id);
CREATE INDEX IF NOT EXISTS idx_parental_consents_student_id ON public.parental_consents USING btree (student_id);
CREATE INDEX IF NOT EXISTS idx_parental_consents_verification_token ON public.parental_consents USING btree (verification_token);
CREATE INDEX IF NOT EXISTS idx_peer_reviews_assignment_id ON public.peer_reviews USING btree (assignment_id);
CREATE INDEX IF NOT EXISTS idx_peer_reviews_reviewee_id ON public.peer_reviews USING btree (reviewee_id);
CREATE INDEX IF NOT EXISTS idx_peer_reviews_reviewer_id ON public.peer_reviews USING btree (reviewer_id);
CREATE INDEX IF NOT EXISTS idx_peer_reviews_submission_id ON public.peer_reviews USING btree (submission_id);
CREATE INDEX IF NOT EXISTS idx_pii_access_logs_accessor_id ON public.pii_access_logs USING btree (accessor_id);
CREATE INDEX IF NOT EXISTS idx_pii_access_logs_student_id ON public.pii_access_logs USING btree (student_id);
CREATE INDEX IF NOT EXISTS idx_portfolio_artifacts_portfolio_id ON public.portfolio_artifacts USING btree (portfolio_id);
CREATE INDEX IF NOT EXISTS idx_portfolio_comments_artifact_id ON public.portfolio_comments USING btree (artifact_id);
CREATE INDEX IF NOT EXISTS idx_portfolio_comments_portfolio_id ON public.portfolio_comments USING btree (portfolio_id);
CREATE INDEX IF NOT EXISTS idx_portfolio_comments_section_id ON public.portfolio_comments USING btree (section_id);
CREATE INDEX IF NOT EXISTS idx_portfolio_reflections_artifact_id ON public.portfolio_reflections USING btree (artifact_id);
CREATE INDEX IF NOT EXISTS idx_portfolio_templates_account_id ON public.portfolio_templates USING btree (account_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_portfolios_public_url ON public.portfolios USING btree (public_url);
CREATE UNIQUE INDEX IF NOT EXISTS idx_portfolios_slug ON public.portfolios USING btree (slug);
CREATE UNIQUE INDEX IF NOT EXISTS idx_qq_outcome ON public.quiz_question_outcome_alignments USING btree (quiz_question_id, outcome_id);
CREATE INDEX IF NOT EXISTS idx_question_bank_entries_question_bank_id ON public.question_bank_entries USING btree (question_bank_id);
CREATE INDEX IF NOT EXISTS idx_question_banks_course_id ON public.question_banks USING btree (course_id);
CREATE INDEX IF NOT EXISTS idx_quiz_item_banks_created_by_user_id ON public.quiz_item_banks USING btree (created_by_user_id);
CREATE INDEX IF NOT EXISTS idx_quiz_question_groups_quiz_id ON public.quiz_question_groups USING btree (quiz_id);
CREATE INDEX IF NOT EXISTS idx_quiz_question_outcome_alignments_outcome_id ON public.quiz_question_outcome_alignments USING btree (outcome_id);
CREATE INDEX IF NOT EXISTS idx_quiz_question_outcome_alignments_quiz_question_id ON public.quiz_question_outcome_alignments USING btree (quiz_question_id);
CREATE INDEX IF NOT EXISTS idx_quiz_questions_quiz_question_group_id ON public.quiz_questions USING btree (quiz_question_group_id);
CREATE INDEX IF NOT EXISTS idx_quiz_submission_answers_quiz_submission_id ON public.quiz_submission_answers USING btree (quiz_submission_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_rating_entry_user ON public.discussion_entry_ratings USING btree (discussion_entry_id, user_id);
CREATE INDEX IF NOT EXISTS idx_role_overrides_account_id ON public.role_overrides USING btree (account_id);
CREATE INDEX IF NOT EXISTS idx_role_overrides_role_id ON public.role_overrides USING btree (role_id);
CREATE INDEX IF NOT EXISTS idx_rubric_assessments_rubric_id ON public.rubric_assessments USING btree (rubric_id);
CREATE INDEX IF NOT EXISTS idx_rubric_associations_rubric_id ON public.rubric_associations USING btree (rubric_id);
CREATE INDEX IF NOT EXISTS idx_shared_content_account_id ON public.shared_content USING btree (account_id);
CREATE INDEX IF NOT EXISTS idx_shared_content_author_user_id ON public.shared_content USING btree (author_user_id);
CREATE INDEX IF NOT EXISTS idx_shared_content_favorites_user_id ON public.shared_content_favorites USING btree (user_id);
CREATE INDEX IF NOT EXISTS idx_shared_content_grade_level ON public.shared_content USING btree (grade_level);
CREATE INDEX IF NOT EXISTS idx_shared_content_resource_type ON public.shared_content USING btree (resource_type);
CREATE INDEX IF NOT EXISTS idx_shared_content_source_content_id ON public.shared_content USING btree (source_content_id);
CREATE INDEX IF NOT EXISTS idx_shared_content_source_course_id ON public.shared_content USING btree (source_course_id);
CREATE INDEX IF NOT EXISTS idx_shared_content_subject ON public.shared_content USING btree (subject);
CREATE INDEX IF NOT EXISTS idx_sis_batch_errors_sis_batch_id ON public.sis_batch_errors USING btree (sis_batch_id);
CREATE INDEX IF NOT EXISTS idx_sis_batches_account_id ON public.sis_batches USING btree (account_id);
CREATE INDEX IF NOT EXISTS idx_student_accommodations_course_id ON public.student_accommodations USING btree (course_id);
CREATE INDEX IF NOT EXISTS idx_student_accommodations_user_id ON public.student_accommodations USING btree (user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_submission_assignment_user ON public.submissions USING btree (assignment_id, user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_template_child ON public.blueprint_subscriptions USING btree (blueprint_template_id, child_course_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_topic_user ON public.discussion_topic_participants USING btree (discussion_topic_id, user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_course ON public.course_visits USING btree (user_id, course_id);
CREATE INDEX IF NOT EXISTS idx_wiki_pages_url ON public.wiki_pages USING btree (url);

