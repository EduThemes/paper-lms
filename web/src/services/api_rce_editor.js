import { api } from './api';

/**
 * API calls owned by the rich content editor (routes_rce_editor.go).
 */

/**
 * The caller's autosave draft key: { key: base64 AES-256 key, algorithm,
 * user_id }. Derived server-side per user; never cached by HTTP.
 */
export async function fetchRceDraftKey() {
  const { data } = await api.request('/users/self/rce_draft_key', { cache: 'no-store' });
  return data;
}

export const rceEditorApi = { fetchRceDraftKey };
export default rceEditorApi;
