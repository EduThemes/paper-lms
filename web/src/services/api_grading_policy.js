// Grading-policy lane API calls: server-computed grades, effective due
// dates, final grade overrides, post policies, grading schemes, late /
// missing statuses, the missing-submission policy and submission history.
//
// Kept apart from api.js so this lane merges without touching the shared
// client; it reuses its CSRF cookie and ApiError contract.

import { ApiError, getCSRFToken } from './api';

const API_URL = import.meta.env.VITE_API_URL || '/api/v1';

async function call(path, { method = 'GET', body } = {}) {
  const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': getCSRFToken() };
  const response = await fetch(`${API_URL}${path}`, {
    method,
    credentials: 'include',
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!response.ok) {
    if (response.status === 401) window.dispatchEvent(new Event('auth:session-expired'));
    const errBody = await response.json().catch(() => ({}));
    throw new ApiError({ status: response.status, body: errBody });
  }
  if (response.status === 204) return null;
  const text = await response.text();
  return text && text.trim() ? JSON.parse(text) : null;
}

const qs = (params) => {
  const parts = Object.entries(params || {})
    .filter(([, v]) => v !== undefined && v !== null && v !== '' && !(Array.isArray(v) && v.length === 0))
    .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(Array.isArray(v) ? v.join(',') : v)}`);
  return parts.length ? `?${parts.join('&')}` : '';
};

export const gradingPolicyApi = {
  // Course grades computed by the server (drop rules, excused, overrides,
  // grading periods, final grade overrides). Staff may pass user_ids; a
  // student gets only their own posted grades; an observer passes user_id.
  getCourseGrades: (courseId, { userId, userIds } = {}) =>
    call(`/courses/${courseId}/grades${qs({ user_id: userId, user_ids: userIds })}`),

  // The caller's (or an observee's) effective dates per assignment.
  getAssignmentDates: (courseId, userId) =>
    call(`/courses/${courseId}/assignment_dates${qs({ user_id: userId })}`),

  // Staff: assignment id -> user id -> effective dates + grading period.
  getEffectiveDueDates: (courseId, { assignmentIds, userIds } = {}) =>
    call(`/courses/${courseId}/effective_due_dates${qs({ assignment_ids: assignmentIds, user_ids: userIds })}`),

  // Final grade override: { override_score } (percent) or
  // { override_grade } (scheme grade); null clears.
  setScoreOverride: (courseId, userId, { overrideScore, overrideGrade, gradingPeriodId } = {}) => {
    const body = { grading_period_id: gradingPeriodId || 0 };
    if (overrideGrade !== undefined) body.override_grade = overrideGrade;
    else body.override_score = overrideScore ?? null;
    return call(`/courses/${courseId}/students/${userId}/score_override`, { method: 'PUT', body });
  },

  getPostPolicy: (courseId) => call(`/courses/${courseId}/post_policy`),
  setPostPolicy: (courseId, postManually) =>
    call(`/courses/${courseId}/post_policy`, { method: 'PUT', body: { post_manually: !!postManually } }),

  // Post / hide grades: { gradedOnly, sectionIds }.
  postGrades: (courseId, assignmentId, { gradedOnly = false, sectionIds = [] } = {}) =>
    call(`/courses/${courseId}/assignments/${assignmentId}/post_grades`, {
      method: 'POST',
      body: { graded_only: !!gradedOnly, section_ids: sectionIds },
    }),
  hideGrades: (courseId, assignmentId, { sectionIds = [] } = {}) =>
    call(`/courses/${courseId}/assignments/${assignmentId}/hide_grades`, {
      method: 'POST',
      body: { section_ids: sectionIds },
    }),

  getCourseGradingSchemes: (courseId) => call(`/courses/${courseId}/grading_schemes`),
  setCourseGradingScheme: (courseId, { gradingStandardId, enabled } = {}) => {
    const body = {};
    if (gradingStandardId !== undefined) body.grading_standard_id = gradingStandardId || null;
    if (enabled !== undefined) body.grading_standard_enabled = !!enabled;
    return call(`/courses/${courseId}/grading_scheme`, { method: 'PUT', body });
  },

  // Manual status: 'late' | 'missing' | 'extended' | 'none' | null (computed).
  setLatePolicyStatus: (courseId, assignmentId, userId, status, secondsLateOverride) =>
    call(`/courses/${courseId}/assignments/${assignmentId}/submissions/${userId}/late_policy_status`, {
      method: 'PUT',
      body: { late_policy_status: status ?? null, seconds_late_override: secondsLateOverride ?? null },
    }),

  setExcused: (courseId, assignmentId, userId, excused) =>
    call(`/courses/${courseId}/assignments/${assignmentId}/submissions/${userId}`, {
      method: 'PUT',
      body: { submission: { excused: !!excused } },
    }),

  applyLatePolicy: (courseId) => call(`/courses/${courseId}/late_policy/apply`, { method: 'POST' }),

  getSubmissionHistory: (courseId, assignmentId, userId) =>
    call(`/courses/${courseId}/assignments/${assignmentId}/submissions/${userId}/history`),

  // Assignment group weight + drop rules.
  updateAssignmentGroup: (courseId, groupId, { groupWeight, rules, name } = {}) => {
    const group = {};
    if (groupWeight !== undefined) group.group_weight = Number(groupWeight) || 0;
    if (rules !== undefined) group.rules = rules;
    if (name !== undefined) group.name = name;
    return call(`/courses/${courseId}/assignment_groups/${groupId}`, { method: 'PUT', body: { assignment_group: group } });
  },

  // Account grading schemes.
  listAccountGradingStandards: (accountId) => call(`/accounts/${accountId}/grading_standards`),
  createAccountGradingStandard: (accountId, standard) =>
    call(`/accounts/${accountId}/grading_standards`, { method: 'POST', body: { grading_standard: standard } }),
  updateAccountGradingStandard: (accountId, id, standard) =>
    call(`/accounts/${accountId}/grading_standards/${id}`, { method: 'PUT', body: { grading_standard: standard } }),
  deleteAccountGradingStandard: (accountId, id) =>
    call(`/accounts/${accountId}/grading_standards/${id}`, { method: 'DELETE' }),

  // Accommodation extensions (G040): applies each listed student's
  // extra-day accommodation on top of their override dates and returns
  // the adjusted dates.
  applyAccommodations: (courseId, assignmentId, userIds) =>
    call(`/courses/${courseId}/assignments/${assignmentId}/apply_accommodations`, {
      method: 'POST',
      body: { user_ids: userIds },
    }),

  // Course sections and students for pickers.
  getSections: (courseId) => call(`/courses/${courseId}/sections?per_page=100`),
};

// Grading types an assignment can use (Canvas).
export const GRADING_TYPES = [
  { value: 'points', label: 'Points' },
  { value: 'percent', label: 'Percentage' },
  { value: 'letter_grade', label: 'Letter grade' },
  { value: 'gpa_scale', label: 'GPA scale' },
  { value: 'pass_fail', label: 'Complete / incomplete' },
  { value: 'not_graded', label: 'Not graded' },
];

// Manual submission statuses.
export const LATE_STATUSES = [
  { value: null, label: 'Automatic' },
  { value: 'none', label: 'None' },
  { value: 'late', label: 'Late' },
  { value: 'missing', label: 'Missing' },
  { value: 'extended', label: 'Extended' },
];

// formatScore renders a percentage (or null) for display.
export function formatPercent(v) {
  if (v === null || v === undefined || Number.isNaN(Number(v))) return '—';
  return `${Number(v).toFixed(2).replace(/\.?0+$/, '')}%`;
}

export default gradingPolicyApi;
