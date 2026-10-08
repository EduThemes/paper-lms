// API calls for the notify-inbox lane: inbox state, attachments and
// forwarding (G092), the recipient address book (G093), announcement
// replies (G090) and the per-category x per-channel notification
// preferences (G091). Kept out of api.js so parallel work merges cleanly.

import { api, ApiError, getCSRFToken } from './api';

const API_URL = import.meta.env.VITE_API_URL || '/api/v1';

const json = (method, body) => ({ method, body: JSON.stringify(body) });

const query = (params) => {
  const qs = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => {
    if (v !== undefined && v !== null && v !== '') qs.set(k, String(v));
  });
  const s = qs.toString();
  return s ? `?${s}` : '';
};

async function data(promise) {
  const res = await promise;
  return res?.data;
}

export const inboxApi = {
  // GET /conversations with Canvas's scope / filter / search.
  listConversations: ({ scope = 'inbox', filter = '', search = '', page = 1, perPage = 20 } = {}) =>
    api.request(`/conversations${query({ scope: scope === 'inbox' ? '' : scope, filter, search, page, per_page: perPage })}`),

  getConversation: (id) => data(api.request(`/conversations/${id}`)),

  getMessages: (id, page = 1, perPage = 50) =>
    api.request(`/conversations/${id}/messages?page=${page}&per_page=${perPage}`),

  // { subject, recipients: [id | "course_5_students" | ...], body,
  //   attachment_ids, context_code, group_conversation }
  createConversation: (conversation) => data(api.request('/conversations', json('POST', { conversation }))),

  // { message, attachment_ids, recipients } — recipients restricts a
  // "Reply" to part of the conversation.
  addMessage: (id, message) => data(api.request(`/conversations/${id}/messages`, json('POST', message))),

  // { workflow_state: read|unread|archived, starred: bool }
  updateConversation: (id, conversation) =>
    data(api.request(`/conversations/${id}`, json('PUT', { conversation }))),

  batchUpdate: (conversationIds, event) =>
    data(api.request('/conversations', json('PUT', { conversation_ids: conversationIds, event }))),

  deleteConversation: (id) => data(api.request(`/conversations/${id}`, { method: 'DELETE' })),

  forward: (id, payload) => data(api.request(`/conversations/${id}/forward`, json('POST', payload))),

  markRead: (id) => data(api.request(`/conversations/${id}/mark_as_read`, { method: 'PUT' })),

  unreadCount: async () => (await data(api.request('/conversations/unread_count')))?.unread_count ?? 0,

  contexts: async () => (await data(api.request('/conversations/contexts'))) || [],

  // Address book search. context = course_5 | course_5_students |
  // section_3 | group_7 lists that context's members.
  searchRecipients: async ({ search = '', context = '', perPage = 10 } = {}) =>
    (await data(api.request(`/search/recipients${query({ search, context, per_page: perPage })}`))) || [],

  // Multipart upload of a message attachment into the caller's files.
  uploadAttachment: async (file) => {
    const form = new FormData();
    form.append('file', file);
    const response = await fetch(`${API_URL}/conversations/attachments`, {
      method: 'POST',
      body: form,
      credentials: 'include',
      headers: { 'X-CSRF-Token': getCSRFToken() },
    });
    if (!response.ok) {
      if (response.status === 401) window.dispatchEvent(new Event('auth:session-expired'));
      const body = await response.json().catch(() => ({}));
      throw new ApiError({ status: response.status, body });
    }
    return response.json();
  },
};

export const announcementRepliesApi = {
  list: async (announcementId, page = 1, perPage = 100) =>
    (await data(api.request(`/announcements/${announcementId}/replies?page=${page}&per_page=${perPage}`))) || [],
  create: (announcementId, message, parentId) =>
    data(api.request(`/announcements/${announcementId}/replies`, json('POST', { message, parent_id: parentId || undefined }))),
  update: (announcementId, replyId, message) =>
    data(api.request(`/announcements/${announcementId}/replies/${replyId}`, json('PUT', { message }))),
  remove: (announcementId, replyId) =>
    data(api.request(`/announcements/${announcementId}/replies/${replyId}`, { method: 'DELETE' })),
};

export const notificationPoliciesApi = {
  get: () => data(api.request('/users/self/notification_policies')),
  // policies: [{ channel_id, category, frequency }] — frequency "default"
  // clears the explicit cell.
  update: (policies) => data(api.request('/users/self/notification_policies', json('PUT', { policies }))),
  channelTypes: async () => (await data(api.request('/users/self/communication_channel_types'))) || [],
};
