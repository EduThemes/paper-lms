import type { Section, SectionType } from '../types';
import type { Ctx, Tone, TypeScale } from '../render/context';
import { section } from '../render/parts';
import * as R from '../render/sections';

export type S<T extends SectionType> = Extract<Section, { type: T }>;

export interface LayoutModule {
  meta: { id: import('../types').LayoutId; name: string; description: string };
  type: TypeScale;
  forceRound?: boolean;
  /** Extra declarations on the root wrapper. */
  rootStyle?: string;
  render(ctx: Ctx): string;
}

export interface Hero {
  title?: S<'title'>;
  banner?: S<'banner'>;
  welcome?: S<'welcome'>;
  rest: Section[];
}

/** Pull the leading title / banner (/ welcome) out of the section list for a layout's hero. Order is kept for everything else. */
export function takeHero(sections: Section[], withWelcome = false): Hero {
  const hero: Hero = { rest: [] };
  let i = 0;
  for (; i < sections.length; i++) {
    const s = sections[i];
    if (s.type === 'title' && !hero.title) hero.title = s;
    else if (s.type === 'banner' && !hero.banner) hero.banner = s;
    else if (withWelcome && s.type === 'welcome' && !hero.welcome && (hero.title || hero.banner)) hero.welcome = s;
    else break;
  }
  hero.rest = sections.slice(i);
  return hero;
}

export interface Variants {
  quickLinks: R.QuickLinkVariant;
  quickLinksMaxCols?: number;
  objectives: 'checks' | 'numbered' | 'stars' | 'panel';
  schedule: 'rows' | 'cards' | 'timeline';
  thisWeek: 'standard' | 'compact' | 'playful';
  instructorPanel?: boolean;
  instructorCompact?: boolean;
  calloutQuiet?: boolean;
  calloutBig?: boolean;
  welcomeLead?: boolean;
  dividerShort?: boolean;
  titleSize?: string;
  bannerRadius?: string;
}

/** Inner HTML for one section (no <section> wrapper). */
export function sectionInner(ctx: Ctx, s: Section, tone: Tone, v: Variants): string {
  switch (s.type) {
    case 'title':
      return R.title(ctx, s, tone, { size: v.titleSize });
    case 'banner':
      return R.bannerImg(ctx, s, { radius: v.bannerRadius });
    case 'welcome':
      return R.welcome(ctx, s, tone, { lead: v.welcomeLead });
    case 'quick-links':
      return R.quickLinks(ctx, s, tone, v.quickLinks, { maxCols: v.quickLinksMaxCols });
    case 'instructor':
      return R.instructor(ctx, s, tone, { panel: v.instructorPanel ? ctx.tones.surface : undefined, compact: v.instructorCompact });
    case 'callout':
      return R.callout(ctx, s, { quiet: v.calloutQuiet, big: v.calloutBig });
    case 'objectives':
      return R.objectives(ctx, s, tone, v.objectives);
    case 'schedule':
      return R.schedule(ctx, s, tone, v.schedule);
    case 'this-week':
      return R.thisWeek(ctx, s, v.thisWeek);
    case 'text':
      return R.text(ctx, s, tone);
    case 'divider':
      return R.divider(ctx, { short: v.dividerShort });
    default:
      return '';
  }
}

/** Render sections in order, each in its own <section>. */
export function renderList(ctx: Ctx, list: Section[], tone: Tone, v: Variants, wrapStyle?: (s: Section, i: number) => string | undefined): string {
  const rendered = list.map(s => [s, sectionInner(ctx, s, tone, v)] as const).filter(([, html]) => html.trim());
  return rendered.map(([s, html], i) => section(ctx, s, html, wrapStyle?.(s, i) ?? (i === rendered.length - 1 ? 'margin:0' : undefined))).join('');
}

/** First quick link, preferring the "start" slot — used for hero call-to-action buttons. */
export function ctaLink(sections: Section[]): { href: string; label: string } | null {
  const links = sections.flatMap(s => (s.type === 'quick-links' ? s.links : []));
  const l = links.find(x => x.slot === 'start') ?? null;
  return l ? { href: l.href, label: l.label } : null;
}
