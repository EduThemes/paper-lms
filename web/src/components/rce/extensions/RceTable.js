import { findParentNode } from '@tiptap/react';
import { Table } from '@tiptap/extension-table';
import TableHeader from '@tiptap/extension-table-header';

/**
 * Accessible tables (G015).
 *
 * - The table keeps a <caption> (stored as the `caption` attribute and
 *   written as the table's first child). Without the ignore rule the caption
 *   text was parsed into a bogus first row.
 * - Header cells keep `scope` (col / row / colgroup / rowgroup).
 * - setTableCaption sets or clears the caption of the table at the cursor;
 *   fixTableHeaderScopes gives every header cell without a scope the
 *   obvious one (first row -> col, otherwise row). insertTable with a header
 *   row is followed by the scope fix in the toolbar, matching Canvas's
 *   table-header-scope a11y rule.
 *
 * Resized column widths are written as `colwidth` on cells (allowed by both
 * sanitizers), so a resize survives a save.
 */

const SCOPES = ['col', 'row', 'colgroup', 'rowgroup'];

export const RceTable = Table.extend({
  addAttributes() {
    return {
      ...this.parent?.(),
      caption: {
        default: null,
        parseHTML: (el) => {
          const cap = Array.from(el.children).find((c) => c.tagName === 'CAPTION');
          const text = cap ? cap.textContent.trim() : '';
          return text || null;
        },
        rendered: false,
      },
    };
  },

  parseHTML() {
    return [...(this.parent?.() || []), { tag: 'caption', ignore: true }];
  },

  renderHTML(props) {
    const spec = this.parent(props);
    const caption = props.node.attrs.caption;
    if (!caption) return spec;
    const withCaption = (tableSpec) => [tableSpec[0], tableSpec[1], ['caption', {}, caption], ...tableSpec.slice(2)];
    if (spec[0] === 'table') return withCaption(spec);
    // renderWrapper: ['div', attrs, tableSpec]
    return [spec[0], spec[1], withCaption(spec[2])];
  },

  addCommands() {
    return {
      ...this.parent?.(),
      setTableCaption: (caption) => ({ state, tr, dispatch }) => {
        const table = findParentNode((n) => n.type.name === this.name)(state.selection);
        if (!table) return false;
        if (dispatch) {
          tr.setNodeMarkup(table.pos, undefined, { ...table.node.attrs, caption: (caption || '').trim() || null });
          dispatch(tr);
        }
        return true;
      },
      fixTableHeaderScopes: () => ({ state, tr, dispatch }) => {
        let changed = false;
        state.doc.descendants((node, pos) => {
          if (node.type.name !== this.name) return true;
          node.forEach((row, rowOffset, rowIndex) => {
            row.forEach((cell, cellOffset, cellIndex) => {
              if (cell.type.name !== 'tableHeader' || cell.attrs.scope) return;
              const scope = rowIndex === 0 ? 'col' : (cellIndex === 0 ? 'row' : 'col');
              tr.setNodeMarkup(pos + 1 + rowOffset + 1 + cellOffset, undefined, { ...cell.attrs, scope });
              changed = true;
            });
          });
          return false;
        });
        if (changed && dispatch) dispatch(tr);
        return changed;
      },
    };
  },
});

export const RceTableHeader = TableHeader.extend({
  addAttributes() {
    return {
      ...this.parent?.(),
      scope: {
        default: null,
        parseHTML: (el) => {
          const s = (el.getAttribute('scope') || '').toLowerCase();
          return SCOPES.includes(s) ? s : null;
        },
        renderHTML: (attrs) => (attrs.scope ? { scope: attrs.scope } : {}),
      },
    };
  },
});
