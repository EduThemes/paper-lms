/**
 * Preview-only CSS scoper. Approximates what Paper's server does to a page stylesheet (csssanitize.Scope)
 * so the designer preview shows what Paper will show. The server's scoper is authoritative; this one only
 * has to be close:
 *
 *   - every selector gets the page root as an ancestor; `:root`, `html` and `body` at the start become it;
 *   - `@media (prefers-color-scheme: dark)` follows a `dark` class on <html> (`light`: its absence);
 *   - `@media`, `@supports`, `@container` and `@keyframes` are kept; `@import`, `@font-face`, `@charset`,
 *     `@namespace`, `@page`, `@layer` and `@property` are dropped;
 *   - `</style` can never appear in the output.
 */

export const PREVIEW_ROOT = '#paper-page-preview';

const DROP_AT = new Set(['import', 'font-face', 'charset', 'namespace', 'page', 'layer', 'property']);
const KEEP_AS_IS = /^(-webkit-|-moz-)?keyframes$/;
const RECURSE_AT = new Set(['media', 'supports', 'container']);

function stripComments(css) {
  let out = '';
  let i = 0;
  let quote = null;
  while (i < css.length) {
    const ch = css[i];
    if (quote) {
      out += ch;
      if (ch === '\\') {
        out += css[i + 1] ?? '';
        i += 2;
        continue;
      }
      if (ch === quote) quote = null;
      i += 1;
      continue;
    }
    if (ch === '"' || ch === "'") {
      quote = ch;
      out += ch;
      i += 1;
      continue;
    }
    if (ch === '/' && css[i + 1] === '*') {
      const end = css.indexOf('*/', i + 2);
      i = end < 0 ? css.length : end + 2;
      continue;
    }
    out += ch;
    i += 1;
  }
  return out;
}

/** Index of the `}` that closes the `{` at `open`, or -1. Strings are skipped. */
function matchBrace(css, open) {
  let depth = 0;
  let quote = null;
  for (let i = open; i < css.length; i++) {
    const ch = css[i];
    if (quote) {
      if (ch === '\\') i += 1;
      else if (ch === quote) quote = null;
      continue;
    }
    if (ch === '"' || ch === "'") quote = ch;
    else if (ch === '{') depth += 1;
    else if (ch === '}') {
      depth -= 1;
      if (depth === 0) return i;
    }
  }
  return -1;
}

/** Top-level statements: { prelude, body } for blocks, { prelude, body: null } for `@x …;`. */
function statements(css) {
  const out = [];
  let i = 0;
  while (i < css.length) {
    while (i < css.length && /\s/.test(css[i])) i += 1;
    if (i >= css.length) break;
    let j = i;
    let quote = null;
    while (j < css.length) {
      const ch = css[j];
      if (quote) {
        if (ch === '\\') j += 1;
        else if (ch === quote) quote = null;
      } else if (ch === '"' || ch === "'") quote = ch;
      else if (ch === '{' || ch === ';' || ch === '}') break;
      j += 1;
    }
    const prelude = css.slice(i, j).trim();
    if (j >= css.length) break;
    if (css[j] === ';' || css[j] === '}') {
      if (prelude) out.push({ prelude, body: null });
      i = j + 1;
      continue;
    }
    const close = matchBrace(css, j);
    if (close < 0) break; // unbalanced: drop the rest
    out.push({ prelude, body: css.slice(j + 1, close) });
    i = close + 1;
  }
  return out;
}

/** Splits on commas that aren't inside (), [] or strings. */
export function splitSelectors(sel) {
  const parts = [];
  let depth = 0;
  let quote = null;
  let start = 0;
  for (let i = 0; i < sel.length; i++) {
    const ch = sel[i];
    if (quote) {
      if (ch === '\\') i += 1;
      else if (ch === quote) quote = null;
      continue;
    }
    if (ch === '"' || ch === "'") quote = ch;
    else if (ch === '(' || ch === '[') depth += 1;
    else if (ch === ')' || ch === ']') depth -= 1;
    else if (ch === ',' && depth === 0) {
      parts.push(sel.slice(start, i));
      start = i + 1;
    }
  }
  parts.push(sel.slice(start));
  return parts.map((s) => s.trim()).filter(Boolean);
}

const ROOTISH = /^(?::root|html|body)(?![\w-])/i;

/** One selector, scoped under `prefix` (the root, possibly with a dark-mode ancestor). */
export function scopeSelector(sel, prefix) {
  let rest = sel.trim();
  let hitRoot = false;
  // Leading :root / html / body (and combinators between them) all mean "the page".
  for (;;) {
    const m = ROOTISH.exec(rest);
    if (!m) break;
    hitRoot = true;
    rest = rest.slice(m[0].length);
    const comb = /^\s*>?\s*/.exec(rest);
    const after = rest.slice(comb[0].length);
    if (ROOTISH.test(after)) rest = after;
    else break;
  }
  if (!hitRoot) return `${prefix} ${rest}`;
  if (!rest.trim()) return prefix;
  if (/^[.[:#]/.test(rest)) return `${prefix}${rest}`; // compound on the root, e.g. :root.k2
  return `${prefix} ${rest.trim()}`;
}

function scopeRules(css, prefix, root) {
  const out = [];
  for (const { prelude, body } of statements(css)) {
    if (prelude.startsWith('@')) {
      const name = /^@([\w-]+)/.exec(prelude)?.[1]?.toLowerCase() ?? '';
      if (DROP_AT.has(name) || body === null) continue;
      if (KEEP_AS_IS.test(name)) {
        out.push(`${prelude}{${body}}`);
        continue;
      }
      if (!RECURSE_AT.has(name)) continue;
      if (name === 'media') {
        const cond = prelude.slice(6).trim();
        const scheme = /\(\s*prefers-color-scheme\s*:\s*(dark|light)\s*\)/i.exec(cond);
        if (scheme) {
          const modePrefix = scheme[1].toLowerCase() === 'dark' ? `html.dark ${root}` : `html:not(.dark) ${root}`;
          const rest = cond
            .replace(scheme[0], '')
            .replace(/^\s*and\s+/i, '')
            .replace(/\s+and\s*$/i, '')
            .replace(/\s+and\s+and\s+/i, ' and ')
            .trim();
          const inner = scopeRules(body, modePrefix, root);
          out.push(rest ? `@media ${rest}{${inner}}` : inner);
          continue;
        }
      }
      out.push(`${prelude}{${scopeRules(body, prefix, root)}}`);
      continue;
    }
    if (body === null) continue;
    const selectors = splitSelectors(prelude).map((s) => scopeSelector(s, prefix));
    if (selectors.length) out.push(`${selectors.join(', ')}{${body.trim()}}`);
  }
  return out.join('\n');
}

/** Scopes a page stylesheet to `root` for the preview. */
export function scopeCss(css, root = PREVIEW_ROOT) {
  if (!css || !String(css).trim()) return '';
  const scoped = scopeRules(stripComments(String(css)), root, root);
  return scoped.replace(/<\/(style)/gi, '<\\/$1');
}
