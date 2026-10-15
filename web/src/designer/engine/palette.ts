/**
 * Custom colour schemes: derive a complete, AA-passing Palette from 1–3 brand colours, check one, and
 * fix one. Pure. The rules are the ones on `Palette` in types.ts:
 *   - every `on*` colour, `text` on background/surface and `link` on background/surface reach 4.5:1;
 *   - `accent` is decorative only (never text), so it is not checked;
 *   - `background` stays near-white, because Canvas pages are white.
 * Fixes move a colour's HSL lightness only, so the hue (the "brand") is kept.
 */
import { contrastRatio, hexToRgb, luminance, mix, normHex, readable, rgbToHex } from './color';
import type { Palette } from './types';

export { contrastRatio };

/** WCAG AA for normal-size text. */
export const AA = 4.5;
/** Minimum relative luminance for a custom page background ("near-white"). */
export const NEAR_WHITE = 0.85;

export type PaletteKey = Exclude<keyof Palette, 'id' | 'name'>;
export const PALETTE_KEYS: readonly PaletteKey[] = [
  'primary',
  'onPrimary',
  'secondary',
  'onSecondary',
  'accent',
  'surface',
  'onSurface',
  'background',
  'text',
  'link',
];

export interface PalettePair {
  /** Stable id, e.g. "onPrimary" or "link-surface". */
  id: string;
  fg: PaletteKey;
  bg: PaletteKey;
  /** Plain-language name a teacher understands. */
  label: string;
}

/** Every text-on-colour pair the palette contract requires to reach AA. */
export const PALETTE_PAIRS: readonly PalettePair[] = [
  { id: 'onPrimary', fg: 'onPrimary', bg: 'primary', label: 'Text on your main colour' },
  { id: 'onSecondary', fg: 'onSecondary', bg: 'secondary', label: 'Text on your second colour' },
  { id: 'onSurface', fg: 'onSurface', bg: 'surface', label: 'Text on panels' },
  { id: 'text-background', fg: 'text', bg: 'background', label: 'Page text' },
  { id: 'text-surface', fg: 'text', bg: 'surface', label: 'Page text on panels' },
  { id: 'link-background', fg: 'link', bg: 'background', label: 'Links' },
  { id: 'link-surface', fg: 'link', bg: 'surface', label: 'Links on panels' },
];

export interface PairResult extends PalettePair {
  fgColor: string;
  bgColor: string;
  /** Exact WCAG ratio (1..21). Round only for display. */
  ratio: number;
  pass: boolean;
}

export interface PaletteBase {
  primary: string;
  secondary?: string;
  accent?: string;
}

const DARK_TEXT = '#1f2328';
const FALLBACK_PRIMARY = '#1f6fb2';
const ID_RE = /^[A-Za-z0-9_-]{1,64}$/;
/** "#rgb" or "#rrggbb" only. */
export const HEX_RE = /^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/;
export const PALETTE_NAME_MAX = 60;

/* ------------------------------------------------------------------ HSL */

/** Hex → [hue 0..360, saturation 0..1, lightness 0..1]. */
export function hexToHsl(c: string): [number, number, number] {
  const [r, g, b] = hexToRgb(c).map(v => v / 255) as [number, number, number];
  const max = Math.max(r, g, b);
  const min = Math.min(r, g, b);
  const l = (max + min) / 2;
  if (max === min) return [0, 0, l];
  const d = max - min;
  const s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
  const h = max === r ? (g - b) / d + (g < b ? 6 : 0) : max === g ? (b - r) / d + 2 : (r - g) / d + 4;
  return [h * 60, s, l];
}

export function hslToHex(h: number, s: number, l: number): string {
  const hue = (((h % 360) + 360) % 360) / 360;
  const sat = Math.max(0, Math.min(1, s));
  const lig = Math.max(0, Math.min(1, l));
  if (sat === 0) return rgbToHex([lig * 255, lig * 255, lig * 255]);
  const q = lig < 0.5 ? lig * (1 + sat) : lig + sat - lig * sat;
  const p = 2 * lig - q;
  const ch = (t: number) => {
    let x = t;
    if (x < 0) x += 1;
    if (x > 1) x -= 1;
    if (x < 1 / 6) return p + (q - p) * 6 * x;
    if (x < 1 / 2) return q;
    if (x < 2 / 3) return p + (q - p) * (2 / 3 - x) * 6;
    return p;
  };
  return rgbToHex([ch(hue + 1 / 3) * 255, ch(hue) * 255, ch(hue - 1 / 3) * 255]);
}

/**
 * The colour with `fg`'s hue and saturation whose lightness is closest to fg's and that reaches `min`
 * against every colour in `bgs`. Returns fg unchanged when it already passes, null when no lightness works.
 */
export function nudgeLightness(fg: string, bgs: string[], min = AA): string | null {
  const f = normHex(fg) ?? DARK_TEXT;
  const ok = (c: string) => bgs.every(b => contrastRatio(c, b) >= min);
  if (ok(f)) return f;
  const [h, s, l] = hexToHsl(f);
  const steps: number[] = [];
  for (let i = 0; i <= 400; i++) steps.push(i / 400);
  steps.sort((a, b) => Math.abs(a - l) - Math.abs(b - l));
  for (const L of steps) {
    const c = hslToHex(h, s, L);
    if (ok(c)) return c;
  }
  return null;
}

/* ------------------------------------------------------------------ validate */

/** Strict: an object with a valid id, a name of 1..60 characters and every colour a hex. Colours come back normalised. */
export function sanitizePalette(x: unknown): Palette | null {
  if (!x || typeof x !== 'object' || Array.isArray(x)) return null;
  const o = x as Record<string, unknown>;
  if (typeof o.id !== 'string' || !ID_RE.test(o.id)) return null;
  if (typeof o.name !== 'string') return null;
  const name = o.name.trim();
  if (!name || name.length > PALETTE_NAME_MAX) return null;
  const out: Record<string, string> = { id: o.id, name };
  for (const k of PALETTE_KEYS) {
    const v = o[k];
    if (typeof v !== 'string' || !HEX_RE.test(v.trim())) return null;
    out[k] = normHex(v)!;
  }
  return out as unknown as Palette;
}

/* ------------------------------------------------------------------ check */

export function checkPalette(p: Palette): PairResult[] {
  return PALETTE_PAIRS.map(pair => {
    const fgColor = normHex(p[pair.fg]) ?? '#000000';
    const bgColor = normHex(p[pair.bg]) ?? '#ffffff';
    const ratio = contrastRatio(fgColor, bgColor);
    return { ...pair, fgColor, bgColor, ratio, pass: ratio >= AA };
  });
}

export const paletteFailures = (p: Palette): PairResult[] => checkPalette(p).filter(r => !r.pass);

export const isNearWhite = (c: string): boolean => luminance(normHex(c) ?? '#000000') >= NEAR_WHITE;

/** Every pair reaches AA and the background is near-white. */
export const palettePasses = (p: Palette): boolean => paletteFailures(p).length === 0 && isNearWhite(p.background);

/* ------------------------------------------------------------------ fix */

/**
 * Makes a palette pass: lightens a dark background to near-white, lightens the panel colour only if no
 * text colour could work on it, then moves each failing text/link/on* colour's lightness the shortest way
 * that passes. Brand colours (primary, secondary, accent) are never changed. Invalid colours are replaced.
 */
export function fixPalette(p: Palette): Palette {
  const primary = normHex(p.primary) ?? FALLBACK_PRIMARY;
  const secondary = normHex(p.secondary) ?? primary;
  const accent = normHex(p.accent) ?? primary;
  let background = normHex(p.background) ?? '#ffffff';
  if (!isNearWhite(background)) {
    const [h, s] = hexToHsl(background);
    background = '#ffffff';
    for (let L = 0.85; L <= 1.0001; L += 0.01) {
      const c = hslToHex(h, s, L);
      if (isNearWhite(c)) {
        background = c;
        break;
      }
    }
  }
  const surface0 = normHex(p.surface) ?? mix(primary, background, 0.92);
  const text0 = normHex(p.text) ?? DARK_TEXT;
  const link0 = normHex(p.link) ?? primary;
  // Text and links sit on both the page and panels, so the panel must leave room for a colour that reads on both.
  const feasible = (sf: string) => !!nudgeLightness(text0, [background, sf]) && !!nudgeLightness(link0, [background, sf]);
  let surface = surface0;
  for (let i = 1; i <= 10 && !feasible(surface); i++) surface = mix(surface0, background, i / 10);
  const text = nudgeLightness(text0, [background, surface]) ?? DARK_TEXT;
  const link = nudgeLightness(link0, [background, surface]) ?? text;
  const on = (fg: string | undefined, bg: string, fallback: string) =>
    nudgeLightness(normHex(fg) ?? fallback, [bg]) ?? readable(bg, ['#000000', '#ffffff']);
  return {
    id: p.id,
    name: p.name,
    primary,
    onPrimary: on(p.onPrimary, primary, readable(primary, ['#ffffff', DARK_TEXT])),
    secondary,
    onSecondary: on(p.onSecondary, secondary, readable(secondary, ['#ffffff', DARK_TEXT])),
    accent,
    surface,
    onSurface: on(p.onSurface, surface, text),
    background,
    text,
    link,
  };
}

/* ------------------------------------------------------------------ derive */

/**
 * A complete palette from 1–3 brand colours: white page, a panel tinted with the primary, dark text,
 * white or dark text on the brand colours (whichever reads), and links in the primary's hue, darkened
 * until they read on the page and the panels. Missing secondary/accent are picked from the primary's hue.
 */
export function derivePalette(base: PaletteBase, name: string, id = 'custom'): Palette {
  const primary = normHex(base.primary) ?? FALLBACK_PRIMARY;
  const [h, s, l] = hexToHsl(primary);
  const secondary = normHex(base.secondary) ?? hslToHex(h + 30, s, Math.max(0.25, Math.min(0.55, l)));
  const accent = normHex(base.accent) ?? hslToHex(h + 180, Math.max(s, 0.45), 0.6);
  const background = '#ffffff';
  const surface = mix(primary, background, 0.92);
  const pick = (bg: string) => readable(bg, ['#ffffff', DARK_TEXT]);
  const cleanName = (name ?? '').trim().slice(0, PALETTE_NAME_MAX) || 'My colours';
  return fixPalette({
    id: ID_RE.test(id) ? id : 'custom',
    name: cleanName,
    primary,
    onPrimary: pick(primary),
    secondary,
    onSecondary: pick(secondary),
    accent,
    surface,
    onSurface: DARK_TEXT,
    background,
    text: DARK_TEXT,
    link: primary,
  });
}
