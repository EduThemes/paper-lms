/**
 * localStorage persistence for RichContentEditorV2 autosave drafts.
 *
 * Drafts live under `paperlms.rce.<key>`. A draft must be cleared once the
 * content it protects has been saved, otherwise the next "new" form (possibly
 * in another course) offers to restore content that was already posted
 * (B103). Pages call clearRceDraft(key) after a successful save, and logout
 * calls clearAllRceDrafts() so drafts don't outlive the session on a shared
 * machine.
 */

export const AUTOSAVE_PREFIX = 'paperlms.rce.';

// Pending debounced writes, by key, so clearing a draft also cancels a write
// that would otherwise land after the save and resurrect it.
const pendingWrites = new Map();

function storage() {
  try {
    return typeof window !== 'undefined' ? window.localStorage : null;
  } catch {
    return null;
  }
}

/** Read a draft. Returns { html, savedAt } or null. */
export function readRceDraft(key) {
  const ls = storage();
  if (!key || !ls) return null;
  try {
    const raw = ls.getItem(AUTOSAVE_PREFIX + key);
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

/** Persist a draft immediately. Silent on quota errors. */
export function writeRceDraft(key, html) {
  const ls = storage();
  if (!key || !ls) return;
  try {
    ls.setItem(AUTOSAVE_PREFIX + key, JSON.stringify({ html, savedAt: Date.now() }));
  } catch {
    /* quota exceeded — ignore */
  }
}

/** Persist a draft after `delayMs`, replacing any pending write for the key. */
export function scheduleRceDraftWrite(key, html, delayMs) {
  if (!key) return;
  cancelRceDraftWrite(key);
  const id = setTimeout(() => {
    pendingWrites.delete(key);
    writeRceDraft(key, html);
  }, delayMs);
  pendingWrites.set(key, id);
}

/** Cancel a pending debounced write for the key, if any. */
export function cancelRceDraftWrite(key) {
  const id = pendingWrites.get(key);
  if (id !== undefined) {
    clearTimeout(id);
    pendingWrites.delete(key);
  }
}

/** Remove a draft (and any pending write for it). Call after a successful save. */
export function clearRceDraft(key) {
  if (!key) return;
  cancelRceDraftWrite(key);
  const ls = storage();
  if (!ls) return;
  try {
    ls.removeItem(AUTOSAVE_PREFIX + key);
  } catch { /* ignore */ }
}

/** Remove every editor draft (logout / account switch). */
export function clearAllRceDrafts() {
  for (const key of Array.from(pendingWrites.keys())) cancelRceDraftWrite(key);
  const ls = storage();
  if (!ls) return;
  try {
    const keys = [];
    for (let i = 0; i < ls.length; i += 1) {
      const k = ls.key(i);
      if (k && k.startsWith(AUTOSAVE_PREFIX)) keys.push(k);
    }
    keys.forEach((k) => ls.removeItem(k));
  } catch { /* ignore */ }
}
