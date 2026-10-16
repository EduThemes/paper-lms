import { describe, expect, it, vi } from 'vitest';
import { LAYOUTS, THEMES, checkAccessibility, renderDesign, renderOptions } from '../lib/engine';
import { defaultPicks, fillModulePage, linkSet, moduleDates, planMakeover, recordCreated } from '../lib/makeover';
import { normalizeModules } from '../lib/modules';
import { createMakeoverRunner, initialRun } from '../makeover/runner';
import { PAPER_MODULES } from './fixtures';

const modules = normalizeModules(PAPER_MODULES, { 77: '2026-09-08T12:00:00Z' });
const theme = THEMES[0];
const palette = theme.palettes.at(-1).id;
const input = (over = {}) => ({
  theme,
  paletteId: palette,
  layout: '',
  course: { name: 'Biology 101', code: 'BIO101' },
  teacher: { name: 'Ms. Example', email: 'teacher@example.test' },
  modules,
  picks: defaultPicks(modules),
  locale: 'en',
  openLabel: (m) => `Open ${m.name}`,
  ...over,
});
const section = (doc, type) => doc.sections.find((s) => s.type === type);

describe('planMakeover', () => {
  it('plans every page in dependency order, Home last, in one look', () => {
    const plan = planMakeover(input());
    expect(plan.map((p) => p.role)).toEqual(['start-here', 'instructor', 'module:5000', 'module:5001', 'overview', 'home']);
    for (const p of plan) {
      expect(p.doc.themeId).toBe(theme.id);
      expect(p.doc.paletteId).toBe(palette);
      expect(p.doc.layout).toBe(theme.defaultLayout);
    }
    expect(new Set(plan.map((p) => p.doc.id)).size).toBe(plan.length);
    expect(section(plan.at(-1).doc, 'title').text).toBe('Biology 101');
    expect(section(plan.find((p) => p.role === 'instructor').doc, 'instructor').name).toBe('Ms. Example');
  });

  it('cleans every planned doc: API names stay plain text, a bad palette is dropped', () => {
    const evilModules = normalizeModules([{ ...PAPER_MODULES[0], name: '<img src=x onerror=alert(1)>' }], {});
    const plan = planMakeover(
      input({
        modules: evilModules,
        picks: defaultPicks(evilModules),
        palette: { id: 'scheme-1', name: 'Mine', primary: 'red;background:url(javascript:1)' },
      }),
    );
    for (const p of plan) {
      expect(p.doc).not.toHaveProperty('palette');
      const { html } = renderDesign(p.doc, renderOptions('42'));
      expect(html).not.toMatch(/<img[^>]*onerror/i);
    }
    expect(section(plan.find((p) => p.module).doc, 'title').text).toBe('<img src=x onerror=alert(1)>');
  });

  it('honours picks and the layout override, and defaults to published modules', () => {
    const plan = planMakeover(input({ layout: 'timeline', picks: { pages: ['home'], modules: ['5001'] } }));
    expect(plan.map((p) => p.role)).toEqual(['module:5001', 'home']);
    expect(plan.every((p) => p.doc.layout === 'timeline')).toBe(true);
    expect(defaultPicks(modules)).toEqual({ pages: ['home', 'start-here', 'overview', 'instructor'], modules: ['5000', '5001'] });
  });

  it('lists the real modules in the Course overview schedule with dates', () => {
    const schedule = section(planMakeover(input()).find((p) => p.role === 'overview').doc, 'schedule');
    expect(schedule.items.map((i) => [i.title, i.href])).toEqual([
      ['Week 1: Cells', '/courses/{course}/modules#module-5000'],
      ['Week 2: Energy', '/courses/{course}/modules#module-5001'],
    ]);
    expect(schedule.items[0].dates).toMatch(/Sep 1 – Sep 8/);
  });

  it('fills module overviews from the module', () => {
    const week1 = planMakeover(input()).find((p) => p.role === 'module:5000').doc;
    expect(section(week1, 'title').text).toBe('Week 1: Cells');
    expect(section(week1, 'this-week').items.map((i) => i.label)).toEqual(['Read chapter 1', 'Quiz 1']);
    expect(section(week1, 'quick-links').links.find((l) => l.slot === 'modules').href).toBe('/courses/{course}/modules#module-5000');
    const base = structuredClone(week1);
    fillModulePage(week1, modules[1], 'en', 'Open');
    expect(week1).toEqual(base);
  });

  it('moduleDates spans unlock and due dates', () => {
    expect(moduleDates({ unlockAt: null }, [{ dueAt: '2026-10-02T12:00:00Z' }, { dueAt: '2026-09-30T12:00:00Z' }], 'en')).toBe('Sep 30 – Oct 2');
    expect(moduleDates({ unlockAt: null }, [], 'en')).toBeUndefined();
  });
});

describe('linkSet', () => {
  const plan = planMakeover(input());
  it('points links at the slugs Paper returned', () => {
    let created = recordCreated({}, { role: 'start-here' }, 'start-here-2');
    created = recordCreated(created, { role: 'instructor' }, 'meet-your-instructor');
    created = recordCreated(created, plan.find((p) => p.role === 'module:5000'), 'week-1-cells');
    const home = linkSet(plan.at(-1).doc, created);
    const links = section(home, 'quick-links').links;
    expect(links.find((l) => l.slot === 'start').href).toBe('/courses/{course}/pages/start-here-2');
    expect(links.find((l) => l.slot === 'help').href).toBe('/courses/{course}/pages/start-here-2');
    const rows = section(linkSet(plan.find((p) => p.role === 'overview').doc, created), 'schedule').items;
    expect(rows[0].href).toBe('/courses/{course}/pages/week-1-cells');
    expect(rows[1].href).toBe('/courses/{course}/modules#module-5001');
    expect(renderDesign(home, renderOptions('42')).html).toContain('href="/courses/42/pages/start-here-2"');
    expect(linkSet(home, {})).toBe(home);
  });
});

describe('accessibility of a planned set', () => {
  it('has no accessibility errors across themes and layouts', () => {
    const failures = [];
    const check = (inp) => {
      for (const p of planMakeover(inp)) {
        for (const issue of checkAccessibility(p.doc, { courseId: '42' })) if (issue.severity === 'error') failures.push(`${inp.theme.id}/${p.doc.layout}/${p.role}: ${issue.message}`);
      }
    };
    for (const th of THEMES.slice(0, 20)) check(input({ theme: th, paletteId: th.palettes[0].id }));
    for (const l of LAYOUTS) check(input({ layout: l.id }));
    expect(failures).toEqual([]);
  });
});

/** A fake Paper: clashing titles get "-2" slugs, like the server. */
function fakeApi({ failTitle } = {}) {
  const slugs = new Set(['start-here']);
  let id = 100;
  const calls = [];
  let failing = failTitle;
  const api = {
    createPage: vi.fn(async (courseId, wp) => {
      calls.push(['createPage', wp]);
      if (failing && wp.title === failing) throw new Error('Server said no');
      let slug = wp.title.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
      if (slugs.has(slug)) slug = `${slug}-2`;
      slugs.add(slug);
      return { page_id: ++id, url: slug, title: wp.title, published: wp.published, front_page: wp.front_page };
    }),
    addToModule: vi.fn(async (courseId, moduleId, item) => calls.push(['addToModule', moduleId, item])),
    setHome: vi.fn(async () => calls.push(['setHome'])),
    setLook: vi.fn(async (courseId, look) => calls.push(['setLook', look])),
  };
  return { api, calls, stopFailing: () => (failing = null) };
}

function harness(api) {
  let state = initialRun;
  const get = () => state;
  const set = (p) => {
    state = typeof p === 'function' ? p(state) : { ...state, ...p };
  };
  return { runner: createMakeoverRunner({ api, courseId: '42', customCssEnabled: false, get, set }), get };
}

describe('makeover runner', () => {
  const options = { publish: false, addToModules: true, makeHome: true, applyLook: true };

  it('creates pages in order with real slugs, adds overviews to modules, sets home and look', async () => {
    const { api, calls } = fakeApi();
    const { runner, get } = harness(api);
    const plan = planMakeover(input());
    await runner.start(plan, options);
    const created = calls.filter((c) => c[0] === 'createPage').map((c) => c[1]);
    expect(created.map((w) => w.title)).toEqual(plan.map((p) => section(p.doc, 'title').text));
    // Start Here clashed with an existing page: Home links to the slug Paper gave it.
    const home = created.at(-1);
    expect(home.front_page).toBe(true);
    expect(home.published).toBe(true);
    expect(home.body).toContain('/courses/42/pages/start-here-2');
    expect(home.design.id).toBe(plan.at(-1).doc.id);
    expect(created.slice(0, -1).every((w) => w.front_page === false && w.published === false)).toBe(true);
    expect(calls.filter((c) => c[0] === 'addToModule').map((c) => c[1])).toEqual(['5000', '5001']);
    expect(api.setHome).toHaveBeenCalledWith('42');
    expect(api.setLook.mock.calls[0][1]).toMatchObject({ version: 1, theme_id: theme.id });
    expect(get().stage).toBe('done');
    expect(get().finish).toMatchObject({ home: 'done', look: 'done' });
  });

  it('stops at a failed page and carries on after a retry, never creating a page twice', async () => {
    const plan = planMakeover(input({ picks: { pages: ['start-here', 'home'], modules: [] } }));
    const failTitle = section(plan[0].doc, 'title').text;
    const { api, stopFailing } = fakeApi({ failTitle });
    const { runner, get } = harness(api);
    await runner.start(plan, options);
    expect(get().stopped).toBe(true);
    expect(get().progress['start-here'].state).toBe('failed');
    expect(get().progress.home.state).toBe('waiting');
    expect(api.setHome).not.toHaveBeenCalled();
    stopFailing();
    await runner.retry('start-here');
    expect(get().progress.home.state).toBe('done');
    expect(api.createPage).toHaveBeenCalledTimes(3);
    expect(get().stage).toBe('done');
  });

  it('can retry a failed module item and the finishing steps', async () => {
    const { api } = fakeApi();
    api.addToModule.mockRejectedValueOnce(new Error('nope'));
    api.setLook.mockRejectedValueOnce(new Error('too big'));
    const { runner, get } = harness(api);
    await runner.start(planMakeover(input({ picks: { pages: ['home'], modules: ['5000'] } })), options);
    expect(get().progress['module:5000'].moduleItem).toBe('failed');
    expect(get().finish.look).toBe('failed');
    await runner.retry('module:5000');
    expect(get().progress['module:5000'].moduleItem).toBe('done');
    await runner.retryFinish();
    expect(get().finish.look).toBe('done');
    expect(api.setHome).toHaveBeenCalledTimes(1);
  });
});
