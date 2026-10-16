import { t, type StringKey } from './strings';
import type { LayoutId, Locale, PageDoc, PageKind, QuickLink, QuickLinkSlot, Section, SectionType, Theme } from './types';

/** Default Paper destinations per slot. `{course}` is replaced with the course id at render/insert time. */
export const SLOT_HREFS: Record<QuickLinkSlot, string> = {
  start: '/courses/{course}/pages/start-here',
  syllabus: '/courses/{course}/syllabus',
  modules: '/courses/{course}/modules',
  assignments: '/courses/{course}/assignments',
  discussions: '/courses/{course}/discussions',
  grades: '/courses/{course}/grades',
  help: '/courses/{course}/pages/getting-help',
  instructor: '/courses/{course}/pages/meet-your-instructor',
  calendar: '/courses/{course}/calendar',
  resources: '/courses/{course}/pages/resources',
};

/** Fallback icons when the theme doesn't choose one. All are Lucide names. */
export const SLOT_ICONS: Record<QuickLinkSlot, string> = {
  start: 'flag',
  syllabus: 'scroll-text',
  modules: 'layers',
  assignments: 'clipboard-list',
  discussions: 'messages-square',
  grades: 'chart-column',
  help: 'life-buoy',
  instructor: 'user-round',
  calendar: 'calendar-days',
  resources: 'library',
};

/** Each layout's natural heading treatment and density. */
export const LAYOUT_DEFAULTS: Record<LayoutId, PageDoc['options']> = {
  'classic-band': { density: 'comfortable', headingStyle: 'band' },
  'split-hero': { density: 'comfortable', headingStyle: 'underline' },
  'card-grid': { density: 'comfortable', headingStyle: 'plain' },
  sidebar: { density: 'comfortable', headingStyle: 'underline' },
  timeline: { density: 'comfortable', headingStyle: 'plain' },
  editorial: { density: 'comfortable', headingStyle: 'plain' },
  playful: { density: 'comfortable', headingStyle: 'band' },
};

const KIND_SLOTS: Record<PageKind, QuickLinkSlot[]> = {
  home: ['start', 'modules', 'assignments', 'discussions', 'grades', 'help'],
  'start-here': ['syllabus', 'modules', 'discussions', 'help'],
  overview: ['syllabus', 'modules', 'calendar', 'grades'],
  module: ['modules', 'discussions', 'assignments', 'help'],
  instructor: ['discussions', 'calendar', 'help'],
};

const uid = (): string => {
  const c = (globalThis as { crypto?: { randomUUID?: () => string } }).crypto;
  if (c?.randomUUID) return c.randomUUID();
  // RFC4122-ish fallback
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, ch => {
    const r = (Math.random() * 16) | 0;
    return (ch === 'x' ? r : (r & 0x3) | 0x8).toString(16);
  });
};
const sid = (type: SectionType) => `${type}-${uid().slice(0, 8)}`;

export function makeQuickLink(theme: Theme, slot: QuickLinkSlot, locale: Locale, young = false): QuickLink {
  return {
    id: `ql-${slot}-${uid().slice(0, 6)}`,
    slot,
    label: t(locale, `slot.${slot}` as StringKey, young),
    description: t(locale, `slot.${slot}.desc` as StringKey),
    href: SLOT_HREFS[slot],
    icon: theme.icons?.[slot] ?? SLOT_ICONS[slot],
  };
}

/** A new, warm, ready-to-edit page for this theme. */
export function createPage(theme: Theme, kind: PageKind, locale: Locale, layout?: LayoutId): PageDoc {
  const lay: LayoutId = layout ?? theme.defaultLayout ?? 'classic-band';
  const young = theme.audiences?.includes('early-elementary') && !theme.audiences.some(a => a === 'higher-ed' || a === 'professional');
  const s = (k: StringKey) => t(locale, k, !!young);
  const banner = theme.images?.find(i => i.kind === 'banner');
  const links = (heading: StringKey): Section => ({
    id: sid('quick-links'),
    type: 'quick-links',
    heading: s(heading),
    links: KIND_SLOTS[kind].map(slot => makeQuickLink(theme, slot, locale, !!young)),
  });
  const bannerSection = (): Section[] =>
    banner ? [{ id: sid('banner'), type: 'banner', imageId: banner.id, alt: banner.alt }] : [];
  const instructor = (heading: StringKey, bio: StringKey): Section => ({
    id: sid('instructor'),
    type: 'instructor',
    heading: s(heading),
    name: s('common.instructorName'),
    role: s('common.instructorRole'),
    email: 'instructor@example.edu',
    officeHours: s('common.officeHours'),
    bio: s(bio),
  });
  let sections: Section[];
  switch (kind) {
    case 'home':
      sections = [
        { id: sid('title'), type: 'title', text: s('common.courseTitle'), subtitle: s('common.subtitle') },
        ...bannerSection(),
        { id: sid('welcome'), type: 'welcome', heading: s('home.welcome.heading'), body: s('home.welcome.body') },
        {
          id: sid('this-week'),
          type: 'this-week',
          heading: s('home.week.heading'),
          title: s('home.week.title'),
          dates: s('home.week.dates'),
          items: [1, 2, 3].map(n => ({
            id: `tw-${n}-${uid().slice(0, 6)}`,
            label: s(`home.week.item${n}` as StringKey),
            due: s(`home.week.item${n}.due` as StringKey),
          })),
        },
        links('home.links.heading'),
        instructor('home.instructor.heading', 'common.instructorBio'),
      ];
      break;
    case 'start-here':
      sections = [
        { id: sid('title'), type: 'title', text: s('start.title'), subtitle: s('start.subtitle') },
        ...bannerSection(),
        { id: sid('welcome'), type: 'welcome', heading: s('start.welcome.heading'), body: s('start.welcome.body') },
        {
          id: sid('objectives'),
          type: 'objectives',
          heading: s('start.steps.heading'),
          items: [1, 2, 3, 4, 5].map(n => s(`start.steps.${n}` as StringKey)),
        },
        { id: sid('text'), type: 'text', heading: s('start.how.heading'), body: s('start.how.body') },
        links('start.links.heading'),
        { id: sid('callout'), type: 'callout', variant: 'important', heading: s('start.help.heading'), body: s('start.help.body') },
      ];
      break;
    case 'overview':
      sections = [
        { id: sid('title'), type: 'title', text: s('overview.title'), subtitle: s('overview.subtitle') },
        ...bannerSection(),
        { id: sid('welcome'), type: 'welcome', heading: s('overview.welcome.heading'), body: s('overview.welcome.body') },
        {
          id: sid('objectives'),
          type: 'objectives',
          heading: s('overview.objectives.heading'),
          items: [1, 2, 3, 4].map(n => s(`overview.objectives.${n}` as StringKey)),
        },
        {
          id: sid('schedule'),
          type: 'schedule',
          heading: s('overview.schedule.heading'),
          items: [1, 2, 3, 4].map(n => ({
            id: `sch-${n}-${uid().slice(0, 6)}`,
            title: s(`overview.schedule.${n}` as StringKey),
            dates: s(`overview.schedule.${n}.dates` as StringKey),
            summary: s(`overview.schedule.${n}.summary` as StringKey),
          })),
        },
        { id: sid('callout'), type: 'callout', variant: 'note', heading: s('overview.grading.heading'), body: s('overview.grading.body') },
        links('overview.links.heading'),
      ];
      break;
    case 'module':
      sections = [
        { id: sid('title'), type: 'title', text: s('module.title'), subtitle: s('module.subtitle') },
        ...bannerSection(),
        { id: sid('welcome'), type: 'welcome', heading: s('module.welcome.heading'), body: s('module.welcome.body') },
        {
          id: sid('objectives'),
          type: 'objectives',
          heading: s('module.objectives.heading'),
          items: [1, 2, 3].map(n => s(`module.objectives.${n}` as StringKey)),
        },
        {
          id: sid('this-week'),
          type: 'this-week',
          heading: s('module.week.heading'),
          title: s('module.week.title'),
          dates: s('module.week.dates'),
          items: [1, 2, 3, 4].map(n => ({
            id: `tw-${n}-${uid().slice(0, 6)}`,
            label: s(`module.week.item${n}` as StringKey),
            due: n >= 3 ? s(`module.week.item${n}.due` as StringKey) : undefined,
          })),
        },
        { id: sid('callout'), type: 'callout', variant: 'tip', heading: s('module.tip.heading'), body: s('module.tip.body') },
        links('module.links.heading'),
      ];
      break;
    case 'instructor':
      sections = [
        { id: sid('title'), type: 'title', text: s('instructor.title'), subtitle: s('instructor.subtitle') },
        ...bannerSection(),
        instructor('instructor.card.heading', 'instructor.bio'),
        { id: sid('text'), type: 'text', heading: s('instructor.approach.heading'), body: s('instructor.approach.body') },
        { id: sid('callout'), type: 'callout', variant: 'note', heading: s('instructor.reach.heading'), body: s('instructor.reach.body') },
        links('instructor.links.heading'),
      ];
      break;
  }
  return {
    version: 1,
    id: uid(),
    themeId: theme.id,
    paletteId: theme.palettes[0]?.id ?? 'default',
    layout: lay,
    kind,
    locale,
    sections,
    options: { ...LAYOUT_DEFAULTS[lay] },
  };
}
