/**
 * HTML clean-up applied before content is loaded into RichContentEditorV2.
 *
 * The legacy editor (V1) stored equations as
 *   <span class="math-tex" data-latex="x^2">...KaTeX markup...</span>
 * while V2 stores \( ... \) text that the viewers typeset. Without this
 * conversion, opening a V1-authored description in V2 flattened the KaTeX
 * markup into garbled text (G017).
 */
export function prepareEditorHtml(html) {
  if (!html || typeof html !== 'string') return html || '';
  if (!html.includes('math-tex') || typeof document === 'undefined') return html;
  const tpl = document.createElement('template');
  tpl.innerHTML = html;
  const spans = tpl.content.querySelectorAll('span.math-tex[data-latex]');
  if (spans.length === 0) return html;
  spans.forEach((span) => {
    const latex = span.getAttribute('data-latex') || '';
    span.replaceWith(document.createTextNode(`\\(${latex}\\)`));
  });
  return tpl.innerHTML;
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
