/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
/**
 * Designer state: the document (with undo/redo history, lib/history.js) plus UI state, in one useReducer,
 * shared through context. Replaces RMT Studio's zustand store.
 *
 * Also wires: autosave of the open doc to localStorage (per course, debounced), and Cmd/Ctrl+Z / Shift+Z /
 * Y for undo/redo outside text fields.
 */
import { useCallback, useEffect, useMemo, useReducer, useRef } from 'react';
import { DesignerContext } from './context';
import { canRedo, canUndo } from '../lib/history';
import { designerReducer, initialState } from './reducer';
import { debounce, saveDraft } from '../lib/autosave';
import { cloneSection, newSection, sectionName } from '../lib/sections';
import { useDesignerT } from '../i18n';

function isEditable(el) {
  return !!el && (el.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(el.tagName));
}

/**
 * props:
 *   courseId, course ({ name, code, uiMode, customCssEnabled }), teacher ({ name, email } | null), locale ('en'|'es'|'pt')
 *   initial: partial state (e.g. { step, history }) for opening an existing page or tests
 */
export function DesignerProvider({ courseId, course, teacher = null, locale = 'en', initial, children }) {
  const { t } = useDesignerT();
  const [state, dispatch] = useReducer(designerReducer, initial, (init) => ({
    ...initialState,
    dark: typeof document !== 'undefined' && document.documentElement.classList.contains('dark'),
    ...(init ?? {}),
  }));
  const ref = useRef(state);
  ref.current = state;
  const doc = state.history.present;

  // ---- autosave (localStorage, ≈0.6 s after the last change) ----
  const saver = useMemo(() => debounce((d, pageUrl) => saveDraft(courseId, d, { pageUrl }), 600), [courseId]);
  const lastSavedDoc = useRef(doc);
  useEffect(() => {
    if (!doc || doc === lastSavedDoc.current) return;
    lastSavedDoc.current = doc;
    saver(doc, ref.current.page?.url ?? null);
  }, [doc, saver]);
  useEffect(() => {
    const flush = () => saver.flush();
    window.addEventListener('pagehide', flush);
    return () => {
      window.removeEventListener('pagehide', flush);
      saver.flush();
    };
  }, [saver]);

  const typeLabel = useCallback((type) => t(`sectionType.${type}`), [t]);
  const nameOf = useCallback((s) => sectionName(s, typeLabel), [typeLabel]);

  const announce = useCallback((message) => {
    dispatch({ type: 'announce', message: '' });
    setTimeout(() => dispatch({ type: 'announce', message }), 30);
  }, []);

  const undo = useCallback(() => {
    if (!canUndo(ref.current.history)) return;
    dispatch({ type: 'doc', action: { type: 'undo' } });
    announce(t('customize.undone'));
  }, [announce, t]);
  const redo = useCallback(() => {
    if (!canRedo(ref.current.history)) return;
    dispatch({ type: 'doc', action: { type: 'redo' } });
    announce(t('customize.redone'));
  }, [announce, t]);

  // ---- Cmd/Ctrl+Z, Shift+Cmd/Ctrl+Z, Cmd/Ctrl+Y (outside text fields, where the browser's own undo applies) ----
  useEffect(() => {
    const onKey = (e) => {
      if (!(e.metaKey || e.ctrlKey) || isEditable(e.target) || ref.current.step === 'choose') return;
      const k = e.key.toLowerCase();
      if (k === 'z' && !e.shiftKey) {
        e.preventDefault();
        undo();
      } else if ((k === 'z' && e.shiftKey) || k === 'y') {
        e.preventDefault();
        redo();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [undo, redo]);

  const actions = useMemo(() => {
    const docDispatch = (action) => dispatch({ type: 'doc', action });
    const showToast = (toast) => dispatch({ type: 'toast', toast });
    return {
      dispatch: docDispatch,
      openDoc: (d, opts = {}) => dispatch({ type: 'openDoc', doc: d, step: opts.step, page: opts.page, saved: opts.saved }),
      setStep: (step) => dispatch({ type: 'setStep', step }),
      select: (id) => dispatch({ type: 'select', id }),
      setTab: (tab) => dispatch({ type: 'setTab', tab }),
      setDevice: (device) => dispatch({ type: 'setDevice', device }),
      setDark: (dark) => dispatch({ type: 'setDark', dark }),
      setPage: (page, saved) => dispatch({ type: 'setPage', page, saved }),
      reset: () => dispatch({ type: 'reset' }),
      markUnpublished: (ids) => dispatch({ type: 'markUnpublished', ids }),
      showToast,
      hideToast: () => dispatch({ type: 'toast', toast: null }),
      announce,
      undo,
      redo,
      addSection: (type, afterId) => {
        const cur = ref.current;
        const section = newSection(type, cur.history.present?.kind, t);
        docDispatch({ type: 'add', section, afterId: afterId ?? cur.selectedId });
        dispatch({ type: 'select', id: section.id });
        announce(t('customize.edit', { name: nameOf(section) }));
        return section.id;
      },
      duplicateSection: (id) => {
        const s = ref.current.history.present?.sections.find((x) => x.id === id);
        if (!s) return;
        const copy = cloneSection(s);
        docDispatch({ type: 'add', section: copy, afterId: id });
        dispatch({ type: 'select', id: copy.id });
      },
      deleteSection: (id) => {
        const cur = ref.current;
        const s = cur.history.present?.sections.find((x) => x.id === id);
        if (!s) return;
        docDispatch({ type: 'remove', id });
        if (cur.selectedId === id) dispatch({ type: 'select', id: null });
        showToast({ message: t('customize.deleted', { name: nameOf(s) }), actionLabel: t('undo'), action: () => undo() });
      },
      moveSection: (id, to) => {
        const d = ref.current.history.present;
        docDispatch({ type: 'move', id, to });
        const s = d?.sections.find((x) => x.id === id);
        if (!s) return;
        const n = Math.max(0, Math.min(d.sections.length - 1, to)) + 1;
        announce(t('customize.moved', { name: nameOf(s), n, total: d.sections.length }));
      },
      /** Swap a banner to a holiday/special-day banner (one undo step); remembers what it replaced. */
      applySeasonal: (sectionId, patch) => {
        const cur = ref.current;
        const s = cur.history.present?.sections.find((x) => x.id === sectionId);
        if (!s || s.type !== 'banner') return;
        if (!cur.seasonalPrev[sectionId]) {
          dispatch({ type: 'seasonalPrev', id: sectionId, value: { imageId: s.imageId, customSrc: s.customSrc, alt: s.alt } });
        }
        docDispatch({ type: 'update', id: sectionId, patch });
      },
      /** Undo the seasonal swap: the previous banner, or the page theme's own banner. */
      switchBack: (sectionId, patch) => {
        docDispatch({ type: 'update', id: sectionId, patch });
        dispatch({ type: 'seasonalPrev', id: sectionId, value: null });
      },
    };
  }, [announce, nameOf, t, undo, redo]);

  const value = useMemo(
    () => ({ state, doc, courseId, course: course ?? {}, teacher, locale, nameOf, typeLabel, ...actions }),
    [state, doc, courseId, course, teacher, locale, nameOf, typeLabel, actions],
  );

  return <DesignerContext.Provider value={value}>{children}</DesignerContext.Provider>;
}

