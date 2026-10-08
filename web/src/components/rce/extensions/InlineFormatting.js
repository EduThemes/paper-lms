import { Mark, mergeAttributes } from '@tiptap/react';
import { collectExtraAttrs, parseStyle } from './PreservedAttributes';

/**
 * Inline formatting marks for RichContentEditorV2 (G003, G009).
 *
 * - textStyle: <span style="color / background-color / font-size /
 *   font-family"> — what Canvas's colour and font-size controls write. Also
 *   reads legacy <font color size face> and keeps any other span attributes
 *   (class, lang, aria-*, data-*) so imported spans survive a save.
 * - subscript / superscript: <sub> / <sup> (H2O, x²).
 * - inlineTag: semantic inline elements the editor has no button for
 *   (abbr, cite, q, small, kbd, ...), kept as-is instead of flattened.
 */

const STYLE_PROPS = {
  color: 'color',
  backgroundColor: 'background-color',
  fontSize: 'font-size',
  fontFamily: 'font-family',
};
// Owned by this mark's attributes, or by the bold / italic / underline /
// strike marks that parse the same declarations.
const OWNED_STYLE = new Set([...Object.values(STYLE_PROPS), 'font-weight', 'font-style', 'text-decoration', 'text-decoration-line']);

// <font size="1..7"> to the CSS sizes browsers use for them.
const FONT_SIZE_MAP = { 1: '10px', 2: '13px', 3: '16px', 4: '18px', 5: '24px', 6: '32px', 7: '48px' };

/** Font sizes offered by the toolbar (Canvas offers the same point sizes). */
export const FONT_SIZES = ['8pt', '10pt', '12pt', '14pt', '18pt', '24pt', '36pt'];

function styleValue(el, prop) {
  const entry = parseStyle(el.getAttribute('style')).find(([p]) => p === prop);
  return entry ? entry[1] : null;
}

function styleAttribute(key) {
  const prop = STYLE_PROPS[key];
  return {
    default: null,
    parseHTML: (el) => {
      const v = styleValue(el, prop);
      if (v) return v;
      if (el.tagName === 'FONT') {
        if (key === 'color') return el.getAttribute('color') || null;
        if (key === 'fontFamily') return el.getAttribute('face') || null;
        if (key === 'fontSize') return FONT_SIZE_MAP[(el.getAttribute('size') || '').trim()] || null;
      }
      return null;
    },
    renderHTML: (attrs) => (attrs[key] ? { style: `${prop}: ${attrs[key]}` } : {}),
  };
}

function isEmptyTextStyle(attrs) {
  return !attrs || (Object.keys(STYLE_PROPS).every((k) => !attrs[k]) && !attrs.extraAttrs);
}

export const TextStyle = Mark.create({
  name: 'textStyle',
  priority: 101,

  addAttributes() {
    return {
      color: styleAttribute('color'),
      backgroundColor: styleAttribute('backgroundColor'),
      fontSize: styleAttribute('fontSize'),
      fontFamily: styleAttribute('fontFamily'),
      extraAttrs: {
        default: null,
        parseHTML: (el) => collectExtraAttrs(el, { skipStyle: (p) => OWNED_STYLE.has(p) }),
        renderHTML: (attrs) => attrs.extraAttrs || {},
      },
    };
  },

  parseHTML() {
    return [
      // A span with nothing worth keeping (no style, no preserved
      // attribute) is unwrapped; its text comes through unformatted.
      {
        tag: 'span',
        getAttrs: (el) => (parseStyle(el.getAttribute('style')).length > 0
          || collectExtraAttrs(el, { skipStyle: () => true }) ? null : false),
      },
      { tag: 'font' },
    ];
  },

  renderHTML({ HTMLAttributes }) {
    return ['span', mergeAttributes(HTMLAttributes), 0];
  },

  addCommands() {
    const setStyle = (key) => (value) => ({ chain }) =>
      chain().setMark(this.name, { [key]: value || null }).removeEmptyTextStyle().run();
    return {
      setColor: setStyle('color'),
      unsetColor: () => ({ chain }) => chain().setMark(this.name, { color: null }).removeEmptyTextStyle().run(),
      setBackgroundColor: setStyle('backgroundColor'),
      unsetBackgroundColor: () => ({ chain }) => chain().setMark(this.name, { backgroundColor: null }).removeEmptyTextStyle().run(),
      setFontSize: setStyle('fontSize'),
      unsetFontSize: () => ({ chain }) => chain().setMark(this.name, { fontSize: null }).removeEmptyTextStyle().run(),
      setFontFamily: setStyle('fontFamily'),
      removeEmptyTextStyle: () => ({ state, tr, dispatch }) => {
        const { from, to, empty } = state.selection;
        const type = state.schema.marks[this.name];
        if (empty) {
          const stored = (state.storedMarks || state.selection.$from.marks()).find((m) => m.type === type);
          if (stored && isEmptyTextStyle(stored.attrs)) {
            tr.removeStoredMark(type);
            if (dispatch) dispatch(tr);
          }
          return true;
        }
        state.doc.nodesBetween(from, to, (node, pos) => {
          if (!node.isText) return;
          node.marks.forEach((m) => {
            if (m.type === type && isEmptyTextStyle(m.attrs)) {
              tr.removeMark(Math.max(pos, from), Math.min(pos + node.nodeSize, to), m);
            }
          });
        });
        if (dispatch) dispatch(tr);
        return true;
      },
    };
  },
});

function verticalAlignRule(value) {
  return {
    style: 'vertical-align',
    getAttrs: (v) => (String(v).trim().toLowerCase() === value ? null : false),
  };
}

export const Subscript = Mark.create({
  name: 'subscript',
  excludes: 'superscript',
  parseHTML() {
    return [{ tag: 'sub' }, verticalAlignRule('sub')];
  },
  renderHTML({ HTMLAttributes }) {
    return ['sub', mergeAttributes(HTMLAttributes), 0];
  },
  addCommands() {
    return {
      toggleSubscript: () => ({ commands }) => commands.toggleMark(this.name),
    };
  },
  addKeyboardShortcuts() {
    return { 'Mod-,': () => this.editor.commands.toggleSubscript() };
  },
});

export const Superscript = Mark.create({
  name: 'superscript',
  excludes: 'subscript',
  parseHTML() {
    return [{ tag: 'sup' }, verticalAlignRule('super')];
  },
  renderHTML({ HTMLAttributes }) {
    return ['sup', mergeAttributes(HTMLAttributes), 0];
  },
  addCommands() {
    return {
      toggleSuperscript: () => ({ commands }) => commands.toggleMark(this.name),
    };
  },
  addKeyboardShortcuts() {
    return { 'Mod-.': () => this.editor.commands.toggleSuperscript() };
  },
});

export const INLINE_TAGS = ['abbr', 'cite', 'q', 'small', 'kbd', 'samp', 'var', 'dfn', 'time', 'ins', 'bdi', 'bdo'];

export const InlineTag = Mark.create({
  name: 'inlineTag',
  // Several different inline tags may wrap the same text.
  excludes: '',

  addAttributes() {
    return {
      tag: {
        default: 'span',
        parseHTML: (el) => el.tagName.toLowerCase(),
        rendered: false,
      },
      extraAttrs: {
        default: null,
        parseHTML: (el) => collectExtraAttrs(el),
        renderHTML: (attrs) => attrs.extraAttrs || {},
      },
    };
  },

  parseHTML() {
    return INLINE_TAGS.map((tag) => ({ tag }));
  },

  renderHTML({ mark, HTMLAttributes }) {
    const tag = INLINE_TAGS.includes(mark.attrs.tag) ? mark.attrs.tag : 'span';
    return [tag, mergeAttributes(HTMLAttributes), 0];
  },
});
