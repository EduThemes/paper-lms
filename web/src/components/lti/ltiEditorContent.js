// Converts deep linking content items returned to the rich content editor
// (editor_button placement, G008/G130) into TipTap insertContent input.
// Kept free of React so it can be unit tested.

const SAFE_URL = /^(https?:\/\/|\/)/i;

export function safeUrl(url) {
  const u = String(url || '').trim();
  return SAFE_URL.test(u) ? u : '';
}

// Returns an array of TipTap content nodes (or HTML strings for `html`
// items, which the server sanitizes when the edited content is saved).
export function contentItemsToEditorContent(items) {
  const out = [];
  (items || []).forEach((item) => {
    switch (item?.type) {
      case 'ltiResourceLink':
      case 'link':
      case 'file': {
        const href = safeUrl(item.url);
        if (!href) return;
        out.push({
          type: 'paragraph',
          content: [{ type: 'text', text: item.title || item.text || href, marks: [{ type: 'link', attrs: { href } }] }],
        });
        return;
      }
      case 'image': {
        const src = safeUrl(item.url);
        if (!src) return;
        out.push({ type: 'image', attrs: { src, alt: item.title || item.text || '' } });
        return;
      }
      case 'html':
        if (item.html) out.push(item.html);
        return;
      default:
    }
  });
  return out;
}
