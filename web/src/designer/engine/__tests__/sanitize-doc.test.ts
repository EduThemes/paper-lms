import { describe, expect, it } from 'vitest';
import { THEMES } from '../../themes';
import { DOC_CSS_MAX, DOC_TEXT_MAX, LAYOUTS, createPage, renderDesign, sanitizeDoc } from '../index';
import type { PageDoc, PageKind, Section } from '../types';
import { everySection } from './helpers';

const theme = THEMES[0];
const opts = { assetBase: '/design-assets', iconBase: '/api/v1/design/icons', courseId: '42' };

/** A doc with hostile content in every field that could carry it. Made-up data only. */
function hostile(): PageDoc {
  const doc = createPage(theme, 'home', 'en', 'classic-band');
  const sections = [
    { id: 'w', type: 'welcome', heading: 'Hi', body: '<p>Hello</p><img src=x onerror=alert(1)><iframe srcdoc="<script>alert(1)</script>" style="position:fixed;inset:0"></iframe>' },
    { id: 'i', type: 'instructor', heading: 'Me', name: 'Jane Doe', bio: '<p onclick="x()">Bio <a href="javascript:alert(1)">link</a></p>', photoSrc: 'data:image/svg+xml,<svg onload=alert(1)>' },
    { id: 'b', type: 'banner', alt: '', customSrc: 'javascript:alert(1)' },
    {
      id: 'q',
      type: 'quick-links',
      heading: 'Links',
      links: [
        { id: 'l1', label: 'Bad', href: 'javascript:alert(1)', icon: 'book' },
        { id: 'l2', label: 'Obfuscated', href: ' jav&#x61;script:alert(1)', icon: 'book' },
        { id: 'l3', label: 'Good', href: '/courses/{course}/modules', icon: 'book' },
      ],
    },
    { id: 's', type: 'schedule', heading: 'S', items: [{ id: 'a', title: 'Unit', href: 'data:text/html,<script>alert(1)</script>' }] },
    { id: 't', type: 'this-week', heading: 'W', title: 'Week', items: [{ id: 'x', label: 'Do', href: 'vbscript:x' }] },
    { id: 'c', type: 'callout', variant: 'note', heading: 'N', body: '<svg><script>alert(1)</script></svg><p>ok</p>' },
    { id: 'z', type: 'script', html: '<script>alert(1)</script>' },
    { type: 'text', body: 'no id' },
    'not a section',
  ] as unknown as Section[];
  return { ...doc, sections, palette: { id: 'x', name: 'Mine', primary: 'red;background:url(javascript:1)' } as never };
}

describe('sanitizeDoc', () => {
  const clean = sanitizeDoc(hostile());
  const sec = (id: string) => clean.sections.find(s => s.id === id) as Record<string, unknown> | undefined;

  it('removes <img onerror> and <iframe srcdoc> from rich text', () => {
    expect(sec('w')!.body).toBe('<p>Hello</p>');
    expect(sec('c')!.body).toBe('<p>ok</p>');
    expect(sec('i')!.bio).toBe('<p>Bio link</p>');
    expect(JSON.stringify(clean)).not.toMatch(/onerror|onclick|<iframe|srcdoc|<script|<svg/i);
  });

  it('drops javascript:, data: and vbscript: hrefs, keeps safe ones', () => {
    const links = sec('q')!.links as { href: string }[];
    expect(links.map(l => l.href)).toEqual(['', '', '/courses/{course}/modules']);
    expect((sec('s')!.items as { href: string }[])[0].href).toBe('');
    expect((sec('t')!.items as { href: string }[])[0].href).toBe('');
  });

  it('drops data: and javascript: image sources', () => {
    expect(sec('i')).not.toHaveProperty('photoSrc');
    expect(sec('b')).not.toHaveProperty('customSrc');
  });

  it('keeps https and Paper-relative image sources', () => {
    const doc = sanitizeDoc({
      ...createPage(theme, 'home', 'en'),
      sections: [
        { id: 'b', type: 'banner', alt: '', customSrc: 'https://example.org/b.png' },
        { id: 'i', type: 'instructor', heading: 'Me', name: 'J', bio: '', photoSrc: '/api/v1/files/7/download' },
      ],
    });
    expect(doc.sections[0]).toHaveProperty('customSrc', 'https://example.org/b.png');
    expect(doc.sections[1]).toHaveProperty('photoSrc', '/api/v1/files/7/download');
  });

  it('drops sections of unknown type or without an id', () => {
    expect(clean.sections.map(s => s.id)).toEqual(['w', 'i', 'b', 'q', 's', 't', 'c']);
  });

  it('drops an invalid custom palette, keeps a valid one', () => {
    expect(clean).not.toHaveProperty('palette');
    const p = { ...theme.palettes[0], id: 'scheme-1', name: 'Mine' };
    const { dark: _dark, ...plain } = p;
    expect(sanitizeDoc({ ...createPage(theme, 'home', 'en'), palette: plain }).palette).toEqual(plain);
    // Extra keys: the cleaned palette replaces it.
    const extra = sanitizeDoc({ ...createPage(theme, 'home', 'en'), palette: { ...plain, evil: '<img src=x onerror=alert(1)>' } as never });
    expect(extra.palette).not.toHaveProperty('evil');
    expect(extra.palette?.primary).toBe(plain.primary.toLowerCase());
  });

  it('caps strings and the custom stylesheet, drops non-JSON values', () => {
    const doc = sanitizeDoc({
      ...createPage(theme, 'home', 'en'),
      customCss: 'a'.repeat(DOC_CSS_MAX + 10),
      sections: [{ id: 't', type: 'title', text: 'x'.repeat(DOC_TEXT_MAX * 2), subtitle: (() => 1) as never, extra: Infinity as never } as Section],
    });
    expect(doc.customCss).toHaveLength(DOC_CSS_MAX);
    expect((doc.sections[0] as { text: string }).text).toHaveLength(DOC_TEXT_MAX);
    expect(doc.sections[0]).not.toHaveProperty('subtitle');
    expect(doc.sections[0]).not.toHaveProperty('extra');
  });

  it('survives garbage', () => {
    expect(sanitizeDoc({ ...createPage(theme, 'home', 'en'), sections: 'nope' as never }).sections).toEqual([]);
    expect(() => sanitizeDoc(null as never)).not.toThrow();
  });

  it('leaves every starter doc (all kinds and layouts) and every section type unchanged', () => {
    const kinds: PageKind[] = ['home', 'start-here', 'overview', 'module', 'instructor'];
    for (const t of THEMES.slice(0, 12)) {
      for (const kind of kinds) {
        for (const { id } of LAYOUTS) {
          const doc = createPage(t, kind, 'en', id);
          expect(sanitizeDoc(doc)).toEqual(doc);
        }
      }
    }
    const all = everySection(theme, 'classic-band');
    expect(sanitizeDoc(all)).toEqual(all);
  });

  it('keeps an accepted link or image as typed (the renderer normalises it), so the saved hash still matches', () => {
    const doc = {
      ...createPage(theme, 'home', 'en'),
      sections: [
        { id: 'q', type: 'quick-links', heading: 'L', links: [{ id: 'a', label: 'A', href: 'http://example.org/a', icon: 'book' }] },
        { id: 'b', type: 'banner', alt: '', customSrc: 'http://example.org/b.png' },
      ],
    } as PageDoc;
    expect(sanitizeDoc(doc)).toEqual(doc);
  });

  it('renders a hostile doc with nothing executable', () => {
    const { html } = renderDesign(clean, theme, opts);
    expect(html).not.toMatch(/onerror|onclick|<iframe|srcdoc|<script|javascript:|data:/i);
  });
});

describe('instructor photo source', () => {
  it('the renderer refuses a data: photo even without sanitizeDoc', () => {
    const doc = {
      ...createPage(theme, 'instructor', 'en'),
      sections: [{ id: 'i', type: 'instructor', heading: 'Me', name: 'Jane Doe', bio: '', photoSrc: 'data:image/png;base64,AAAA' }],
    } as PageDoc;
    const { html } = renderDesign(doc, theme, opts);
    expect(html).not.toContain('data:image');
    expect(html).toContain('JD');
  });
});
