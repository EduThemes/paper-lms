// API calls for the account-administration lane (admin users, logins,
// observer links, reports, theme, admin tools, storage quotas, custom
// roles, SIS imports). Kept out of api.js so parallel work merges cleanly;
// it reuses api.request (CSRF, ApiError, 401 handling) from there.

import { api, ApiError, getCSRFToken } from './api';
import { getCurrentAccountId } from './currentAccount';

const API_URL = import.meta.env.VITE_API_URL || '/api/v1';

const acct = (accountId) => accountId ?? getCurrentAccountId();

async function get(path) {
  const { data } = await api.request(path);
  return data;
}

async function send(path, method, body) {
  const { data } = await api.request(path, {
    method,
    ...(body !== undefined ? { body: JSON.stringify(body) } : {}),
  });
  return data;
}

// Multipart upload (FormData). Mirrors api.js's multipart mode: no
// Content-Type header (the browser sets the boundary), CSRF header sent.
async function upload(path, formData) {
  const response = await fetch(`${API_URL}${path}`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'X-CSRF-Token': getCSRFToken() },
    body: formData,
  });
  const text = await response.text();
  const body = text ? JSON.parse(text) : null;
  if (!response.ok) {
    if (response.status === 401) window.dispatchEvent(new Event('auth:session-expired'));
    throw new ApiError({ status: response.status, body });
  }
  return body;
}

// URL helpers for plain links (downloads, images): same-origin, cookie auth.
export const adminUrls = {
  reportDownload: (report, id, accountId) =>
    `${API_URL}/accounts/${acct(accountId)}/reports/${report}/${id}/download`,
  brandLogo: (version) => `${API_URL}/brand_config/logo${version ? `?v=${version}` : ''}`,
};

export const adminAccountsApi = {
  // Users (G114)
  createUser: (user, accountId) => send(`/accounts/${acct(accountId)}/users`, 'POST', { user }),
  getUserDetail: (userId, accountId) => get(`/accounts/${acct(accountId)}/users/${userId}`),
  updateUser: (userId, user, accountId) => send(`/accounts/${acct(accountId)}/users/${userId}`, 'PUT', { user }),
  getUserHistory: (userId, accountId) => get(`/accounts/${acct(accountId)}/users/${userId}/history`),
  setSuspended: (userId, suspended) => send(`/users/${userId}/suspension`, 'PUT', { suspended }),
  bulkSetSuspended: (userIds, suspended) => send('/users/bulk_suspension', 'POST', { user_ids: userIds, suspended }),
  forcePasswordReset: (userId) => send(`/users/${userId}/force_password_reset`, 'PUT', {}),
  startMasquerade: (userId) => send(`/users/${userId}/masquerade`, 'POST'),

  // Logins (G115)
  createLogin: (userId, login, accountId) => send(`/accounts/${acct(accountId)}/users/${userId}/logins`, 'POST', { login }),
  updateLogin: (userId, loginId, login, accountId) =>
    send(`/accounts/${acct(accountId)}/users/${userId}/logins/${loginId}`, 'PUT', { login }),
  deleteLogin: (userId, loginId, accountId) =>
    send(`/accounts/${acct(accountId)}/users/${userId}/logins/${loginId}`, 'DELETE'),

  // Observer links (G121)
  linkObservee: (observerId, studentId, accountId) =>
    send(`/accounts/${acct(accountId)}/users/${observerId}/observees/${studentId}`, 'PUT'),
  unlinkObservee: (observerId, studentId, accountId) =>
    send(`/accounts/${acct(accountId)}/users/${observerId}/observees/${studentId}`, 'DELETE'),

  // Reports (G117)
  listReports: (accountId) => get(`/accounts/${acct(accountId)}/reports`),
  listReportRuns: (report, accountId) => get(`/accounts/${acct(accountId)}/reports/${report}`),
  startReport: (report, parameters, accountId) =>
    send(`/accounts/${acct(accountId)}/reports/${report}`, 'POST', { parameters }),
  getReportRun: (report, id, accountId) => get(`/accounts/${acct(accountId)}/reports/${report}/${id}`),
  deleteReportRun: (report, id, accountId) => send(`/accounts/${acct(accountId)}/reports/${report}/${id}`, 'DELETE'),

  // Theme (G118)
  getMyBrandConfig: () => get('/brand_config'),
  getBrandConfig: (accountId) => get(`/accounts/${acct(accountId)}/brand_config`),
  updateBrandConfig: (variables, accountId) => send(`/accounts/${acct(accountId)}/brand_config`, 'PUT', { variables }),
  resetBrandConfig: (accountId) => send(`/accounts/${acct(accountId)}/brand_config`, 'DELETE'),
  uploadBrandLogo: (file, accountId) => {
    const fd = new FormData();
    fd.append('logo', file);
    return upload(`/accounts/${acct(accountId)}/brand_config/logo`, fd);
  },
  deleteBrandLogo: (accountId) => send(`/accounts/${acct(accountId)}/brand_config/logo`, 'DELETE'),

  // Admin tools (G119)
  listDeletedCourses: (searchTerm = '', accountId) =>
    get(`/accounts/${acct(accountId)}/deleted_courses?search_term=${encodeURIComponent(searchTerm)}`),
  restoreCourse: (courseId, accountId) => send(`/accounts/${acct(accountId)}/deleted_courses/${courseId}/restore`, 'POST'),
  getAccountAuditLog: async (page = 1, perPage = 25, filters = {}, accountId) => {
    const params = new URLSearchParams({ page: String(page), per_page: String(perPage) });
    Object.entries(filters).forEach(([k, v]) => { if (v) params.set(k, v); });
    return api.request(`/accounts/${acct(accountId)}/audit_log?${params.toString()}`);
  },

  // Storage quotas (G120)
  getQuotaDefaults: (accountId) => get(`/accounts/${acct(accountId)}/storage_quotas`),
  updateQuotaDefaults: (defaults, accountId) => send(`/accounts/${acct(accountId)}/storage_quotas`, 'PUT', defaults),
  getContextQuota: (contextType, contextId, accountId) =>
    get(`/accounts/${acct(accountId)}/storage_quotas/${contextType}/${contextId}`),
  setContextQuota: (contextType, contextId, quotaMb, accountId) =>
    send(`/accounts/${acct(accountId)}/storage_quotas/${contextType}/${contextId}`, 'PUT', { quota_mb: quotaMb }),
  getCourseQuota: (courseId) => get(`/courses/${courseId}/storage_quota`),

  // Custom roles (G102)
  getBaseRolePermissions: (baseRoleType, accountId) =>
    get(`/accounts/${acct(accountId)}/role_base_permissions?base_role_type=${encodeURIComponent(baseRoleType)}`),
  getAssignableRoles: (courseId) => get(`/courses/${courseId}/assignable_roles`),
  getCoursePermissions: (courseId) => get(`/courses/${courseId}/permissions`),

  // SIS imports (G111)
  startSISImport: (file, options = {}, accountId) => {
    const fd = new FormData();
    fd.append('attachment', file);
    fd.append('import_type', 'instructure_csv');
    if (options.batchMode) fd.append('batch_mode', 'true');
    if (options.batchModeTermId) fd.append('batch_mode_term_id', String(options.batchModeTermId));
    if (options.diffingDataSetIdentifier) fd.append('diffing_data_set_identifier', options.diffingDataSetIdentifier);
    if (options.changeThreshold) fd.append('change_threshold', String(options.changeThreshold));
    return upload(`/accounts/${acct(accountId)}/sis_imports`, fd);
  },
  getSISImport: (id, accountId) => get(`/accounts/${acct(accountId)}/sis_imports/${id}`),
  getSISImportFiles: (id, accountId) => get(`/accounts/${acct(accountId)}/sis_imports/${id}/files`),
};
