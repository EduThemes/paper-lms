import { Node, mergeAttributes } from '@tiptap/react';
import { collectExtraAttrs } from './PreservedAttributes';
import { sanitizeHTML } from '../../RichContentViewer';

/**
 * Block nodes that keep imported Canvas/IMSCC structure intact (G003).
 *
 * - container: <div>/<section>/<article>/<aside>/<header>/<footer>/<main>/
 *   <nav> wrappers with their classes, ids and inline styles (Canvas
 *   templates and DesignPLUS-style callout boxes are built from them). The
 *   content stays editable.
 * - details / detailsSummary: collapsible sections. The editor always shows
 *   them expanded (a collapsed body cannot be edited); the saved `open`
 *   state is kept.
 * - rawHtmlBlock: an atom that stores the sanitized outer HTML of blocks the
 *   editor has no model for (figure, video, audio, definition lists,
 *   picture). It renders as-is and is edited through the HTML source view.
 *   Its parse rules have a low priority so a dedicated node (for example a
 *   media node) always wins.
 */

export const CONTAINER_TAGS = ['div', 'section', 'article', 'aside', 'header', 'footer', 'main', 'nav'];

// Wrappers that belong to another node's own rendering (the embed node
// renders div.rce-embed around its iframe; media nodes use rce-* wrappers).
const OWNED_WRAPPER = /(^|\s)(rce-[\w-]+|tableWrapper)(\s|$)/;

export const Container = Node.create({
  name: 'container',
  group: 'block',
  content: 'block+',

  addAttributes() {
    return {
      tag: {
        default: 'div',
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
    return CONTAINER_TAGS.map((tag) => ({
      tag,
      priority: 40,
      getAttrs: (el) => (OWNED_WRAPPER.test(el.getAttribute('class') || '') ? false : null),
    }));
  },

  renderHTML({ node, HTMLAttributes }) {
    const tag = CONTAINER_TAGS.includes(node.attrs.tag) ? node.attrs.tag : 'div';
    return [tag, mergeAttributes(HTMLAttributes), 0];
  },
});

export const DetailsSummary = Node.create({
  name: 'detailsSummary',
  content: 'inline*',
  defining: true,
  isolating: true,

  parseHTML() {
    return [{ tag: 'summary' }];
  },

  renderHTML({ HTMLAttributes }) {
    return ['summary', mergeAttributes(HTMLAttributes), 0];
  },

  addNodeView() {
    return () => {
      const dom = document.createElement('div');
      dom.className = 'rce-details-summary';
      return { dom, contentDOM: dom };
    };
  },
});

export const Details = Node.create({
  name: 'details',
  group: 'block',
  content: 'detailsSummary block+',
  defining: true,

  addAttributes() {
    return {
      open: {
        default: false,
        parseHTML: (el) => el.hasAttribute('open'),
        renderHTML: (attrs) => (attrs.open ? { open: '' } : {}),
      },
      extraAttrs: {
        default: null,
        parseHTML: (el) => collectExtraAttrs(el),
        renderHTML: (attrs) => attrs.extraAttrs || {},
      },
    };
  },

  parseHTML() {
    return [{ tag: 'details' }];
  },

  renderHTML({ HTMLAttributes }) {
    return ['details', mergeAttributes(HTMLAttributes), 0];
  },

  addNodeView() {
    return () => {
      const dom = document.createElement('div');
      dom.className = 'rce-details';
      return { dom, contentDOM: dom };
    };
  },

  addCommands() {
    return {
      insertDetails: (summary = 'Summary') => ({ commands }) => commands.insertContent({
        type: this.name,
        content: [
          { type: 'detailsSummary', content: [{ type: 'text', text: summary }] },
          { type: 'paragraph' },
        ],
      }),
    };
  },
});

export const RAW_BLOCK_TAGS = ['figure', 'video', 'audio', 'dl', 'picture'];

// ProseMirror's renderSpec only accepts arrays (and text nodes), so the
// sanitized element is converted to a nested DOMOutputSpec.
function elementToSpec(el) {
  const attrs = {};
  for (const { name, value } of Array.from(el.attributes)) attrs[name] = value;
  const children = [];
  el.childNodes.forEach((child) => {
    if (child.nodeType === 1) children.push(elementToSpec(child));
    else if (child.nodeType === 3) children.push(document.createTextNode(child.nodeValue));
  });
  return [el.tagName.toLowerCase(), attrs, ...children];
}

function htmlToSpec(html) {
  if (typeof document === 'undefined') return null;
  const tpl = document.createElement('template');
  tpl.innerHTML = sanitizeHTML(html || '');
  const el = tpl.content.firstElementChild;
  return el ? elementToSpec(el) : null;
}

export const RawHtmlBlock = Node.create({
  name: 'rawHtmlBlock',
  group: 'block',
  atom: true,
  selectable: true,
  draggable: true,

  addAttributes() {
    return {
      html: {
        default: '',
        parseHTML: (el) => el.outerHTML,
        rendered: false,
      },
    };
  },

  parseHTML() {
    return RAW_BLOCK_TAGS.map((tag) => ({ tag, priority: 30 }));
  },

  renderHTML({ node }) {
    return htmlToSpec(node.attrs.html) || ['div', {}];
  },
});
