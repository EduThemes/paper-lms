/** The designer's reducer: the document history (lib/history.js) plus UI state. */
import { docReducer, emptyHistory } from '../lib/history';
import { sanitizeDoc } from '../lib/engine';

export const initialState = {
  history: emptyHistory,
  step: 'choose', // 'choose' | 'customize' | 'finish'
  selectedId: null,
  panelTab: 'content', // 'content' | 'look' | 'check' | 'advanced'
  device: 'desktop', // 'desktop' | 'tablet' | 'phone'
  dark: false,
  toast: null, // { id, message, variant, actionLabel, action }
  announcement: '',
  seasonalPrev: {}, // banner section id → { imageId, customSrc, alt } before a seasonal swap
  unpublished: {}, // this-week item id → true (item unpublished in its module)
  page: null, // the Paper page this doc is saved to: { url, pageId, title, published, frontPage }
  saved: null, // the doc as last saved to the course (to show "not saved yet")
};

let toastSeq = 0;

export function designerReducer(state, action) {
  switch (action.type) {
    case 'doc':
      return { ...state, history: docReducer(state.history, action.action) };
    case 'openDoc':
      // A doc opened from outside (a draft, a new page, a saved design) is cleaned once more on the way in.
      return {
        ...state,
        history: docReducer(state.history, { type: 'load', doc: sanitizeDoc(action.doc) }),
        step: action.step ?? 'customize',
        selectedId: null,
        panelTab: 'content',
        page: action.page !== undefined ? action.page : state.page,
        saved: action.saved !== undefined ? action.saved : state.saved,
        seasonalPrev: {},
        unpublished: {},
      };
    case 'setStep':
      return { ...state, step: action.step };
    case 'select':
      return { ...state, selectedId: action.id, panelTab: 'content' };
    case 'setTab':
      return { ...state, panelTab: action.tab };
    case 'setDevice':
      return { ...state, device: action.device };
    case 'setDark':
      return { ...state, dark: action.dark };
    case 'toast':
      return { ...state, toast: action.toast ? { ...action.toast, id: ++toastSeq } : null };
    case 'announce':
      return { ...state, announcement: action.message };
    case 'seasonalPrev': {
      const next = { ...state.seasonalPrev };
      if (action.value) next[action.id] = action.value;
      else delete next[action.id];
      return { ...state, seasonalPrev: next };
    }
    case 'markUnpublished':
      return { ...state, unpublished: { ...state.unpublished, ...Object.fromEntries(action.ids.map((i) => [i, true])) } };
    case 'reset':
      return { ...initialState, dark: state.dark, device: state.device };
    case 'setPage':
      return { ...state, page: action.page, saved: action.saved ?? state.saved };
    default:
      return state;
  }
}

