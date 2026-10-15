/**
 * Runs a whole-course makeover against Paper's API (ported from RMT Studio's state/makeover.ts, without
 * zustand). Pages are created one at a time, in the order planMakeover gives, each with its links pointing at
 * the pages already made. A failed page stops the run (later pages may link to it); retry carries on from
 * there. Nothing is ever overwritten: every page is a POST, and Paper gives a clashing title a fresh slug.
 *
 * After the pages: the new Home becomes the front page (it is created with front_page: true, and the course
 * default_view is set to 'wiki'), and the course look is applied.
 *
 * createMakeoverRunner({ api, courseId, customCssEnabled, get, set }) where get() returns the run state and
 * set(patch | fn) updates it. `api` = { createPage(courseId, wikiPage), addToModule(courseId, moduleId,
 * { pageId, title }), setHome(courseId), setLook(courseId, look) }.
 */
import { courseLook } from '../lib/engine';
import { docTitle } from '../lib/course';
import { linkSet, recordCreated } from '../lib/makeover';
import { renderForSave, wikiPagePayload } from '../lib/savePage';

export const initialRun = {
  stage: 'idle', // 'idle' | 'creating' | 'finishing' | 'done'
  plan: [],
  progress: {}, // role → { role, title, state, error?, pageUrl?, pageId?, moduleItem, moduleError? }
  created: {},
  options: { publish: false, addToModules: true, makeHome: true, applyLook: true },
  running: false,
  stopped: false,
  finish: { home: 'none', homeError: null, look: 'none', lookError: null },
};

const errText = (e) => e?.message || String(e ?? 'Error');

export function createMakeoverRunner({ api, courseId, customCssEnabled, get, set, kindTitle = (k) => k }) {
  const patchPage = (role, p) => set((s) => ({ ...s, progress: { ...s.progress, [role]: { ...s.progress[role], ...p } } }));

  async function addToModule(page) {
    const prog = get().progress[page.role];
    if (!page.module || !prog?.pageId) return;
    patchPage(page.role, { moduleItem: 'working', moduleError: null });
    try {
      await api.addToModule(courseId, page.module.id, { pageId: prog.pageId, title: prog.title });
      patchPage(page.role, { moduleItem: 'done' });
    } catch (e) {
      patchPage(page.role, { moduleItem: 'failed', moduleError: errText(e) });
    }
  }

  /** Creates one page; false when it failed (the run stops there so later links stay right). */
  async function createOne(page) {
    const { created, options } = get();
    patchPage(page.role, { state: 'working', error: null });
    const doc = linkSet(page.doc, created);
    const isHome = page.role === 'home' && options.makeHome;
    try {
      const r = renderForSave(doc, { courseId, customCssEnabled });
      const title = docTitle(r.doc, kindTitle(page.kind));
      const made = await api.createPage(courseId, wikiPagePayload(r.doc, { title, html: r.html, css: r.css, published: options.publish, frontPage: isHome }));
      set((s) => ({ ...s, created: recordCreated(s.created, page, made.url) }));
      patchPage(page.role, { state: 'done', title: made.title ?? title, pageUrl: made.url, pageId: made.page_id, published: !!made.published });
    } catch (e) {
      patchPage(page.role, { state: 'failed', error: errText(e) });
      set({ stopped: true });
      return false;
    }
    if (page.module && options.addToModules) await addToModule(page);
    return true;
  }

  async function finishUp() {
    const { progress, options, plan } = get();
    set({ stage: 'finishing' });
    const home = progress.home;
    if (options.makeHome && home?.state === 'done' && get().finish.home !== 'done') {
      set((s) => ({ ...s, finish: { ...s.finish, home: 'working', homeError: null } }));
      try {
        await api.setHome(courseId);
        set((s) => ({ ...s, finish: { ...s.finish, home: 'done' } }));
      } catch (e) {
        set((s) => ({ ...s, finish: { ...s.finish, home: 'failed', homeError: errText(e) } }));
      }
    }
    if (options.applyLook && plan.length && get().finish.look !== 'done') {
      const source = plan.find((p) => p.role === 'home') ?? plan[plan.length - 1];
      set((s) => ({ ...s, finish: { ...s.finish, look: 'working', lookError: null } }));
      try {
        await api.setLook(courseId, courseLook(source.doc));
        set((s) => ({ ...s, finish: { ...s.finish, look: 'done' } }));
      } catch (e) {
        set((s) => ({ ...s, finish: { ...s.finish, look: 'failed', lookError: errText(e) } }));
      }
    }
    set({ stage: 'done' });
  }

  async function runQueue() {
    set({ running: true, stopped: false, stage: 'creating' });
    try {
      for (const page of get().plan) {
        if (get().progress[page.role]?.state === 'done') continue;
        if (!(await createOne(page))) return;
      }
      await finishUp();
    } finally {
      set({ running: false });
    }
  }

  return {
    /** Start a run for `plan` with `options`. */
    async start(plan, options) {
      if (get().running || !plan.length) return;
      set({
        ...initialRun,
        plan,
        options: { ...initialRun.options, ...options },
        progress: Object.fromEntries(
          plan.map((p) => [
            p.role,
            { role: p.role, title: docTitle(p.doc, kindTitle(p.kind)), state: 'waiting', moduleItem: p.module && options.addToModules ? 'waiting' : 'none' },
          ]),
        ),
      });
      await runQueue();
    },
    /** Retry a failed page and carry on, or re-add a created page to its module. */
    async retry(role) {
      if (get().running) return;
      const page = get().plan.find((p) => p.role === role);
      const prog = get().progress[role];
      if (!page || !prog) return;
      if (prog.state === 'done') {
        set({ running: true });
        try {
          await addToModule(page);
        } finally {
          set({ running: false });
        }
        return;
      }
      await runQueue();
    },
    /** Retry setting the home page / applying the look. */
    async retryFinish() {
      if (get().running) return;
      set({ running: true });
      try {
        await finishUp();
      } finally {
        set({ running: false });
      }
    },
  };
}
