/**
 * Whole-course makeover (ported from RMT Studio's lib/makeover.ts): plans one page per course role (Home,
 * Start Here, Course overview, Meet your instructor, and one overview per module) in a single look, then
 * wires the links between them once Paper has told us each page's real slug. Pure, so it is easy to test.
 *
 * Creation order matters: Paper gives a clashing title a fresh slug ("start-here-2"), and no page is edited
 * after it's created, so a page is only created after every page it links to.
 */
import { createPage, sanitizeDoc, SLOT_HREFS } from './engine';
import { fillCourseDetails } from './course';
import { newId } from './ids';
import { isPublished, itemsToWeek } from './modules';
import { paperizeDoc } from './sections';

export const COURSE_PAGES = ['home', 'start-here', 'overview', 'instructor'];

export const moduleRole = (moduleId) => `module:${moduleId}`;

/** Default picks: every course page, and an overview for each published module. */
export function defaultPicks(modules) {
  return { pages: [...COURSE_PAGES], modules: modules.filter(isPublished).map((m) => m.id) };
}

const pageHref = (slug) => `/courses/{course}/pages/${slug}`;

function fmtDay(d, locale) {
  const opts = { month: 'short', day: 'numeric' };
  try {
    return new Intl.DateTimeFormat(locale, opts).format(d);
  } catch {
    return new Intl.DateTimeFormat('en', opts).format(d);
  }
}

/** "Sep 1 – Sep 8": from the module's unlock date and its items' due dates. Undefined when none are known. */
export function moduleDates(module, items, locale) {
  const times = [module.unlockAt, ...items.map((i) => i.dueAt)]
    .filter(Boolean)
    .map((d) => new Date(d).getTime())
    .filter((n) => !isNaN(n));
  if (!times.length) return undefined;
  const first = fmtDay(new Date(Math.min(...times)), locale);
  const last = fmtDay(new Date(Math.max(...times)), locale);
  return first === last ? first : `${first} – ${last}`;
}

/**
 * A Module overview for one real module: its name as the title, its dates as the subtitle, the checklist
 * from its items, and the "Modules" quick link pointing at this module. Objectives, welcome text and the tip
 * keep their [bracketed] placeholders for the teacher to fill in.
 */
export function fillModulePage(doc, module, locale, openLabel) {
  const items = module.items ?? [];
  const dates = moduleDates(module, items, locale);
  const week = itemsToWeek(module, items, locale, openLabel);
  const sections = doc.sections.map((s) => {
    switch (s.type) {
      case 'title':
        return { ...s, text: module.name, ...(dates ? { subtitle: dates } : {}) };
      case 'this-week':
        return { ...s, ...week.patch, dates };
      case 'quick-links':
        return { ...s, links: s.links.map((l) => (l.slot === 'modules' ? { ...l, href: module.href } : l)) };
      default:
        return s;
    }
  });
  return { ...doc, sections };
}

/** The Course overview's schedule: the real (published) modules, linked to each module. */
function scheduleFromModules(modules, locale) {
  return modules.filter(isPublished).map((m) => {
    const dates = moduleDates(m, m.items ?? [], locale);
    return { id: newId('sch'), title: m.name, href: m.href, ...(dates ? { dates } : {}) };
  });
}

/**
 * The pages to create, in dependency order: Start Here, Meet your instructor, each Module overview, Course
 * overview (lists the modules), then Home (links to Start Here) last.
 *
 * input: { theme, paletteId, palette?, layout ('' = theme default), course: { name, code }, teacher?,
 *          modules (normalized, in course order), picks: { pages, modules }, locale, openLabel(module) }
 */
export function planMakeover({ theme, paletteId, palette, layout, course, teacher = null, modules, picks, locale, openLabel = (m) => m.name }) {
  const make = (kind) => ({
    ...paperizeDoc(fillCourseDetails(createPage(theme, kind, locale, layout || undefined), course, teacher)),
    paletteId,
    ...(palette ? { palette } : {}),
  });
  const want = (p) => picks.pages.includes(p);
  const out = [];
  if (want('start-here')) out.push({ role: 'start-here', kind: 'start-here', doc: make('start-here') });
  if (want('instructor')) out.push({ role: 'instructor', kind: 'instructor', doc: make('instructor') });
  for (const m of modules) {
    if (!picks.modules.includes(m.id)) continue;
    out.push({ role: moduleRole(m.id), kind: 'module', module: m, doc: fillModulePage(make('module'), m, locale, openLabel(m)) });
  }
  if (want('overview')) {
    let doc = make('overview');
    const schedule = scheduleFromModules(modules, locale);
    if (schedule.length) doc = { ...doc, sections: doc.sections.map((s) => (s.type === 'schedule' ? { ...s, items: schedule } : s)) };
    out.push({ role: 'overview', kind: 'overview', doc });
  }
  if (want('home')) out.push({ role: 'home', kind: 'home', doc: make('home') });
  // Course, teacher and module names come from the API, the palette from a saved colour scheme.
  return out.map((p) => ({ ...p, doc: sanitizeDoc(p.doc) }));
}

/**
 * Points links at the pages this makeover created: Start Here and Meet your instructor quick links (Get help
 * goes to Start Here, which has a "Need help?" section), and Course overview schedule rows that link to a
 * module with its own overview page. Links to pages that weren't created are left alone. Same doc when
 * nothing changes.
 */
export function linkSet(doc, created) {
  const slotTarget = (slot, href) => {
    if ((slot === 'start' || href === SLOT_HREFS.start) && created['start-here']) return pageHref(created['start-here']);
    if ((slot === 'instructor' || href === SLOT_HREFS.instructor) && created.instructor) return pageHref(created.instructor);
    if ((slot === 'help' || href === SLOT_HREFS.help) && created['start-here']) return pageHref(created['start-here']);
    return undefined;
  };
  let changed = false;
  const sections = doc.sections.map((s) => {
    if (s.type === 'quick-links') {
      let touched = false;
      const links = s.links.map((l) => {
        const href = slotTarget(l.slot, l.href);
        if (!href || href === l.href) return l;
        touched = true;
        return { ...l, href };
      });
      if (!touched) return s;
      changed = true;
      return { ...s, links };
    }
    if (s.type === 'schedule') {
      let touched = false;
      const items = s.items.map((it) => {
        const slug = it.href ? created.modules?.[it.href] : undefined;
        if (!slug) return it;
        touched = true;
        return { ...it, href: pageHref(slug) };
      });
      if (!touched) return s;
      changed = true;
      return { ...s, items };
    }
    return s;
  });
  return changed ? { ...doc, sections } : doc;
}

/** Records a created page's slug under the role other pages link to it by. */
export function recordCreated(created, page, slug) {
  if (page.role === 'start-here') return { ...created, 'start-here': slug };
  if (page.role === 'instructor') return { ...created, instructor: slug };
  if (page.module) return { ...created, modules: { ...created.modules, [page.module.href]: slug } };
  return created;
}
