// API calls for the grading-UI features (anonymous grading, moderated
// grading, custom grade statuses, comment files / drafts / group
// comments, student annotation, peer-review management, gradebook
// helpers). Kept out of api.js so the lanes merge cleanly; it reuses the
// shared request helper (api.request) and the CSRF / ApiError contract.

import { api, ApiError, getCSRFToken } from './api';

const API_URL = import.meta.env.VITE_API_URL || '/api/v1';

const req = (path, options) => api.request(path, options);
const json = (method, body) => ({ method, body: JSON.stringify(body) });
const data = async (p) => (await p).data;

// Multipart upload with the same CSRF header / error envelope as api.js.
async function uploadMultipart(path, file) {
  const formData = new FormData();
  formData.append('file', file);
  const response = await fetch(`${API_URL}${path}`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'X-CSRF-Token': getCSRFToken() },
    body: formData,
  });
  if (!response.ok) {
    if (response.status === 401) window.dispatchEvent(new Event('auth:session-expired'));
    const body = await response.json().catch(() => ({}));
    throw new ApiError({ status: response.status, body });
  }
  return response.json();
}

// Path segment that addresses a submission: by user id, or by anonymous
// id while the assignment anonymizes students.
export const submissionKeyPath = ({ anonymousId, userId }) =>
  anonymousId ? `anonymous_submissions/${anonymousId}` : `submissions/${userId}`;

const base = (courseId, assignmentId) => `/courses/${courseId}/assignments/${assignmentId}`;

export const gradingApi = {
  // --- grading a submission (user or anonymous addressing) -------------
  updateSubmission: (courseId, assignmentId, key, submission) =>
    data(req(`${base(courseId, assignmentId)}/${submissionKeyPath(key)}`, json('PUT', { submission }))),

  listComments: async (courseId, assignmentId, key) =>
    (await data(req(`${base(courseId, assignmentId)}/${submissionKeyPath(key)}/comments`))) || [],

  createComment: (courseId, assignmentId, key, comment) =>
    data(req(`${base(courseId, assignmentId)}/${submissionKeyPath(key)}/comments`, json('POST', { comment }))),

  // Comment edit / delete / publish-draft, by user id or (graders, under
  // anonymous grading) by anonymous id.
  updateComment: (courseId, assignmentId, key, commentId, comment) =>
    data(req(`${base(courseId, assignmentId)}/${submissionKeyPath(key)}/comments/${commentId}`, json('PUT', { comment }))),

  deleteComment: (courseId, assignmentId, key, commentId) =>
    data(req(`${base(courseId, assignmentId)}/${submissionKeyPath(key)}/comments/${commentId}`, { method: 'DELETE' })),

  uploadCommentFile: (courseId, assignmentId, key, file) =>
    uploadMultipart(`${base(courseId, assignmentId)}/${submissionKeyPath(key)}/comment_files`, file),

  // --- rubric assessment by anonymous id (anonymous grading) -----------
  getAnonymousRubricAssessment: async (courseId, assignmentId, anonymousId) =>
    (await data(req(`${base(courseId, assignmentId)}/anonymous_submissions/${anonymousId}/rubric_assessment`)))?.rubric_assessment || null,
  saveAnonymousRubricAssessment: async (courseId, assignmentId, anonymousId, assessmentData) =>
    (await data(req(`${base(courseId, assignmentId)}/anonymous_submissions/${anonymousId}/rubric_assessment`,
      json('PUT', { rubric_assessment: { data: assessmentData } }))))?.rubric_assessment || null,

  // --- student annotation ----------------------------------------------
  startAnnotation: (courseId, assignmentId) =>
    data(req(`${base(courseId, assignmentId)}/annotation_session`, { method: 'POST' })),
  annotatableDocumentUrl: (courseId, assignmentId) =>
    `${API_URL}${base(courseId, assignmentId)}/annotatable_document`,

  // --- moderated grading -----------------------------------------------
  saveProvisionalGrade: (courseId, assignmentId, { userId, anonymousId, postedGrade, final = false }) =>
    data(req(`${base(courseId, assignmentId)}/provisional_grades`, json('PUT', {
      ...(anonymousId ? { anonymous_id: anonymousId } : { user_id: userId }),
      posted_grade: postedGrade,
      final,
    }))),
  getModeration: (courseId, assignmentId) => data(req(`${base(courseId, assignmentId)}/moderation`)),
  selectProvisionalGrade: (courseId, assignmentId, { userId, anonymousId, provisionalGradeId }) =>
    data(req(`${base(courseId, assignmentId)}/moderation/selections`, json('PUT', {
      ...(anonymousId ? { anonymous_id: anonymousId } : { user_id: userId }),
      provisional_grade_id: provisionalGradeId,
    }))),
  publishModeratedGrades: (courseId, assignmentId) =>
    data(req(`${base(courseId, assignmentId)}/moderation/publish`, { method: 'POST' })),

  // --- custom grade statuses -------------------------------------------
  listAccountGradeStatuses: async (accountId) =>
    (await data(req(`/accounts/${accountId}/custom_grade_statuses`))) || [],
  createGradeStatus: (accountId, status) =>
    data(req(`/accounts/${accountId}/custom_grade_statuses`, json('POST', { custom_grade_status: status }))),
  updateGradeStatus: (accountId, id, status) =>
    data(req(`/accounts/${accountId}/custom_grade_statuses/${id}`, json('PUT', { custom_grade_status: status }))),
  deleteGradeStatus: (accountId, id) =>
    data(req(`/accounts/${accountId}/custom_grade_statuses/${id}`, { method: 'DELETE' })),
  listCourseGradeStatuses: async (courseId) =>
    (await data(req(`/courses/${courseId}/custom_grade_statuses`))) || [],

  // --- gradebook helpers -----------------------------------------------
  listCourseGradingPeriods: async (courseId) =>
    (await data(req(`/courses/${courseId}/grading_periods`))) || [],

  // --- peer reviews ----------------------------------------------------
  assignPeerReview: (courseId, assignmentId, reviewerId, revieweeId) =>
    data(req(`/courses/${courseId}/assignments/${assignmentId}/peer_reviews/manual`, json('POST', { reviewer_id: reviewerId, reviewee_id: revieweeId }))),
  deletePeerReview: (courseId, assignmentId, reviewId) =>
    data(req(`/courses/${courseId}/assignments/${assignmentId}/peer_reviews/${reviewId}`, { method: 'DELETE' })),
  listReceivedPeerReviews: async (courseId, assignmentId) =>
    (await data(req(`/courses/${courseId}/assignments/${assignmentId}/peer_reviews/received`))) || [],
  submitPeerReview: (reviewId, { score, comments, rubricData }) =>
    data(req(`/peer_reviews/${reviewId}`, json('PUT', {
      ...(score !== undefined && score !== null && score !== '' ? { score: Number(score) } : {}),
      comments: comments || '',
      ...(rubricData && Object.keys(rubricData).length ? { rubric_data: rubricData } : {}),
    }))),
};

export default gradingApi;
