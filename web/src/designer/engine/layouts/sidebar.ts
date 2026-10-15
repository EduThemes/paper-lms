import { esc } from '../html';
import type { Ctx } from '../render/context';
import { heading, paint, px, section } from '../render/parts';
import * as R from '../render/sections';
import type { Section } from '../types';
import { renderList, takeHero, type LayoutModule, type Variants } from './shared';

const V: Variants = { quickLinks: 'rows', quickLinksMaxCols: 2, objectives: 'checks', schedule: 'rows', thisWeek: 'compact', dividerShort: false };
const RAIL = new Set<Section['type']>(['quick-links', 'instructor', 'this-week']);

/**
 * Main column + a side rail of essentials (this week, quick links, instructor). The rail comes first in the
 * DOM and the row is reversed, so on desktop it sits on the right and on phones it lands right under the
 * banner — the things students need most, first.
 */
export const sidebar: LayoutModule = {
  meta: {
    id: 'sidebar',
    name: 'Sidebar',
    description: 'A reading column with a side rail for this week, quick links and your contact card. On phones the rail comes first.',
  },
  type: { title: '2.125rem', h2: '1.375rem', h3: '1.125rem', lead: '1.125rem', body: '1rem', small: '.875rem' },
  render(ctx: Ctx) {
    const hero = takeHero(ctx.doc.sections);
    const page = ctx.tones.page;
    const rail = ctx.tones.surface;
    let header = '';
    if (hero.title || hero.banner) {
      const t = hero.title
        ? `<div data-rmt-section="${esc(hero.title.id)}" data-rmt-type="title" style="margin:0 0 1.25rem; padding:0 0 1rem; border-bottom:1px solid ${page.border};">${R.title(ctx, hero.title, page, { subtitleFirst: false })}</div>`
        : '';
      // Without theme art the banner renders nothing; skip the wrapper so no empty <div> is left behind.
      const bannerHtml = hero.banner ? R.bannerImg(ctx, hero.banner) : '';
      const img = bannerHtml ? `<div data-rmt-section="${esc(hero.banner!.id)}" data-rmt-type="banner">${bannerHtml}</div>` : '';
      header = `<header style="margin:0 0 ${ctx.compact ? '1.5rem' : '2rem'};">${t}${img}</header>`;
    }
    // Balance: when the reading column would be nearly empty, quick links stay in it (as rows).
    const mainCount = hero.rest.filter(s => !RAIL.has(s.type) && s.type !== 'divider').length;
    const inRail = (s: Section) => RAIL.has(s.type) && !(s.type === 'quick-links' && mainCount < 2);
    const railSecs = hero.rest.filter(inRail);
    const mainSecs = hero.rest.filter(s => !inRail(s));
    if (!railSecs.length) return header + renderList(ctx, mainSecs, page, V);
    const railHtml = railSecs
      .map(s => {
        if (s.type === 'this-week') return section(ctx, s, R.thisWeek(ctx, s, 'compact'), 'margin:0');
        if (s.type === 'instructor')
          return section(ctx, s, R.instructor(ctx, s, rail, { compact: true, panel: rail, headingInside: true }), 'margin:0');
        if (s.type === 'quick-links')
          return section(
            ctx,
            s,
            `<div style="${paint(rail)}; padding:1rem 1.25rem; border-radius:${px(ctx.r.lg)}; border:1px solid ${rail.border};">${heading(ctx, s.heading, { level: 2, tone: rail, size: '1.125rem', margin: '0 0 .5rem' })}${R.quickLinks(ctx, { ...s, heading: '' }, rail, 'list')}</div>`,
            'margin:0',
          );
        return '';
      })
      .join('');
    const main = renderList(ctx, mainSecs, page, V);
    return (
      header +
      `<div style="display:flex; flex-direction:row-reverse; flex-wrap:wrap; gap:${ctx.compact ? '1.5rem' : '2.5rem'}; align-items:flex-start;">` +
      `<aside aria-label="${esc(ctx.s['ui.essentialsLabel'])}" style="flex:1 1 260px; min-width:0; display:flex; flex-direction:column; gap:1.25rem;">${railHtml}</aside>` +
      `<div style="flex:3 1 400px; min-width:0;">${main}</div>` +
      `</div>`
    );
  },
};
