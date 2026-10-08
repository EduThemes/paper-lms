import React from 'react';

// Assignment authoring fields for the grading features: availability
// window, attempt limit, allowed file extensions, the document students
// annotate, group grading, moderated grading, peer-review options and
// "omit from final grade". The parent owns the form state; this
// component edits it through onChange(patch).

const toLocalInput = (iso) => {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  const pad = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
};

const toISO = (local) => (local ? new Date(local).toISOString() : null);

// Form state from an assignment as the API returns it.
export const gradingSettingsFromAssignment = (a = {}) => ({
  unlock_at: toLocalInput(a.unlock_at),
  lock_at: toLocalInput(a.lock_at),
  limit_attempts: a.allowed_attempts != null && a.allowed_attempts > 0,
  allowed_attempts: a.allowed_attempts > 0 ? a.allowed_attempts : 1,
  allowed_extensions: Array.isArray(a.allowed_extensions) ? a.allowed_extensions.join(', ') : (a.allowed_extensions || ''),
  annotatable_attachment_id: a.annotatable_attachment_id || '',
  grade_group_students_individually: !!a.grade_group_students_individually,
  moderated_grading: !!a.moderated_grading,
  grader_count: a.grader_count > 0 ? a.grader_count : 2,
  final_grader_id: a.final_grader_id || '',
  grader_comments_visible_to_graders: a.grader_comments_visible_to_graders ?? true,
  graders_anonymous_to_graders: !!a.graders_anonymous_to_graders,
  grader_names_visible_to_final_grader: a.grader_names_visible_to_final_grader ?? true,
  anonymous_peer_reviews: !!a.anonymous_peer_reviews,
  peer_reviews_due_at: toLocalInput(a.peer_reviews_due_at),
  omit_from_final_grade: !!a.omit_from_final_grade,
});

// The API payload for the settings. submissionTypes decides which
// type-specific settings are sent.
export const gradingSettingsPayload = (form, submissionTypes = []) => {
  const types = Array.isArray(submissionTypes) ? submissionTypes : String(submissionTypes).split(',').map((t) => t.trim());
  const payload = {
    unlock_at: toISO(form.unlock_at),
    lock_at: toISO(form.lock_at),
    allowed_attempts: form.limit_attempts ? Math.max(1, parseInt(form.allowed_attempts, 10) || 1) : -1,
    allowed_extensions: types.includes('online_upload')
      ? String(form.allowed_extensions || '').split(/[\s,]+/).map((e) => e.replace(/^\./, '').trim()).filter(Boolean)
      : [],
    grade_group_students_individually: !!form.grade_group_students_individually,
    moderated_grading: !!form.moderated_grading,
    anonymous_peer_reviews: !!form.anonymous_peer_reviews,
    peer_reviews_due_at: toISO(form.peer_reviews_due_at),
    omit_from_final_grade: !!form.omit_from_final_grade,
  };
  if (types.includes('student_annotation') && form.annotatable_attachment_id) {
    payload.annotatable_attachment_id = Number(form.annotatable_attachment_id);
  }
  if (form.moderated_grading) {
    payload.grader_count = Math.min(10, Math.max(1, parseInt(form.grader_count, 10) || 2));
    payload.final_grader_id = form.final_grader_id ? Number(form.final_grader_id) : null;
    payload.grader_comments_visible_to_graders = !!form.grader_comments_visible_to_graders;
    payload.graders_anonymous_to_graders = !!form.graders_anonymous_to_graders;
    payload.grader_names_visible_to_final_grader = !!form.grader_names_visible_to_final_grader;
  }
  return payload;
};

// Validation the browser can do before the API does; returns an error
// message or null.
export const validateGradingSettings = (form, submissionTypes = []) => {
  const types = Array.isArray(submissionTypes) ? submissionTypes : String(submissionTypes).split(',');
  if (form.unlock_at && form.lock_at && new Date(form.unlock_at) >= new Date(form.lock_at)) {
    return '"Available from" must be before "Until".';
  }
  if (types.includes('student_annotation') && !form.annotatable_attachment_id) {
    return 'Choose the document students will annotate.';
  }
  if (form.moderated_grading && (!form.grader_count || form.grader_count < 1 || form.grader_count > 10)) {
    return 'The number of graders must be between 1 and 10.';
  }
  return null;
};

const Field = ({ id, label, children, hint }) => (
  <div>
    <label htmlFor={id} className="block text-sm font-medium text-text-secondary mb-1">{label}</label>
    {children}
    {hint && <p className="text-xs text-text-disabled mt-1">{hint}</p>}
  </div>
);

const Check = ({ id, checked, onChange, children }) => (
  <label htmlFor={id} className="flex items-center gap-2 text-sm text-text-secondary">
    <input id={id} type="checkbox" checked={!!checked} onChange={(e) => onChange(e.target.checked)} className="rounded border-border-strong" />
    {children}
  </label>
);

const input = 'w-full border border-border-strong rounded-md px-3 py-2 text-sm';

const AssignmentGradingSettings = ({
  value,
  onChange,
  submissionTypes = [],
  isGroupAssignment = false,
  graders = [], // [{ id, name }] teachers and TAs (final grader choices)
  courseFiles = [], // [{ id, display_name }] for the annotatable document
  showAvailability = true,
}) => {
  const types = Array.isArray(submissionTypes) ? submissionTypes : String(submissionTypes).split(',').map((t) => t.trim());
  const set = (patch) => onChange(patch);

  return (
    <div className="space-y-4">
      {showAvailability && (
        <fieldset className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <legend className="sr-only">Availability</legend>
          <Field id="grading-unlock-at" label="Available from" hint="Students can't open or submit before this date.">
            <input id="grading-unlock-at" type="datetime-local" value={value.unlock_at || ''} onChange={(e) => set({ unlock_at: e.target.value })} className={input} />
          </Field>
          <Field id="grading-lock-at" label="Until" hint="Submissions close at this date.">
            <input id="grading-lock-at" type="datetime-local" value={value.lock_at || ''} onChange={(e) => set({ lock_at: e.target.value })} className={input} />
          </Field>
        </fieldset>
      )}

      {types.includes('student_annotation') && (
        <Field id="grading-annotatable" label="Document to annotate" hint="Students mark up their own copy of this course file.">
          <select id="grading-annotatable" value={value.annotatable_attachment_id || ''} onChange={(e) => set({ annotatable_attachment_id: e.target.value })} className={input}>
            <option value="">Choose a course file...</option>
            {courseFiles.map((f) => (
              <option key={f.id} value={f.id}>{f.display_name || f.filename || `File ${f.id}`}</option>
            ))}
          </select>
        </Field>
      )}

      {types.includes('online_upload') && (
        <Field id="grading-extensions" label="Allowed file extensions" hint="Comma separated, e.g. pdf, docx. Leave empty to allow any file.">
          <input id="grading-extensions" type="text" value={value.allowed_extensions || ''} onChange={(e) => set({ allowed_extensions: e.target.value })} placeholder="pdf, docx" className={input} />
        </Field>
      )}

      <div className="flex flex-wrap items-center gap-4">
        <Check id="grading-limit-attempts" checked={value.limit_attempts} onChange={(v) => set({ limit_attempts: v })}>
          Limit submission attempts
        </Check>
        {value.limit_attempts && (
          <label htmlFor="grading-allowed-attempts" className="flex items-center gap-2 text-sm text-text-secondary">
            Allowed attempts
            <input id="grading-allowed-attempts" type="number" min="1" value={value.allowed_attempts} onChange={(e) => set({ allowed_attempts: e.target.value })} className="w-20 border border-border-strong rounded-md px-2 py-1 text-sm" />
          </label>
        )}
      </div>

      {isGroupAssignment && (
        <Check id="grading-individually" checked={value.grade_group_students_individually} onChange={(v) => set({ grade_group_students_individually: v })}>
          Assign grades to each student individually
        </Check>
      )}

      <Check id="grading-omit" checked={value.omit_from_final_grade} onChange={(v) => set({ omit_from_final_grade: v })}>
        Do not count this assignment towards the final grade
      </Check>

      <fieldset className="border border-border-default rounded-md p-3 space-y-3">
        <legend className="text-sm font-medium text-text-secondary px-1">Moderated grading</legend>
        <Check id="grading-moderated" checked={value.moderated_grading} onChange={(v) => set({ moderated_grading: v })}>
          Several graders give provisional grades; a moderator picks the final grade
        </Check>
        {value.moderated_grading && (
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 pl-6">
            <Field id="grading-grader-count" label="Number of graders">
              <input id="grading-grader-count" type="number" min="1" max="10" value={value.grader_count} onChange={(e) => set({ grader_count: e.target.value })} className={input} />
            </Field>
            <Field id="grading-final-grader" label="Moderator (final grader)">
              <select id="grading-final-grader" value={value.final_grader_id || ''} onChange={(e) => set({ final_grader_id: e.target.value })} className={input}>
                <option value="">Course teachers</option>
                {graders.map((g) => (
                  <option key={g.id} value={g.id}>{g.name}</option>
                ))}
              </select>
            </Field>
            <Check id="grading-comments-visible" checked={value.grader_comments_visible_to_graders} onChange={(v) => set({ grader_comments_visible_to_graders: v })}>
              Graders can see each other&apos;s comments
            </Check>
            <Check id="grading-graders-anon" checked={value.graders_anonymous_to_graders} onChange={(v) => set({ graders_anonymous_to_graders: v })}>
              Graders cannot see each other&apos;s names
            </Check>
            <Check id="grading-names-to-final" checked={value.grader_names_visible_to_final_grader} onChange={(v) => set({ grader_names_visible_to_final_grader: v })}>
              The moderator can see grader names
            </Check>
          </div>
        )}
      </fieldset>

      <fieldset className="border border-border-default rounded-md p-3 space-y-3">
        <legend className="text-sm font-medium text-text-secondary px-1">Peer reviews</legend>
        <Check id="grading-anon-peer" checked={value.anonymous_peer_reviews} onChange={(v) => set({ anonymous_peer_reviews: v })}>
          Reviewers don&apos;t see whose work they review
        </Check>
        <Field id="grading-peer-due" label="Peer reviews due">
          <input id="grading-peer-due" type="datetime-local" value={value.peer_reviews_due_at || ''} onChange={(e) => set({ peer_reviews_due_at: e.target.value })} className={input} />
        </Field>
      </fieldset>
    </div>
  );
};

export default AssignmentGradingSettings;
