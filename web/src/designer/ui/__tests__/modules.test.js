import { describe, expect, it } from 'vitest';
import { formatDue, itemHref, itemsToWeek, moduleLinks, normalizeModules } from '../lib/modules';
import { PAPER_MODULES } from './fixtures';

describe('normalizeModules', () => {
  it('reads Paper modules and items, with course-token links and assignment due dates', () => {
    const mods = normalizeModules(PAPER_MODULES, { 77: '2026-09-08T12:00:00Z' });
    expect(mods.map((m) => [m.id, m.name, m.published])).toEqual([
      ['5000', 'Week 1: Cells', true],
      ['5001', 'Week 2: Energy', true],
      ['5002', 'Draft unit', false],
    ]);
    expect(mods[0].href).toBe('/courses/{course}/modules#module-5000');
    expect(mods[0].items.map((i) => [i.title, i.htmlUrl, i.dueAt])).toEqual([
      ['Read chapter 1', '/courses/{course}/pages/chapter-1', null],
      ['Quiz 1', '/courses/{course}/assignments/77', '2026-09-08T12:00:00Z'],
    ]);
    expect(normalizeModules(null)).toEqual([]);
  });

  it('builds item addresses per type', () => {
    expect(itemHref({ type: 'Quiz', content_id: 3 })).toBe('/courses/{course}/quizzes/3');
    expect(itemHref({ type: 'Discussion', content_id: 4 })).toBe('/courses/{course}/discussions/4');
    expect(itemHref({ type: 'ExternalUrl', url: 'https://example.test/x' })).toBe('https://example.test/x');
    expect(itemHref({ type: 'ExternalUrl', url: 'javascript:alert(1)' })).toBeNull();
    expect(itemHref({ type: 'SubHeader' })).toBeNull();
  });
});

describe('itemsToWeek', () => {
  const mod = { id: '12', name: 'Week 3', href: '/courses/{course}/modules#module-12' };
  const item = (o) => ({ id: '1', title: 'Item', type: 'Page', htmlUrl: '/courses/{course}/pages/item', dueAt: null, published: true, ...o });

  it('uses a leading SubHeader as the title and skips other SubHeaders', () => {
    const { patch } = itemsToWeek(
      mod,
      [
        item({ type: 'SubHeader', title: 'Week 3: Cell structure', htmlUrl: null }),
        item({ id: '2', title: 'Read chapter 3' }),
        item({ id: '3', type: 'SubHeader', title: 'Extras', htmlUrl: null }),
        item({ id: '4', title: 'Quiz 3', type: 'Assignment', htmlUrl: '/courses/{course}/assignments/4', dueAt: '2026-09-18T23:59:00Z' }),
      ],
      'en-US',
      'Open Week 3',
    );
    expect(patch.title).toBe('Week 3: Cell structure');
    expect(patch.moduleId).toBe('12');
    expect(patch.items.map((i) => i.label)).toEqual(['Read chapter 3', 'Quiz 3']);
    expect(patch.items[1].due).toMatch(/Sep/);
  });

  it('falls back to the module name, flags unpublished items, links an empty module', () => {
    const { patch, unpublished } = itemsToWeek(mod, [item({ title: 'Draft', published: false }), item({ id: '2', title: 'Live' })], 'en', 'Open');
    expect(patch.title).toBe('Week 3');
    expect(unpublished).toEqual([patch.items[0].id]);
    expect(itemsToWeek(mod, [], 'en', 'Open Week 3').patch.items).toEqual([expect.objectContaining({ label: 'Open Week 3', href: mod.href })]);
  });

  it('formats due dates in the locale', () => {
    const iso = '2026-09-18T12:00:00Z';
    expect(formatDue(iso, 'es')).toBe(new Intl.DateTimeFormat('es', { weekday: 'short', month: 'short', day: 'numeric' }).format(new Date(iso)));
    expect(formatDue('next Friday', 'en')).toBe('next Friday');
    expect(formatDue(null, 'en')).toBeUndefined();
  });

  it('makes quick links to modules', () => {
    const links = moduleLinks(normalizeModules(PAPER_MODULES).slice(0, 2));
    expect(links.map((l) => [l.label, l.href, l.moduleId])).toEqual([
      ['Week 1: Cells', '/courses/{course}/modules#module-5000', '5000'],
      ['Week 2: Energy', '/courses/{course}/modules#module-5001', '5001'],
    ]);
  });
});
