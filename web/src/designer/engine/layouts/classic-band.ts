import { esc } from '../html';
import type { Ctx } from '../render/context';
import { paint, px } from '../render/parts';
import * as R from '../render/sections';
import { renderList, takeHero, type LayoutModule, type Variants } from './shared';

const V: Variants = { quickLinks: 'tiles', objectives: 'panel', schedule: 'rows', thisWeek: 'standard', instructorPanel: true, dividerShort: true };

/** The familiar RMT homepage — title band, banner, welcome, icon tiles — with a real type scale and rhythm. */
export const classicBand: LayoutModule = {
  meta: {
    id: 'classic-band',
    name: 'Classic band',
    description: 'A bold title band joined to the banner, then welcome text and icon tiles. The familiar homepage, refined.',
  },
  type: { title: '2.25rem', h2: '1.375rem', h3: '1.1875rem', lead: '1.125rem', body: '1rem', small: '.875rem' },
  render(ctx: Ctx) {
    const hero = takeHero(ctx.doc.sections);
    let header = '';
    if (hero.title || hero.banner) {
      const band = ctx.tones.primary;
      const img = hero.banner ? R.bannerImg(ctx, hero.banner, { radius: hero.title ? `0 0 ${px(ctx.r.lg)} ${px(ctx.r.lg)}` : px(ctx.r.lg) }) : '';
      const bandHtml = hero.title
        ? `<div data-rmt-section="${esc(hero.title.id)}" data-rmt-type="title" style="${paint(band)}; padding:${ctx.compact ? '1.25rem 1.5rem' : '1.75rem 2rem'}; border-radius:${img ? `${px(ctx.r.lg)} ${px(ctx.r.lg)} 0 0` : px(ctx.r.lg)}; border-bottom:6px solid ${ctx.p.accent};">${R.title(ctx, hero.title, band)}</div>`
        : '';
      const imgHtml = img ? `<div data-rmt-section="${esc(hero.banner!.id)}" data-rmt-type="banner">${img}</div>` : '';
      header = `<header style="margin:0 0 ${ctx.gap};">${bandHtml}${imgHtml}</header>`;
    }
    return header + renderList(ctx, hero.rest, ctx.tones.page, V);
  },
};
