// Helpers for entries returned by GET .../discussion_topics/:id/view_v2.

/**
 * The view_v2 API sends read_state as a boolean (true = read). Deleted
 * placeholders are never "unread".
 */
export function isEntryUnread(entry) {
  return entry?.read_state === false && entry?.workflow_state !== 'deleted';
}

/** A soft-deleted entry kept as a placeholder because it still has replies. */
export function isEntryDeleted(entry) {
  return entry?.workflow_state === 'deleted';
}

/**
 * Edit history follows Canvas: the first edit stores the original as
 * version 1 and the edit as version 2, so any stored version means the
 * post was edited. Rows versioned under the older scheme carry a single
 * version after one edit, which also counts.
 */
export function wasEntryEdited(entry) {
  return (entry?.version_count || 0) > 0;
}

/**
 * api.js list helpers resolve to `{ data, pagination }`. Returns the list
 * inside (or the value itself when it is already an array), and [] for
 * anything else, so callers can always .map/.length the result.
 */
export function unwrapList(result) {
  if (Array.isArray(result)) return result;
  if (Array.isArray(result?.data)) return result.data;
  return [];
}

/** Number of unread entries in a (nested) entry tree. */
export function countUnreadEntries(entries) {
  let count = 0;
  for (const entry of entries || []) {
    if (isEntryUnread(entry)) count++;
    if (entry.replies) count += countUnreadEntries(entry.replies);
  }
  return count;
}
