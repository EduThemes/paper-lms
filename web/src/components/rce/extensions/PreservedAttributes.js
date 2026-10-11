import { Extension } from '@tiptap/react';

/**
 * Round-trip fidelity for markup the editor has no dedicated control for
 * (G003, G013).
 *
 * ProseMirror only keeps attributes a node or mark declares, so content
 * imported from Canvas lost its ids (anchor targets), classes, lang, role,
 * aria-* and data-* attributes (Canvas's data-api-endpoint links, for
 * example) and any inline style the editor does not model the first time a
 * teacher opened and saved it. This extension gives the listed node and
 * mark types an `extraAttrs` attribute that captures those attributes on
 * parse and writes them back unchanged.
 *
 * It also adds two block attributes with editor commands:
 *  - `dir` (ltr / rtl / auto): setTextDirection / unsetTextDirection.
 *  - `indent` (0-10 steps of 40px padding-left, TinyMCE's indentation unit,
 *    so Canvas-indented paragraphs map onto it): indent / outdent, which
 *    sink/lift list items when the cursor is in a list.
 *
 * Event handlers and unknown attributes are never collected; the sanitizers
 * on both sides remain the authority on what is stored.
 */

const PRESERVED_ATTR = /^(id|class|role|lang|title|aria-[a-z-]+|data-[a-z0-9_.-]+)$/;

export const INDENT_STEP_PX = 40;
export const MAX_INDENT = 10;

const TEXTBLOCK_TYPES = ['paragraph', 'heading'];
const DIRECTION_TYPES = [
  'paragraph', 'heading', 'blockquote', 'bulletList', 'orderedList', 'listItem',
  'tableCell', 'tableHeader', 'container', 'details',
];

/** Parse "a: b; c: d" into [[prop, value], ...] (lower-cased props). */
export function parseStyle(style) {
  if (!style) return [];
  return String(style)
    .split(';')
    .map((decl) => {
      const idx = decl.indexOf(':');
      if (idx === -1) return null;
      const prop = decl.slice(0, idx).trim().toLowerCase();
      const value = decl.slice(idx + 1).trim();
      return prop && value ? [prop, value] : null;
    })
    .filter(Boolean);
}

export function serializeStyle(entries) {
  return entries.map(([p, v]) => `${p}: ${v}`).join('; ');
}

/** padding-left in multiples of INDENT_STEP_PX -> indent level, else 0. */
export function indentFromStyle(style) {
  const entry = parseStyle(style).find(([p]) => p === 'padding-left');
  if (!entry) return 0;
  const m = /^(\d+(?:\.\d+)?)px$/.exec(entry[1]);
  if (!m) return 0;
  const px = Number(m[1]);
  if (px <= 0 || px % INDENT_STEP_PX !== 0) return 0;
  return Math.min(px / INDENT_STEP_PX, MAX_INDENT);
}

/**
 * Collect the attributes worth preserving from an element.
 * @param {Element} el
 * @param {{skipAttrs?: string[], skipStyle?: (prop: string, el: Element) => boolean}} cfg
 */
export function collectExtraAttrs(el, cfg = {}) {
  const out = {};
  const skipAttrs = cfg.skipAttrs || [];
  for (const { name, value } of Array.from(el.attributes || [])) {
    const n = name.toLowerCase();
    if (skipAttrs.includes(n)) continue;
    if (n === 'style') {
      const kept = parseStyle(value).filter(([p]) => !(cfg.skipStyle && cfg.skipStyle(p, el)));
      if (kept.length) out.style = serializeStyle(kept);
      continue;
    }
    if (PRESERVED_ATTR.test(n)) out[n] = value;
  }
  return Object.keys(out).length ? out : null;
}

function extraAttrsAttribute(cfg) {
  return {
    extraAttrs: {
      default: null,
      keepOnSplit: false,
      parseHTML: (el) => collectExtraAttrs(el, cfg),
      renderHTML: (attrs) => attrs.extraAttrs || {},
    },
  };
}

/** Name of the closest list item type around the selection, or null. */
function listItemTypeAt(state) {
  const { $from } = state.selection;
  for (let d = $from.depth; d > 0; d -= 1) {
    const name = $from.node(d).type.name;
    if (name === 'listItem' || name === 'taskItem') return name;
  }
  return null;
}

// Style properties another attribute of the same node already owns.
const textblockSkipStyle = (prop, el) =>
  prop === 'text-align' || (prop === 'padding-left' && indentFromStyle(el.getAttribute('style')) > 0);
const cellSkipStyle = (prop) => prop === 'text-align';

export const PreservedAttributes = Extension.create({
  name: 'preservedAttributes',

  addGlobalAttributes() {
    return [
      { types: TEXTBLOCK_TYPES, attributes: extraAttrsAttribute({ skipStyle: textblockSkipStyle }) },
      {
        types: ['blockquote', 'bulletList', 'orderedList', 'listItem', 'table', 'tableRow', 'horizontalRule'],
        attributes: extraAttrsAttribute({}),
      },
      { types: ['tableCell', 'tableHeader'], attributes: extraAttrsAttribute({ skipAttrs: ['scope'], skipStyle: cellSkipStyle }) },
      // Link owns class/title/target/rel; keep ids, aria and Canvas data-api-*.
      { types: ['link'], attributes: extraAttrsAttribute({ skipAttrs: ['class', 'title'] }) },
      {
        types: DIRECTION_TYPES,
        attributes: {
          dir: {
            default: null,
            parseHTML: (el) => {
              const d = (el.getAttribute('dir') || '').toLowerCase();
              return d === 'ltr' || d === 'rtl' || d === 'auto' ? d : null;
            },
            renderHTML: (attrs) => (attrs.dir ? { dir: attrs.dir } : {}),
          },
        },
      },
      {
        types: TEXTBLOCK_TYPES,
        attributes: {
          indent: {
            default: 0,
            parseHTML: (el) => indentFromStyle(el.getAttribute('style')),
            renderHTML: (attrs) => (attrs.indent > 0 ? { style: `padding-left: ${attrs.indent * INDENT_STEP_PX}px` } : {}),
          },
        },
      },
    ];
  },

  addCommands() {
    const setDir = (dir) => ({ tr, state, dispatch }) => {
      const { from, to } = state.selection;
      let changed = false;
      state.doc.nodesBetween(from, to, (node, pos) => {
        if (TEXTBLOCK_TYPES.includes(node.type.name) || node.type.name === 'listItem') {
          if (node.attrs.dir !== dir) {
            tr.setNodeMarkup(pos, undefined, { ...node.attrs, dir });
            changed = true;
          }
        }
      });
      if (changed && dispatch) dispatch(tr);
      return changed;
    };

    const shiftIndent = (delta) => ({ tr, state, dispatch }) => {
      const { from, to } = state.selection;
      let changed = false;
      state.doc.nodesBetween(from, to, (node, pos) => {
        if (!TEXTBLOCK_TYPES.includes(node.type.name)) return true;
        const next = Math.max(0, Math.min(MAX_INDENT, (node.attrs.indent || 0) + delta));
        if (next !== node.attrs.indent) {
          tr.setNodeMarkup(pos, undefined, { ...node.attrs, indent: next });
          changed = true;
        }
        return false;
      });
      if (changed && dispatch) dispatch(tr);
      return changed;
    };

    return {
      setTextDirection: (dir) => setDir(dir === 'ltr' || dir === 'rtl' || dir === 'auto' ? dir : null),
      unsetTextDirection: () => setDir(null),
      // In a list, indent nests the item (as in Canvas); elsewhere it pads
      // the paragraph. Uses the command's own state so it works mid-chain.
      indent: () => ({ state, commands }) => {
        const item = listItemTypeAt(state);
        if (item) return commands.sinkListItem(item);
        return commands.command(shiftIndent(1));
      },
      outdent: () => ({ state, commands }) => {
        const item = listItemTypeAt(state);
        if (item) return commands.liftListItem(item);
        return commands.command(shiftIndent(-1));
      },
    };
  },
});

export default PreservedAttributes;
