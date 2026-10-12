// API calls for the learner-outcomes lane: course analytics (G154), the
// student Learning Mastery view (G152), outcome management (G151) and
// ePortfolio additions (G155). Built on the shared helpers in api.js so the
// CSRF header, ApiError and 401 handling stay in one place.

import { api, getCSRFToken } from './api';

const API_URL = import.meta.env.VITE_API_URL || '/api/v1';

const json = async (path, options) => {
  const { data } = await api.request(path, options);
  return data;
};

const body = (method, payload) => ({ method, body: JSON.stringify(payload ?? {}) });

// ---------------------------------------------------------------------------
// Page views (G154). The tracker reports a course page when it opens, then
// the time it was visible when it closes (keepalive so the update survives
// the tab closing).
// ---------------------------------------------------------------------------

export async function recordPageView({ courseId, url, category }) {
  return json('/page_views', body('POST', {
    context_type: 'Course',
    context_id: Number(courseId),
    url,
    asset_category: category || '',
  }));
}

export function sendPageViewSeconds(pageViewId, seconds) {
  if (!pageViewId) return Promise.resolve();
  try {
    return fetch(`${API_URL}/page_views/${pageViewId}`, {
      method: 'PUT',
      credentials: 'include',
      keepalive: true,
      headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': getCSRFToken() },
      body: JSON.stringify({ interaction_seconds: Math.max(0, Math.round(seconds)) }),
    }).catch(() => undefined);
  } catch {
    return Promise.resolve();
  }
}

// ---------------------------------------------------------------------------
// Course analytics (G154)
// ---------------------------------------------------------------------------

export const analyticsApi = {
  courseActivity: (courseId, since) =>
    json(`/courses/${courseId}/analytics/activity${since ? `?since=${encodeURIComponent(since)}` : ''}`),
  courseCategories: (courseId) => json(`/courses/${courseId}/analytics/activity/categories`),
  courseAssignments: (courseId) => json(`/courses/${courseId}/analytics/assignments`),
  // Walks every page of student summaries (the endpoint pages by student).
  studentSummaries: async (courseId) => {
    const all = [];
    for (let page = 1; page <= 50; page += 1) {
      const { data, pagination } = await api.request(
        `/courses/${courseId}/analytics/student_summaries?page=${page}&per_page=100`,
      );
      const rows = Array.isArray(data) ? data : [];
      all.push(...rows);
      if (!pagination?.next || rows.length === 0) break;
    }
    return all;
  },
  studentActivity: (courseId, userId) => json(`/courses/${courseId}/analytics/users/${userId}/activity`),
  studentAssignments: (courseId, userId) => json(`/courses/${courseId}/analytics/users/${userId}/assignments`),
};

/**
 * Sends one private conversation per student (Canvas "Message students
 * who…" sends individual messages, so students never see each other).
 * Returns { sent, failed }.
 */
export async function messageStudentsIndividually(userIds, subject, message) {
  let sent = 0;
  const failed = [];
  for (const id of userIds) {
    try {
      const conv = await api.createConversation({ subject, recipients: [Number(id)] });
      await api.createConversationMessage(conv.id, message);
      sent += 1;
    } catch {
      failed.push(id);
    }
  }
  return { sent, failed };
}

// ---------------------------------------------------------------------------
// Learning mastery (G152 / G153)
// ---------------------------------------------------------------------------

export const masteryApi = {
  // userId: a number, or 'self'.
  studentMastery: (courseId, userId = 'self') => json(`/courses/${courseId}/outcome_mastery/users/${userId}`),
};

// ---------------------------------------------------------------------------
// Outcome management (G151)
// ---------------------------------------------------------------------------

export const outcomesApi = {
  updateOutcome: (courseId, outcomeId, patch) => json(`/courses/${courseId}/outcomes/${outcomeId}`, body('PUT', patch)),
  updateGroup: (courseId, groupId, patch) => json(`/courses/${courseId}/outcome_groups/${groupId}`, body('PUT', patch)),
  alignments: (courseId) => json(`/courses/${courseId}/outcome_alignments`),
  align: (courseId, outcomeId, assignmentId) =>
    json(`/courses/${courseId}/outcome_alignments`, body('POST', { learning_outcome_id: outcomeId, assignment_id: assignmentId })),
  unalign: (courseId, alignmentId) => json(`/courses/${courseId}/outcome_alignments/${alignmentId}`, { method: 'DELETE' }),
};

// ---------------------------------------------------------------------------
// ePortfolios (G155)
// ---------------------------------------------------------------------------

export const portfolioApi = {
  importableSubmissions: (portfolioId, courseId) =>
    json(`/portfolios/${portfolioId}/importable_submissions?course_id=${Number(courseId)}`),
  importSubmissions: (portfolioId, submissionIds, sectionId) =>
    json(`/portfolios/${portfolioId}/import`, body('POST', {
      submission_ids: submissionIds.map(Number),
      ...(sectionId ? { section_id: Number(sectionId) } : {}),
    })),
  deleteComment: (portfolioId, commentId) => json(`/portfolios/${portfolioId}/comments/${commentId}`, { method: 'DELETE' }),
  createShareLink: (portfolioId) => json(`/portfolios/${portfolioId}/share_link`, { method: 'POST' }),
  revokeShareLink: (portfolioId) => json(`/portfolios/${portfolioId}/share_link`, { method: 'DELETE' }),
  sharedPortfolio: (token) => json(`/portfolios/shared/${encodeURIComponent(token)}`),
};
