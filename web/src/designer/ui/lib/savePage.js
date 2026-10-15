/**
 * Turning a design into a Paper wiki page, and checking a saved page before re-opening it.
 */
import { docHash, renderDesign, renderOptions, sanitizeDoc } from './engine';
import { isPageDoc } from './autosave';

/**
 * The page stylesheet: the designer's CSS, then the teacher's own (Advanced → Custom CSS) when the course
 * allows hand-written CSS.
 */
export function pageCss(designCss, doc, customCssEnabled) {
  const custom = customCssEnabled && doc.customCss?.trim() ? doc.customCss : '';
  return [designCss, custom].filter((x) => x && x.trim()).join('\n');
}

/**
 * The doc with its object keys in a stable (sorted) order. The server stores the design as jsonb, which
 * reorders keys; rendering and comparing the canonical form keeps the `data-pd-hash` in the HTML equal to the
 * hash of the design read back later.
 */
export function canonicalDoc(value) {
  if (Array.isArray(value)) return value.map(canonicalDoc);
  if (value && typeof value === 'object') {
    const out = {};
    for (const k of Object.keys(value).sort()) if (value[k] !== undefined) out[k] = canonicalDoc(value[k]);
    return out;
  }
  return value;
}

/**
 * The page for a doc in a course: { doc (canonical), html, css } with the custom CSS appended when allowed.
 * Save `doc` as the design, so the hash in `html` matches it.
 */
export function renderForSave(doc, { courseId, customCssEnabled }) {
  const canon = canonicalDoc(doc);
  const { html, css } = renderDesign(canon, renderOptions(courseId));
  return { doc: canon, html, css: pageCss(css, canon, customCssEnabled) };
}

/**
 * The `wiki_page` payload for create/update:
 *   { title, body: html, css, design: doc, published, front_page }
 * A front page has to be published (the server refuses otherwise), so front_page forces published.
 */
export function wikiPagePayload(doc, { title, html, css, published, frontPage }) {
  return {
    title: String(title ?? '').trim(),
    body: html,
    css,
    design: doc,
    published: Boolean(published || frontPage),
    front_page: Boolean(frontPage),
  };
}

const HASH_RE = /data-(?:pd|rmt)-hash="([0-9a-f]{8})"/i;

const textOf = (html) => {
  try {
    const d = new DOMParser().parseFromString(String(html ?? ''), 'text/html');
    return (d.body.textContent ?? '').replace(/\s+/g, ' ').trim();
  } catch {
    return String(html ?? '');
  }
};

/**
 * What re-opening a saved page in the designer means:
 *   { status: 'not-designed' }                      no design: don't open it here
 *   { status: 'ok', doc }                           safe to open
 *   { status: 'changed', doc }                      the page's HTML was changed outside the designer
 *
 * "Changed" = the body's data-pd-hash doesn't match the design, or its text differs from a fresh render.
 * (A body without a hash at all is judged by its text alone.)
 *
 * The design is stored data any course teacher or TA can write through the API (and it arrives through
 * Commons and Common Cartridge imports too), so `doc` is always the sanitizeDoc'd copy. A design the
 * designer wrote is unchanged by that; one that wasn't fails the hash check and opens as "changed".
 */
export function inspectSavedPage(page, courseId) {
  if (!isPageDoc(page?.design)) return { status: 'not-designed' };
  const doc = canonicalDoc(sanitizeDoc(page.design));
  const body = String(page.body ?? '');
  const m = HASH_RE.exec(body);
  if (m && m[1].toLowerCase() !== docHash(doc)) return { status: 'changed', doc };
  const fresh = renderDesign(doc, renderOptions(courseId)).html;
  if (fresh && textOf(fresh) !== textOf(body)) return { status: 'changed', doc };
  return { status: 'ok', doc };
}
