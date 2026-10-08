import { Node } from '@tiptap/react';
import { Plugin, PluginKey } from '@tiptap/pm/state';
import katex from 'katex';
import { latexOfMathElement } from '../../../utils/mathRender';

/**
 * Inline equation node for RichContentEditorV2 (G007).
 *
 * Equations used to be inserted as plain `\(latex\)` text: they showed as
 * raw LaTeX while editing and could not be reopened in the equation editor.
 * This node renders the equation with KaTeX inside the editor; clicking it
 * (or Enter on it) dispatches an `rce-math-edit` event on the editor DOM,
 * which the toolbar answers by reopening MathInputDialog with the LaTeX.
 *
 * Stored as <span class="math-tex" data-latex="...">\(...\)</span>, which
 * every display surface typesets (utils/mathRender.js) and which degrades
 * to readable LaTeX anywhere else. It also parses Canvas's equation markup
 * (span.math_equation_latex, img.equation_image[data-equation-content]),
 * and converts bare \( \) / \[ \] text (older Paper content, typed or
 * pasted equations) into equation nodes.
 */

export const MATH_EDIT_EVENT = 'rce-math-edit';

const TEXT_MATH_RE = /\\\(([\s\S]+?)\\\)|\\\[([\s\S]+?)\\\]/g;
const mathScanKey = new PluginKey('rceMathScan');

/** Find \( \) and \[ \] equations in a string. */
export function findTextEquations(text) {
  const out = [];
  if (!text || (!text.includes('\\(') && !text.includes('\\['))) return out;
  TEXT_MATH_RE.lastIndex = 0;
  let m;
  while ((m = TEXT_MATH_RE.exec(text))) {
    const latex = (m[1] ?? m[2] ?? '').trim();
    if (latex) out.push({ start: m.index, end: m.index + m[0].length, latex, display: m[2] != null });
  }
  return out;
}

function renderKatex(dom, latex, display) {
  try {
    katex.render(latex || '\\square', dom, { throwOnError: false, displayMode: !!display });
  } catch {
    dom.textContent = `\\(${latex}\\)`;
  }
}

const mathAttrs = (el) => {
  const m = latexOfMathElement(el);
  return m ? { latex: m.latex, display: m.display } : false;
};

export const MathInline = Node.create({
  name: 'mathInline',
  group: 'inline',
  inline: true,
  atom: true,
  selectable: true,

  addAttributes() {
    return {
      latex: { default: '' },
      display: { default: false },
    };
  },

  parseHTML() {
    return [
      // Ahead of the Image extension's img rule, so an equation image is
      // an equation, not a broken picture.
      { tag: 'span.math-tex', getAttrs: mathAttrs, priority: 100 },
      { tag: 'span.math_equation_latex', getAttrs: mathAttrs, priority: 100 },
      { tag: 'img[data-equation-content]', getAttrs: mathAttrs, priority: 100 },
    ];
  },

  renderHTML({ node }) {
    const { latex, display } = node.attrs;
    const attrs = { class: 'math-tex', 'data-latex': latex };
    if (display) attrs['data-display'] = 'block';
    return ['span', attrs, display ? `\\[${latex}\\]` : `\\(${latex}\\)`];
  },

  renderText({ node }) {
    return `\\(${node.attrs.latex}\\)`;
  },

  addNodeView() {
    return ({ node, getPos, editor }) => {
      let current = node;
      const dom = document.createElement('span');
      dom.className = 'math-tex rce-math-node';
      dom.contentEditable = 'false';
      dom.tabIndex = 0;
      dom.setAttribute('role', 'button');
      const label = () => `Equation ${current.attrs.latex}. Activate to edit.`;
      dom.setAttribute('aria-label', label());
      renderKatex(dom, current.attrs.latex, current.attrs.display);

      const open = (e) => {
        e.preventDefault();
        if (!editor.isEditable) return;
        const pos = typeof getPos === 'function' ? getPos() : null;
        editor.view.dom.dispatchEvent(new CustomEvent(MATH_EDIT_EVENT, {
          detail: { pos, latex: current.attrs.latex },
        }));
      };
      dom.addEventListener('click', open);
      dom.addEventListener('keydown', (e) => { if (e.key === 'Enter' || e.key === ' ') open(e); });

      return {
        dom,
        update(updated) {
          if (updated.type.name !== 'mathInline') return false;
          current = updated;
          renderKatex(dom, updated.attrs.latex, updated.attrs.display);
          dom.setAttribute('aria-label', label());
          return true;
        },
        ignoreMutation: () => true,
      };
    };
  },

  addCommands() {
    return {
      insertMath: (latex, display = false) => ({ commands }) => {
        if (!latex) return false;
        return commands.insertContent({ type: this.name, attrs: { latex, display } });
      },
      updateMath: (pos, latex) => ({ tr, dispatch }) => {
        const node = typeof pos === 'number' ? tr.doc.nodeAt(pos) : null;
        if (!node || node.type.name !== this.name || !latex) return false;
        if (dispatch) tr.setNodeMarkup(pos, undefined, { ...node.attrs, latex });
        return true;
      },
    };
  },

  addProseMirrorPlugins() {
    const type = this.type;
    return [
      new Plugin({
        key: mathScanKey,
        // Turn \( \) / \[ \] text into equation nodes whenever the document
        // changes (load, paste, typing the closing delimiter). Text inside
        // code blocks and inline code is left alone.
        appendTransaction(transactions, _old, state) {
          if (!transactions.some((tr) => tr.docChanged || tr.getMeta(mathScanKey))) return null;
          const found = [];
          state.doc.descendants((node, pos, parent) => {
            if (!node.isText) return true;
            if (parent?.type.spec.code || node.marks.some((m) => m.type.spec.code)) return false;
            findTextEquations(node.text).forEach((eq) => {
              found.push({ from: pos + eq.start, to: pos + eq.end, latex: eq.latex, display: eq.display });
            });
            return false;
          });
          if (!found.length) return null;
          const tr = state.tr;
          for (let i = found.length - 1; i >= 0; i -= 1) {
            const f = found[i];
            tr.replaceWith(f.from, f.to, type.create({ latex: f.latex, display: f.display }));
          }
          // A conversion of loaded content is not an undoable edit; one the
          // user just typed stays in the undo history with their typing.
          if (transactions.some((t) => t.getMeta('addToHistory') === false)) tr.setMeta('addToHistory', false);
          return tr;
        },
      }),
    ];
  },

  onCreate() {
    // Convert equations in the initial content without marking it edited.
    const { view, state } = this.editor;
    view.dispatch(state.tr.setMeta(mathScanKey, true).setMeta('preventUpdate', true).setMeta('addToHistory', false));
  },
});

export default MathInline;
