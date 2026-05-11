#!/usr/bin/env node
import { mkdir } from 'node:fs/promises';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import puppeteer from 'puppeteer';
import { AxePuppeteer } from '@axe-core/puppeteer';

const BASE_URL = process.env.AXE_BASE_URL || 'http://localhost:5174';
const ROUTES = ['/login', '/setup', '/portfolios/public/sample'];
const FAIL_IMPACTS = new Set(['critical', 'serious']);

const __dirname = dirname(fileURLToPath(import.meta.url));
const SHOT_DIR = resolve(__dirname, '..', 'axe-screenshots');

const slug = (route) => route.replace(/^\/+/, '').replace(/[^a-z0-9]+/gi, '-') || 'root';

async function auditRoute(browser, route) {
  const page = await browser.newPage();
  await page.setViewport({ width: 1280, height: 900 });
  const url = `${BASE_URL}${route}`;
  try {
    await page.goto(url, { waitUntil: 'networkidle0', timeout: 30000 });
  } catch (err) {
    await page.close();
    return { route, url, error: err.message, violations: [] };
  }
  const { violations } = await new AxePuppeteer(page).analyze();
  if (violations.length > 0) {
    await mkdir(SHOT_DIR, { recursive: true });
    await page.screenshot({ path: resolve(SHOT_DIR, `${slug(route)}.png`), fullPage: true });
  }
  await page.close();
  return { route, url, violations };
}

function summarize(results) {
  const rows = [];
  let blocking = 0;
  for (const { route, url, error, violations } of results) {
    if (error) {
      rows.push({ route, status: 'ERROR', critical: '-', serious: '-', moderate: '-', minor: '-', note: error });
      continue;
    }
    const counts = { critical: 0, serious: 0, moderate: 0, minor: 0 };
    for (const v of violations) counts[v.impact ?? 'minor'] = (counts[v.impact ?? 'minor'] || 0) + 1;
    blocking += counts.critical + counts.serious;
    rows.push({ route, status: violations.length === 0 ? 'PASS' : 'FAIL', ...counts, note: url });
  }
  return { rows, blocking };
}

(async () => {
  const browser = await puppeteer.launch({ args: ['--no-sandbox', '--disable-setuid-sandbox'] });
  try {
    const results = [];
    for (const route of ROUTES) results.push(await auditRoute(browser, route));
    const { rows, blocking } = summarize(results);
    console.log('\naxe-core accessibility audit');
    console.table(rows);
    for (const { route, violations = [], error } of results) {
      if (error || violations.length === 0) continue;
      console.log(`\n${route}`);
      for (const v of violations) {
        console.log(`  [${v.impact}] ${v.id}: ${v.help} (${v.nodes.length} node${v.nodes.length === 1 ? '' : 's'})`);
        console.log(`    ${v.helpUrl}`);
      }
    }
    if (blocking > 0) {
      console.error(`\nFAIL: ${blocking} critical/serious violation(s).`);
      process.exit(1);
    }
    console.log('\nPASS: no critical or serious violations.');
  } finally {
    await browser.close();
  }
})().catch((err) => {
  console.error(err);
  process.exit(1);
});
