import { esc } from '../html';
import type { Ctx } from '../render/context';
import { button, eyebrowText, heading, paint, px, rich } from '../render/parts';
import * as R from '../render/sections';
import { ctaLink, renderList, takeHero, type LayoutModule, type Variants } from './shared';

const V: Variants = { quickLinks: 'rows', objectives: 'checks', schedule: 'rows', thisWeek: 'standard', instructorPanel: true, dividerShort: true };

/** Banner + a two-column welcome panel (title & call-to-action | welcome copy). Stacks on phones. */
export const splitHero: LayoutModule = {
  meta: {
    id: 'split-hero',
    name: 'Split hero',
    description: 'The banner sits on a two-column panel: course title and a Start button beside your welcome. Link rows below.',
  },
  type: { title: '2.375rem', h2: '1.5rem', h3: '1.25rem', lead: '1.125rem', body: '1rem', small: '.875rem' },
  render(ctx: Ctx) {
    const hero = takeHero(ctx.doc.sections, true);
    const panel = ctx.tones.surface;
    let header = '';
    if (hero.title || hero.banner || hero.welcome) {
      const img = hero.banner ? R.bannerImg(ctx, hero.banner, { radius: `${px(ctx.r.lg)} ${px(ctx.r.lg)} 0 0` }) : '';
      const cta = ctaLink(ctx.doc.sections);
      const ctaHtml = cta ? `<p style="margin:1.5rem 0 0;">${button(ctx, ctx.href(cta.href), cta.label, ctx.tones.primary)}</p>` : '';
      const sub = hero.title?.subtitle?.trim()
        ? `<p style="margin:0 0 .625rem; font-size:${ctx.type.small}; background:${panel.bg}; color:${panel.muted};"><strong>${esc(eyebrowText(hero.title.subtitle, ctx.doc.locale))}</strong></p>`
        : '';
      const left = hero.title
        ? `<div data-pd-section="${esc(hero.title.id)}" data-pd-type="title" style="flex:1 1 280px; min-width:0;">${sub}${heading(ctx, hero.title.text, { level: 2, tone: panel, size: ctx.type.title, margin: '0', lineHeight: 1.12 })}${ctaHtml}</div>`
        : '';
      const right = hero.welcome
        ? `<div data-pd-section="${esc(hero.welcome.id)}" data-pd-type="welcome" style="flex:1.3 1 300px; min-width:0; padding:${hero.title ? '0 0 0 1.75rem' : '0'}; border-left:${hero.title ? `3px solid ${ctx.p.accent}` : '0'};">${heading(ctx, hero.welcome.heading, { level: hero.title ? 3 : 2, tone: panel, size: ctx.type.h3, margin: '0 0 .625rem' })}${rich(ctx, hero.welcome.body, panel, { size: '1.0625rem' })}</div>`
        : '';
      const body = left || right ? `<div style="display:flex; flex-wrap:wrap; gap:2rem; align-items:flex-start; ${paint(panel)}; padding:${ctx.compact ? '1.5rem' : '2.25rem 2.25rem 2.5rem'}; border:1px solid ${panel.border}; border-top:0; border-radius:${img ? `0 0 ${px(ctx.r.lg)} ${px(ctx.r.lg)}` : px(ctx.r.lg)};">${left}${right}</div>` : '';
      const fixedImg = img && !body ? R.bannerImg(ctx, hero.banner!) : img;
      header = `<header style="margin:0 0 ${ctx.gap};">${fixedImg ? `<div data-pd-section="${esc(hero.banner!.id)}" data-pd-type="banner">${fixedImg}</div>` : ''}${body}</header>`;
    }
    return header + renderList(ctx, hero.rest, ctx.tones.page, V);
  },
};
