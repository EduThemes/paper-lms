import { Node, mergeAttributes } from '@tiptap/react';
import { TRUSTED_IFRAME_SRC } from '../../RichContentViewer';

/**
 * Block-level media embed node for RichContentEditorV2.
 *
 * Without a schema node ProseMirror silently dropped every <iframe>: "Embed
 * media" inserted nothing, and saving imported content from the editor
 * deleted its videos (B006). Only iframes whose src is on the trusted media
 * allowlist (the same list the server sanitizer enforces) are parsed.
 *
 * Serialized as <div class="rce-embed"><iframe ...></iframe></div>; the
 * wrapper's responsive 16:9 layout comes from the .rce-embed rule in
 * index.css.
 */
export const Iframe = Node.create({
  name: 'iframe',
  group: 'block',
  atom: true,
  selectable: true,
  draggable: true,

  addAttributes() {
    return {
      src: { default: null },
      title: { default: 'Embedded media' },
      width: { default: null },
      height: { default: null },
      frameborder: { default: '0' },
      allowfullscreen: {
        default: true,
        parseHTML: (el) => el.hasAttribute('allowfullscreen'),
        renderHTML: (attrs) => (attrs.allowfullscreen ? { allowfullscreen: 'true' } : {}),
      },
    };
  },

  parseHTML() {
    return [
      {
        tag: 'iframe[src]',
        getAttrs: (el) => (TRUSTED_IFRAME_SRC.test((el.getAttribute('src') || '').trim()) ? null : false),
      },
    ];
  },

  renderHTML({ HTMLAttributes }) {
    return ['div', { class: 'rce-embed' }, ['iframe', mergeAttributes(HTMLAttributes)]];
  },

  addCommands() {
    return {
      setIframe: (attrs) => ({ commands }) => {
        if (!attrs?.src || !TRUSTED_IFRAME_SRC.test(attrs.src)) return false;
        return commands.insertContent({ type: this.name, attrs });
      },
    };
  },
});

export default Iframe;
