// API calls for the pages / files / course-shell lane: page history,
// file management and visibility, storage quota, Student View.
//
// Built on the shared request helper (`api.request`) so CSRF, 401 handling
// and ApiError extraction match the rest of the client.

import { api, getCSRFToken, ApiError, requestPaged } from './api';

const API_URL = import.meta.env.VITE_API_URL || '/api/v1';

const json = (method, body) => ({ method, body: JSON.stringify(body) });

async function data(promise) {
  const { data: body } = await promise;
  return body;
}

// Multipart PUT/POST (the shared helper only does multipart POST).
async function sendMultipart(path, formData, method = 'POST') {
  const response = await fetch(`${API_URL}${path}`, {
    method,
    body: formData,
    credentials: 'include',
    headers: { 'X-CSRF-Token': getCSRFToken() },
  });
  if (!response.ok) {
    if (response.status === 401) window.dispatchEvent(new Event('auth:session-expired'));
    const body = await response.json().catch(() => ({}));
    throw new ApiError({ status: response.status, body });
  }
  const text = await response.text();
  return text ? JSON.parse(text) : null;
}

export const pagesFilesApi = {
  // ---- Page history (G065) ----
  listPageRevisions: (courseId, urlOrId, perPage = 50) =>
    data(api.request(`/courses/${courseId}/pages/${encodeURIComponent(urlOrId)}/revisions?per_page=${perPage}`)),
  getPageRevision: (courseId, urlOrId, revision = 'latest') =>
    data(api.request(`/courses/${courseId}/pages/${encodeURIComponent(urlOrId)}/revisions/${revision}`)),
  restorePageRevision: (courseId, urlOrId, revision) =>
    data(api.request(`/courses/${courseId}/pages/${encodeURIComponent(urlOrId)}/revisions/${revision}`, { method: 'POST' })),

  // ---- Files and folders (G070 / G071) ----
  // The folder tree: every live folder of the course (flat; build the
  // tree client-side with buildFolderTree). requestPaged walks past the
  // 100-per-page cap.
  listCourseFolderTree: (courseId) => data(requestPaged(`/courses/${courseId}/folders`, 1, 5000)),
  // Files directly inside a folder (null = the course root).
  listFolderFiles: (courseId, folderId, page = 1, perPage = 50) =>
    api.request(`/courses/${courseId}/files?folder_id=${folderId ?? 'root'}&page=${page}&per_page=${perPage}`),
  listChildFolders: (courseId, folderId) =>
    data(api.request(`/courses/${courseId}/folders?parent_folder_id=${folderId ?? 'root'}&per_page=100`)),
  updateFile: (courseId, fileId, fields) =>
    data(api.request(`/courses/${courseId}/files/${fileId}`, json('PUT', fields))),
  replaceFile: (courseId, fileId, file) => {
    const fd = new FormData();
    fd.append('file', file);
    return sendMultipart(`/courses/${courseId}/files/${fileId}/replace`, fd, 'POST');
  },
  updateFolder: (folderId, fields) => data(api.request(`/folders/${folderId}`, json('PUT', fields))),
  getCourseQuota: (courseId) => data(api.request(`/courses/${courseId}/files/quota`)),

  // ---- Student View (G076) ----
  enterStudentView: (courseId) => data(api.request(`/courses/${courseId}/student_view`, { method: 'POST' })),
  resetStudentView: (courseId) => data(api.request(`/courses/${courseId}/student_view`, { method: 'DELETE' })),
  leaveStudentView: () => data(api.request('/masquerade', { method: 'DELETE' })),
  getStudentViewStatus: () => data(api.request('/users/self/student_view')),
};

export default pagesFilesApi;
