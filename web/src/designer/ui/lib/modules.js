/**
 * Paper's modules API → the shapes the designer uses, and the module-driven helpers ported from RMT
 * Studio: quick links from modules, "This week" from a module's items. Pure, so they are easy to test.
 *
 * Links use the `{course}` token (filled in on render), so a copied course keeps working links.
 */
import { newId } from './ids';

/** `/courses/{course}/modules#module-<id>`: the Modules page scrolls to the module. */
export const moduleHref = (id) => `/courses/{course}/modules#module-${id}`;

/** The course-relative address of a module item, or null (sub-headers, unknown types). */
export function itemHref(item) {
  const id = item.content_id;
  switch (item.type) {
    case 'Page':
      return item.page_url ? `/courses/{course}/pages/${encodeURIComponent(item.page_url)}` : null;
    case 'Assignment':
      return id ? `/courses/{course}/assignments/${id}` : null;
    case 'Quiz':
      return id ? `/courses/{course}/quizzes/${id}` : null;
    case 'Discussion':
      return id ? `/courses/{course}/discussions/${id}` : null;
    case 'File':
      return '/courses/{course}/files';
    case 'ExternalUrl':
      return /^https:\/\//i.test(item.url ?? '') ? item.url : null;
    default:
      return null;
  }
}

/**
 * Paper modules (GET /courses/:id/modules?include[]=items) → { id, name, href, unlockAt, published, items }.
 * `dueDates` maps assignment id → due_at, so checklist entries can show when work is due.
 */
export function normalizeModules(raw, dueDates = {}) {
  const list = Array.isArray(raw) ? raw : [];
  return list
    .filter((m) => m && m.workflow_state !== 'deleted')
    .sort((a, b) => (a.position ?? 0) - (b.position ?? 0))
    .map((m) => ({
      id: String(m.id),
      name: String(m.name ?? ''),
      href: moduleHref(m.id),
      position: m.position,
      unlockAt: m.unlock_at ?? null,
      published: typeof m.published === 'boolean' ? m.published : m.workflow_state ? m.workflow_state === 'active' : null,
      items: (Array.isArray(m.items) ? m.items : [])
        .filter((i) => i && i.workflow_state !== 'deleted')
        .sort((a, b) => (a.position ?? 0) - (b.position ?? 0))
        .map((i) => ({
          id: String(i.id),
          title: String(i.title ?? ''),
          type: i.type,
          htmlUrl: itemHref(i),
          dueAt: i.type === 'Assignment' && i.content_id ? dueDates[String(i.content_id)] ?? null : null,
          published: typeof i.published === 'boolean' ? i.published : null,
        })),
    }));
}

/** Published unless Paper says otherwise. */
export const isPublished = (m) => m.published !== false;

/** "Mon, Sep 8" in the page's locale; the raw string if it isn't a date. */
export function formatDue(iso, locale) {
  if (!iso) return undefined;
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso;
  const opts = { weekday: 'short', month: 'short', day: 'numeric' };
  try {
    return new Intl.DateTimeFormat(locale, opts).format(d);
  } catch {
    return new Intl.DateTimeFormat('en', opts).format(d);
  }
}

/**
 * A module's items → a "This week" section patch. Sub-headers are skipped, except a leading one, which
 * becomes the week's title. Also returns the ids of entries whose item is unpublished.
 */
export function itemsToWeek(module, items, locale, openLabel) {
  const first = items[0];
  const title = first?.type === 'SubHeader' && first.title.trim() ? first.title.trim() : module.name;
  const unpublished = [];
  const entries = items
    .filter((i) => i.type !== 'SubHeader')
    .map((i) => {
      const id = newId('i');
      if (i.published === false) unpublished.push(id);
      const due = formatDue(i.dueAt, locale);
      return { id, label: i.title, ...(i.htmlUrl ? { href: i.htmlUrl } : {}), ...(due ? { due } : {}) };
    });
  return {
    patch: {
      title,
      moduleId: module.id,
      items: entries.length ? entries : [{ id: newId('i'), label: openLabel, href: module.href }],
    },
    unpublished,
  };
}

/** Quick links to the given modules. */
export function moduleLinks(modules) {
  return modules.map((m) => ({ id: newId('l'), label: m.name, href: m.href, icon: 'layers', moduleId: m.id }));
}
