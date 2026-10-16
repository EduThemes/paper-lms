/**
 * Section renderers shared by the layouts. Each takes the container tone and a variant so a layout can
 * restyle a section without re-implementing it. Inline styles only (Paper's rich-content allowlist; what Canvas
 * strips on export degrades gracefully), bg+color paired, links never wrap blocks, headings start at h2.
 */
import { ensureContrast, mix, readable } from '../color';
import { esc, h } from '../html';
import type { QuickLink, Section } from '../types';
import { safeImageSrc, type Ctx, type Tone } from './context';
import {
  LIST_RESET,
  bestCols,
  eyebrowText,
  fontAttr,
  gridStyle,
  heading,
  hr,
  iconImg,
  iconTag,
  initials,
  paint,
  pill,
  px,
  rich,
  sectionHeading,
  vh,
} from './parts';

type S<T extends Section['type']> = Extract<Section, { type: T }>;

/* ---------------------------------------------------------------- title & banner */

export function title(ctx: Ctx, s: S<'title'>, tone: Tone, o: { size?: string; align?: 'left' | 'center'; subtitleFirst?: boolean } = {}): string {
  const t = heading(ctx, s.text, { level: 2, tone, size: o.size ?? ctx.type.title, margin: '0', lineHeight: 1.15, align: o.align });
  const sub = s.subtitle?.trim()
    ? `<p style="margin:${o.subtitleFirst ? '0 0 .5rem' : '.5rem 0 0'}; font-size:${ctx.type.lead}; line-height:1.4; background:${tone.bg}; color:${tone.muted};${o.align === 'center' ? ' text-align:center;' : ''}">${esc(s.subtitle)}</p>`
    : '';
  return o.subtitleFirst ? sub + t : t + sub;
}

/** The hero banner: never cropped, never lazy (it is above the fold). Theme art gets its @2x twin. */
export function bannerImg(ctx: Ctx, s: S<'banner'>, o: { radius?: string; style?: string } = {}): string {
  const img = ctx.image(s);
  if (!img) return '';
  return h('img', {
    src: img.src,
    srcset: img.srcset,
    'data-pd-art': img.srcset ? 'banner' : undefined,
    alt: (s.alt ?? '').trim(),
    width: img.width,
    height: img.height,
    style: `display:block; width:100%; height:auto; border-radius:${o.radius ?? px(ctx.r.lg)};${o.style ? ` ${o.style}` : ''}`,
  });
}

/* ---------------------------------------------------------------- welcome & text */

export function welcome(ctx: Ctx, s: S<'welcome'>, tone: Tone, o: { lead?: boolean; headingSize?: string; headingStyle?: 'band' | 'underline' | 'plain' } = {}): string {
  return (
    sectionHeading(ctx, s.heading, tone, { size: o.headingSize, style: o.headingStyle }) +
    rich(ctx, s.body, tone, { size: o.lead ? ctx.type.lead : undefined, lineHeight: o.lead ? 1.6 : undefined })
  );
}

export function text(ctx: Ctx, s: S<'text'>, tone: Tone, o: { headingSize?: string } = {}): string {
  return (s.heading ? sectionHeading(ctx, s.heading, tone, { size: o.headingSize }) : '') + rich(ctx, s.body, tone);
}

export function divider(ctx: Ctx, o: { short?: boolean } = {}): string {
  return o.short
    ? hr(ctx.p.accent, { width: 4, margin: `0 auto`, maxWidth: '5rem' })
    : hr(ctx.tones.page.border, { width: 1, margin: '0' });
}

/* ---------------------------------------------------------------- quick links */

export type QuickLinkVariant = 'tiles' | 'cards' | 'list' | 'rows' | 'index' | 'chips' | 'playful';

function linkOpen(ctx: Ctx, l: QuickLink, style: string, variant: QuickLinkVariant): [string, string] {
  const href = ctx.href(l.href);
  // A link that can't be made safe is shown as a plain tile (the checker reports it).
  // data-pd-tile hooks the hover / focus styles in the designer stylesheet.
  return href === null ? [`<span style="${esc(style)}">`, '</span>'] : [`<a href="${esc(href)}" data-pd-tile="${variant}" style="${esc(style)}">`, '</a>'];
}

export function quickLinks(ctx: Ctx, s: S<'quick-links'>, tone: Tone, variant: QuickLinkVariant, o: { maxCols?: number; headingSize?: string; headingAlign?: 'left' | 'center' } = {}): string {
  const links = (s.links ?? []).filter(l => l.label?.trim());
  const head = sectionHeading(ctx, s.heading, tone, { size: o.headingSize, align: o.headingAlign });
  if (!links.length) return head;
  const g = ctx.gridGap;
  const label = esc(ctx.s['ui.quickLinksLabel']);
  const items = (inner: (l: QuickLink, i: number) => string, listStyle: string) =>
    `<nav aria-label="${label}"><ul role="list" style="${listStyle};">${links.map((l, i) => `<li style="margin:0; display:flex; min-width:0;">${inner(l, i)}</li>`).join('')}</ul></nav>`;
  const desc = (l: QuickLink, t: Tone, size = ctx.type.small) =>
    l.description?.trim() ? `<span style="display:block; margin:.25rem 0 0; font-size:${size}; line-height:1.4; ${paint({ ...t, fg: t.muted })};">${esc(l.description)}</span>` : '';

  switch (variant) {
    case 'tiles': {
      const t = ctx.tones.page;
      const n = bestCols(links.length, o.maxCols ?? 4);
      return (
        head +
        items(l => {
          const [a, z] = linkOpen(ctx, l, `display:block; flex:1 1 auto; min-width:0; padding:1.5rem 1rem 1.375rem; border:1px solid ${t.border}; border-radius:${px(ctx.r.lg)}; ${paint(t)}; text-align:center; text-decoration:none;`, 'tiles');
          return `${a}${iconImg(ctx, l.icon, 64, t, { style: 'margin:0 auto .875rem;' })}<span style="display:block; font-size:1.125rem; line-height:1.3; background:${t.bg}; color:${t.heading};"><strong>${esc(l.label)}</strong></span>${desc(l, t)}${z}`;
        }, `${LIST_RESET}; ${gridStyle(150, n, g)}`)
      );
    }
    case 'cards': {
      const t = ctx.tones.page;
      const n = bestCols(links.length, o.maxCols ?? 3);
      return (
        head +
        items(l => {
          const [a, z] = linkOpen(ctx, l, `display:block; flex:1 1 auto; min-width:0; padding:1.25rem 1.25rem 1.125rem; border:1px solid ${t.border}; border-radius:${px(ctx.r.lg)}; ${paint(t)}; text-decoration:none;`, 'cards');
          return `${a}${iconImg(ctx, l.icon, 48, t, { style: 'margin:0 0 1rem;', iconStyle: ctx.iconStyle === 'solid-circle' ? 'soft-square' : ctx.iconStyle })}<span style="display:block; font-size:1.125rem; line-height:1.3; background:${t.bg}; color:${t.heading};"><strong>${esc(l.label)}</strong> <span aria-hidden="true">→</span></span>${desc(l, t)}${z}`;
        }, `${LIST_RESET}; ${gridStyle(150, n, g)}`)
      );
    }
    case 'list': {
      const t = tone;
      return (
        head +
        items((l, i) => {
          const [a, z] = linkOpen(ctx, l, `display:flex; flex:1 1 auto; min-width:0; align-items:center; gap:.75rem; min-height:48px; padding:.5rem 0; ${i ? `border-top:1px solid ${t.border}; ` : ''}${paint(t)}; text-decoration:none;`, 'list');
          return `${a}${iconImg(ctx, l.icon, 36, t, { style: 'flex:0 0 auto;' })}<span style="display:block; flex:1 1 auto; font-size:1rem; line-height:1.3; background:${t.bg}; color:${t.heading};"><strong>${esc(l.label)}</strong></span><span aria-hidden="true" style="flex:0 0 auto; font-size:1.25rem; background:${t.bg}; color:${t.muted};">›</span>${z}`;
        }, LIST_RESET)
      );
    }
    case 'rows': {
      const t = ctx.tones.page;
      const n = bestCols(links.length, o.maxCols ?? 2);
      return (
        head +
        items(l => {
          const [a, z] = linkOpen(ctx, l, `display:flex; align-items:center; gap:1rem; flex:1 1 auto; min-width:0; padding:.875rem 1.125rem .875rem .875rem; border:1px solid ${t.border}; border-radius:${px(ctx.r.lg)}; ${paint(t)}; text-decoration:none;`, 'rows');
          return `${a}${iconImg(ctx, l.icon, 48, t, { style: 'flex:0 0 auto;' })}<span style="display:block; flex:1 1 auto; min-width:0;"><span style="display:block; font-size:1.0625rem; line-height:1.3; background:${t.bg}; color:${t.heading};"><strong>${esc(l.label)}</strong></span>${desc(l, t)}</span><span aria-hidden="true" style="flex:0 0 auto; font-size:1.25rem; background:${t.bg}; color:${t.heading};">→</span>${z}`;
        }, `${LIST_RESET}; ${gridStyle(260, n, g)}`)
      );
    }
    case 'index': {
      const t = ctx.tones.page;
      const n = bestCols(links.length, o.maxCols ?? 2);
      return (
        head +
        items((l, i) => {
          const [a, z] = linkOpen(ctx, l, `display:flex; align-items:baseline; gap:1rem; flex:1 1 auto; min-width:0; padding:1rem 0 1.125rem; border-top:2px solid ${t.fg}; ${paint(t)}; text-decoration:none;`, 'index');
          return `${a}<span aria-hidden="true" style="flex:0 0 2rem; font-size:${ctx.type.small}; background:${t.bg}; color:${t.muted};">${String(i + 1).padStart(2, '0')}</span><span style="display:block; flex:1 1 auto; min-width:0;"><span style="display:block; font-size:1.25rem; line-height:1.3; background:${t.bg}; color:${t.heading};"${fontAttr(ctx)}>${esc(l.label)}</span>${desc(l, t, '.9375rem')}</span><span aria-hidden="true" style="flex:0 0 auto; font-size:1.25rem; background:${t.bg}; color:${t.heading};">→</span>${z}`;
        }, `${LIST_RESET}; display:grid; grid-template-columns:repeat(auto-fill, minmax(min(100%, max(260px, calc((100% - ${(n - 1) * 32}px) / ${n}))), 1fr)); column-gap:32px; row-gap:0`)
      );
    }
    case 'chips': {
      const t = ctx.tones.tint;
      return (
        head +
        items(l => {
          const [a, z] = linkOpen(ctx, l, `display:inline-flex; align-items:center; gap:.625rem; min-height:44px; padding:.375rem 1.125rem .375rem .375rem; border:1px solid ${t.border}; border-radius:999px; ${paint(t)}; text-decoration:none;`, 'chips');
          return `${a}${iconImg(ctx, l.icon, 36, t, { style: 'flex:0 0 auto;' })}<span style="font-size:1rem; line-height:1.3; background:${t.bg}; color:${t.heading};"><strong>${esc(l.label)}</strong></span>${l.description ? vh(` — ${l.description}`) : ''}${z}`;
        }, `${LIST_RESET}; display:flex; flex-wrap:wrap; gap:.625rem`)
      );
    }
    case 'playful': {
      const colours = [ctx.p.primary, ctx.p.secondary, ctx.p.accent];
      const n = bestCols(links.length, o.maxCols ?? 3);
      return (
        head +
        items((l, i) => {
          const c = colours[i % colours.length];
          const bg = mix(c, ctx.p.background, 0.88);
          const t = { ...ctx.tones.page, role: undefined, bg, fg: readable(bg, [ctx.p.text]), heading: readable(bg, [ctx.p.primary, ctx.p.text]) };
          const iconBg = ensureContrast(c, bg, 3);
          const iconFg = readable(iconBg, ['#ffffff', ctx.p.text], 3);
          const [a, z] = linkOpen(ctx, l, `display:block; flex:1 1 auto; min-width:0; min-height:64px; padding:1.5rem 1rem 1.25rem; border:3px solid ${iconBg}; border-radius:${px(ctx.r.xl)}; ${paint(t)}; text-align:center; text-decoration:none;`, 'playful');
          const img = iconTag(ctx.icon(l.icon, iconFg, iconBg, 'solid-circle'), 96, 'margin:0 auto .75rem;');
          return `${a}${img}<span style="display:block; font-size:1.375rem; line-height:1.2; background:${t.bg}; color:${t.heading};"><strong>${esc(l.label)}</strong></span>${l.description ? vh(` — ${l.description}`) : ''}${z}`;
        }, `${LIST_RESET}; ${gridStyle(140, n, 20)}`)
      );
    }
  }
}

/* ---------------------------------------------------------------- instructor */

export function instructor(ctx: Ctx, s: S<'instructor'>, tone: Tone, o: { compact?: boolean; panel?: Tone; headingInside?: boolean; photo?: number } = {}): string {
  const t = o.panel ?? tone;
  const size = o.photo ?? (o.compact ? 56 : 96);
  // Same source rules as the banner's custom image: https / Paper-relative only (no data:, javascript: …).
  const photoSrc = s.photoSrc ? safeImageSrc(s.photoSrc) : null;
  const photo = photoSrc
    ? // square crop where supported (Canvas drops aspect-ratio/object-fit and shows the photo uncropped)
      h('img', { src: photoSrc, alt: s.photoAlt ?? '', width: size, height: size, loading: 'lazy', style: `display:block; width:${size}px; max-width:100%; height:auto; aspect-ratio:1 / 1; object-fit:cover; border-radius:${px(ctx.r.lg)};` })
    : `<span aria-hidden="true"${fontAttr(ctx)} style="display:inline-block; width:${size}px; line-height:${size}px; border-radius:999px; text-align:center; font-size:${Math.round((size / 16) * 0.38 * 100) / 100}rem; ${paint(ctx.tones.primary)};"><strong>${esc(initials(s.name))}</strong></span>`;
  const contact: string[] = [];
  if (s.email?.trim())
    contact.push(
      `<li style="display:flex; align-items:center; gap:.5rem; margin:0 0 .375rem;">${iconImg(ctx, 'mail', 24, t, { style: 'flex:0 0 auto;', iconStyle: 'outline' })}<span>${vh(`${ctx.s['ui.email']}: `)}<a href="mailto:${esc(s.email.trim())}" style="background:${t.bg}; color:${t.link}; text-decoration:underline;">${esc(s.email.trim())}</a></span></li>`,
    );
  if (s.officeHours?.trim())
    contact.push(
      `<li style="display:flex; align-items:center; gap:.5rem; margin:0;">${iconImg(ctx, 'clock', 24, t, { style: 'flex:0 0 auto;', iconStyle: 'outline' })}<span>${vh(`${ctx.s['ui.officeHours']}: `)}${esc(s.officeHours)}</span></li>`,
    );
  const nameH = heading(ctx, s.name, { level: 3, tone: t, size: o.compact ? '1.125rem' : '1.375rem', margin: '0', color: t.heading });
  const role = s.role?.trim() ? `<p style="margin:.25rem 0 0; font-size:${ctx.type.small}; background:${t.bg}; color:${t.muted};">${esc(s.role)}</p>` : '';
  const body = rich(ctx, s.bio, t, { size: o.compact ? '.9375rem' : undefined });
  const contactList = contact.length ? `<ul role="list" style="${LIST_RESET}; margin:1rem 0 0; font-size:${ctx.type.small};">${contact.join('')}</ul>` : '';
  const head = sectionHeading(ctx, s.heading, o.headingInside ? t : tone, o.headingInside ? { size: '1.125rem', style: 'plain' } : {});
  const inner = o.compact
    ? `<div style="display:flex; gap:.875rem; align-items:center;"><div style="flex:0 0 auto;">${photo}</div><div style="flex:1 1 auto; min-width:0;">${nameH}${role}</div></div>` +
      `${body ? `<div style="margin:.875rem 0 0;">${body}</div>` : ''}${contactList}`
    : `<div style="display:flex; flex-wrap:wrap; gap:1.5rem; align-items:flex-start;">` +
      `<div style="flex:0 0 auto;">${photo}</div>` +
      `<div style="flex:1 1 280px; min-width:0;">${nameH}${role}${body ? `<div style="margin:.875rem 0 0;">${body}</div>` : ''}${contactList}</div>` +
      `</div>`;
  if (o.headingInside) return `${o.panel ? `<div style="${paint(t)}; padding:${ctx.pad}; border-radius:${px(ctx.r.lg)}; border:1px solid ${t.border};">` : '<div>'}${head}${inner}</div>`;
  return head + (o.panel ? `<div style="${paint(t)}; padding:${ctx.pad}; border-radius:${px(ctx.r.lg)}; border:1px solid ${t.border};">${inner}</div>` : inner);
}

/* ---------------------------------------------------------------- callout */

const CALLOUT_ICON = { note: 'info', tip: 'lightbulb', important: 'circle-alert' } as const;

export function callout(ctx: Ctx, s: S<'callout'>, o: { quiet?: boolean; big?: boolean } = {}): string {
  const variant = s.variant ?? 'note';
  const t = variant === 'tip' ? ctx.tones.tint2 : variant === 'important' ? ctx.tones.page : ctx.tones.tint;
  const edge = variant === 'tip' ? ctx.p.secondary : ctx.p.primary;
  const border = variant === 'important' ? `border:2px solid ${edge}; border-left:8px solid ${edge}` : o.quiet ? `border:0; border-left:4px solid ${edge}` : `border:1px solid ${mix(edge, t.bg, 0.6)}; border-left:6px solid ${edge}`;
  const iconSize = o.big ? 56 : 40;
  return (
    `<div style="display:flex; gap:1rem; align-items:flex-start; ${paint(t)}; ${border}; border-radius:${px(o.quiet ? 0 : ctx.r.lg)}; padding:${ctx.pad};">` +
    `${iconImg(ctx, CALLOUT_ICON[variant] ?? 'info', iconSize, t, { which: variant === 'tip' ? 'secondary' : 'primary', style: 'flex:0 0 auto;' })}` +
    `<div style="flex:1 1 220px; min-width:0;">${heading(ctx, s.heading, { level: 2, tone: t, size: o.big ? '1.375rem' : '1.1875rem', margin: '.25rem 0 .5rem', color: variant === 'tip' ? undefined : t.heading })}${rich(ctx, s.body, t, { size: o.big ? '1.125rem' : undefined })}</div>` +
    `</div>`
  );
}

/* ---------------------------------------------------------------- objectives */

export function objectives(ctx: Ctx, s: S<'objectives'>, tone: Tone, variant: 'checks' | 'numbered' | 'stars' | 'panel', o: { maxCols?: number; headingSize?: string } = {}): string {
  const items = (s.items ?? []).filter(x => x?.trim());
  const head = sectionHeading(ctx, s.heading, tone, { size: o.headingSize });
  if (!items.length) return head;
  if (variant === 'numbered') {
    return (
      head +
      `<ol role="list" style="${LIST_RESET}; ${gridStyle(260, bestCols(items.length, o.maxCols ?? 2), 32)}; row-gap:0;">` +
      items
        .map(
          (x, i) =>
            `<li style="display:flex; gap:1rem; align-items:baseline; margin:0; padding:.875rem 0; border-top:1px solid ${tone.border};"><span aria-hidden="true"${fontAttr(ctx)} style="flex:0 0 2.25rem; font-size:1.5rem; line-height:1; background:${tone.bg}; color:${tone.heading};">${i + 1}</span><span style="flex:1 1 auto; font-size:1.0625rem; line-height:1.5;">${esc(x)}</span></li>`,
        )
        .join('') +
      `</ol>`
    );
  }
  const stars = variant === 'stars';
  const t = variant === 'panel' ? ctx.tones.tint : tone;
  const size = stars ? 44 : 28;
  const list =
    `<ul role="list" style="${LIST_RESET}; ${gridStyle(stars ? 240 : 280, bestCols(items.length, o.maxCols ?? (stars ? 2 : 1)), stars ? 16 : 12)};">` +
    items
      .map(
        x =>
          `<li style="display:flex; gap:.875rem; align-items:${stars ? 'center' : 'flex-start'}; margin:0;">${iconImg(ctx, stars ? 'star' : 'check', size, t, { style: 'flex:0 0 auto;', which: stars ? 'secondary' : 'primary' })}<span style="flex:1 1 auto; padding:${stars ? '0' : '.125rem 0 0'}; font-size:${stars ? '1.25rem' : '1.0625rem'}; line-height:1.45;">${esc(x)}</span></li>`,
      )
      .join('') +
    `</ul>`;
  if (variant === 'panel')
    return `<div style="${paint(t)}; padding:${ctx.pad}; border-radius:${px(ctx.r.lg)};">${heading(ctx, s.heading, { level: 2, tone: t, size: o.headingSize ?? '1.25rem' })}${list}</div>`;
  return head + list;
}

/* ---------------------------------------------------------------- schedule */

export function schedule(ctx: Ctx, s: S<'schedule'>, tone: Tone, variant: 'rows' | 'cards' | 'timeline', o: { headingSize?: string } = {}): string {
  const items = (s.items ?? []).filter(x => x.title?.trim());
  const head = sectionHeading(ctx, s.heading, tone, { size: o.headingSize });
  if (!items.length) return head;
  const titleOf = (it: (typeof items)[number], t: Tone, size: string) => {
    const href = it.href ? ctx.href(it.href) : null;
    const inner = href ? `<a href="${esc(href)}" style="background:${t.bg}; color:${t.link}; text-decoration:underline;">${esc(it.title)}</a>` : esc(it.title);
    return `<h3 style="margin:0; font-size:${size}; font-weight:600; line-height:1.3; background:${t.bg}; color:${href ? t.link : t.heading};">${inner}</h3>`;
  };
  const summary = (it: (typeof items)[number], t: Tone) =>
    it.summary?.trim() ? `<p style="margin:.375rem 0 0; background:${t.bg}; color:${t.muted};">${esc(it.summary)}</p>` : '';
  if (variant === 'cards') {
    const t = ctx.tones.page;
    return (
      head +
      `<ol role="list" style="${LIST_RESET}; ${gridStyle(200, bestCols(items.length, 4), ctx.gridGap)};">` +
      items
        .map(
          (it, i) =>
            `<li style="margin:0; padding:1.25rem; border:1px solid ${t.border}; border-radius:${px(ctx.r.lg)}; ${paint(t)};"><p style="margin:0 0 .5rem;">${pill(ctx, it.dates?.trim() || `${i + 1}`, ctx.tones.tint)}</p>${titleOf(it, t, '1.125rem')}${summary(it, t)}</li>`,
        )
        .join('') +
      `</ol>`
    );
  }
  if (variant === 'timeline') {
    const t = tone;
    return (
      head +
      `<ol role="list" style="${LIST_RESET}; border-left:3px solid ${mix(ctx.p.primary, t.bg, 0.65)}; margin:0 0 0 .5rem;">` +
      items
        .map(
          (it, i) =>
            `<li style="position:relative; margin:0; padding:0 0 ${i === items.length - 1 ? '0' : '1.5rem'} 1.75rem;"><span aria-hidden="true" style="position:absolute; left:-11px; top:.2rem; display:block; width:13px; line-height:13px; font-size:.5rem; border:3px solid ${t.bg}; border-radius:999px; ${paint(ctx.tones.primary)};">&nbsp;</span>${it.dates?.trim() ? `<p style="margin:0 0 .25rem; font-size:${ctx.type.small}; background:${t.bg}; color:${t.muted};"><strong>${esc(it.dates)}</strong></p>` : ''}${titleOf(it, t, '1.1875rem')}${summary(it, t)}</li>`,
        )
        .join('') +
      `</ol>`
    );
  }
  const t = tone;
  return (
    head +
    `<ol role="list" style="${LIST_RESET};">` +
    items
      .map(
        (it, i) =>
          `<li style="display:flex; flex-wrap:wrap; gap:.5rem 1.5rem; margin:0; padding:1rem 0; border-top:1px solid ${t.border};${i === items.length - 1 ? ` border-bottom:1px solid ${t.border};` : ''}"><div style="flex:0 0 9rem; padding:.125rem 0 0;">${pill(ctx, it.dates?.trim() || `${i + 1}`, ctx.tones.tint)}</div><div style="flex:1 1 240px; min-width:0;">${titleOf(it, t, '1.125rem')}${summary(it, t)}</div></li>`,
      )
      .join('') +
    `</ol>`
  );
}

/* ---------------------------------------------------------------- this week (the standout) */

export function thisWeek(ctx: Ctx, s: S<'this-week'>, variant: 'standard' | 'compact' | 'playful'): string {
  const head = ctx.tones.tint;
  const body = ctx.tones.page;
  const compact = variant === 'compact';
  const playful = variant === 'playful';
  const items = (s.items ?? []).filter(x => x.label?.trim());
  const padX = compact ? '1rem' : '1.5rem';
  const dates = s.dates?.trim() ? `<p style="margin:.375rem 0 0; font-size:${ctx.type.small}; ${paint({ ...head, fg: head.muted })};">${esc(s.dates)}</p>` : '';
  const header =
    `<div style="${paint(head)}; padding:${compact ? '1rem' : playful ? '1.5rem 1.5rem 1.25rem' : '1.25rem 1.5rem'};">` +
    heading(ctx, eyebrowText(s.heading, ctx.doc.locale), { level: 2, tone: head, size: compact ? '.8125rem' : '.875rem', margin: '0 0 .375rem', color: head.heading, lineHeight: 1.3 }) +
    heading(ctx, s.title, { level: 3, tone: head, size: compact ? '1.25rem' : playful ? '1.75rem' : '1.5rem', margin: '0', color: head.fg, lineHeight: 1.2 }) +
    dates +
    `</div>`;
  const numSize = playful ? 48 : compact ? 28 : 32;
  const list = items.length
    ? `<ol role="list" style="${LIST_RESET}; ${paint(body)};">` +
      items
        .map((it, i) => {
          const href = it.href ? ctx.href(it.href) : null;
          const label = href
            ? `<a href="${esc(href)}" style="background:${body.bg}; color:${body.link}; text-decoration:underline;">${esc(it.label)}</a>`
            : esc(it.label);
          const due = it.due?.trim() ? pill(ctx, `${ctx.s['ui.due']} ${it.due.trim()}`, ctx.tones.tint, 'margin-left:auto;') : '';
          return (
            `<li style="display:flex; flex-wrap:wrap; align-items:center; gap:.5rem .875rem; margin:0; padding:${playful ? '1rem 1.5rem' : compact ? '.75rem 1rem' : '.875rem 1.5rem'}; ${i ? `border-top:1px solid ${body.border};` : ''}">` +
            `<span aria-hidden="true" style="flex:0 0 auto; display:inline-block; width:${numSize}px; line-height:${numSize}px; border-radius:999px; text-align:center; font-size:${playful ? '1.5rem' : '.9375rem'}; ${paint(ctx.tones.primary)};"><strong>${i + 1}</strong></span>` +
            `<span style="flex:1 1 ${compact ? '120px' : '200px'}; min-width:0; font-size:${playful ? '1.375rem' : compact ? '.9375rem' : '1.0625rem'}; line-height:1.4;">${label}</span>` +
            due +
            `</li>`
          );
        })
        .join('') +
      `</ol>`
    : '';
  return `<div style="${paint(body)}; border:1px solid ${mix(ctx.p.primary, body.bg, 0.55)}; border-top:5px solid ${ctx.p.primary}; border-radius:${px(playful ? ctx.r.xl : ctx.r.lg)}; overflow:hidden;">${header}${list}</div>`;
}
