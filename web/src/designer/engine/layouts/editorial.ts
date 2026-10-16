import { esc } from '../html';
import type { Ctx } from '../render/context';
import { eyebrowText, heading, hr, px } from '../render/parts';
import * as R from '../render/sections';
import type { Section } from '../types';
import { sectionInner, takeHero, type LayoutModule, type Variants } from './shared';

const V: Variants = { quickLinks: 'index', objectives: 'numbered', schedule: 'rows', thisWeek: 'standard', calloutQuiet: true, welcomeLead: true, dividerShort: true };
const MEASURE = new Set<Section['type']>(['welcome', 'text', 'callout', 'instructor']);

/** Magazine feel: large display type, an eyebrow, hairline rules, generous whitespace, a typographic link index. */
export const editorial: LayoutModule = {
  meta: {
    id: 'editorial',
    name: 'Editorial',
    description: 'Large type, generous whitespace and hairline rules. Links become a numbered index. Calm and grown-up.',
  },
  type: { title: 'clamp(2.25rem, 1.5rem + 2.8vw, 3.25rem)', h2: '1.75rem', h3: '1.25rem', lead: '1.25rem', body: '1rem', small: '.875rem' },
  render(ctx: Ctx) {
    const hero = takeHero(ctx.doc.sections);
    const page = ctx.tones.page;
    let header = '';
    if (hero.title || hero.banner) {
      const sub = hero.title?.subtitle?.trim()
        ? `<p style="margin:0 0 1rem; font-size:${ctx.type.small}; line-height:1.4; background:${page.bg}; color:${page.heading};"><strong>${esc(eyebrowText(hero.title.subtitle, ctx.doc.locale))}</strong></p>`
        : '';
      const t = hero.title
        ? `<div data-pd-section="${esc(hero.title.id)}" data-pd-type="title" style="margin:0 0 ${ctx.compact ? '1.5rem' : '2.25rem'};">${sub}${heading(ctx, hero.title.text, { level: 2, tone: page, size: ctx.type.title, margin: '0', lineHeight: 1.08, color: page.fg })}${hr(page.fg, { width: 2, margin: `${ctx.compact ? '1.25rem' : '1.75rem'} 0 0` })}</div>`
        : '';
      // Without theme art the banner renders nothing; skip the wrapper so no empty <div> is left behind.
      const bannerHtml = hero.banner ? R.bannerImg(ctx, hero.banner, { radius: px(ctx.r.sm) }) : '';
      const img = bannerHtml ? `<div data-pd-section="${esc(hero.banner!.id)}" data-pd-type="banner">${bannerHtml}</div>` : '';
      header = `<header style="margin:0 0 ${ctx.compact ? '2.5rem' : '4rem'};">${t}${img}</header>`;
    }
    const gap = ctx.compact ? '2.5rem' : '4rem';
    const items = hero.rest
      .map((s, i) => {
        const inner = sectionInner(ctx, s, page, V);
        if (!inner.trim()) return '';
        const last = i === hero.rest.length - 1;
        const measure = MEASURE.has(s.type) ? ' max-width:46rem;' : '';
        return `<section data-pd-section="${esc(s.id)}" data-pd-type="${s.type}" style="margin:0 0 ${last ? '0' : gap};${measure}">${inner}</section>`;
      })
      .join('');
    return header + items;
  },
};
