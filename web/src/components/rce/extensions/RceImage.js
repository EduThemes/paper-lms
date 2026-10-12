import Image from '@tiptap/extension-image';
import { collectExtraAttrs } from './PreservedAttributes';

/**
 * Image node with Canvas's image options (G006):
 *  - `decorative`: written as alt="" role="presentation"
 *    data-decorative="true" (the markup Canvas's image options tray writes),
 *    read back from either data-decorative or role="presentation".
 *  - width / height (from the base extension) for sizing.
 *  - class, style, ids, aria-* and data-* (Canvas equation images carry
 *    data-equation-content) are preserved.
 *
 * updateImage(attrs) changes the selected image; the options dialog uses it
 * to edit alt text after insertion.
 */
export const RceImage = Image.extend({
  addAttributes() {
    const parent = this.parent?.() || {};
    return {
      ...parent,
      alt: {
        default: null,
        parseHTML: (el) => el.getAttribute('alt'),
        renderHTML: (attrs) => {
          if (attrs.decorative) return { alt: '' };
          return attrs.alt == null ? {} : { alt: attrs.alt };
        },
      },
      decorative: {
        default: false,
        parseHTML: (el) => el.getAttribute('data-decorative') === 'true'
          || (el.getAttribute('role') || '').toLowerCase() === 'presentation',
        renderHTML: (attrs) => (attrs.decorative ? { role: 'presentation', 'data-decorative': 'true' } : {}),
      },
      extraAttrs: {
        default: null,
        parseHTML: (el) => collectExtraAttrs(el, { skipAttrs: ['role', 'data-decorative', 'title'] }),
        renderHTML: (attrs) => attrs.extraAttrs || {},
      },
    };
  },

  addCommands() {
    return {
      ...this.parent?.(),
      updateImage: (attrs) => ({ commands }) => commands.updateAttributes(this.name, attrs),
    };
  },
});

export default RceImage;
