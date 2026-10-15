/**
 * Tiny HTML helpers with no DOM dependency (core runs in Node and the browser):
 * escaping, a forgiving tokenizer/tree builder, inline-style parsing and serialisation.
 * Used by sanitizeRich, the rich-text styler, the accessibility scan and the tests.
 */

export const esc = (s: string | number | undefined | null): string =>
  String(s ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');

/** Escape text but keep well-formed entity references (&eacute; &#233; &#xE9;) intact. */
export const escTextKeepEntities = (raw: string): string =>
  raw
    .replace(/&(?!(?:#\d{1,7}|#x[0-9a-fA-F]{1,6}|[a-zA-Z][a-zA-Z0-9]{1,31});)/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');

/** Join CSS declarations, dropping falsy parts: st('margin:0', cond && 'color:#000'). */
export const st = (...decls: (string | false | null | undefined | 0)[]): string =>
  decls.filter(Boolean).join('; ') + ';';

type AttrVal = string | number | boolean | null | undefined;

/** Build an element. Attribute values are escaped; `true` renders a bare value-less attribute is avoided (Canvas-safe). */
export function h(name: string, attrs: Record<string, AttrVal>, ...children: (string | false | null | undefined)[]): string {
  const a = Object.entries(attrs)
    .filter(([, v]) => v !== undefined && v !== null && v !== false)
    .map(([k, v]) => ` ${k}="${esc(v === true ? k : String(v))}"`)
    .join('');
  if (VOID.has(name)) return `<${name}${a}>`;
  return `<${name}${a}>${children.filter(c => c !== false && c !== null && c !== undefined).join('')}</${name}>`;
}

export const VOID = new Set(['area', 'base', 'br', 'col', 'embed', 'hr', 'img', 'input', 'link', 'meta', 'param', 'source', 'track', 'wbr']);
const RAW_TEXT = new Set(['script', 'style', 'textarea', 'title', 'xmp', 'noscript']);

export interface HText {
  type: 'text';
  /** Raw source text, entities NOT decoded. */
  raw: string;
}
export interface HElement {
  type: 'el';
  name: string;
  attrs: [string, string][];
  children: HNode[];
}
export interface HComment {
  type: 'comment';
  raw: string;
}
export type HNode = HText | HElement | HComment;

/** Parse an HTML fragment into a tree. Lenient: unknown end tags are ignored, unclosed tags close at the end. */
export function parseHtml(src: string): HNode[] {
  const root: HElement = { type: 'el', name: '#root', attrs: [], children: [] };
  const stack: HElement[] = [root];
  const top = () => stack[stack.length - 1];
  let i = 0;
  const pushText = (raw: string) => {
    if (!raw) return;
    const kids = top().children;
    const last = kids[kids.length - 1];
    if (last && last.type === 'text') last.raw += raw;
    else kids.push({ type: 'text', raw });
  };
  while (i < src.length) {
    const lt = src.indexOf('<', i);
    if (lt === -1) {
      pushText(src.slice(i));
      break;
    }
    if (lt > i) pushText(src.slice(i, lt));
    i = lt;
    if (src.startsWith('<!--', i)) {
      const end = src.indexOf('-->', i + 4);
      const stop = end === -1 ? src.length : end + 3;
      top().children.push({ type: 'comment', raw: src.slice(i, stop) });
      i = stop;
      continue;
    }
    if (src[i + 1] === '!' || src[i + 1] === '?') {
      const end = src.indexOf('>', i);
      i = end === -1 ? src.length : end + 1;
      continue;
    }
    const endTag = /^<\/([a-zA-Z][a-zA-Z0-9:-]*)[^>]*>/.exec(src.slice(i));
    if (endTag) {
      const name = endTag[1].toLowerCase();
      for (let k = stack.length - 1; k > 0; k--) {
        if (stack[k].name === name) {
          stack.length = k;
          break;
        }
      }
      i += endTag[0].length;
      continue;
    }
    const open = /^<([a-zA-Z][a-zA-Z0-9:-]*)/.exec(src.slice(i));
    if (!open) {
      pushText('<');
      i += 1;
      continue;
    }
    const name = open[1].toLowerCase();
    i += open[0].length;
    const attrs: [string, string][] = [];
    let selfClose = false;
    // attributes
    for (;;) {
      while (i < src.length && /\s/.test(src[i])) i++;
      if (i >= src.length) break;
      if (src[i] === '>') {
        i++;
        break;
      }
      if (src[i] === '/' && src[i + 1] === '>') {
        selfClose = true;
        i += 2;
        break;
      }
      if (src[i] === '/') {
        i++;
        continue;
      }
      const m = /^([^\s"'>\/=]+)/.exec(src.slice(i));
      if (!m) {
        i++;
        continue;
      }
      const an = m[1].toLowerCase();
      i += m[1].length;
      while (i < src.length && /\s/.test(src[i])) i++;
      let val = '';
      if (src[i] === '=') {
        i++;
        while (i < src.length && /\s/.test(src[i])) i++;
        const q = src[i];
        if (q === '"' || q === "'") {
          const end = src.indexOf(q, i + 1);
          const stop = end === -1 ? src.length : end;
          val = src.slice(i + 1, stop);
          i = stop + 1;
        } else {
          const u = /^[^\s>]*/.exec(src.slice(i))![0];
          val = u;
          i += u.length;
        }
      }
      if (!attrs.some(([k]) => k === an)) attrs.push([an, val]);
    }
    const el: HElement = { type: 'el', name, attrs, children: [] };
    top().children.push(el);
    if (VOID.has(name) || selfClose) continue;
    if (RAW_TEXT.has(name)) {
      const close = src.toLowerCase().indexOf(`</${name}`, i);
      const stop = close === -1 ? src.length : close;
      if (stop > i) el.children.push({ type: 'text', raw: src.slice(i, stop) });
      const gt = close === -1 ? src.length : src.indexOf('>', close);
      i = gt === -1 ? src.length : gt + 1;
      continue;
    }
    stack.push(el);
  }
  return root.children;
}

export const attr = (el: HElement, name: string): string | undefined => el.attrs.find(([k]) => k === name)?.[1];

const NAMED: Record<string, string> = { amp: '&', lt: '<', gt: '>', quot: '"', apos: "'", nbsp: ' ', colon: ':', tab: '\t', newline: '\n', rsquo: '’', lsquo: '‘', ldquo: '“', rdquo: '”', mdash: '—', ndash: '–', hellip: '…', middot: '·', rarr: '→', larr: '←', copy: '©' };

/** Decode entity references (numeric + the common named ones). */
export function decodeEntities(s: string): string {
  return s.replace(/&(#\d+|#x[0-9a-fA-F]+|[a-zA-Z][a-zA-Z0-9]*);?/g, (m, ref: string) => {
    if (ref[0] === '#') {
      const code = ref[1] === 'x' || ref[1] === 'X' ? parseInt(ref.slice(2), 16) : parseInt(ref.slice(1), 10);
      return Number.isFinite(code) && code > 0 && code < 0x110000 ? String.fromCodePoint(code) : '';
    }
    const v = NAMED[ref] ?? NAMED[ref.toLowerCase()];
    return v !== undefined && m.endsWith(';') ? v : v !== undefined && ['amp', 'lt', 'gt', 'quot'].includes(ref) ? v : m;
  });
}

/** Visible text of a subtree (entities decoded, whitespace collapsed). */
export function textOf(nodes: HNode[]): string {
  let out = '';
  const walk = (n: HNode) => {
    if (n.type === 'text') out += decodeEntities(n.raw);
    else if (n.type === 'el') {
      if (n.name === 'br') out += ' ';
      n.children.forEach(walk);
      if (BLOCKISH.has(n.name)) out += ' ';
    }
  };
  nodes.forEach(walk);
  return out.replace(/\s+/g, ' ').trim();
}
const BLOCKISH = new Set(['p', 'div', 'li', 'h2', 'h3', 'h4', 'h5', 'h6', 'section', 'ul', 'ol']);

/** Parse an inline style attribute into ordered [prop, value] pairs (props lower-cased). */
export function parseStyle(style: string | undefined): [string, string][] {
  if (!style) return [];
  const out: [string, string][] = [];
  let depth = 0;
  let cur = '';
  const flush = () => {
    const idx = cur.indexOf(':');
    if (idx > 0) out.push([cur.slice(0, idx).trim().toLowerCase(), cur.slice(idx + 1).trim()]);
    cur = '';
  };
  for (const ch of style) {
    if (ch === '(') depth++;
    if (ch === ')') depth = Math.max(0, depth - 1);
    if (ch === ';' && depth === 0) flush();
    else cur += ch;
  }
  flush();
  return out;
}

/** Depth-first walk over elements. */
export function walk(nodes: HNode[], fn: (el: HElement, ancestors: HElement[]) => void, ancestors: HElement[] = []): void {
  for (const n of nodes) {
    if (n.type !== 'el') continue;
    fn(n, ancestors);
    walk(n.children, fn, [...ancestors, n]);
  }
}
