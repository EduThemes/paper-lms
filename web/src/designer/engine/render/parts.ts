import { ensureContrast, mix } from '../color';
import { esc, h, parseHtml, attr, type HNode } from '../html';
import { sanitizeRich } from '../sanitize';
import type { IconStyle, Section } from '../types';
import type { Ctx, Tone } from './context';

/** Every element that sets a background also sets a colour (Canvas checker + dark-mode rule). */
export const paint = (t: Tone) => `background:${t.bg}; color:${t.fg}`;

export const px = (n: number) => `${n}px`;

export const LIST_RESET = 'list-style:none; margin:0; padding:0';

/** Visually hidden but read aloud (inline — Canvas's .screenreader-only class is unstyled on mobile). */
export const vh = (text: string) =>
  `<span style="position:absolute; width:1px; height:1px; margin:-1px; padding:0; overflow:hidden; clip:rect(0 0 0 0); white-space:nowrap; border:0;">${esc(text)}</span>`;

/**
 * Marks a non-heading element (an index label, a big number) that should use the theme's heading font. The
 * font itself is set in the designer stylesheet (render/design.ts), never inline, so a reader's own font wins.
 */
export const fontAttr = (ctx: Ctx) => (ctx.headingFont ? ' data-pd-font="1"' : '');

export type HeadingStyle = 'band' | 'underline' | 'plain';

export interface HeadingOpts {
  level?: 2 | 3 | 4;
  tone: Tone;
  size?: string;
  style?: HeadingStyle;
  margin?: string;
  align?: 'left' | 'center';
  lineHeight?: number;
  /** Override the heading colour (must already contrast with tone.bg). */
  color?: string;
  /** Defaults to 700 for h2 and 600 below (Paper's CSS reset makes headings inherit the body weight). */
  weight?: number;
}

/** A heading in the document's heading style. Empty text renders nothing (no empty headings). */
export function heading(ctx: Ctx, text: string, o: HeadingOpts): string {
  if (!text?.trim()) return '';
  const level = o.level ?? 2;
  const style = o.style ?? 'plain';
  const size = o.size ?? (level === 2 ? ctx.type.h2 : ctx.type.h3);
  const weight = o.weight ?? (level === 2 ? 700 : 600);
  const common = [`margin:${o.margin ?? '0 0 1rem'}`, `font-size:${size}`, `font-weight:${weight}`, `line-height:${o.lineHeight ?? 1.25}`, o.align === 'center' && 'text-align:center'];
  let decl: (string | false | undefined)[];
  if (style === 'band') {
    // A light band: tinted strip with a brand bar. One strong band per page is the title; these stay quiet.
    const t = ctx.tones.tint;
    decl = [
      ...common,
      'display:block',
      `padding:.5em .875em .5em .75em`,
      `background:${t.bg}`,
      `color:${t.heading}`,
      `border-radius:${px(ctx.r.sm)}`,
      `border-left:5px solid ${ctx.p.primary}`,
    ];
  } else if (style === 'underline') {
    decl = [
      ...common,
      'display:inline-block',
      'padding:0 0 .3em',
      `border-bottom:3px solid ${ctx.p.accent}`,
      `background:${o.tone.bg}`,
      `color:${o.color ?? o.tone.heading}`,
    ];
  } else {
    decl = [...common, `background:${o.tone.bg}`, `color:${o.color ?? o.tone.heading}`];
  }
  const s = decl.filter(Boolean).join('; ');
  return `<h${level} style="${esc(s)};">${esc(text)}</h${level}>`;
}

/** Section-level heading honouring doc.options.headingStyle (only on the page tone; panels use plain). */
export function sectionHeading(ctx: Ctx, text: string, tone: Tone, o: Partial<HeadingOpts> = {}): string {
  const onPage = tone.role === 'page';
  const style = o.style ?? (onPage ? ctx.headingStyle : 'plain');
  const hs = heading(ctx, text, { level: 2, tone, ...o, style });
  // inline-block underline headings need a line break after them so the next block starts below
  return style === 'underline' && hs ? `<div>${hs}</div>` : hs;
}

/** Restricted rich text → styled HTML for a given tone. Loose inline runs are wrapped in <p>. */
export function rich(ctx: Ctx, html: string, tone: Tone, o: { size?: string; lineHeight?: number; gap?: string } = {}): string {
  const clean = sanitizeRich(html ?? '');
  if (!clean) return '';
  const nodes = parseHtml(clean);
  // group loose inline runs into <p>
  const blocks: HNode[] = [];
  let run: HNode[] = [];
  const flush = () => {
    if (run.some(n => n.type === 'el' || (n.type === 'text' && n.raw.trim()))) blocks.push({ type: 'el', name: 'p', attrs: [], children: run });
    run = [];
  };
  for (const n of nodes) {
    if (n.type === 'el' && ['p', 'ul', 'ol'].includes(n.name)) {
      flush();
      blocks.push(n);
    } else run.push(n);
  }
  flush();
  const gap = o.gap ?? '.75em';
  const ser = (n: HNode, last: boolean): string => {
    if (n.type === 'text') return n.raw;
    if (n.type === 'comment') return '';
    const kids = n.children.map((c, i) => ser(c, i === n.children.length - 1)).join('');
    switch (n.name) {
      case 'p':
        return `<p style="margin:0 0 ${last ? '0' : gap};">${kids}</p>`;
      case 'ul':
      case 'ol':
        // list-style is explicit: Paper's CSS reset removes list markers
        return `<${n.name} style="margin:0 0 ${last ? '0' : gap}; padding:0 0 0 1.5em; list-style:${n.name === 'ul' ? 'disc' : 'decimal'} outside;">${kids}</${n.name}>`;
      case 'li':
        return `<li style="margin:0 0 .35em;">${kids}</li>`;
      case 'a': {
        const href = ctx.href(attr(n, 'href'));
        if (href === null) return kids;
        return `<a href="${esc(href)}" style="background:${tone.bg}; color:${tone.link}; text-decoration:underline;">${kids}</a>`;
      }
      case 'br':
        return '<br>';
      default:
        return `<${n.name}>${kids}</${n.name}>`;
    }
  };
  const inner = blocks.map((b, i) => ser(b, i === blocks.length - 1)).join('');
  const wrapStyle = [o.size && `font-size:${o.size}`, o.lineHeight && `line-height:${o.lineHeight}`].filter(Boolean).join('; ');
  return wrapStyle ? `<div style="${wrapStyle};">${inner}</div>` : inner;
}

/** Icon colours that contrast (≥3:1, WCAG non-text) on the given tone. */
export function iconColors(ctx: Ctx, tone: Tone, which: 'primary' | 'secondary' = 'primary', style: IconStyle = ctx.iconStyle): { fg: string; bg: string } {
  const base = which === 'primary' ? ctx.tones.primary : ctx.tones.secondary;
  const onBase = tone.role ? tone.role === base.role : base.bg === tone.bg;
  switch (style) {
    case 'solid-circle':
    case 'soft-square':
      return onBase ? { fg: base.bg, bg: base.fg } : { fg: base.fg, bg: base.bg };
    case 'duotone': {
      if (onBase) return { fg: base.bg, bg: base.fg };
      const bg = mix(base.bg, tone.bg, 0.84);
      return { fg: ensureContrast(base.bg, bg, 3), bg };
    }
    case 'outline':
    default:
      return { fg: ensureContrast(onBase ? base.fg : base.bg, tone.bg, 3), bg: tone.bg };
  }
}

/** Decorative icon image (alt=""): the label beside it carries the meaning. Icons never sit in the hero, so lazy. */
export function iconImg(ctx: Ctx, name: string, size: number, tone: Tone, o: { which?: 'primary' | 'secondary'; style?: string; iconStyle?: IconStyle } = {}): string {
  const st = o.iconStyle ?? ctx.iconStyle;
  const c = iconColors(ctx, tone, o.which ?? 'primary', st);
  return iconTag(ctx.icon(name, c.fg, c.bg, st), size, o.style);
}

/** An icon <img>. Margin is explicit: Paper's rich-content CSS gives small images inside li/span a side margin. */
export function iconTag(src: string, size: number, style = ''): string {
  const margin = /(^|[\s;])margin\s*:/.test(style) ? '' : ' margin:0;';
  return h('img', {
    src,
    alt: '',
    width: size,
    height: size,
    loading: 'lazy',
    style: `display:block; width:${size}px; max-width:100%; height:auto;${margin} ${style}`.trim().replace(/;?\s*$/, ';'),
  });
}

/** Button-style link. It navigates, so no role="button". */
export function button(ctx: Ctx, href: string | null, label: string, tone: Tone, o: { arrow?: boolean; size?: string } = {}): string {
  if (!href || !label.trim()) return '';
  return `<a href="${esc(href)}" data-pd-button="1" style="display:inline-block; padding:.75rem 1.375rem; border:2px solid ${tone.bg}; border-radius:${px(Math.max(ctx.r.md, 6))}; background:${tone.bg}; color:${tone.fg}; text-decoration:none; font-size:${o.size ?? '1rem'}; line-height:1.25;"><strong>${esc(label)}</strong>${o.arrow === false ? '' : ' <span aria-hidden="true">→</span>'}</a>`;
}

/** Best column count ≤ max for n items: avoids a lonely orphan row (6 → 3, 8 → 4, 5 → 3, 7 → 4). */
export function bestCols(n: number, max: number): number {
  if (n <= max) return Math.max(1, n);
  let best = max;
  let bestScore = -1;
  for (let c = max; c >= 2; c--) {
    const rem = n % c;
    const score = rem === 0 ? c * 10 : rem * 2 + c; // full rows win, then fuller last rows
    if (score > bestScore) {
      bestScore = score;
      best = c;
    }
  }
  return best;
}

/** Tested Canvas grid: N columns max, collapsing by container width, 1 column on phones. */
export const gridStyle = (minPx: number, n: number, gap: number) =>
  `display:grid; grid-template-columns:repeat(auto-fill, minmax(min(100%, max(${minPx}px, calc((100% - ${(n - 1) * gap}px) / ${n}))), 1fr)); gap:${gap}px`;

export function section(ctx: Ctx, s: Section, inner: string, style?: string): string {
  if (!inner.trim()) return '';
  return `<section data-pd-section="${esc(s.id)}" data-pd-type="${s.type}" style="${esc(style ?? `margin:0 0 ${ctx.gap}`)};">${inner}</section>`;
}

export function hr(color: string, o: { width?: number; margin?: string; maxWidth?: string } = {}): string {
  return `<hr style="height:0; margin:${o.margin ?? '0'}; border:0; border-top:${o.width ?? 1}px solid ${color};${o.maxWidth ? ` max-width:${o.maxWidth};` : ''}">`;
}

export const initials = (name: string) =>
  (name ?? '')
    .replace(/[[\]().,]/g, ' ')
    .split(/\s+/)
    .filter(w => /\p{L}/u.test(w))
    .slice(0, 2)
    .map(w => w[0].toLocaleUpperCase())
    .join('') || '•';

/** A pill (dates, "Due Fri"). */
export const pill = (ctx: Ctx, text: string, tone: Tone, extra = '') =>
  `<span style="display:inline-block; padding:.2em .75em; border-radius:999px; ${paint(tone)}; font-size:${ctx.type.small}; line-height:1.4; white-space:nowrap;${extra ? ` ${extra}` : ''}">${esc(text)}</span>`;

/** Eyebrow text (short label above a heading). Upper-cased in the text, not with text-transform, which Canvas strips on export. */
export const eyebrowText = (text: string, lang: string) => (text.length <= 24 ? text.toLocaleUpperCase(lang) : text);
