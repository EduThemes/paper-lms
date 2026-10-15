#!/usr/bin/env node
// Authenticated axe sweep: signs in as each role, visits the app's main
// routes in light and dark mode at desktop and phone widths, and runs axe
// with the WCAG 2.1 A/AA rules (best-practice rules are reported, not
// counted). Complements run-axe.mjs (anonymous, CI-gated).
//
// Needs a running frontend (AXE_BASE_URL, default http://localhost:5174)
// with its /api proxy pointed at a backend holding test data (course 1 with
// a front page; see cmd/seedtestdata). Credentials come from the
// environment only; a role without them is skipped:
//
//   AXE_TEACHER_EMAIL / AXE_TEACHER_PASSWORD
//   AXE_STUDENT_EMAIL / AXE_STUDENT_PASSWORD
//   AXE_ADMIN_EMAIL   / AXE_ADMIN_PASSWORD
//
// Options (env): AXE_COURSE_ID (1), AXE_FRONT_PAGE (slug of a page to open),
// AXE_ROLES (anon,teacher,student,admin), AXE_THEMES (light,dark),
// AXE_WIDTHS (1280,390), AXE_ROUTE (regex filter), AXE_CONCURRENCY (4),
// AXE_SWEEP_OUT (JSON results path, default in the OS temp dir).
//
//   npm run axe:sweep
import { writeFile } from 'node:fs/promises';
import { createRequire } from 'node:module';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import puppeteer from 'puppeteer';

// The CommonJS build: the ESM one fails to find axe-core when the checkout
// path contains a space.
const { AxePuppeteer } = createRequire(import.meta.url)('@axe-core/puppeteer');

const env = process.env;
const BASE = env.AXE_BASE_URL || 'http://localhost:5174';
const C = env.AXE_COURSE_ID || '1';
const FRONT = env.AXE_FRONT_PAGE;
const WCAG = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'];
const list = (v, d) => (v || d).split(',').map((s) => s.trim()).filter(Boolean);

const COMMON = [
  '/', '/courses', `/courses/${C}`, `/courses/${C}/modules`, `/courses/${C}/assignments`,
  `/courses/${C}/pages`, ...(FRONT ? [`/courses/${C}/pages/${FRONT}`] : []),
  `/courses/${C}/discussions`, `/courses/${C}/announcements`, `/courses/${C}/grades`,
  `/courses/${C}/people`, `/courses/${C}/quizzes`, `/courses/${C}/files`, `/courses/${C}/calendar`,
  `/courses/${C}/syllabus`, `/courses/${C}/leaderboard`, '/calendar', '/inbox', '/notifications',
  '/settings/notifications', '/settings/language', '/profile', '/profile/settings',
  '/profile/reading-preferences', '/profile/badges', '/profile/gamification', '/planner',
];
const TEACHER = [
  `/courses/${C}/gradebook`, `/courses/${C}/settings`, `/courses/${C}/designer`,
  ...(FRONT ? [`/courses/${C}/designer?page=${FRONT}`] : []),
  `/courses/${C}/designer/makeover`, `/courses/${C}/designer/makeover?look=1`,
  // #use: pick the first look in the gallery, then scan the look-only stage
  `/courses/${C}/designer/makeover?look=1#use`,
  `/courses/${C}/rubrics`, `/courses/${C}/outcomes`, `/courses/${C}/analytics`,
  `/courses/${C}/sections`, `/courses/${C}/question_banks`,
];
const ADMIN = [
  '/', '/courses', `/courses/${C}`, `/courses/${C}/designer`, '/inbox', '/profile/settings',
  '/admin', '/admin/courses', '/admin/people', '/admin/settings', '/admin/theme', '/admin/roles',
  '/admin/terms', '/admin/auth_providers', '/admin/feature_flags', '/admin/gamification/currencies',
  '/admin/gamification/badges', '/admin/reports', '/admin/developer_keys', '/admin/lti',
  '/admin/sis_import', '/admin/grading_periods', '/admin/grade_statuses', '/admin/ferpa',
  '/admin/oneroster', '/admin/tools',
];
const ROLES = {
  anon: ['/login', '/privacy', '/accessibility'],
  teacher: [...COMMON, ...TEACHER],
  student: COMMON,
  admin: ADMIN,
};
const credsFor = (role) => ({
  email: env[`AXE_${role.toUpperCase()}_EMAIL`],
  password: env[`AXE_${role.toUpperCase()}_PASSWORD`],
});

const routeRe = env.AXE_ROUTE ? new RegExp(env.AXE_ROUTE) : null;
const roles = list(env.AXE_ROLES, 'anon,teacher,student,admin').filter((r) => {
  if (!ROLES[r]) throw new Error(`unknown role ${r}`);
  if (r === 'anon') return true;
  const { email, password } = credsFor(r);
  if (!email || !password) console.warn(`skipping ${r}: set AXE_${r.toUpperCase()}_EMAIL and _PASSWORD`);
  return !!(email && password);
});

async function settle(page) {
  try { await page.waitForNetworkIdle({ idleTime: 500, timeout: 15000 }); } catch { /* scan anyway */ }
  try {
    await page.waitForFunction(() => !document.querySelector('.animate-spin, [aria-busy="true"]'), { timeout: 10000 });
  } catch { /* scan anyway */ }
  await new Promise((r) => setTimeout(r, 600));
}

async function scanCombo(browser, role, theme, width) {
  const ctx = await browser.createBrowserContext();
  const page = await ctx.newPage();
  await page.setViewport({ width, height: width < 600 ? 844 : 900 });
  // useTheme reads localStorage before first paint, so React keeps this theme.
  await page.evaluateOnNewDocument((t) => { try { localStorage.setItem('paper.theme', t); } catch { /* */ } }, theme);
  if (role !== 'anon') {
    await page.goto(`${BASE}/login`, { waitUntil: 'networkidle0' });
    const { email, password } = credsFor(role);
    // POST /login is limited to 10 a minute per IP: wait out a 429 and retry.
    let status = 0;
    for (let i = 0; i < 10 && status !== 200; i++) {
      status = await page.evaluate(async (e, p) => {
        const res = await fetch('/api/v1/login', { method: 'POST', credentials: 'include', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ email: e, password: p }) });
        if (res.status !== 200) return res.status;
        return (await fetch('/api/v1/users/self', { credentials: 'include' })).status;
      }, email, password);
      if (status === 429) await new Promise((r) => setTimeout(r, 15000));
      else if (status !== 200) break;
    }
    if (status !== 200) throw new Error(`sign-in as ${role} failed (${status})`);
  }
  const out = [];
  for (const route of ROLES[role]) {
    if (routeRe && !routeRe.test(route)) continue;
    const rec = { role, theme, width, route };
    try {
      const [path, action] = route.split('#');
      await page.goto(`${BASE}${path}`, { waitUntil: 'domcontentloaded', timeout: 30000 });
      await settle(page);
      if (action === 'use') {
        await page.evaluate(() => {
          const b = [...document.querySelectorAll('li[aria-labelledby] button')]
            .find((x) => x.getAttribute('aria-hidden') !== 'true' && /use/i.test(x.textContent));
          b?.click();
        });
        await settle(page);
      }
      rec.finalPath = new URL(page.url()).pathname;
      const res = await new AxePuppeteer(page).withTags([...WCAG, 'best-practice']).analyze();
      rec.violations = res.violations.map((v) => ({
        id: v.id, impact: v.impact, wcag: v.tags.some((t) => WCAG.includes(t)), help: v.help,
        nodes: v.nodes.map((n) => ({ target: n.target.join(' '), html: n.html.slice(0, 200), summary: n.failureSummary })),
      }));
    } catch (err) {
      rec.error = String(err?.message || err);
    }
    out.push(rec);
    const w = (rec.violations || []).filter((v) => v.wcag);
    const nodes = w.reduce((s, v) => s + v.nodes.length, 0);
    console.log(`${rec.error ? 'ERR ' : nodes ? 'FAIL' : 'ok  '} ${role}/${theme}/${width} ${route}${nodes ? ` ${nodes} [${w.map((v) => v.id).join(',')}]` : ''}${rec.error ? ` ${rec.error}` : ''}`);
  }
  await ctx.close();
  return out;
}

const combos = [];
for (const r of roles) for (const t of list(env.AXE_THEMES, 'light,dark')) for (const w of list(env.AXE_WIDTHS, '1280,390')) combos.push([r, t, Number(w)]);
const browser = await puppeteer.launch({ args: ['--no-sandbox', '--disable-setuid-sandbox'] });
const results = [];
try {
  const queue = [...combos];
  await Promise.all(Array.from({ length: Number(env.AXE_CONCURRENCY || 4) }, async () => {
    while (queue.length) results.push(...await scanCombo(browser, ...queue.shift()));
  }));
} finally {
  await browser.close();
}
const outPath = env.AXE_SWEEP_OUT || join(tmpdir(), 'paper-axe-sweep.json');
await writeFile(outPath, JSON.stringify(results, null, 1));
const failing = results.filter((r) => r.error || (r.violations || []).some((v) => v.wcag));
const byRule = {};
for (const r of results) for (const v of r.violations || []) if (v.wcag) byRule[v.id] = (byRule[v.id] || 0) + v.nodes.length;
console.log(`\n${results.length} scans, ${failing.length} with WCAG violations or errors. Results: ${outPath}`);
if (Object.keys(byRule).length) console.table(byRule);
process.exit(failing.length ? 1 : 0);
