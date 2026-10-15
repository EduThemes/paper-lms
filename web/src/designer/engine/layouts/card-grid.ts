import { esc } from '../html';
import type { Ctx } from '../render/context';
import { paint, px } from '../render/parts';
import * as R from '../render/sections';
import type { Section } from '../types';
import { sectionInner, takeHero, type LayoutModule, type Variants } from './shared';

const V: Variants = { quickLinks: 'cards', quickLinksMaxCols: 3, objectives: 'checks', schedule: 'cards', thisWeek: 'compact', instructorCompact: false };

const WIDE = new Set<Section['type']>(['title', 'banner', 'quick-links', 'schedule', 'divider']);
const SELF_FRAMED = new Set<Section['type']>(['this-week', 'callout']);

/** A modular bento of cards on a flex-wrap grid: wide rows for links and schedules, paired cards for the rest. */
export const cardGrid: LayoutModule = {
  meta: {
    id: 'card-grid',
    name: 'Card grid',
    description: 'Everything in tidy cards: a title card over the banner, welcome beside this week, and link cards with icons and one-line descriptions.',
  },
  type: { title: '2.125rem', h2: '1.25rem', h3: '1.125rem', lead: '1.0625rem', body: '1rem', small: '.875rem' },
  render(ctx: Ctx) {
    const hero = takeHero(ctx.doc.sections);
    const page = ctx.tones.page;
    let header = '';
    if (hero.title || hero.banner) {
      const img = hero.banner ? R.bannerImg(ctx, hero.banner) : '';
      const card = hero.title
        ? `<div data-rmt-section="${esc(hero.title.id)}" data-rmt-type="title" style="position:relative; ${img ? 'margin:max(-4.5rem, -7vw) auto 0 clamp(.75rem, 3vw, 2rem)' : 'margin:0'}; max-width:36rem; ${paint(page)}; padding:${ctx.compact ? '1rem 1.25rem' : '1.375rem 1.75rem 1.5rem'}; border:1px solid ${page.border}; border-top:6px solid ${ctx.p.primary}; border-radius:${px(ctx.r.lg)};">${R.title(ctx, hero.title, page)}</div>`
        : '';
      header = `<header style="margin:0 0 ${ctx.compact ? '1.5rem' : '2rem'};">${img ? `<div data-rmt-section="${esc(hero.banner!.id)}" data-rmt-type="banner">${img}</div>` : ''}${card}</header>`;
    }
    const gap = ctx.compact ? 16 : 24;
    const items = hero.rest
      .map(s => {
        const inner = sectionInner(ctx, s, page, V);
        if (!inner.trim()) return '';
        const wide = WIDE.has(s.type);
        const flex = wide ? 'flex:1 1 100%' : s.type === 'welcome' ? 'flex:2 1 400px' : 'flex:1 1 280px';
        const framed = !wide && !SELF_FRAMED.has(s.type);
        const frame = framed ? `; ${paint(page)}; padding:${ctx.pad}; border:1px solid ${page.border}; border-radius:${px(ctx.r.lg)}` : '';
        return `<section data-rmt-section="${esc(s.id)}" data-rmt-type="${s.type}" style="${flex}; min-width:0; margin:0${frame};">${inner}</section>`;
      })
      .join('');
    return header + `<div style="display:flex; flex-wrap:wrap; gap:${gap}px; align-items:stretch;">${items}</div>`;
  },
};
