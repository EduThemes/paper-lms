/**
 * Drafts: the open design is kept in this browser's localStorage, one draft per course, so a teacher who
 * closes the tab can pick up where they left off. Storage that throws (private windows, blocked storage,
 * full quota) never breaks the editor. Also: validation of drafts and of `wiki_pages.design` docs.
 */
import { readJson, removeKey, writeJson } from './storage';
import { sanitizeDoc } from './engine';

export const DRAFT_PREFIX = 'paper-designer:draft:v1:';
export const draftKey = (courseId) => `${DRAFT_PREFIX}${courseId ?? 'none'}`;

const KINDS = ['home', 'start-here', 'overview', 'module', 'instructor'];

/** True when `d` looks like a PageDoc the designer can open (drafts, saved designs). */
export function isPageDoc(d) {
  if (!d || typeof d !== 'object' || Array.isArray(d)) return false;
  return (
    d.version === 1 &&
    typeof d.id === 'string' &&
    /^[A-Za-z0-9_-]{1,100}$/.test(d.id) &&
    typeof d.themeId === 'string' &&
    typeof d.paletteId === 'string' &&
    (d.palette === undefined || (!!d.palette && typeof d.palette === 'object' && !Array.isArray(d.palette))) &&
    (d.customCss === undefined || typeof d.customCss === 'string') &&
    typeof d.layout === 'string' &&
    KINDS.includes(d.kind) &&
    Array.isArray(d.sections) &&
    d.sections.every((s) => !!s && typeof s === 'object' && typeof s.id === 'string' && typeof s.type === 'string') &&
    !!d.options &&
    typeof d.options === 'object'
  );
}

/**
 * Saves the draft for a course. `pageUrl` is the Paper page the doc belongs to (null for a page not saved
 * yet), so re-opening that page can offer its unsaved changes.
 */
export function saveDraft(courseId, doc, { pageUrl = null, now = new Date() } = {}) {
  return writeJson(draftKey(courseId), { version: 1, savedAt: now.toISOString(), pageUrl, doc });
}

/** The draft for a course, or null. Its doc is sanitizeDoc'd: localStorage is outside the designer's control. */
export function loadDraft(courseId) {
  const d = readJson(draftKey(courseId));
  if (!d || d.version !== 1 || !isPageDoc(d.doc)) return null;
  return { savedAt: String(d.savedAt ?? ''), pageUrl: typeof d.pageUrl === 'string' ? d.pageUrl : null, doc: sanitizeDoc(d.doc) };
}

export function clearDraft(courseId) {
  removeKey(draftKey(courseId));
}

/** Calls `fn` after `ms` of quiet. `flush()` runs a pending call now; `cancel()` drops it. */
export function debounce(fn, ms) {
  let timer = null;
  let last = null;
  const run = (...a) => {
    last = a;
    if (timer) clearTimeout(timer);
    timer = setTimeout(() => {
      timer = null;
      fn(...a);
    }, ms);
  };
  run.flush = () => {
    if (timer && last) {
      clearTimeout(timer);
      timer = null;
      fn(...last);
    }
  };
  run.cancel = () => {
    if (timer) clearTimeout(timer);
    timer = null;
  };
  return run;
}
