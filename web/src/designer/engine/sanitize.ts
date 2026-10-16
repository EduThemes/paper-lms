import { decodeEntities, esc, escTextKeepEntities, parseHtml, attr, type HElement, type HNode } from './html';
import type { RichText } from './types';

/**
 * RichText allows only: p, strong, em, a[href], ul, ol, li, br.
 * - b → strong, i → em. Block wrappers (div, h1–h6, blockquote…) become <p> (or unwrap if they hold lists).
 * - script/style/svg/iframe/… are dropped with their contents; every other tag is unwrapped, text kept.
 * - href: https, mailto and relative (incl. {course} tokens) only. http is upgraded to https; anything else
 *   (javascript:, data:, tel:, ftp:) drops the link and keeps its text.
 * - Empty paragraphs are removed. Output is stable: sanitizeRich(sanitizeRich(x)) === sanitizeRich(x).
 */
export function sanitizeRich(html: string): RichText {
  if (!html) return '';
  return serialize(blocks(parseHtml(String(html)), false)).trim();
}

const DROP = new Set(['script', 'style', 'svg', 'math', 'iframe', 'object', 'embed', 'template', 'noscript', 'head', 'title', 'canvas', 'video', 'audio', 'select', 'textarea', 'img', 'picture', 'source', 'input']);
const BLOCK_WRAP = new Set(['p', 'div', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'blockquote', 'section', 'article', 'header', 'footer', 'aside', 'nav', 'main', 'figure', 'figcaption', 'address', 'pre', 'details', 'summary', 'dd', 'dt', 'dl', 'table', 'tbody', 'thead', 'tfoot', 'tr', 'td', 'th', 'caption', 'hr']);
const LIST = new Set(['ul', 'ol']);

type Item = { kind: 'inline' | 'block'; html: string };

/** Validate and normalise a link target. Returns null when the link must be dropped. */
export function safeHref(raw: string | undefined): string | null {
  if (raw === undefined) return null;
  const decoded = decodeEntities(raw).trim();
  if (!decoded) return null;
  const probe = decoded.replace(/[\u0000- \u007f-\u009f]/g, '').toLowerCase();
  if (probe.startsWith('//')) return `https:${decoded.replace(/^\s*/, '')}`;
  const m = /^([a-z][a-z0-9+.-]*):/.exec(probe);
  if (!m) {
    // a colon before the first / ? # would be a scheme we couldn't parse cleanly → reject
    const firstColon = probe.indexOf(':');
    const firstSep = probe.search(/[/?#]/);
    if (firstColon !== -1 && (firstSep === -1 || firstColon < firstSep)) return null;
    return decoded;
  }
  if (m[1] === 'https' || m[1] === 'mailto') return decoded;
  if (m[1] === 'http') return decoded.replace(/^\s*http:/i, 'https:');
  return null;
}

function inl(nodes: HNode[], inA: boolean): string {
  let out = '';
  for (const n of nodes) {
    if (n.type === 'comment') continue;
    if (n.type === 'text') {
      out += escTextKeepEntities(n.raw);
      continue;
    }
    const name = n.name;
    if (DROP.has(name)) continue;
    if (name === 'br') out += '<br>';
    else if (name === 'strong' || name === 'b') out += wrapIf('strong', inl(n.children, inA));
    else if (name === 'em' || name === 'i') out += wrapIf('em', inl(n.children, inA));
    else if (name === 'a') {
      const href = inA ? null : safeHref(attr(n, 'href'));
      const inner = inl(n.children, true);
      out += href !== null && inner.trim() ? `<a href="${esc(href)}">${inner}</a>` : inner;
    } else if (name === 'li' || BLOCK_WRAP.has(name) || LIST.has(name)) {
      const inner = inl(n.children, inA);
      if (inner.trim()) out += (out && !/(<br>|\s)$/.test(out) ? ' ' : '') + inner;
    } else out += inl(n.children, inA);
  }
  return out;
}

const wrapIf = (tag: string, inner: string) => (inner.trim() ? `<${tag}>${inner}</${tag}>` : inner);

const hasBlockDescendant = (n: HElement): boolean =>
  n.children.some(c => c.type === 'el' && !DROP.has(c.name) && (LIST.has(c.name) || BLOCK_WRAP.has(c.name) || hasBlockDescendant(c)));

const isEmptyInline = (s: string) => !s.replace(/<br>/g, '').replace(/&nbsp;|&#160;| /g, '').trim();

function blocks(nodes: HNode[], inLi: boolean): Item[] {
  const items: Item[] = [];
  for (const n of nodes) {
    if (n.type === 'comment') continue;
    if (n.type === 'text') {
      items.push({ kind: 'inline', html: escTextKeepEntities(n.raw) });
      continue;
    }
    if (DROP.has(n.name)) continue;
    if (LIST.has(n.name)) {
      const lis: string[] = [];
      let stray: HNode[] = [];
      const flushStray = () => {
        const s = serialize(blocks(stray, true)).trim();
        if (s && !isEmptyInline(s)) lis.push(`<li>${s}</li>`);
        stray = [];
      };
      for (const c of n.children) {
        if (c.type === 'el' && c.name === 'li') {
          flushStray();
          const s = serialize(blocks(c.children, true)).trim();
          if (s && !isEmptyInline(s)) lis.push(`<li>${s}</li>`);
        } else stray.push(c);
      }
      flushStray();
      if (lis.length) items.push({ kind: 'block', html: `<${n.name}>${lis.join('')}</${n.name}>` });
      continue;
    }
    if (n.name === 'li') {
      // li outside a list: treat as a paragraph
      const s = inl(n.children, false);
      if (!isEmptyInline(s)) items.push({ kind: 'block', html: `<p>${s.trim()}</p>` });
      continue;
    }
    if (n.name === 'hr') continue;
    if (BLOCK_WRAP.has(n.name)) {
      if (hasBlockDescendant(n)) items.push(...blocks(n.children, inLi));
      else {
        const s = inl(n.children, false).trim();
        if (!isEmptyInline(s)) items.push({ kind: 'block', html: `<p>${s}</p>` });
      }
      continue;
    }
    if (hasBlockDescendant(n) && !['strong', 'b', 'em', 'i', 'a'].includes(n.name)) {
      items.push(...blocks(n.children, inLi));
      continue;
    }
    items.push({ kind: 'inline', html: inl([n], false) });
  }
  return items;
}

function serialize(items: Item[]): string {
  let out = '';
  let run = '';
  const flush = () => {
    const t = run.replace(/^\s+|\s+$/g, '');
    if (t && !isEmptyInline(t)) out += t;
    run = '';
  };
  for (const it of items) {
    if (it.kind === 'inline') run += it.html;
    else {
      flush();
      out += it.html;
    }
  }
  flush();
  return out;
}
