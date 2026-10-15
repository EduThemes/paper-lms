/**
 * The document reducer with undo/redo (ported from RMT Studio's state/history.ts). Pure: no ids are
 * generated here (callers pass fully-formed sections), so it is easy to test.
 *
 * Typing produces many `update` actions; consecutive updates with the same `coalesce` key within
 * COALESCE_MS are merged into one undo step.
 *
 * Actions:
 *   { type: 'load', doc }                        new history (no undo)
 *   { type: 'replace', doc }                     undoable wholesale replace
 *   { type: 'add', section, afterId? }
 *   { type: 'update', id, patch, coalesce?, at? }
 *   { type: 'move', id, to } / { type: 'moveBy', id, delta }
 *   { type: 'remove', id }
 *   { type: 'setDoc', patch, coalesce?, at? }    theme, palette, layout, options, customCss…
 *   { type: 'undo' } / { type: 'redo' }
 */

export const HISTORY_LIMIT = 100;
export const COALESCE_MS = 1200;

export const emptyHistory = Object.freeze({ past: [], present: null, future: [], lastKey: null, lastAt: 0 });

function commit(h, next, key = null, at = 0) {
  if (!h.present || next === h.present) return h;
  if (key && key === h.lastKey && at - h.lastAt < COALESCE_MS) {
    return { ...h, present: next, future: [], lastAt: at };
  }
  const past = [...h.past, h.present].slice(-HISTORY_LIMIT);
  return { past, present: next, future: [], lastKey: key, lastAt: at };
}

const withSections = (doc, sections) => ({ ...doc, sections });

export function docReducer(h, action) {
  const doc = h.present;
  switch (action.type) {
    case 'load':
      return { ...emptyHistory, present: action.doc };
    case 'replace':
      return doc ? commit(h, action.doc) : { ...emptyHistory, present: action.doc };
    case 'undo': {
      if (!doc || !h.past.length) return h;
      const prev = h.past[h.past.length - 1];
      return { past: h.past.slice(0, -1), present: prev, future: [doc, ...h.future], lastKey: null, lastAt: 0 };
    }
    case 'redo': {
      if (!doc || !h.future.length) return h;
      const [next, ...rest] = h.future;
      return { past: [...h.past, doc], present: next, future: rest, lastKey: null, lastAt: 0 };
    }
    default:
      break;
  }
  if (!doc) return h;
  switch (action.type) {
    case 'add': {
      const list = [...doc.sections];
      const i = action.afterId ? list.findIndex((s) => s.id === action.afterId) : -1;
      list.splice(i >= 0 ? i + 1 : list.length, 0, action.section);
      return commit(h, withSections(doc, list));
    }
    case 'update': {
      let changed = false;
      const list = doc.sections.map((s) => {
        if (s.id !== action.id) return s;
        changed = true;
        return { ...s, ...action.patch, id: s.id, type: s.type };
      });
      if (!changed) return h;
      return commit(
        h,
        withSections(doc, list),
        action.coalesce ? `u:${action.id}:${action.coalesce}` : null,
        action.at ?? Date.now(),
      );
    }
    case 'move':
    case 'moveBy': {
      const from = doc.sections.findIndex((s) => s.id === action.id);
      if (from < 0) return h;
      const target = action.type === 'move' ? action.to : from + action.delta;
      const to = Math.max(0, Math.min(doc.sections.length - 1, target));
      if (to === from) return h;
      const list = [...doc.sections];
      const [moved] = list.splice(from, 1);
      list.splice(to, 0, moved);
      return commit(h, withSections(doc, list));
    }
    case 'remove': {
      if (!doc.sections.some((s) => s.id === action.id)) return h;
      return commit(h, withSections(doc, doc.sections.filter((s) => s.id !== action.id)));
    }
    case 'setDoc': {
      const { options, ...rest } = action.patch;
      const next = { ...doc, ...rest, options: { ...doc.options, ...(options ?? {}) } };
      // `palette: undefined` means "back to the theme's colours": drop the key so the doc is as it was.
      if ('palette' in rest && rest.palette === undefined) delete next.palette;
      if ('customCss' in rest && !rest.customCss) delete next.customCss;
      return commit(h, next, action.coalesce ? `d:${action.coalesce}` : null, action.at ?? Date.now());
    }
    default:
      return h;
  }
}

export const canUndo = (h) => h.past.length > 0;
export const canRedo = (h) => h.future.length > 0;
