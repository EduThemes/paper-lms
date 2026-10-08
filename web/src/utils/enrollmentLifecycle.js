// Pure helpers for the enrollment lifecycle UI (People page, Sections
// page, invitations). Mirrors the server rules in
// internal/api/v1/handlers/enrollment_lifecycle.go and
// internal/domain/models/enrollment_access.go so the UI only offers
// actions the API will accept.

export const ROLE_LABELS = {
  TeacherEnrollment: 'Teacher',
  TaEnrollment: 'TA',
  StudentEnrollment: 'Student',
  ObserverEnrollment: 'Observer',
  DesignerEnrollment: 'Designer',
};

export const ROLE_OPTIONS = [
  'StudentEnrollment',
  'TeacherEnrollment',
  'TaEnrollment',
  'DesignerEnrollment',
  'ObserverEnrollment',
];

const LEARNER_TYPES = new Set(['StudentEnrollment', 'ObserverEnrollment']);

/**
 * Whether the caller may add / manage an enrollment of `type`: teachers
 * and admins any role, TAs (and designers) only students and observers.
 * @param {{isAdmin?: boolean, isTeacher?: boolean, isStaff?: boolean}} caller
 */
export function canManageType(caller, type) {
  if (!caller) return false;
  if (caller.isAdmin || caller.isTeacher) return true;
  if (caller.isStaff) return LEARNER_TYPES.has(type);
  return false;
}

/** The role options the caller may assign. */
export function assignableRoles(caller) {
  return ROLE_OPTIONS.filter((t) => canManageType(caller, t));
}

const ACTIONS_BY_STATE = {
  active: ['edit', 'conclude', 'deactivate', 'delete'],
  invited: ['edit', 'delete'],
  inactive: ['edit', 'reactivate', 'conclude', 'delete'],
  completed: ['edit', 'reactivate', 'deactivate', 'delete'],
  rejected: ['delete'],
};

/**
 * Lifecycle actions the caller may take on an enrollment.
 * @param {object} enrollment  roster row ({type, user_id, enrollment_state})
 * @param {{userId?: number, isAdmin?: boolean, isTeacher?: boolean, isStaff?: boolean}} caller
 * @returns {string[]} subset of edit | conclude | deactivate | reactivate | delete
 */
export function enrollmentActions(enrollment, caller) {
  if (!enrollment || !canManageType(caller, enrollment.type)) return [];
  if (!caller.isAdmin && caller.userId != null && enrollment.user_id === caller.userId) return [];
  return ACTIONS_BY_STATE[enrollment.enrollment_state || 'active'] || [];
}

export const ACTION_LABELS = {
  edit: 'Edit role, section and dates',
  conclude: 'Conclude',
  deactivate: 'Deactivate',
  reactivate: 'Re-activate',
  delete: 'Remove from course',
};

export const ACTION_CONFIRM = {
  conclude: 'Conclude this enrollment? The person keeps read-only access to the course.',
  deactivate: 'Deactivate this enrollment? The person loses access but stays on the roster.',
  delete: 'Remove this person from the course? Their enrollment is deleted.',
};

/**
 * Display status for a roster row, applying the enrollment's own dates
 * (section / course / term dates are enforced server-side).
 * @returns {{key: string, label: string}}
 */
export function enrollmentStatus(enrollment, now = new Date()) {
  const state = enrollment?.enrollment_state || 'active';
  if (state === 'active' && enrollment?.start_at && enrollment?.end_at) {
    if (new Date(enrollment.end_at) < now) return { key: 'concluded', label: 'Concluded' };
    if (new Date(enrollment.start_at) > now) return { key: 'pending', label: 'Not started' };
  }
  switch (state) {
    case 'active': return { key: 'active', label: 'Active' };
    case 'invited': return { key: 'invited', label: 'Invited' };
    case 'inactive': return { key: 'inactive', label: 'Inactive' };
    case 'completed': return { key: 'concluded', label: 'Concluded' };
    case 'rejected': return { key: 'rejected', label: 'Declined' };
    default: return { key: state, label: state };
  }
}

/** Section name for a roster row; null section = the default section. */
export function sectionLabel(sectionsById, sectionId) {
  if (sectionId == null) return 'Default section';
  const s = sectionsById?.[sectionId];
  return s?.name || `Section ${sectionId}`;
}

/** `YYYY-MM-DD` for a date input from an ISO timestamp (UTC date). */
export function toDateInput(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  return d.toISOString().slice(0, 10);
}

/** ISO timestamp (UTC midnight) from a date input; '' -> null. */
export function fromDateInput(value) {
  if (!value) return null;
  const d = new Date(`${value}T00:00:00Z`);
  return Number.isNaN(d.getTime()) ? null : d.toISOString();
}

/** Form state for the edit dialog. */
export function enrollmentForm(enrollment) {
  return {
    type: enrollment?.type || 'StudentEnrollment',
    course_section_id: enrollment?.course_section_id != null ? String(enrollment.course_section_id) : '',
    start_at: toDateInput(enrollment?.start_at),
    end_at: toDateInput(enrollment?.end_at),
    limit_privileges_to_course_section: Boolean(enrollment?.limit_privileges_to_course_section),
  };
}

/**
 * The minimal PUT body for the edit dialog: only fields that changed.
 * Returns { patch, error } — error when the dates are reversed.
 */
export function buildEnrollmentPatch(enrollment, form) {
  const before = enrollmentForm(enrollment);
  const patch = {};
  if (form.type !== before.type) patch.type = form.type;
  if (form.course_section_id !== before.course_section_id) {
    patch.course_section_id = form.course_section_id ? Number(form.course_section_id) : null;
  }
  if (form.start_at !== before.start_at) patch.start_at = fromDateInput(form.start_at);
  if (form.end_at !== before.end_at) patch.end_at = fromDateInput(form.end_at);
  if (form.limit_privileges_to_course_section !== before.limit_privileges_to_course_section) {
    patch.limit_privileges_to_course_section = form.limit_privileges_to_course_section;
  }
  if (form.start_at && form.end_at && form.end_at < form.start_at) {
    return { patch, error: 'The end date must be after the start date.' };
  }
  return { patch, error: null };
}

/** People per section id (null / default section counted under 0). */
export function sectionPeopleCounts(enrollments = []) {
  const counts = {};
  for (const e of enrollments) {
    if (e.enrollment_state === 'deleted') continue;
    const key = e.course_section_id ?? 0;
    counts[key] = (counts[key] || 0) + 1;
  }
  return counts;
}

/** Courses a section can be cross-listed into (any but its own). */
export function crosslistTargets(courses = [], section) {
  if (!section) return [];
  return courses.filter((c) => c.id !== section.course_id && c.workflow_state !== 'deleted');
}

/** Roster filter chips → server states. */
export const STATUS_FILTERS = [
  { key: 'current', label: 'Current', states: ['active', 'invited'] },
  { key: 'inactive', label: 'Inactive', states: ['inactive'] },
  { key: 'concluded', label: 'Concluded', states: ['completed'] },
  { key: 'all', label: 'All', states: [] },
];
