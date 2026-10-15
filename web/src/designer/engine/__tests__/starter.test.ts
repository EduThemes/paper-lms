import { describe, expect, it } from 'vitest';
import { createPage, LAYOUTS, renderPage, sanitizeRich, strings } from '../index';
import type { Locale, PageKind } from '../types';
import { THEMES } from './fixtures/themes';

const KINDS: PageKind[] = ['home', 'start-here', 'overview', 'module', 'instructor'];
const LOCALES: Locale[] = ['en', 'es', 'pt'];

describe('createPage', () => {
  for (const kind of KINDS)
    for (const locale of LOCALES)
      it(`${kind} / ${locale}`, () => {
        const doc = createPage(THEMES[0], kind, locale);
        expect(doc.version).toBe(1);
        expect(doc.locale).toBe(locale);
        expect(doc.layout).toBe(THEMES[0].defaultLayout);
        expect(doc.paletteId).toBe(THEMES[0].palettes[0].id);
        expect(doc.sections[0].type).toBe('title');
        expect(new Set(doc.sections.map(s => s.id)).size).toBe(doc.sections.length);
        // starter rich text is already clean, and welcome bodies are paragraphs (not lists)
        for (const s of doc.sections) {
          const body = 'body' in s ? s.body : 'bio' in s ? s.bio : undefined;
          if (body) expect(sanitizeRich(body)).toBe(body);
          if (s.type === 'welcome') expect(s.body.startsWith('<p>')).toBe(true);
        }
        expect(JSON.stringify(doc)).not.toMatch(/lorem ipsum/i);
      });

  it('translates starter copy', () => {
    const en = createPage(THEMES[0], 'home', 'en');
    const es = createPage(THEMES[0], 'home', 'es');
    const pt = createPage(THEMES[0], 'home', 'pt');
    const w = (d: typeof en) => d.sections.find(s => s.type === 'welcome');
    expect(w(es)).toMatchObject({ heading: '¡Bienvenidos al curso!' });
    expect(w(pt)).toMatchObject({ heading: 'Boas-vindas ao curso!' });
    expect(w(en)).toMatchObject({ heading: 'Welcome to the course!' });
    for (const l of LOCALES) expect(Object.keys(strings(l)).sort()).toEqual(Object.keys(strings('en')).sort());
  });

  it('quick links take theme icons per slot and {course} hrefs', () => {
    const doc = createPage(THEMES[1], 'home', 'en');
    const ql = doc.sections.find(s => s.type === 'quick-links');
    if (ql?.type !== 'quick-links') throw new Error('no quick links');
    expect(ql.links.find(l => l.slot === 'modules')).toMatchObject({ icon: 'book-open', href: '/courses/{course}/modules' });
    expect(ql.links.find(l => l.slot === 'help')).toMatchObject({ icon: 'hand-helping' });
  });

  it('early-elementary themes get short, picture-first labels', () => {
    const doc = createPage(THEMES[1], 'home', 'en');
    const ql = doc.sections.find(s => s.type === 'quick-links');
    if (ql?.type !== 'quick-links') throw new Error('no quick links');
    expect(ql.links.map(l => l.label)).toContain('Lessons');
  });

  it('honours an explicit layout and its heading defaults', () => {
    for (const L of LAYOUTS) {
      const doc = createPage(THEMES[2], 'overview', 'en', L.id);
      expect(doc.layout).toBe(L.id);
      expect(renderPage(doc, THEMES[2], { assetBase: 'https://a.org' })).toContain(`paper-design--${L.id}`);
    }
  });
});
