// Pure helpers for the content-migration UIs (course copy, package
// import, course export, blueprint locks). Kept free of React so the
// request shapes can be unit-tested.

// Copyable content types, in the order the selective-copy picker lists
// them. Keys match the backend's copy types (service.CopyType*).
export const COPY_TYPES = [
  { key: 'course_settings', label: 'Course settings', itemized: false },
  { key: 'syllabus_body', label: 'Syllabus body', itemized: false },
  { key: 'modules', label: 'Modules', itemized: true },
  { key: 'assignments', label: 'Assignments', itemized: true },
  { key: 'assignment_groups', label: 'Assignment groups', itemized: true },
  { key: 'quizzes', label: 'Quizzes', itemized: true },
  { key: 'pages', label: 'Pages', itemized: true },
  { key: 'discussion_topics', label: 'Discussions', itemized: true },
  { key: 'files', label: 'Files', itemized: true },
  { key: 'calendar_events', label: 'Calendar events', itemized: true },
  { key: 'rubrics', label: 'Rubrics', itemized: true },
];

// Types a package import can include or exclude (packages are selected
// by type only: their ids are unknown until parsed).
export const PACKAGE_TYPES = COPY_TYPES.filter((t) => !['course_settings', 'syllabus_body'].includes(t.key));

// defaultDateShift is the initial date-shift form: dates kept, the
// browser's time zone remembered for when the teacher shifts them.
export function defaultDateShift() {
  let timeZone = '';
  try {
    timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone || '';
  } catch {
    timeZone = '';
  }
  return { mode: 'keep', oldStart: '', oldEnd: '', newStart: '', newEnd: '', substitutions: [], timeZone };
}

export const WEEKDAYS = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];

export function copyTypeLabel(key) {
  const found = COPY_TYPES.find((t) => t.key === key);
  if (found) return found.label;
  if (key === 'module_items') return 'Module items';
  if (key === 'quiz_questions') return 'Quiz questions';
  if (key === 'dates_shifted') return 'Dates shifted';
  return String(key || '').replace(/_/g, ' ');
}

// emptySelection is the picker state: per type, either every item
// ("all": true) or a set of item ids.
export function emptySelection() {
  return {};
}

export function toggleType(selection, type, on) {
  const next = { ...selection };
  if (on) next[type] = { all: true, ids: [] };
  else delete next[type];
  return next;
}

export function toggleItem(selection, type, id, on, allIds = []) {
  const current = selection[type] || { all: false, ids: [] };
  let ids = current.all ? [...allIds] : [...current.ids];
  if (on && !ids.includes(id)) ids.push(id);
  if (!on) ids = ids.filter((x) => x !== id);
  const next = { ...selection };
  if (ids.length === 0) delete next[type];
  else if (allIds.length > 0 && ids.length === allIds.length) next[type] = { all: true, ids: [] };
  else next[type] = { all: false, ids };
  return next;
}

export function isItemSelected(selection, type, id) {
  const s = selection[type];
  return !!s && (s.all || s.ids.includes(id));
}

export function selectionCount(selection) {
  return Object.keys(selection).length;
}

// toSelectSettings turns the picker state into the migration settings
// keys the backend reads: include_only_types for whole types, select for
// individual items.
export function toSelectSettings(selection) {
  const include = [];
  const select = {};
  for (const [type, s] of Object.entries(selection)) {
    if (s.all) include.push(type);
    else if (s.ids.length > 0) select[type] = [...s.ids];
  }
  const out = {};
  if (include.length > 0) out.include_only_types = include;
  if (Object.keys(select).length > 0) out.select = select;
  return out;
}

// buildDateShiftOptions maps the date-shift form onto the API's
// date_shift_options (or null when dates are kept as they are).
//   form = { mode: 'keep'|'shift'|'remove', oldStart, oldEnd, newStart,
//            newEnd, substitutions: [{from, to}], timeZone }
export function buildDateShiftOptions(form) {
  if (!form || form.mode === 'keep' || !form.mode) return null;
  if (form.mode === 'remove') return { remove_dates: true };
  const out = { shift_dates: true };
  if (form.oldStart) out.old_start_date = form.oldStart;
  if (form.oldEnd) out.old_end_date = form.oldEnd;
  if (form.newStart) out.new_start_date = form.newStart;
  if (form.newEnd) out.new_end_date = form.newEnd;
  const subs = {};
  for (const s of form.substitutions || []) {
    if (s && s.from !== '' && s.to !== '' && s.from != null && s.to != null) {
      subs[String(s.from)] = String(s.to);
    }
  }
  if (Object.keys(subs).length > 0) out.day_substitutions = subs;
  if (form.timeZone) out.time_zone = form.timeZone;
  return out;
}

// dateShiftProblem returns a message when the shift form cannot work, or
// null when it is fine to submit.
export function dateShiftProblem(form) {
  if (!form || form.mode !== 'shift') return null;
  if (!form.newStart) return 'Pick the new start date.';
  if (form.oldEnd && form.oldStart && form.oldEnd < form.oldStart) return 'The old end date is before the old start date.';
  if (form.newEnd && form.newEnd < form.newStart) return 'The new end date is before the new start date.';
  return null;
}

// buildCourseCopyRequest builds the POST /content_migrations body.
export function buildCourseCopyRequest({ sourceCourseId, mode, selection, dateShift }) {
  const settings = { source_course_id: Number(sourceCourseId) };
  if (mode === 'select') Object.assign(settings, toSelectSettings(selection || {}));
  else settings.copy_everything = true;
  const body = { migration_type: 'course_copy', settings };
  const dso = buildDateShiftOptions(dateShift);
  if (dso) body.date_shift_options = dso;
  return body;
}

// buildPackageSettings builds the "settings" form field of a package
// import (type selection + date shifting), or null when defaulted.
export function buildPackageSettings({ excludeTypes = [], dateShift } = {}) {
  const out = {};
  if (excludeTypes.length > 0) out.exclude_types = [...excludeTypes];
  const dso = buildDateShiftOptions(dateShift);
  if (dso) out.date_shift_options = dso;
  return Object.keys(out).length > 0 ? out : null;
}

const ACTIVE_STATES = new Set(['created', 'queued', 'pre_processing', 'running', 'exporting']);

export function isJobActive(job) {
  return !!job && ACTIVE_STATES.has(job.workflow_state);
}

export function anyJobActive(jobs) {
  return Array.isArray(jobs) && jobs.some(isJobActive);
}

// summarizeCounts renders migration/export result counts as
// "3 assignments, 1 page" style text (zero counts omitted).
export function summarizeCounts(counts) {
  if (!counts) return '';
  return Object.entries(counts)
    .filter(([, n]) => Number(n) > 0)
    .map(([k, n]) => `${n} ${copyTypeLabel(k).toLowerCase()}`)
    .join(', ');
}

// exportCounts reads the exporter's result shape.
export function exportCounts(results) {
  if (!results) return {};
  return {
    modules: results.modules_exported || 0,
    pages: results.pages_exported || 0,
    assignments: results.assignments_exported || 0,
    quizzes: results.quizzes_exported || 0,
    quiz_questions: results.questions_exported || 0,
    discussion_topics: results.discussions_exported || 0,
  };
}

export function formatBytes(n) {
  const v = Number(n) || 0;
  if (v < 1024) return `${v} B`;
  if (v < 1024 * 1024) return `${(v / 1024).toFixed(1)} KB`;
  return `${(v / (1024 * 1024)).toFixed(1)} MB`;
}

// ---------------------------------------------------------- blueprints

export const RESTRICTION_KEYS = [
  { key: 'content', label: 'Content' },
  { key: 'points', label: 'Points' },
  { key: 'due_dates', label: 'Due dates' },
  { key: 'availability_dates', label: 'Availability dates' },
];

export function parseRestrictions(value) {
  let obj = value;
  if (typeof value === 'string') {
    try {
      obj = JSON.parse(value || '{}');
    } catch {
      obj = {};
    }
  }
  const out = {};
  for (const { key } of RESTRICTION_KEYS) out[key] = !!(obj && obj[key]);
  return out;
}

export function restrictionsToString(r) {
  return JSON.stringify(parseRestrictions(r));
}

export function anyRestriction(r) {
  const p = parseRestrictions(r);
  return RESTRICTION_KEYS.some(({ key }) => p[key]);
}

export function describeRestrictions(r) {
  const p = parseRestrictions(r);
  const on = RESTRICTION_KEYS.filter(({ key }) => p[key]).map(({ label }) => label.toLowerCase());
  return on.length ? `Locked: ${on.join(', ')}` : 'Unlocked';
}

// blueprintTypeParam maps Paper content types onto restrict_item's
// content_type values.
export function blueprintTypeParam(contentType) {
  switch (contentType) {
    case 'Assignment':
      return 'assignment';
    case 'Quiz':
      return 'quiz';
    case 'WikiPage':
      return 'wiki_page';
    case 'DiscussionTopic':
      return 'discussion_topic';
    default:
      return String(contentType || '').toLowerCase();
  }
}

export function blueprintTypeLabel(contentType) {
  switch (contentType) {
    case 'Assignment':
      return 'Assignment';
    case 'Quiz':
      return 'Quiz';
    case 'WikiPage':
      return 'Page';
    case 'DiscussionTopic':
      return 'Discussion';
    default:
      return contentType;
  }
}
