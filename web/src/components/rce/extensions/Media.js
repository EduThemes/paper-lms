import { Node, ReactNodeViewRenderer } from '@tiptap/react';
import MediaNodeView from '../MediaNodeView';

/**
 * Block-level audio/video node for RichContentEditorV2 (G002).
 *
 * Without a schema node ProseMirror silently dropped every <video> and
 * <audio>: imported media disappeared the first time the content was saved
 * from the editor, and uploaded or recorded media had nowhere to go.
 *
 * Serialized as
 *   <video controls preload="metadata" src="..." data-media-id="m-...">
 * Caption tracks of a media object are not stored in the HTML: display
 * surfaces attach them from the API (utils/mediaEnhance.js), so captions
 * added later reach every page that embeds the clip. Static <track>
 * children of imported media are kept as they are.
 */

const SAFE_SRC = /^(https?:\/\/|\/(?!\/))/i;

function trackAttrs(el) {
  return Array.from(el.querySelectorAll('track'))
    .map((t) => ({
      kind: t.getAttribute('kind') || 'subtitles',
      srclang: t.getAttribute('srclang') || '',
      label: t.getAttribute('label') || '',
      src: t.getAttribute('src') || '',
    }))
    .filter((t) => SAFE_SRC.test(t.src));
}

function parseMedia(el, mediaType) {
  const src = (el.getAttribute('src') || el.querySelector('source')?.getAttribute('src') || '').trim();
  if (!SAFE_SRC.test(src)) return false;
  const width = el.getAttribute('width');
  return {
    src,
    mediaType,
    mediaId: el.getAttribute('data-media-id') || null,
    title: el.getAttribute('title') || '',
    width: width && /^[0-9]+%?$/.test(width) ? width : null,
    tracks: trackAttrs(el),
  };
}

export const Media = Node.create({
  name: 'media',
  group: 'block',
  atom: true,
  selectable: true,
  draggable: true,

  addAttributes() {
    return {
      src: { default: null },
      mediaId: { default: null },
      mediaType: { default: 'video' },
      title: { default: '' },
      width: { default: null },
      tracks: { default: [] },
    };
  },

  parseHTML() {
    return [
      { tag: 'video', getAttrs: (el) => parseMedia(el, 'video') },
      { tag: 'audio', getAttrs: (el) => parseMedia(el, 'audio') },
    ];
  },

  renderHTML({ node }) {
    const { src, mediaId, mediaType, title, width, tracks } = node.attrs;
    const tag = mediaType === 'audio' ? 'audio' : 'video';
    const attrs = { src, controls: 'controls', preload: 'metadata' };
    if (mediaId) attrs['data-media-id'] = mediaId;
    if (title) attrs.title = title;
    if (tag === 'video') {
      if (width) attrs.width = width;
      attrs.style = 'max-width: 100%';
    }
    const children = (tracks || []).map((t) => ['track', {
      kind: t.kind, srclang: t.srclang, label: t.label, src: t.src,
    }]);
    return [tag, attrs, ...children];
  },

  addNodeView() {
    return ReactNodeViewRenderer(MediaNodeView);
  },

  addCommands() {
    return {
      setMedia: (attrs) => ({ commands }) => {
        if (!attrs?.src || !SAFE_SRC.test(attrs.src)) return false;
        return commands.insertContent({ type: this.name, attrs });
      },
    };
  },
});

export default Media;
