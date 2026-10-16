/**
 * The document inside the designer's preview iframes. It borrows the app's own stylesheets (so Tailwind's
 * preflight, Paper's colour tokens and the `.paper-page` rules apply exactly as on a real page), wraps the
 * page in the same two containment boxes DesignedPageView uses (an outer frame the page stylesheet can't
 * reach, then the page root; no prose wrapper: designed pages carry their own styling), and adds the page
 * stylesheet scoped to the root. Dark mode is the `dark` class on the iframe's <html>, like Paper's.
 */
import { CONTENT_CONTAINMENT, PAGE_FRAME_CLASS, PAGE_FRAME_CONTAINMENT, styleText } from '@/components/contentContainment';
import { PREVIEW_ROOT, scopeCss } from './previewCss';

export const DEVICE_WIDTHS = { desktop: 1100, tablet: 768, phone: 375 };

/** Class of the root DesignedPageView puts around a designed page's HTML. */
export const CONTENT_CLASS = 'paper-page';

/** The outer frame and the root, as DesignedPageView renders them (components/contentContainment.js). */
export const FRAME_CLASS = PAGE_FRAME_CLASS;
const FRAME_STYLE = styleText(PAGE_FRAME_CONTAINMENT);
const ROOT_STYLE = styleText(CONTENT_CONTAINMENT);

const BASE_CSS = `
  html { overflow: hidden; }
  body { margin: 0; padding: 24px; background: rgb(var(--color-surface-0, 252 252 250)); color: rgb(var(--color-text-primary, 31 35 40));
    font-family: system-ui, -apple-system, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; line-height: 1.625; overflow-wrap: break-word; }
  html.dark body { background: rgb(var(--color-surface-0, 24 24 27)); color: rgb(var(--color-text-primary, 244 244 245)); }
  body.phone { padding: 16px; }
  img { max-width: 100%; height: auto; }
  body.interactive [data-pd-section], body.interactive [data-rmt-section] { cursor: pointer; }
  body.interactive [data-pd-section]:hover, body.interactive [data-rmt-section]:hover { outline: 2px dashed rgba(43, 122, 188, .6); outline-offset: 2px; }
`;

const escAttr = (s) => String(s).replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;');

/**
 * The app's stylesheets as HTML to copy into a preview <head>: every <link rel=stylesheet> (the built CSS
 * in production) and, when there are none (the Vite dev server), the <style> elements.
 */
export function appStylesHtml(doc = typeof document !== 'undefined' ? document : null) {
  if (!doc) return '';
  const nodes = Array.from(doc.querySelectorAll('link[rel="stylesheet"], style'));
  const links = nodes.filter((n) => n.tagName === 'LINK' && n.getAttribute('href'));
  if (links.length) return links.map((l) => `<link rel="stylesheet" href="${escAttr(l.href)}">`).join('');
  return nodes
    .filter((n) => n.tagName === 'STYLE' && !n.hasAttribute('data-pd-preview'))
    .map((s) => `<style>${s.textContent.replace(/<\/(style)/gi, '<\\/$1')}</style>`)
    .join('');
}

/** The full preview document. `html` and `css` may be empty and filled in later (live preview). */
export function previewShell({ html = '', css = '', dark = false, bodyClass = '', lang = 'en', appStyles = appStylesHtml() } = {}) {
  return (
    `<!doctype html><html lang="${escAttr(lang)}" class="${dark ? 'dark' : ''}"><head><meta charset="utf-8">` +
    `<meta name="viewport" content="width=device-width, initial-scale=1"><base target="_blank">` +
    `${appStyles}<style>${BASE_CSS}</style><style id="pd-page-css">${scopeCss(css)}</style><style id="pd-sel"></style></head>` +
    `<body class="${escAttr(bodyClass)}"><div class="${FRAME_CLASS}" data-paper-page-frame="" style="${FRAME_STYLE}">` +
    `<div id="${PREVIEW_ROOT.slice(1)}" class="${CONTENT_CLASS}" style="${ROOT_STYLE}">${html}</div></div></body></html>`
  );
}

/** The section id under a clicked element, from the engine's section markers. */
export function sectionIdOf(el) {
  const hit = el?.closest?.('[data-pd-section], [data-rmt-section]');
  if (!hit) return null;
  return hit.getAttribute('data-pd-section') ?? hit.getAttribute('data-rmt-section');
}

export function sectionSelector(id) {
  const safe = typeof CSS !== 'undefined' && CSS.escape ? CSS.escape(id) : String(id).replace(/[^\w-]/g, '');
  return `[data-pd-section="${safe}"], [data-rmt-section="${safe}"]`;
}
