// API calls for the discussions lane: the topic index with counts,
// search and paging, topic create/update with graded / group / section /
// anonymous options, rich replies with attachments and @mentions, and the
// graded-discussion lookups for the assignment page and SpeedGrader.
//
// Built on the shared request helper (`api.request`) so CSRF, 401
// handling and ApiError extraction match the rest of the client.

import { api, getCSRFToken, ApiError } from './api';

const API_URL = import.meta.env.VITE_API_URL || '/api/v1';

const json = (method, body) => ({ method, body: JSON.stringify(body) });

async function data(promise) {
  const { data: body } = await promise;
  return body;
}

async function postMultipart(path, formData) {
  const response = await fetch(`${API_URL}${path}`, {
    method: 'POST',
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

/** Page number named by a Link-header URL, or null. */
export function pageFromLink(link) {
  if (!link) return null;
  const match = String(link).match(/[?&]page=(\d+)/);
  return match ? Number(match[1]) : null;
}

/** Builds the topic index query string. */
export function topicIndexQuery({ page = 1, perPage = 20, search = '', filter = 'all' } = {}) {
  const params = new URLSearchParams();
  params.set('page', String(page));
  params.set('per_page', String(perPage));
  const term = search.trim();
  if (term) params.set('search_term', term);
  if (filter === 'unread') params.set('filter_by', 'unread');
  return params.toString();
}

export const discussionsApi = {
  /** One page of the index: `{ topics, page, lastPage, hasNext }`. */
  listTopics: async (courseId, opts = {}) => {
    const { data: topics, pagination = {} } = await api.request(
      `/courses/${courseId}/discussion_topics?${topicIndexQuery(opts)}`,
    );
    const page = opts.page || 1;
    const lastPage = pageFromLink(pagination.last) || page;
    return {
      topics: Array.isArray(topics) ? topics : [],
      page,
      lastPage,
      hasNext: Boolean(pagination.next),
    };
  },
  getTopic: (courseId, topicId) => data(api.request(`/courses/${courseId}/discussion_topics/${topicId}`)),
  createTopic: (courseId, topic) =>
    data(api.request(`/courses/${courseId}/discussion_topics`, json('POST', { discussion_topic: topic }))),
  updateTopic: (courseId, topicId, topic) =>
    data(api.request(`/courses/${courseId}/discussion_topics/${topicId}`, json('PUT', { discussion_topic: topic }))),

  /** Body: `{ message, attachment_id?, anonymous?, mentioned_user_ids? }`. */
  createEntry: (courseId, topicId, body) =>
    data(api.request(`/courses/${courseId}/discussion_topics/${topicId}/entries`, json('POST', body))),
  createReply: (courseId, topicId, entryId, body) =>
    data(api.request(`/courses/${courseId}/discussion_topics/${topicId}/entries/${entryId}/replies`, json('POST', body))),
  /** Body: `{ message, mentioned_user_ids? }`; keeps edit history. */
  updateEntry: (courseId, topicId, entryId, body) =>
    data(api.request(`/courses/${courseId}/discussion_topics/${topicId}/entries/${entryId}/v2`, json('PUT', body))),

  uploadAttachment: (courseId, topicId, file) => {
    const formData = new FormData();
    formData.append('file', file);
    return postMultipart(`/courses/${courseId}/discussion_topics/${topicId}/attachments`, formData);
  },
  mentionableUsers: (courseId, topicId, search = '') =>
    data(api.request(
      `/courses/${courseId}/discussion_topics/${topicId}/mentionable_users?search=${encodeURIComponent(search)}`,
    )),

  /** The graded discussion behind an assignment (404 when none). */
  assignmentTopic: (courseId, assignmentId) =>
    data(api.request(`/courses/${courseId}/assignments/${assignmentId}/discussion_topic`)),
  /** A student's posts on a graded discussion, for SpeedGrader. */
  studentEntries: (courseId, assignmentId, userId) =>
    data(api.request(`/courses/${courseId}/assignments/${assignmentId}/discussion_entries?user_id=${userId}`)),

  // Options for the topic form.
  sections: (courseId) => data(api.request(`/courses/${courseId}/sections?per_page=100`)),
  groupCategories: (courseId) => data(api.request(`/courses/${courseId}/group_categories?per_page=100`)),
  assignmentGroups: (courseId) => data(api.request(`/courses/${courseId}/assignment_groups?per_page=100`)),
};

export default discussionsApi;
