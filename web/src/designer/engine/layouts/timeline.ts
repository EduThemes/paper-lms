import { mix } from '../color';
import { esc } from '../html';
import type { Ctx } from '../render/context';
import { eyebrowText, iconImg, px } from '../render/parts';
import * as R from '../render/sections';
import type { Section } from '../types';
import { sectionInner, takeHero, type LayoutModule, type Variants } from './shared';

const V: Variants = { quickLinks: 'chips', objectives: 'checks', schedule: 'timeline', thisWeek: 'standard', dividerShort: true };

const MARKER: Partial<Record<Section['type'], string>> = {
  welcome: 'hand',
  'quick-links': 'compass',
  'this-week': 'calendar-check',
  objectives: 'target',
  schedule: 'calendar-range',
  instructor: 'user-round',
  text: 'file-text',
  title: 'bookmark',
  banner: 'image',
};
const CALLOUT_MARKER = { note: 'info', tip: 'lightbulb', important: 'circle-alert' } as const;

/** The whole page as a journey: every section is a stop on a vertical line, marked with its own icon. */
export const timeline: LayoutModule = {
  meta: {
    id: 'timeline',
    name: 'Timeline',
    description: 'The page reads as a path: each section is a stop on a vertical line with an icon marker. Great for modules and weekly flow.',
  },
  type: { title: '2.25rem', h2: '1.375rem', h3: '1.1875rem', lead: '1.125rem', body: '1rem', small: '.875rem' },
  render(ctx: Ctx) {
    const hero = takeHero(ctx.doc.sections);
    const page = ctx.tones.page;
    let header = '';
    if (hero.title || hero.banner) {
      const sub = hero.title?.subtitle?.trim()
        ? `<p style="margin:0 0 .5rem; font-size:${ctx.type.small}; background:${page.bg}; color:${page.heading};"><strong>${esc(eyebrowText(hero.title.subtitle, ctx.doc.locale))}</strong></p>`
        : '';
      const t = hero.title
        ? `<div data-rmt-section="${esc(hero.title.id)}" data-rmt-type="title" style="margin:0 0 1.5rem;">${sub}${R.title(ctx, { ...hero.title, subtitle: undefined }, page)}</div>`
        : '';
      // Without theme art the banner renders nothing; skip the wrapper so no empty <div> is left behind.
      const bannerHtml = hero.banner ? R.bannerImg(ctx, hero.banner) : '';
      const img = bannerHtml ? `<div data-rmt-section="${esc(hero.banner!.id)}" data-rmt-type="banner">${bannerHtml}</div>` : '';
      header = `<header style="margin:0 0 ${ctx.gap};">${t}${img}</header>`;
    }
    const line = mix(ctx.p.primary, page.bg, 0.7);
    const M = 36; // marker size
    const items = hero.rest
      .map((s, i) => {
        const inner = sectionInner(ctx, s, page, V);
        if (!inner.trim()) return '';
        const name = s.type === 'callout' ? CALLOUT_MARKER[s.variant] ?? 'info' : MARKER[s.type];
        const last = i === hero.rest.length - 1;
        const marker =
          name && s.type !== 'divider'
            ? iconImg(ctx, name, M, page, { iconStyle: 'solid-circle', style: `position:absolute; left:-${29.5 + M / 2}px; top:-2px;` })
            : '';
        return `<section data-rmt-section="${esc(s.id)}" data-rmt-type="${s.type}" style="position:relative; margin:0; padding:0 0 ${last ? '0' : ctx.compact ? '2rem' : '2.75rem'};">${marker}${inner}</section>`;
      })
      .join('');
    return header + `<div style="margin:0 0 0 18px; padding:0 0 .5rem 28px; border-left:3px solid ${line}; border-radius:0 0 0 ${px(2)};">${items}</div>`;
  },
};
