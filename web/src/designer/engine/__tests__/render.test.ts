import { describe, expect, it } from 'vitest';
import { createPage, LAYOUTS, renderPage, SECTION_TYPES, shortHash } from '../index';
import { attr, parseHtml, walk } from '../html';
import type { LayoutId, PageKind, Theme } from '../types';
import { canvasViolations } from './canvas-rules';
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
          expect(canvasViolations(html)).toEqual([]);
          // every section with content made it into the output
          for (const s of doc.sections) expect(html).toContain(`data-rmt-section="${s.id}"`);
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
          expect(canvasViolations(html)).toEqual([]);
          const types = new Set<string>();
          walk(parseHtml(html), el => {
            const t = attr(el, 'data-rmt-type');
            if (t) types.add(t);
          });
          for (const st of SECTION_TYPES) expect(types, `${st.type} missing`).toContain(st.type);
        }
      });
});

describe('root wrapper and re-edit markers', () => {
  it('carries id, data-rmt-doc, version, hash of the doc JSON, lang and paired colours', () => {
    const theme = THEMES[0];
    const doc = createPage(theme, 'home', 'pt', 'editorial');
    const html = renderPage(doc, theme, opts);
    const root = parseHtml(html)[0];
    expect(root.type).toBe('el');
    if (root.type !== 'el') return;
    expect(attr(root, 'id')).toBe(`rmt-${doc.id}`);
    expect(attr(root, 'data-rmt-doc')).toBe(doc.id);
    expect(attr(root, 'data-rmt-v')).toBe('1');
    expect(attr(root, 'data-rmt-hash')).toBe(shortHash(JSON.stringify(doc)));
    expect(attr(root, 'lang')).toBe('pt');
    expect(attr(root, 'style')).toMatch(/background:#ffffff; color:#222b2f/);
  });
  it('hash changes when the doc changes', () => {
    const theme = THEMES[0];
    const doc = createPage(theme, 'home', 'en');
    const a = renderPage(doc, theme, opts);
    const b = renderPage({ ...doc, options: { ...doc.options, density: 'compact' } }, theme, opts);
    expect(/data-rmt-hash="(\w+)"/.exec(a)![1]).not.toBe(/data-rmt-hash="(\w+)"/.exec(b)![1]);
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
    if (b?.type === 'banner') b.customSrc = 'data:image/png;base64,AAAA';
    html = renderPage(doc, theme, opts);
    expect(html).not.toContain('data:image');
  });
  it('icons use the theme icon style and contrasting colours without #', () => {
    const theme: Theme = THEMES[2];
    const html = renderPage(createPage(theme, 'home', 'en', 'classic-band'), theme, opts);
    expect(html).toMatch(/\/icons\/outline\/[a-z-]+\.svg\?fg=[0-9a-f]{6}&amp;bg=[0-9a-f]{6}/);
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
  it('heading font only for display faces, never Canvas’s own Lato stack', () => {
    const lato = { ...THEMES[0], headingFont: "'Lato Extended', Lato, sans-serif" };
    expect(renderPage(createPage(lato, 'home', 'en'), lato, opts)).not.toContain('font-family');
    expect(renderPage(createPage(THEMES[2], 'home', 'en'), THEMES[2], opts)).toContain('font-family:Georgia');
  });
  it('a theme without art leaves no empty banner wrapper in any layout', () => {
    const bare: Theme = { ...THEMES[0], images: [] };
    for (const L of LAYOUTS) {
      const html = renderPage(createPage(THEMES[0], 'home', 'en', L.id), bare, opts);
      expect(html, L.id).not.toContain('data-rmt-type="banner"');
      expect(canvasViolations(html), L.id).toEqual([]);
    }
  });
});
