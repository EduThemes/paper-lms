import { esc, h } from '../html';
import { themeImageUrl } from '../icons';
import type { Ctx } from '../render/context';
import { heading, paint, px, rich } from '../render/parts';
import * as R from '../render/sections';
import { sectionInner, takeHero, type LayoutModule, type Variants } from './shared';

const V: Variants = { quickLinks: 'playful', objectives: 'stars', schedule: 'cards', thisWeek: 'playful', instructorPanel: true, calloutBig: true, welcomeLead: true, dividerShort: true };

/**
 * For K-2 and 3-5: chunky rounded shapes, picture-first tiles with 96px icons, ≥64px tap targets,
 * bigger body text and a speech-bubble welcome. Corners are always round here.
 */
export const playful: LayoutModule = {
  meta: {
    id: 'playful',
    name: 'Playful',
    description: 'Chunky, rounded and picture-first for young learners: giant icon tiles, big tap targets, a speech-bubble welcome.',
  },
  forceRound: true,
  rootStyle: 'font-size:1.125rem',
  type: { title: '2.5rem', h2: '1.625rem', h3: '1.375rem', lead: '1.25rem', body: '1.125rem', small: '1rem' },
  render(ctx: Ctx) {
    const hero = takeHero(ctx.doc.sections);
    let header = '';
    if (hero.title || hero.banner) {
      const t = ctx.tones.primary;
      const bubble = hero.title
        ? `<div data-pd-section="${esc(hero.title.id)}" data-pd-type="title" style="${paint(t)}; padding:${ctx.compact ? '1.25rem 1.5rem' : '1.75rem 2rem'}; border-radius:${px(ctx.r.xl)}; text-align:center;">${R.title(ctx, hero.title, t, { align: 'center' })}</div>`
        : '';
      // Without theme art the banner renders nothing; skip the wrapper so no empty <div> is left behind.
      const bannerHtml = hero.banner ? R.bannerImg(ctx, hero.banner, { radius: px(ctx.r.xl) }) : '';
      const img = bannerHtml ? `<div data-pd-section="${esc(hero.banner!.id)}" data-pd-type="banner" style="margin:${bubble ? '1rem' : '0'} 0 0;">${bannerHtml}</div>` : '';
      header = `<header style="margin:0 0 ${ctx.gap};">${bubble}${img}</header>`;
    }
    const page = ctx.tones.page;
    const bubbleTone = ctx.tones.surface;
    const items = hero.rest
      .map((s, i) => {
        let inner: string;
        if (s.type === 'welcome') {
          // picture-first: the theme's welcome spot art sits beside the speech bubble (stacks on phones)
          const spot = ctx.theme.images?.find(im => im.id === 'spot-welcome') ?? ctx.theme.images?.find(im => im.kind === 'spot');
          const art = spot
            ? `<div style="flex:0 0 160px; max-width:100%;">${h('img', { src: themeImageUrl(ctx.opts.assetBase, ctx.theme.id, spot), srcset: ctx.srcset(spot.file), 'data-pd-art': 'spot', alt: spot.alt ?? '', width: 160, height: Math.round((160 * spot.height) / spot.width), loading: 'lazy', style: `display:block; width:160px; max-width:100%; height:auto; border-radius:${px(ctx.r.xl)};` })}</div>`
            : '';
          const text = `${heading(ctx, s.heading, { level: 2, tone: bubbleTone, size: ctx.type.h2, margin: '0 0 .75rem' })}${rich(ctx, s.body, bubbleTone, { size: ctx.type.lead, lineHeight: 1.55 })}`;
          inner = `<div style="${paint(bubbleTone)}; padding:${ctx.compact ? '1.25rem 1.5rem' : '1.5rem 2rem 1.5rem 1.5rem'}; border-radius:${px(ctx.r.xl)} ${px(ctx.r.xl)} ${px(ctx.r.xl)} ${px(6)};">${art ? `<div style="display:flex; flex-wrap:wrap; gap:1.5rem; align-items:center;">${art}<div style="flex:1 1 260px; min-width:0;">${text}</div></div>` : text}</div>`;
        } else inner = sectionInner(ctx, s, page, V);
        if (!inner.trim()) return '';
        const last = i === hero.rest.length - 1;
        return `<section data-pd-section="${esc(s.id)}" data-pd-type="${s.type}" style="margin:0 0 ${last ? '0' : ctx.gap};">${inner}</section>`;
      })
      .join('');
    return header + items;
  },
};
