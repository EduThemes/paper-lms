import { describe, expect, it } from 'vitest';
import { SECTION_TYPES, THEMES, createPage, courseLook, lookBanner } from '../lib/engine';
import { cloneSection, newSection, paperizeDoc, sectionName, toPaperHref } from '../lib/sections';
import { normalizeRich, richToText } from '../lib/richtext';
import { toScheme, schemeIdOf, cleanSchemeName } from '../lib/schemes';
import { makeDoc } from './fixtures';

describe('sections', () => {
  it('makes a starter section of every type, in the teacher’s language', () => {
    const t = (k, v) => (v ? `${k}:${JSON.stringify(v)}` : k);
    for (const { type } of SECTION_TYPES) expect(newSection(type, 'home', t).type).toBe(type);
    expect(newSection('quick-links', 'home', t).links.map((l) => l.href)).toEqual([
      '/courses/{course}/modules',
      '/courses/{course}/assignments',
      '/courses/{course}/discussions',
      '/courses/{course}/grades',
    ]);
  });

  it('clones with fresh ids', () => {
    const s = { id: 'q', type: 'quick-links', heading: 'Go', links: [{ id: 'l1', label: 'A', href: '/', icon: 'flag' }] };
    const copy = cloneSection(s);
    expect(copy.id).not.toBe('q');
    expect(copy.links[0].id).not.toBe('l1');
    expect(s.links[0].id).toBe('l1');
  });

  it('names sections for the outline', () => {
    expect(sectionName({ id: 'a', type: 'title', text: 'Bio' }, () => 'Course title')).toBe('Course title: Bio');
    expect(sectionName({ id: 'd', type: 'divider' }, () => 'Divider')).toBe('Divider');
  });

  it('rewrites Canvas-only link targets in starter content to Paper’s', () => {
    expect(toPaperHref('/courses/{course}/assignments/syllabus')).toBe('/courses/{course}/syllabus');
    expect(toPaperHref('/courses/{course}/discussion_topics')).toBe('/courses/{course}/discussions');
    expect(toPaperHref('/calendar?include_contexts=course_{course}')).toBe('/calendar');
    const doc = paperizeDoc(createPage(THEMES[0], 'overview', 'en'));
    const hrefs = doc.sections.flatMap((s) => (s.type === 'quick-links' ? s.links.map((l) => l.href) : []));
    expect(hrefs.some((h) => /discussion_topics|assignments\/syllabus|include_contexts/.test(h))).toBe(false);
    const plain = makeDoc();
    expect(paperizeDoc(plain)).toBe(plain);
  });
});

describe('rich text', () => {
  it('keeps only the allowed subset', () => {
    expect(normalizeRich('<div><b>Hi</b> <i>there</i> <span style="color:red">you</span></div>')).toBe('<p><strong>Hi</strong> <em>there</em> you</p>');
    expect(normalizeRich('<p><a href="javascript:alert(1)">x</a><a href="https://a.test">y</a></p><script>bad()</script>')).toBe('<p>x<a href="https://a.test">y</a></p>');
    expect(normalizeRich('<p><br></p>')).toBe('');
    expect(richToText('<p>A <b>b</b></p>')).toBe('A b');
  });
});

describe('colour schemes', () => {
  it('maps server schemes to palettes with scheme ids', () => {
    const p = THEMES[0].palettes[0];
    const s = toScheme({ id: 7, name: '  School <colours> ', palette: p });
    expect(s.palette.id).toBe('scheme-7');
    expect(s.name).toBe('School colours');
    expect(schemeIdOf(s.palette.id)).toBe('7');
    expect(toScheme({ id: 8, name: 'Bad', palette: { primary: 'red' } })).toBeNull();
    expect(cleanSchemeName('a\u0000b')).toBe('ab');
  });
});

describe('course look', () => {
  it('is the engine look, with the page’s own uploaded banner when it has one', () => {
    const doc = makeDoc([{ id: 'b', type: 'banner', customSrc: '/api/v1/files/9/download', alt: 'Our garden' }]);
    const look = courseLook(doc);
    expect(look.version).toBe(1);
    expect(look.theme_id).toBe(doc.themeId);
    expect(look.colors.primary).toMatch(/^#[0-9a-f]{6}$/);
    expect(look.dark_colors).toBeTruthy();
    expect(look.banner).toEqual({ src: '/api/v1/files/9/download', src_2x: '/api/v1/files/9/download', alt: 'Our garden' });
    expect(lookBanner(makeDoc([{ id: 'b', type: 'banner', customSrc: 'https://elsewhere.test/a.png', alt: '' }]))).toBeUndefined();
    expect(courseLook(makeDoc()).banner?.src ?? '').toMatch(/^(\/design-assets\/|$)/);
  });
});
