/** Section helpers: friendly names, starter content for new sections, deep copies, Paper link targets. */
import { SECTION_TYPES, SLOT_HREFS } from './engine';
import { newId } from './ids';
import { richToText } from './richtext';

/**
 * Where each quick-link slot goes in Paper. `{course}` is filled in on render. Pages that a makeover can
 * create (Start Here, Meet your instructor) keep the engine's slugs, so the makeover can re-point them.
 */
export const PAPER_SLOT_HREFS = {
  start: SLOT_HREFS.start,
  syllabus: '/courses/{course}/syllabus',
  modules: '/courses/{course}/modules',
  assignments: '/courses/{course}/assignments',
  discussions: '/courses/{course}/discussions',
  grades: '/courses/{course}/grades',
  calendar: '/calendar',
  help: SLOT_HREFS.help,
  instructor: SLOT_HREFS.instructor,
  resources: SLOT_HREFS.resources,
};

export const SLOT_ORDER = ['start', 'syllabus', 'modules', 'assignments', 'discussions', 'grades', 'calendar', 'help', 'instructor', 'resources'];

export const PAPER_LINKS = SLOT_ORDER.map((slot) => ({
  slot,
  href: PAPER_SLOT_HREFS[slot],
  icon: { start: 'flag', syllabus: 'file-text', modules: 'layers', assignments: 'clipboard-list', discussions: 'messages-square', grades: 'chart-column', calendar: 'calendar-days', help: 'life-buoy', instructor: 'user-round', resources: 'library' }[slot],
}));

/** Canvas-only addresses in starter content → Paper's. */
const CANVAS_TO_PAPER = [
  [/^\/courses\/([^/]+)\/assignments\/syllabus(?=$|[?#])/, '/courses/$1/syllabus'],
  [/^\/courses\/([^/]+)\/discussion_topics(?=$|[/?#])/, '/courses/$1/discussions'],
  [/^\/calendar\?include_contexts=course_[^&#]*$/, '/calendar'],
];

export function toPaperHref(href) {
  let out = String(href ?? '');
  for (const [re, to] of CANVAS_TO_PAPER) out = out.replace(re, to);
  return out;
}

/** Rewrites Canvas-only link targets in a doc's quick links and lists to Paper's. Same doc when nothing changes. */
export function paperizeDoc(doc) {
  let changed = false;
  const fix = (href) => {
    if (!href) return href;
    const next = toPaperHref(href);
    if (next !== href) changed = true;
    return next;
  };
  const sections = doc.sections.map((s) => {
    if (s.type === 'quick-links') return { ...s, links: s.links.map((l) => ({ ...l, href: fix(l.href) })) };
    if (s.type === 'schedule' || s.type === 'this-week') return { ...s, items: s.items.map((i) => (i.href ? { ...i, href: fix(i.href) } : i)) };
    return s;
  });
  return changed ? { ...doc, sections } : doc;
}

/** Lucide icon names offered first in the icon picker. The theme's own slot icons are added on top. */
export const ICON_CHOICES = [
  'flag', 'file-text', 'layers', 'clipboard-list', 'messages-square', 'chart-column', 'calendar-days',
  'life-buoy', 'user-round', 'library', 'book-open', 'graduation-cap', 'pencil', 'video', 'headphones',
  'lightbulb', 'star', 'puzzle', 'flask-conical', 'calculator', 'palette', 'music', 'globe', 'heart',
  'rocket', 'mail', 'circle-help', 'link', 'folder', 'house', 'trophy', 'users',
];

export const sectionTypeInfo = (type) => SECTION_TYPES.find((s) => s.type === type);

/** A short name for a section in the outline: its type plus its own heading when it has one. */
export function sectionName(s, typeLabel = (type) => sectionTypeInfo(type)?.label ?? type) {
  const label = typeLabel(s.type);
  const own =
    s.type === 'title'
      ? s.text
      : s.type === 'banner' || s.type === 'divider'
        ? ''
        : s.type === 'text'
          ? s.heading || richToText(s.body).slice(0, 40)
          : s.heading;
  return own?.trim() ? `${label}: ${own.trim()}` : label;
}

/**
 * A new section with starter text. `t` is the designer's translate function (keys under `newSection.*`),
 * so new sections start in the teacher's language.
 */
export function newSection(type, kind = 'home', t = (k) => k) {
  const id = newId('s');
  switch (type) {
    case 'title':
      return { id, type, text: t('newSection.title'), subtitle: '' };
    case 'banner':
      return { id, type, imageId: 'banner', alt: '' };
    case 'welcome':
      return { id, type, heading: t('newSection.welcomeHeading'), body: `<p>${t('newSection.welcomeBody')}</p>` };
    case 'quick-links':
      return {
        id,
        type,
        heading: t('newSection.linksHeading'),
        links: ['modules', 'assignments', 'discussions', 'grades'].map((slot) => {
          const l = PAPER_LINKS.find((x) => x.slot === slot);
          return { id: newId('l'), label: t(`slot.${slot}`), href: l.href, icon: l.icon, slot };
        }),
      };
    case 'instructor':
      return { id, type, heading: t('newSection.instructorHeading'), name: t('newSection.instructorName'), role: '', email: '', officeHours: '', bio: `<p>${t('newSection.instructorBio')}</p>` };
    case 'callout':
      return { id, type, variant: 'tip', heading: t('newSection.calloutHeading'), body: `<p>${t('newSection.calloutBody')}</p>` };
    case 'objectives':
      return {
        id,
        type,
        heading: kind === 'module' ? t('newSection.objectivesModule') : t('newSection.objectivesCourse'),
        items: [t('newSection.objective1'), t('newSection.objective2'), t('newSection.objective3')],
      };
    case 'schedule':
      return {
        id,
        type,
        heading: t('newSection.scheduleHeading'),
        items: [1, 2, 3].map((n) => ({ id: newId('i'), title: t('newSection.weekN', { n }), dates: '', summary: '' })),
      };
    case 'this-week':
      return { id, type, heading: t('newSection.thisWeekHeading'), title: t('newSection.weekN', { n: 1 }), dates: '', items: [{ id: newId('i'), label: t('newSection.thisWeekItem') }] };
    case 'text':
      return { id, type, heading: t('newSection.textHeading'), body: `<p>${t('newSection.textBody')}</p>` };
    case 'divider':
    default:
      return { id, type: 'divider' };
  }
}

/** Deep copy with fresh ids for the section and its nested items. */
export function cloneSection(s) {
  const copy = JSON.parse(JSON.stringify(s));
  copy.id = newId('s');
  if (copy.type === 'quick-links') copy.links = copy.links.map((l) => ({ ...l, id: newId('l') }));
  if (copy.type === 'schedule' || copy.type === 'this-week') copy.items = copy.items.map((i) => ({ ...i, id: newId('i') }));
  return copy;
}
