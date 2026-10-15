import { ensureContrast, mix, normHex, readable } from '../color';
import { iconSrc, themeImageUrl, themeImageUrl2x } from '../icons';
import { sanitizePalette } from '../palette';
import { safeHref } from '../sanitize';
import { strings } from '../strings';
import type { IconStyle, LayoutId, Palette, PageDoc, RenderOptions, Section, Theme } from '../types';

/** A surface plus everything that may sit on it, all checked for contrast. */
export interface Tone {
  /**
   * Which of ctx.tones this is. Layout decisions compare roles, never colours, so the light and dark renders
   * of a page always have the same structure (render/design.ts relies on it).
   */
  role?: 'page' | 'surface' | 'primary' | 'secondary' | 'tint' | 'tint2';
  bg: string;
  fg: string;
  /** Heading colour: the brand primary when it reads well here, else the text colour. */
  heading: string;
  link: string;
  /** Secondary text (dates, descriptions) — still ≥ 4.5:1. */
  muted: string;
  /** Decorative hairline colour. */
  border: string;
}

export interface TypeScale {
  title: string;
  h2: string;
  h3: string;
  lead: string;
  body: string;
  small: string;
}

export interface Ctx {
  doc: PageDoc;
  theme: Theme;
  p: Palette;
  opts: RenderOptions;
  layout: LayoutId;
  young: boolean;
  compact: boolean;
  /** Radii in px. */
  r: { sm: number; md: number; lg: number; xl: number };
  /** Vertical rhythm between sections, inside panels, and between grid items. */
  gap: string;
  pad: string;
  gridGap: number;
  type: TypeScale;
  tones: { page: Tone; surface: Tone; primary: Tone; secondary: Tone; tint: Tone; tint2: Tone };
  headingStyle: PageDoc['options']['headingStyle'];
  headingFont?: string;
  iconStyle: IconStyle;
  s: Record<string, string>;
  href(raw: string | undefined): string | null;
  icon(name: string, fg: string, bg: string, style?: IconStyle): string;
  /** Theme art carries `srcset` (its @2x twin); a teacher's own banner does not. */
  image(section: Extract<Section, { type: 'banner' }>): { src: string; srcset?: string; width?: number; height?: number } | null;
  /** `srcset` for a theme image: "<1x> 1x, <2x> 2x". */
  srcset(file: string): string;
}

export function makeTone(bg: string, fg: string, p: Palette, role?: Tone['role']): Tone {
  const b = normHex(bg) ?? '#ffffff';
  const f = ensureContrast(normHex(fg) ?? '#1f2328', b, 4.5);
  return {
    role,
    bg: b,
    fg: f,
    heading: readable(b, [p.primary, f], 4.5),
    link: readable(b, [p.link, p.primary, f], 4.5),
    muted: ensureContrast(mix(f, b, 0.22), b, 4.6),
    border: mix(f, b, 0.8),
  };
}

const RADII = {
  square: { sm: 2, md: 3, lg: 4, xl: 4 },
  soft: { sm: 6, md: 10, lg: 14, xl: 20 },
  round: { sm: 10, md: 16, lg: 22, xl: 28 },
} as const;

export function makeCtx(doc: PageDoc, theme: Theme, opts: RenderOptions, type: TypeScale, extra?: { forceRound?: boolean; palette?: Palette }): Ctx {
  const raw = extra?.palette ?? resolvePalette(doc, theme);
  // 6-digit lower-case hex everywhere (Canvas's course checker reads only hex)
  const p = Object.fromEntries(Object.entries(raw).map(([k, v]) => [k, k === 'id' || k === 'name' ? v : normHex(v) ?? v])) as unknown as Palette;
  const compact = doc.options?.density === 'compact';
  const young = !!theme.audiences?.includes('early-elementary');
  const corners = extra?.forceRound ? 'round' : theme.corners ?? 'soft';
  const page = makeTone(p.background, p.text, p, 'page');
  const tintBg = mix(p.primary, p.background, 0.92);
  const tint2Bg = mix(p.secondary, p.background, 0.9);
  const ctx: Ctx = {
    doc,
    theme,
    p,
    opts,
    layout: doc.layout,
    young,
    compact,
    r: { ...RADII[corners] },
    gap: compact ? '2rem' : '3rem',
    pad: compact ? '1rem 1.25rem' : '1.5rem 1.75rem',
    gridGap: compact ? 12 : 16,
    type,
    tones: {
      page,
      surface: makeTone(p.surface, p.onSurface, p, 'surface'),
      primary: makeTone(p.primary, p.onPrimary, p, 'primary'),
      secondary: makeTone(p.secondary, p.onSecondary, p, 'secondary'),
      tint: makeTone(tintBg, readable(tintBg, [p.text, p.onSurface]), p, 'tint'),
      tint2: makeTone(tint2Bg, readable(tint2Bg, [p.text, p.onSurface]), p, 'tint2'),
    },
    headingStyle: doc.options?.headingStyle ?? 'plain',
    headingFont: displayFont(theme.headingFont),
    iconStyle: theme.iconStyle ?? 'solid-circle',
    s: strings(doc.locale),
    href(raw) {
      const safe = safeHref(raw ?? '');
      if (safe === null) return null;
      return opts.courseId ? safe.split('{course}').join(encodeURIComponent(opts.courseId)) : safe;
    },
    icon(name, fg, bg, style) {
      return iconSrc(opts.iconBase ?? `${String(opts.assetBase ?? '').replace(/\/+$/, '')}/icons`, name, { style: style ?? ctx.iconStyle, fg, bg });
    },
    image(sec) {
      if (sec.customSrc) {
        const src = safeImageSrc(sec.customSrc);
        return src ? { src } : null;
      }
      const img = theme.images?.find(i => i.id === sec.imageId) ?? (sec.imageId ? undefined : theme.images?.find(i => i.kind === 'banner'));
      if (!img) return null;
      return { src: themeImageUrl(opts.assetBase, theme.id, img), srcset: ctx.srcset(img.file), width: img.width, height: img.height };
    },
    srcset(file) {
      return `${themeImageUrl(opts.assetBase, theme.id, { file })} 1x, ${themeImageUrl2x(opts.assetBase, theme.id, { file })} 2x`;
    },
  };
  return ctx;
}

/**
 * The palette a doc renders with: its own custom palette when present and valid (hex colours only, so nothing
 * but a colour can reach an inline style), else the theme palette named by paletteId, else the theme default.
 */
export function resolvePalette(doc: Pick<PageDoc, 'paletteId' | 'palette'>, theme: Theme): Palette {
  const custom = doc.palette ? sanitizePalette(doc.palette) : null;
  return custom ?? theme.palettes.find(x => x.id === doc.paletteId) ?? theme.palettes[0];
}

/** https only (http is upgraded); data:, javascript: and the like are refused. */
export function safeImageSrc(src: string): string | null {
  const s = src.trim();
  if (/^https:\/\//i.test(s)) return s;
  if (/^http:\/\//i.test(s)) return s.replace(/^http:/i, 'https:');
  if (/^\/\//.test(s)) return `https:${s}`;
  if (/^\/(?!\/)/.test(s)) return s; // Canvas-relative file URL
  return null;
}

/**
 * The heading font goes in the designer stylesheet (never inline, so a reader's own font can win), and only
 * for a real display face — never for a Lato/Helvetica stack, which reads as the default UI face anyway.
 */
function displayFont(f: string | undefined): string | undefined {
  const v = f?.trim();
  if (!v) return undefined;
  const first = v.split(',')[0].replace(/["']/g, '').trim().toLowerCase();
  return ['lato', 'lato extended', 'inherit', 'helvetica neue', 'helvetica', 'arial', 'sans-serif'].includes(first) ? undefined : v;
}
