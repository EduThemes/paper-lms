import { sanitizeRich, safeHref } from './sanitize';
import { safeImageSrc } from './render/context';
import { sanitizePalette } from './palette';
import { SECTION_TYPES } from './sections-meta';
import type { PageDoc, Section } from './types';

/**
 * Cleans a PageDoc that came from outside the designer's own state: a stored `wiki_pages.design` (which any
 * course teacher or TA can write through the API, and which also arrives through Commons and IMSCC
 * imports), a localStorage draft, or a makeover plan. The renderer escapes and sanitizes on output already;
 * this is the designer's input guard, so nothing in a doc reaches the editing UI unsanitized:
 *
 *   - rich-text fields (`body`, `bio`) go through sanitizeRich;
 *   - every `href` is checked with safeHref (unsafe → '');
 *   - image sources (`customSrc`, `photoSrc`) are checked with safeImageSrc (unsafe → removed);
 *   - sections of an unknown type, or without a string id, are dropped;
 *   - strings are capped, arrays are capped, nesting is bounded, and non-JSON values are removed;
 *   - a custom `palette` goes through sanitizePalette (invalid → removed).
 *
 * A doc the designer wrote itself comes back unchanged (deep-equal), so a saved page's `data-pd-hash` still
 * matches it after cleaning. That is why an accepted href or image source keeps its original spelling (an
 * `http:` link the teacher typed stays `http:`): the renderer normalises both again on output, and only
 * values those checks refuse are removed.
 */
export function sanitizeDoc(doc: PageDoc): PageDoc {
  const src = (doc && typeof doc === 'object' ? doc : {}) as unknown as Record<string, unknown>;
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(src)) {
    if (k === 'sections' || k === 'palette' || k === 'customCss') continue;
    const clean = cleanValue(v, 0, TEXT_MAX);
    if (clean !== DROP) out[k] = clean;
  }
  out.sections = Array.isArray(src.sections)
    ? src.sections.slice(0, ARRAY_MAX).map(cleanSection).filter((s): s is Section => s !== null)
    : [];
  if (src.palette !== undefined) {
    const palette = sanitizePalette(src.palette);
    // Valid with nothing extra: keep the colours as written (sanitizePalette lower-cases them, and
    // resolvePalette runs it again at render time), so the saved hash still matches.
    if (palette) out.palette = sameKeys(src.palette as object, palette) ? { ...(src.palette as object) } : palette;
  }
  if (typeof src.customCss === 'string') out.customCss = src.customCss.slice(0, CSS_MAX);
  return out as unknown as PageDoc;
}

/** Plain text fields. Generous: nothing the designer offers comes close. */
export const DOC_TEXT_MAX = 4000;
/** Rich-text fields, before sanitizing. */
export const DOC_RICH_MAX = 50_000;
/** `customCss`: the server's page stylesheet limit (csssanitize.MaxBytes). */
export const DOC_CSS_MAX = 200_000;

const TEXT_MAX = DOC_TEXT_MAX;
const RICH_MAX = DOC_RICH_MAX;
const CSS_MAX = DOC_CSS_MAX;
const ARRAY_MAX = 200;
const DEPTH_MAX = 4;

const KNOWN_TYPES = new Set<string>(SECTION_TYPES.map(t => t.type));
const RICH_KEYS = new Set(['body', 'bio']);
const IMAGE_KEYS = new Set(['customSrc', 'photoSrc']);

const DROP = Symbol('drop');

const sameKeys = (a: object, b: object) => {
  const ka = Object.keys(a);
  return ka.length === Object.keys(b).length && ka.every(k => Object.prototype.hasOwnProperty.call(b, k));
};

function cleanSection(raw: unknown): Section | null {
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return null;
  const s = raw as Record<string, unknown>;
  if (typeof s.id !== 'string' || !s.id || typeof s.type !== 'string' || !KNOWN_TYPES.has(s.type)) return null;
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(s)) {
    if (RICH_KEYS.has(k)) {
      out[k] = typeof v === 'string' ? sanitizeRich(v.slice(0, RICH_MAX)) : '';
      continue;
    }
    const clean = cleanField(k, v, 1);
    if (clean !== DROP) out[k] = clean;
  }
  return out as unknown as Section;
}

/** A value under key `k`: hrefs and image sources get their own checks, everything else is generic. */
function cleanField(k: string, v: unknown, depth: number): unknown {
  if (k === 'href') {
    if (typeof v !== 'string' || v.length > TEXT_MAX) return '';
    return safeHref(v) === null ? '' : v;
  }
  if (IMAGE_KEYS.has(k)) {
    if (typeof v !== 'string' || v.length > TEXT_MAX) return DROP;
    return safeImageSrc(v) === null ? DROP : v;
  }
  return cleanValue(v, depth, TEXT_MAX);
}

function cleanValue(v: unknown, depth: number, max: number): unknown {
  if (v === null) return null;
  if (typeof v === 'string') return v.length > max ? v.slice(0, max) : v;
  if (typeof v === 'boolean') return v;
  if (typeof v === 'number') return Number.isFinite(v) ? v : DROP;
  if (depth >= DEPTH_MAX) return DROP;
  if (Array.isArray(v)) {
    return v
      .slice(0, ARRAY_MAX)
      .map(x => cleanValue(x, depth + 1, max))
      .filter(x => x !== DROP);
  }
  if (typeof v === 'object') {
    const out: Record<string, unknown> = {};
    for (const [k, x] of Object.entries(v as Record<string, unknown>)) {
      const clean = cleanField(k, x, depth + 1);
      if (clean !== DROP) out[k] = clean;
    }
    return out;
  }
  return DROP;
}
