/**
 * Small, dependency-free HTML helpers shared by the rich content editor and
 * the content views.
 *
 * Every helper that has to parse HTML uses DOMParser, which builds an *inert*
 * document: no scripts run and no images load. Never parse untrusted HTML by
 * assigning innerHTML to an element created with document.createElement —
 * that element belongs to the live document, so `<img src=x onerror=...>`
 * fires even though the element is never attached.
 */

/** Escape text for safe interpolation into HTML (element content or attribute). */
export function escapeHTML(text) {
  return String(text ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

/** Parse HTML into an inert document's <body>. */
function parseInert(html) {
  return new DOMParser().parseFromString(`<!DOCTYPE html><body>${html}</body>`, 'text/html').body;
}

/**
 * Convert HTML to whitespace-collapsed plain text (for previews, list rows,
 * screen-reader text, TTS). Safe on untrusted input.
 */
export function htmlToText(html) {
  if (!html) return '';
  if (typeof DOMParser === 'undefined') {
    return String(html).replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim();
  }
  return (parseInert(String(html)).textContent || '').replace(/\s+/g, ' ').trim();
}

/**
 * Canonical form of an HTML string for "is this the same content?" checks.
 * The server re-serializes saved rich content (entity encoding, rel/target on
 * links, style spacing), so a byte comparison reports a difference for
 * content that is identical to the author.
 */
export function normalizeHTML(html) {
  if (!html) return '';
  if (typeof DOMParser === 'undefined') return String(html).replace(/\s+/g, ' ').trim();
  const body = parseInert(String(html));
  body.querySelectorAll('*').forEach((el) => {
    el.removeAttribute('rel');
    el.removeAttribute('target');
    if (el.hasAttribute('style')) {
      const css = el.style?.cssText || '';
      if (css) el.setAttribute('style', css);
      else el.removeAttribute('style');
    }
  });
  return body.innerHTML.replace(/>\s+</g, '><').replace(/\s+/g, ' ').trim();
}

/** Plain-text preview of an HTML string, truncated with an ellipsis. */
export function htmlPreview(html, maxLength = 200) {
  const text = htmlToText(html);
  return text.length > maxLength ? `${text.slice(0, maxLength).trimEnd()}…` : text;
}

// A body counts as HTML when it contains a closing tag, a self-contained
// <br>/<hr>/<img>, or an entity — the shapes every HTML editor emits. Plain
// text such as "x<y and y>z" or "a < b" does not match.
const HTML_SIGNATURE = /<\/[a-z][a-z0-9]*\s*>|<(br|hr|img)\b[^>]*>|&(#\d+|#x[0-9a-f]+|[a-z]+);/i;

/** Heuristic: does this string look like HTML rather than typed plain text? */
export function looksLikeHTML(text) {
  return HTML_SIGNATURE.test(String(text ?? ''));
}

const BULLET_LINE = /^\s*[-*•]\s+(.*)$/;
const NUMBER_LINE = /^\s*\d+[.)]\s+(.*)$/;
const HEADING_LINE = /^\s*(#{1,3})\s+(.*)$/;

/**
 * Convert typed plain text into equivalent, fully escaped HTML so its
 * structure survives being stored and rendered as HTML:
 *   - blank lines separate paragraphs, single newlines become <br>
 *   - runs of "- item" / "* item" lines become <ul>, "1. item" lines <ol>
 *   - "# Heading" lines (1-3 #) become <h2>-<h4> when `markdownLite` is set
 * Every character of the input is escaped, so "x<y and y>z" stays literal.
 */
export function plainTextToHtml(text, { markdownLite = false } = {}) {
  const src = String(text ?? '').replace(/\r\n?/g, '\n');
  if (!src.trim()) return '';
  const blocks = src.split(/\n{2,}/);
  const out = [];
  for (const block of blocks) {
    const lines = block.split('\n').filter((l, i, arr) => !(l.trim() === '' && (i === 0 || i === arr.length - 1)));
    if (lines.length === 0) continue;
    let i = 0;
    let para = [];
    const flushPara = () => {
      if (para.length) {
        out.push(`<p>${para.map(escapeHTML).join('<br>')}</p>`);
        para = [];
      }
    };
    while (i < lines.length) {
      const line = lines[i];
      const heading = markdownLite && line.match(HEADING_LINE);
      if (heading) {
        flushPara();
        const level = heading[1].length + 1;
        out.push(`<h${level}>${escapeHTML(heading[2])}</h${level}>`);
        i += 1;
        continue;
      }
      const listRe = BULLET_LINE.test(line) ? BULLET_LINE : (NUMBER_LINE.test(line) ? NUMBER_LINE : null);
      if (markdownLite && listRe) {
        flushPara();
        const tag = listRe === BULLET_LINE ? 'ul' : 'ol';
        const items = [];
        while (i < lines.length && listRe.test(lines[i])) {
          items.push(`<li>${escapeHTML(lines[i].match(listRe)[1])}</li>`);
          i += 1;
        }
        out.push(`<${tag}>${items.join('')}</${tag}>`);
        continue;
      }
      para.push(line);
      i += 1;
    }
    flushPara();
  }
  return out.join('');
}

const MENTION_CLASS = 'bg-brand-100 text-brand-700 rounded px-0.5 font-medium';
// "@name" only when it starts a word: not part of an email address
// (jane@school.edu) or a path/URL (medium.com/@user).
const MENTION_RE = /(^|[^\w@./:-])(@\w+)/g;
const MENTION_SKIP = new Set(['A', 'CODE', 'PRE', 'SCRIPT', 'STYLE', 'TEXTAREA']);

/**
 * Wrap @mentions in a highlight span. Works on the parsed DOM, touching text
 * nodes only, so "@" inside attributes (mailto: links, image URLs) and inside
 * links/code is never rewritten. Returns HTML; the caller still sanitizes it
 * before rendering.
 */
export function highlightMentions(html) {
  if (!html) return '';
  if (typeof DOMParser === 'undefined' || !String(html).includes('@')) return html;
  const body = parseInert(String(html));
  const doc = body.ownerDocument;
  const walker = doc.createTreeWalker(body, NodeFilter.SHOW_TEXT);
  const targets = [];
  let node;
  while ((node = walker.nextNode())) {
    let skip = false;
    for (let p = node.parentElement; p && p !== body; p = p.parentElement) {
      if (MENTION_SKIP.has(p.tagName)) { skip = true; break; }
    }
    MENTION_RE.lastIndex = 0;
    if (!skip && MENTION_RE.test(node.nodeValue)) targets.push(node);
  }
  for (const textNode of targets) {
    const text = textNode.nodeValue;
    const frag = doc.createDocumentFragment();
    let last = 0;
    MENTION_RE.lastIndex = 0;
    let m;
    while ((m = MENTION_RE.exec(text))) {
      const start = m.index + m[1].length;
      if (start > last) frag.appendChild(doc.createTextNode(text.slice(last, start)));
      const span = doc.createElement('span');
      span.className = MENTION_CLASS;
      span.textContent = m[2];
      frag.appendChild(span);
      last = start + m[2].length;
    }
    if (last < text.length) frag.appendChild(doc.createTextNode(text.slice(last)));
    textNode.parentNode.replaceChild(frag, textNode);
  }
  return body.innerHTML;
}
