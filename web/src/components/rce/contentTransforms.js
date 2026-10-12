// Inline declarations word processors (Google Docs, Word) put on every run
// of pasted text that only restate the defaults. Kept, they would bloat
// every paste with spans now that the editor preserves inline styles.
const NOISE_STYLE = {
  'background-color': /^(transparent|rgba\(0,\s*0,\s*0,\s*0\)|initial|inherit)$/i,
  'font-weight': /^(400|normal|inherit)$/i,
  'font-style': /^(normal|inherit)$/i,
  'font-variant': /./,
  'font-variant-ligatures': /./,
  'font-variant-caps': /./,
  'font-variant-east-asian': /./,
  'font-variant-alternates': /./,
  'font-variant-numeric': /./,
  'font-variant-position': /./,
  'text-decoration': /^(none|inherit)$/i,
  'text-decoration-line': /^none$/i,
  'vertical-align': /^baseline$/i,
  'white-space': /./,
  'white-space-collapse': /./,
  'text-wrap': /./,
  'color': /^(#000000|#000|rgb\(0,\s*0,\s*0\)|windowtext|inherit)$/i,
  'font-family': /./,
  'font-size': /^(11pt|12pt|medium|inherit)$/i,
  'line-height': /./,
  'margin-top': /^0(pt|px)?$/i,
  'margin-bottom': /^0(pt|px)?$/i,
};

function isNoise(prop, value) {
  if (prop.startsWith('mso-') || prop.startsWith('-webkit-')) return true;
  const re = NOISE_STYLE[prop];
  return !!(re && re.test(value));
}

/**
 * Clean HTML pasted from word processors before the editor parses it:
 * drops default-restating inline styles and Google Docs' wrapper ids, so a
 * paste keeps real formatting (colours, highlights, sizes) without a span
 * around every run.
 */
export function cleanPastedHtml(html) {
  if (!html || typeof document === 'undefined') return html;
  const tpl = document.createElement('template');
  tpl.innerHTML = html;
  // Google Docs wraps the whole paste in <b style="font-weight:normal"
  // id="docs-internal-guid-...">; unwrap it (before the style pass would
  // turn it into a real <b> around everything).
  tpl.content.querySelectorAll('[id^="docs-internal-guid"]').forEach((el) => {
    if (el.tagName === 'B' || el.tagName === 'SPAN') el.replaceWith(...el.childNodes);
    else el.removeAttribute('id');
  });
  tpl.content.querySelectorAll('[style]').forEach((el) => {
    const kept = (el.getAttribute('style') || '').split(';').map((d) => {
      const i = d.indexOf(':');
      if (i === -1) return null;
      const prop = d.slice(0, i).trim().toLowerCase();
      const value = d.slice(i + 1).trim();
      return prop && value && !isNoise(prop, value) ? `${prop}: ${value}` : null;
    }).filter(Boolean);
    if (kept.length) el.setAttribute('style', kept.join('; '));
    else el.removeAttribute('style');
  });

  tpl.content.querySelectorAll('[class^="Mso"], [class*=" Mso"]').forEach((el) => el.removeAttribute('class'));
  return tpl.innerHTML;
}

/**
 * Whether pasted HTML carries real content (text, a table, Office markup)
 * rather than just wrapping a copied image.
 *
 * Word, Excel, PowerPoint and OneNote put an image rendering of the
 * selection on the clipboard next to the HTML. Treating that paste as an
 * image upload replaced a pasted table or paragraph with a screenshot of
 * it; Canvas hands such pastes to the editor for the same reason. A copied
 * image (a browser's "Copy image") comes with HTML holding only an <img>,
 * so it is still uploaded.
 */
export function pastedHtmlHasContent(html) {
  if (!html || !html.trim()) return false;
  if (/urn:schemas-microsoft-com:office|ProgId["']?\s+content=["']?(Word|Excel|PowerPoint|OneNote)/i.test(html)) return true;
  if (typeof document === 'undefined') return true;
  const tpl = document.createElement('template');
  tpl.innerHTML = html;
  tpl.content.querySelectorAll('style, script, meta, title, link').forEach((el) => el.remove());
  if (tpl.content.querySelector('table, ul, ol, h1, h2, h3, h4, h5, h6')) return true;
  return (tpl.content.textContent || '').trim().length > 0;
}

/**
 * Pretty-print editor HTML for the source view: each block element starts on
 * its own line, nested blocks are indented, and a block that holds only
 * inline content stays on one line. Text and inline markup are never split,
 * and <pre> content is left exactly as is, so parsing the result gives the
 * same document.
 */
const BLOCK_TAGS = new Set([
  'p', 'div', 'section', 'article', 'aside', 'header', 'footer', 'main', 'nav',
  'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'ul', 'ol', 'li', 'blockquote', 'pre',
  'table', 'caption', 'colgroup', 'col', 'thead', 'tbody', 'tfoot', 'tr', 'td', 'th',
  'details', 'summary', 'figure', 'figcaption', 'hr', 'dl', 'dt', 'dd',
  'video', 'audio', 'iframe',
]);
const VOID_TAGS = new Set(['hr', 'col', 'br', 'img', 'source', 'track']);

export function formatHtml(html) {
  if (!html) return '';
  const tokens = html.split(/(<[^>]+>)/).filter(Boolean);
  const stack = [];
  let out = '';
  let preDepth = 0;
  const indent = (n) => '  '.repeat(n);
  for (const tok of tokens) {
    const m = /^<(\/?)([a-zA-Z][a-zA-Z0-9]*)\b[^>]*?(\/?)>$/.exec(tok);
    const tag = m ? m[2].toLowerCase() : null;
    const closing = m ? m[1] === '/' : false;
    if (preDepth > 0 && !(tag === 'pre' && closing)) {
      if (tag === 'pre') preDepth += 1;
      out += tok;
      continue;
    }
    if (!tag || !BLOCK_TAGS.has(tag)) {
      out += tok;
      continue;
    }
    if (closing) {
      const top = stack.pop();
      if (tag === 'pre') preDepth -= 1;
      if (top && top.hasBlockChild) out += `\n${indent(stack.length)}`;
      out += tok;
      continue;
    }
    if (stack.length) stack[stack.length - 1].hasBlockChild = true;
    if (out) out += '\n';
    out += indent(stack.length) + tok;
    if (!VOID_TAGS.has(tag) && !m[3]) {
      stack.push({ tag, hasBlockChild: false });
      if (tag === 'pre') preDepth = 1;
    }
  }
  return out;
}
