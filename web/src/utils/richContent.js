// Helpers for fields edited with the rich content editor (G010).
import { htmlToText, looksLikeHTML, plainTextToHtml } from './html';

/**
 * True when editor HTML holds something worth saving: text, or an embedded
 * image, equation, media player or iframe. An empty editor emits "<p></p>".
 */
export function hasRichContent(html) {
  const value = String(html ?? '');
  if (!value.trim()) return false;
  if (/<(img|video|audio|iframe)\b|class="math-tex"/i.test(value)) return true;
  return htmlToText(value).trim().length > 0;
}

/**
 * Editor HTML for a field that may hold legacy plain text (stored before the
 * field moved to the rich content editor): plain text becomes escaped
 * paragraphs so its line breaks survive and "x<y" stays text.
 */
export function toEditorHTML(value) {
  const v = String(value ?? '');
  if (!v.trim() || looksLikeHTML(v)) return v;
  return plainTextToHtml(v);
}

/** The value to save for an editor field: '' when it is empty. */
export function richValueOrEmpty(html) {
  return hasRichContent(html) ? html : '';
}
