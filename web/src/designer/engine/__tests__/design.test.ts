import { describe, expect, it } from 'vitest';
import {
  AA,
  checkPalette,
  contrastRatio,
  createPage,
  darkPalette,
  derivePalette,
  fixDarkPalette,
  flattenDesignVars,
  hexToHsl,
  LAYOUTS,
  luminance,
  PAPER_DARK_BACKGROUND,
  READER_FONT_VAR,
  renderDesign,
  renderDesignDetail,
  renderPage,
  resolveDarkPalette,
  themeDarkPalette,
} from '../index';
import { renderBody } from '../render';
import { mergeRenders } from '../render/design';
import type { PageDoc, PageKind, RenderOptions, Theme } from '../types';
import { paperViolations } from './canvas-rules';
import { THEMES } from './fixtures/themes';
import { everySection } from './helpers';

const KINDS: PageKind[] = ['home', 'start-here', 'overview', 'module', 'instructor'];
const opts: RenderOptions = { assetBase: '/design-assets', iconBase: '/api/v1/design/icons', courseId: '42' };

/** The dark block's custom properties and icon swaps, parsed back out of the stylesheet. */
function darkBlock(css: string): { vars: Map<string, string>; icons: Map<string, string> } {
  const block = /@media \(prefers-color-scheme: dark\)\{(.*)\}\n/.exec(css)?.[1] ?? '';
  const vars = new Map<string, string>();
  for (const [, k, v] of block.matchAll(/(--pd-c\d+):(#[0-9a-f]{6})/g)) vars.set(k, v);
  const icons = new Map<string, string>();
  for (const [, n, src] of block.matchAll(/img\[data-pd-icon="(\d+)"\]\{content:url\("([^"]+)"\)\}/g)) icons.set(n, src);
  return { vars, icons };
}

/** The page as dark mode shows it: every var resolved to its dark value and every swapped icon to its dark src. */
function applyDark(html: string, css: string): string {
  const { vars, icons } = darkBlock(css);
  return html
    .replace(/var\((--pd-c\d+), #[0-9a-f]{6}\)/g, (_m, k: string) => vars.get(k) ?? 'MISSING')
    .replace(/ data-pd-icon="(\d+)" src="[^"]*"/g, (_m, n: string) => ` src="${(icons.get(n) ?? 'MISSING').replace(/&/g, '&amp;')}"`);
}
const plainLight = (html: string) => flattenDesignVars(html).replace(/ data-pd-icon="\d+"/g, '');

function docs(theme: Theme): PageDoc[] {
  return [...LAYOUTS.flatMap(L => KINDS.map(k => createPage(theme, k, 'en', L.id))), ...LAYOUTS.map(L => everySection(theme, L.id))];
}

describe('renderDesign', () => {
  it('returns { html, css }; renderPage is its html', () => {
    const theme = THEMES[0];
    const doc = createPage(theme, 'home', 'en', 'classic-band');
    const out = renderDesign(doc, theme, opts);
    expect(Object.keys(out).sort()).toEqual(['css', 'html']);
    expect(renderPage(doc, theme, opts)).toBe(out.html);
    expect(out.html.startsWith(`<div id="pd-${doc.id}" class="paper-design paper-design--classic-band"`)).toBe(true);
    expect(out.css).toContain('@media (prefers-color-scheme: dark){:root{--pd-c1:');
    expect(out.css).toContain('@media (prefers-reduced-motion: reduce){');
    expect(out.css).toContain(':root a:focus-visible{outline:3px solid var(--pd-focus)');
    expect(out.css).toContain(':root a[data-pd-tile]:hover');
  });

  it('is deterministic', () => {
    const theme = THEMES[2];
    const doc = everySection(theme, 'sidebar');
    expect(renderDesign(doc, theme, opts)).toEqual(renderDesign(doc, theme, opts));
  });

  it('dark: false → plain hex and no dark block', () => {
    const theme = THEMES[0];
    const doc = everySection(theme, 'timeline');
    const out = renderDesign(doc, theme, { ...opts, dark: false });
    expect(out.html).not.toContain('var(');
    expect(out.html).not.toContain('data-pd-icon');
    expect(out.css).not.toContain('prefers-color-scheme');
    expect(out.css).toContain(':root{--pd-focus:');
  });

  for (const theme of THEMES)
    it(`${theme.id}: the HTML alone is the light page; vars + icon swaps rebuild the dark page exactly`, () => {
      for (const doc of docs(theme)) {
        const d = renderDesignDetail(doc, theme, opts);
        expect(d.diverged, `${doc.layout}/${doc.kind}`).toBe(false);
        expect(paperViolations(d.html)).toEqual([]);
        // light: flattening gives the light-only render byte for byte
        expect(plainLight(d.html)).toBe(renderDesign(doc, theme, { ...opts, dark: false }).html);
        // dark: the stylesheet's values give the dark render byte for byte
        expect(applyDark(d.html, d.css)).toBe(renderBody(doc, theme, opts, resolveDarkPalette(doc, theme)).html);
        // every var used has a dark value; each (light, dark) pair has one var
        const used = new Set([...d.html.matchAll(/var\((--pd-c\d+), /g)].map(m => m[1]));
        expect([...used].sort()).toEqual(Object.keys(d.vars).sort());
        const pairs = Object.values(d.vars).map(([l, k]) => `${l}|${k}`);
        expect(new Set(pairs).size).toBe(pairs.length);
        for (const [, [l, k]] of Object.entries(d.vars)) expect(l).not.toBe(k);
      }
    });

  it('swaps icons drawn in page colours, by data-pd-icon, with same-origin urls', () => {
    const theme = THEMES[1];
    const d = renderDesignDetail(createPage(theme, 'home', 'en', 'classic-band'), theme, opts);
    expect(Object.keys(d.icons).length).toBeGreaterThan(0);
    for (const [n, src] of Object.entries(d.icons)) {
      expect(d.html).toMatch(new RegExp(`<img data-pd-icon="${n}" src="/api/v1/design/icons/[a-z-]+/[a-z0-9-]+\\.svg\\?fg=[0-9a-f]{6}&amp;bg=[0-9a-f]{6}"`));
      expect(src).toMatch(/^\/api\/v1\/design\/icons\/[a-z-]+\/[a-z0-9-]+\.svg\?fg=[0-9a-f]{6}&bg=[0-9a-f]{6}$/);
      expect(d.css).toContain(`:root img[data-pd-icon="${n}"]{content:url("${src}")}`);
    }
    // theme art is dimmed a touch in the dark, never swapped
    expect(d.css).toContain(':root img[data-pd-art]{filter:brightness(.92)}');
    expect(d.html).toMatch(/<img src="\/design-assets\/themes\/[^"]+\/banner\.jpg" srcset=/);
  });

  it('the stylesheet stays inside what the page-stylesheet sanitizer keeps', () => {
    for (const theme of THEMES)
      for (const L of LAYOUTS) {
        const { css } = renderDesign(everySection(theme, L.id), theme, opts);
        expect(css).not.toMatch(/<\/style|@import|@font-face|expression\(|javascript:/i);
        for (const [, u] of css.matchAll(/url\("([^"]*)"\)/g)) expect(u).toMatch(/^(https:|\/api\/v1\/design\/icons\/|\/design-assets\/)/);
        // every rule is written against :root (the page) or is an allowed at-rule
        const selectors = css.replace(/@media [^{]+\{/g, '').match(/[^{}]+(?=\{)/g) ?? [];
        for (const s of selectors) for (const one of s.split(',')) expect(one.trim(), one).toMatch(/^:root\b/);
      }
  });

  it('the heading font defers to the reader’s chosen font', () => {
    const theme = THEMES[2]; // Georgia
    const { css } = renderDesign(createPage(theme, 'home', 'en'), theme, opts);
    expect(css).toContain(`font-family:var(${READER_FONT_VAR}, Georgia`);
  });

  it('a structural difference between the renders is refused (the page then falls back to light only)', () => {
    expect(mergeRenders('<p style="color:#000000;">a</p>', '<p style="color:#ffffff;">a</p>')?.html).toBe('<p style="color:var(--pd-c1, #000000);">a</p>');
    expect(mergeRenders('<p style="color:#000000;">a</p>', '<div style="color:#ffffff;">a</div>')).toBeNull();
    expect(mergeRenders('<p style="color:#000000;">a</p>', '<p style="color:#ffffff; margin:0;">a</p>')).toBeNull();
    expect(mergeRenders('<p style="color:#000000;">a</p><b>x</b>', '<p style="color:#000000;">a</p><b>y</b>')).toBeNull();
    expect(mergeRenders('<p>#000000</p>', '<p>#ffffff</p>')).toBeNull(); // colours outside style="" are content
  });
});

describe('darkPalette', () => {
  const hueDistance = (a: number, b: number) => Math.min(Math.abs(a - b), 360 - Math.abs(a - b));
  /** Two colours look like the same brand colour: same hue when both are clearly coloured. */
  const sameHue = (a: string, b: string, tol = 12) => {
    const [ha, sa, la] = hexToHsl(a);
    const [hb, sb, lb] = hexToHsl(b);
    if (sa < 0.2 || sb < 0.2 || la < 0.08 || lb < 0.08 || la > 0.95 || lb > 0.95) return true;
    return hueDistance(ha, hb) <= tol;
  };

  it('is a dark sheet near Paper’s dark background, tinted with the primary', () => {
    for (const theme of THEMES)
      for (const p of theme.palettes) {
        const d = darkPalette(p);
        expect(checkPalette(d).filter(r => !r.pass)).toEqual([]);
        expect(luminance(d.background)).toBeLessThan(0.03);
        expect(contrastRatio(d.background, PAPER_DARK_BACKGROUND)).toBeLessThan(1.25);
        expect(luminance(d.surface)).toBeGreaterThan(luminance(d.background));
        expect(luminance(d.text)).toBeGreaterThan(0.75);
        expect(sameHue(d.primary, p.primary)).toBe(true);
        expect([d.id, d.name]).toEqual([p.id, p.name]);
      }
  });

  it('property: 600 random derived palettes have a dark twin that passes every pair and keeps the hues', () => {
    let seed = 1234;
    const r = () => ((seed = (seed * 1103515245 + 12345) >>> 0) / 4294967296);
    const hex = () => `#${Math.floor(r() * 0x1000000).toString(16).padStart(6, '0')}`;
    for (let i = 0; i < 600; i++) {
      const light = derivePalette({ primary: hex(), secondary: hex(), accent: hex() }, `R${i}`);
      const d = darkPalette(light);
      expect(checkPalette(d).filter(x => !x.pass), JSON.stringify(light)).toEqual([]);
      expect(luminance(d.background)).toBeLessThan(0.03);
      for (const k of ['primary', 'secondary', 'link'] as const) expect(sameHue(d[k], light[k]), `${k} ${light[k]} → ${d[k]}`).toBe(true);
      expect(contrastRatio(d.accent, d.background)).toBeGreaterThanOrEqual(3);
    }
  });

  it('hand-tuned dark colours are merged and fixed to pass', () => {
    const p = { ...THEMES[0].palettes[0], dark: { primary: '#ff9966', text: '#333333', link: '#101010' } };
    const d = themeDarkPalette(p);
    expect(d.primary).toBe('#ff9966');
    expect(checkPalette(d).filter(x => !x.pass)).toEqual([]);
    expect(themeDarkPalette({ ...THEMES[0].palettes[0] })).toEqual(darkPalette(THEMES[0].palettes[0]));
  });

  it('fixDarkPalette leaves a passing palette alone', () => {
    const d = darkPalette(THEMES[1].palettes[0]);
    expect(fixDarkPalette(d)).toEqual(d);
  });

  it('a custom palette renders with its own dark twin', () => {
    const theme = THEMES[0];
    const palette = derivePalette({ primary: '#7a1fa2' }, 'Purple');
    const doc = { ...createPage(theme, 'home', 'en'), palette };
    expect(resolveDarkPalette(doc, theme)).toEqual(darkPalette(palette));
    const { css } = renderDesign(doc, theme, opts);
    expect(css).toContain(darkPalette(palette).primary);
  });

  it('AA is the bar', () => {
    expect(AA).toBe(4.5);
  });
});
