/**
 * Normalises editor HTML (contentEditable output) to the RichText subset: p, strong, em, a[href], ul, ol,
 * li, br. b→strong, i→em, div/h1–h4→p; everything else is unwrapped. The result then goes through the
 * engine's sanitizeRich, which is the authority.
 */
import { sanitizeRich } from './engine';

const KEEP = new Set(['P', 'STRONG', 'EM', 'A', 'UL', 'OL', 'LI', 'BR']);
const RENAME = { B: 'strong', I: 'em', DIV: 'p', H1: 'p', H2: 'p', H3: 'p', H4: 'p' };
const SAFE_HREF = /^(https?:|mailto:|\/|#)/i;

function clean(node, out) {
  if (node.nodeType === Node.TEXT_NODE) return [out.createTextNode(node.textContent ?? '')];
  if (node.nodeType !== Node.ELEMENT_NODE) return [];
  const el = node;
  const kids = () => Array.from(el.childNodes).flatMap((c) => clean(c, out));
  if (['SCRIPT', 'STYLE', 'TEMPLATE'].includes(el.tagName)) return [];
  const tag = RENAME[el.tagName] ?? (KEEP.has(el.tagName) ? el.tagName.toLowerCase() : null);
  if (!tag) return kids();
  const copy = out.createElement(tag);
  if (tag === 'a') {
    const href = (el.getAttribute('href') ?? '').trim();
    if (!SAFE_HREF.test(href)) return kids();
    copy.setAttribute('href', href);
  }
  kids().forEach((k) => copy.appendChild(k));
  return [copy];
}

export function normalizeRich(html) {
  if (!String(html ?? '').trim()) return '';
  const parsed = new DOMParser().parseFromString(`<body>${html}</body>`, 'text/html');
  const out = document.implementation.createHTMLDocument('');
  const wrap = out.createElement('div');
  Array.from(parsed.body.childNodes)
    .flatMap((n) => clean(n, out))
    .forEach((n) => wrap.appendChild(n));
  // Wrap stray top-level text/inline nodes into paragraphs.
  const blocks = new Set(['P', 'UL', 'OL']);
  let para = null;
  for (const n of Array.from(wrap.childNodes)) {
    if (n.nodeType === Node.ELEMENT_NODE && blocks.has(n.tagName)) {
      para = null;
      continue;
    }
    if (!para) {
      para = out.createElement('p');
      wrap.insertBefore(para, n);
    }
    para.appendChild(n);
  }
  // Drop empty paragraphs (only whitespace or <br>).
  wrap.querySelectorAll('p').forEach((p) => {
    if (!p.textContent?.trim() && !p.querySelector('a')) p.remove();
  });
  let result = wrap.innerHTML;
  try {
    result = sanitizeRich(result);
  } catch {
    /* keep our own cleaning */
  }
  return result;
}

/** Plain-text version, e.g. for labels and titles. */
export function richToText(html) {
  const d = new DOMParser().parseFromString(String(html ?? ''), 'text/html');
  return (d.body.textContent ?? '').replace(/\s+/g, ' ').trim();
}
