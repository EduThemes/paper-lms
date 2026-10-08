// Canvas REST API compatibility surface (the "api" parity lane):
// account course/user lists, course rosters, the
// multi-student submissions feed and personal access token lifecycle.
// Kept out of api.js so the lane's calls merge cleanly.

import { api, requestPaged } from './api';

// buildCanvasQuery turns {include: ['term'], search_term: 'bio', page: 2}
// into Canvas's query form: arrays become repeated name[]= pairs, empty
// values are dropped.
export function buildCanvasQuery(params = {}) {
  const parts = [];
  Object.entries(params).forEach(([key, value]) => {
    if (value === undefined || value === null || value === '') return;
    if (Array.isArray(value)) {
      value
        .filter((v) => v !== undefined && v !== null && v !== '')
        .forEach((v) => parts.push(`${encodeURIComponent(`${key}[]`)}=${encodeURIComponent(v)}`));
      return;
    }
    parts.push(`${encodeURIComponent(key)}=${encodeURIComponent(value)}`);
  });
  return parts.length ? `?${parts.join('&')}` : '';
}

// Account reports (G117/G138) live in api_admin_accounts.js; one
// implementation serves the admin page and the Canvas API.

// --- Account / course collections ------------------------------------------

export function listAccountCourses(accountId = 'self', params = {}) {
  const { per_page: perPage = 100, ...rest } = params;
  return requestPaged(`/accounts/${accountId}/courses${buildCanvasQuery(rest)}`, 1, perPage);
}

export async function listAccountUsers(accountId = 'self', params = {}) {
  const { data } = await api.request(`/accounts/${accountId}/users${buildCanvasQuery({ per_page: 20, ...params })}`);
  return Array.isArray(data) ? data : [];
}

export async function listCourseUsers(courseId, params = {}) {
  const { data } = await api.request(`/courses/${courseId}/users${buildCanvasQuery({ per_page: 100, ...params })}`);
  return Array.isArray(data) ? data : [];
}

export async function listStudentSubmissions(courseId, params = {}) {
  const { data } = await api.request(`/courses/${courseId}/students/submissions${buildCanvasQuery({ per_page: 100, ...params })}`);
  return Array.isArray(data) ? data : [];
}

// --- Personal access tokens ------------------------------------------------

export async function listUserTokens(userId) {
  const { data } = await api.request(`/users/${userId}/tokens?per_page=100`);
  return Array.isArray(data) ? data : [];
}

export async function updateAccessToken(userId, tokenId, token) {
  const { data } = await api.request(`/users/${userId}/tokens/${tokenId}`, {
    method: 'PUT',
    body: JSON.stringify({ token }),
  });
  return data;
}

export async function deleteUserToken(userId, tokenId) {
  const { data } = await api.request(`/users/${userId}/tokens/${tokenId}`, { method: 'DELETE' });
  return data;
}

// tokenStatus summarises a token for display: 'expired', 'expiring' (within
// seven days), 'active', or 'integration' for an OAuth app's token.
export function tokenStatus(token, now = Date.now()) {
  if (!token) return 'active';
  if (token.kind === 'oauth') return 'integration';
  if (token.workflow_state === 'expired') return 'expired';
  if (token.expires_at) {
    const at = new Date(token.expires_at).getTime();
    if (at <= now) return 'expired';
    if (at - now <= 7 * 24 * 60 * 60 * 1000) return 'expiring';
  }
  return 'active';
}

// expiryToISO turns a yyyy-mm-dd date input into the end of that day in the
// user's time zone (what "expires on" means to them), as an ISO string.
export function expiryToISO(dateValue) {
  if (!dateValue) return undefined;
  const [y, m, d] = dateValue.split('-').map(Number);
  if (!y || !m || !d) return undefined;
  return new Date(y, m - 1, d, 23, 59, 59).toISOString();
}
