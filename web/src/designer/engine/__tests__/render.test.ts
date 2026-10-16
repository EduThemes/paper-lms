import { describe, expect, it } from 'vitest';
import { createPage, LAYOUTS, renderDesign, renderPage, SECTION_TYPES, shortHash } from '../index';
import { attr, parseHtml, walk } from '../html';
import type { LayoutId, PageKind, Theme } from '../types';
import { canvasViolations, paperViolations } from './canvas-rules';
import { THEMES } from './fixtures/themes';
import { everySection } from './helpers';

const KINDS: PageKind[] = ['home', 'start-here', 'overview', 'module', 'instructor'];
const opts = { assetBase: 'https://rmt.example.org/assets', courseId: '4821' };

describe('render matrix: every layout × kind × theme', () => {
  for (const theme of THEMES)
    for (const L of LAYOUTS)
      for (const kind of KINDS)
        it(`${theme.id} / ${L.id} / ${kind}`, () => {
          const doc = createPage(theme, kind, 'en', L.id);
          const html = renderPage(doc, theme, opts);
          expect(html.length).toBeGreaterThan(1500);
          expect(paperViolations(html)).toEqual([]);
          expect(canvasViolations(html)).toEqual([]);
          // every section with content made it into the output
          for (const s of doc.sections) expect(html).toContain(`data-pd-section="${s.id}"`);
        });
});

describe('every section type in every layout', () => {
  for (const L of LAYOUTS)
    for (const opt of [
      { density: 'comfortable', headingStyle: 'band' },
      { density: 'compact', headingStyle: 'underline' },
      { density: 'comfortable', headingStyle: 'plain' },
    ] as const)
      it(`${L.id} ${opt.density}/${opt.headingStyle}`, () => {
        for (const theme of THEMES) {
          const doc = { ...everySection(theme, L.id), options: { ...opt } };
          const html = renderPage(doc, theme, opts);
          expect(paperViolations(html)).toEqual([]);
          expect(canvasViolations(html)).toEqual([]);
          const types = new Set<string>();
          walk(parseHtml(html), el => {
            const t = attr(el, 'data-pd-type');
            if (t) types.add(t);
          });
          for (const st of SECTION_TYPES) expect(types, `${st.type} missing`).toContain(st.type);
        }
      });
});

describe('root wrapper and re-edit markers', () => {
  it('carries id, class, data-pd-doc, version, hash of the doc JSON, lang and paired colours', () => {
    const theme = THEMES[0];
    const doc = createPage(theme, 'home', 'pt', 'editorial');
    const html = renderPage(doc, theme, opts);
    const root = parseHtml(html)[0];
    expect(root.type).toBe('el');
    if (root.type !== 'el') return;
    expect(attr(root, 'id')).toBe(`pd-${doc.id}`);
    expect(attr(root, 'class')).toBe('paper-design paper-design--editorial');
    expect(attr(root, 'data-pd-doc')).toBe(doc.id);
    expect(attr(root, 'data-pd-v')).toBe('1');
    expect(attr(root, 'data-pd-hash')).toBe(shortHash(JSON.stringify(doc)));
    expect(attr(root, 'data-pd-theme')).toBe(theme.id);
    expect(attr(root, 'data-pd-layout')).toBe('editorial');
    expect(attr(root, 'data-pd-kind')).toBe('home');
    expect(attr(root, 'lang')).toBe('pt');
    expect(attr(root, 'style')).toMatch(/background:var\(--pd-c\d+, #ffffff\); color:var\(--pd-c\d+, #222b2f\)/);
    // without the dark colourway: plain hex
    const plain = parseHtml(renderPage(doc, theme, { ...opts, dark: false }))[0];
    if (plain.type === 'el') expect(attr(plain, 'style')).toMatch(/background:#ffffff; color:#222b2f/);
  });
  it('hash changes when the doc changes', () => {
    const theme = THEMES[0];
    const doc = createPage(theme, 'home', 'en');
    const a = renderPage(doc, theme, opts);
    const b = renderPage({ ...doc, options: { ...doc.options, density: 'compact' } }, theme, opts);
    expect(/data-pd-hash="(\w+)"/.exec(a)![1]).not.toBe(/data-pd-hash="(\w+)"/.exec(b)![1]);
  });
});

describe('links', () => {
  it('substitutes {course} with the course id', () => {
    const theme = THEMES[1];
    const doc = everySection(theme, 'classic-band');
    const html = renderPage(doc, theme, { assetBase: opts.assetBase, courseId: '77' });
    expect(html).not.toContain('{course}');
    expect(html).toContain('href="/courses/77/modules"');
    expect(html).toContain('href="/courses/77/modules/9"');
  });
  it('leaves the token when no course id is known (the checker warns)', () => {
    const theme = THEMES[1];
    const html = renderPage(createPage(theme, 'home', 'en'), theme, { assetBase: opts.assetBase });
    expect(html).toContain('/courses/{course}/modules');
  });
  it('drops unsafe hrefs and never emits javascript:', () => {
    const theme = THEMES[0];
    const doc = createPage(theme, 'home', 'en', 'card-grid');
    const ql = doc.sections.find(s => s.type === 'quick-links');
    if (ql?.type === 'quick-links') ql.links[0].href = 'javascript:alert(1)';
    const html = renderPage(doc, theme, opts);
    expect(html).not.toMatch(/javascript:/i);
    expect(canvasViolations(html)).toEqual([]);
  });
  it('one link per tile and no adjacent duplicate links', () => {
    for (const L of LAYOUTS) {
      const theme = THEMES[0];
      const html = renderPage(createPage(theme, 'home', 'en', L.id), theme, opts);
      walk(parseHtml(html), el => {
        if (el.name !== 'li') return;
        const links: string[] = [];
        walk(el.children, c => c.name === 'a' && links.push(attr(c, 'href') ?? ''));
        expect(new Set(links).size).toBe(links.length);
      });
    }
  });
});

describe('images', () => {
  it('https only; http custom banners are upgraded, data: dropped; banners never cropped', () => {
    const theme = THEMES[0];
    const doc = createPage(theme, 'home', 'en', 'split-hero');
    const b = doc.sections.find(s => s.type === 'banner');
    if (b?.type === 'banner') b.customSrc = 'http://example.org/b.jpg';
    let html = renderPage(doc, theme, opts);
    expect(html).toContain('src="https://example.org/b.jpg"');
    expect(html).toMatch(/width:100%; height:auto/);
    expect(html).not.toMatch(/srcset="https:\/\/example\.org/); // a teacher's own banner has no @2x twin
    if (b?.type === 'banner') b.customSrc = 'data:image/png;base64,AAAA';
    html = renderPage(doc, theme, opts);
    expect(html).not.toContain('data:image');
  });
  it('icons use the theme icon style and contrasting colours without #', () => {
    const theme: Theme = THEMES[2];
    const html = renderPage(createPage(theme, 'home', 'en', 'classic-band'), theme, opts);
    expect(html).toMatch(/\/icons\/outline\/[a-z-]+\.svg\?fg=[0-9a-f]{6}&amp;bg=[0-9a-f]{6}/);
  });
  it('icons come from iconBase when given', () => {
    const theme: Theme = THEMES[2];
    const html = renderPage(createPage(theme, 'home', 'en', 'classic-band'), theme, { ...opts, iconBase: '/api/v1/design/icons' });
    expect(html).toMatch(/src="\/api\/v1\/design\/icons\/outline\/[a-z-]+\.svg\?fg=[0-9a-f]{6}&amp;bg=[0-9a-f]{6}"/);
    expect(html).not.toContain('/assets/icons/');
  });
  it('theme art carries its @2x twin; the banner is eager, spots and icons lazy', () => {
    const theme: Theme = THEMES[0];
    const html = renderPage(createPage(theme, 'home', 'en', 'classic-band'), theme, opts);
    expect(html).toContain(`srcset="${opts.assetBase}/themes/${theme.id}/banner.jpg 1x, ${opts.assetBase}/themes/${theme.id}/banner@2x.jpg 2x"`);
    const banner = /<img[^>]*data-pd-art="banner"[^>]*>/.exec(html)![0];
    expect(banner).not.toContain('loading=');
    walk(parseHtml(html), el => {
      if (el.name === 'img' && attr(el, 'data-pd-art') !== 'banner') expect(attr(el, 'loading')).toBe('lazy');
    });
  });
  it('rich-text lists keep their markers and headings set their weight (Paper resets both)', () => {
    const theme = THEMES[0];
    const html = renderPage(everySection(theme, 'classic-band'), theme, opts);
    expect(html).toMatch(/<ul style="[^"]*list-style:disc outside;/);
    walk(parseHtml(html), el => {
      if (/^h[2-6]$/.test(el.name)) expect(attr(el, 'style'), el.name).toMatch(/font-weight:(600|700)/);
    });
  });
});

describe('layout specifics', () => {
  it('LAYOUTS lists the seven layouts', () => {
    expect(LAYOUTS.map(l => l.id).sort()).toEqual(['card-grid', 'classic-band', 'editorial', 'playful', 'sidebar', 'split-hero', 'timeline'] as LayoutId[]);
  });
  it('playful tiles are big tap targets with picture-first 96px icons', () => {
    const theme = THEMES[1];
    const html = renderPage(createPage(theme, 'home', 'en', 'playful'), theme, opts);
    expect(html).toMatch(/min-height:64px/);
    expect(html).toMatch(/width="96" height="96"/);
  });
  it('the course title is real text (h2), never only an image', () => {
    for (const L of LAYOUTS) {
      const theme = THEMES[0];
      const html = renderPage(createPage(theme, 'home', 'en', L.id), theme, opts);
      expect(html).toMatch(/<h2[^>]*>\[Course name\]<\/h2>/);
    }
  });
  it('heading font lives in the stylesheet (so a reader’s font wins), only for display faces', () => {
    const lato = { ...THEMES[0], headingFont: "'Lato Extended', Lato, sans-serif" };
    const a = renderDesign(createPage(lato, 'home', 'en'), lato, opts);
    expect(a.html + a.css).not.toContain('font-family');
    const b = renderDesign(createPage(THEMES[2], 'home', 'en', 'editorial'), THEMES[2], opts);
    expect(b.html).not.toContain('font-family');
    expect(b.css).toContain(':root h2,:root h3,:root h4,:root [data-pd-font]{font-family:var(--reading-font-choice, Georgia');
    expect(b.html).toContain('data-pd-font="1"');
  });
  it('a theme without art leaves no empty banner wrapper in any layout', () => {
    const bare: Theme = { ...THEMES[0], images: [] };
    for (const L of LAYOUTS) {
      const html = renderPage(createPage(THEMES[0], 'home', 'en', L.id), bare, opts);
      expect(html, L.id).not.toContain('data-pd-type="banner"');
      expect(canvasViolations(html), L.id).toEqual([]);
    }
  });
});
