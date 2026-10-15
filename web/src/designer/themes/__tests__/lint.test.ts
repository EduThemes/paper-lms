import { existsSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { createPage, LAYOUTS, renderPage, type PageKind } from '@/designer/engine';
import { THEME_ENTRIES } from '../index';
import type { ThemeEntry } from '../define';
import { ACCENT_IN_LAYOUT, DESIGN_ASSET_DIR, lintTheme, paletteProblems, personIn } from '../lint';
import { seedPalette } from '../scripts/lib';

const KINDS: PageKind[] = ['home', 'start-here', 'overview', 'module', 'instructor'];
const kintsugi = THEME_ENTRIES.find((e) => e.theme.id === 'kintsugi')!;
/** kintsugi with some fields replaced. */
const variant = (theme: Partial<ThemeEntry['theme']>): ThemeEntry => ({ ...kintsugi, theme: { ...kintsugi.theme, ...theme } });

describe('prompt-people', () => {
  it.each([
    'A smiling face drawn on a round stone',
    "A child's face peeking over the wall",
    "A cat's sleepy face in the window",
    'A paper sun with a face',
    'Her face lit by a lantern',
    'Faces of children in a crowd',
    'A cartoon face with big eyes',
    'A friendly teacher at the desk',
    'Two hands holding a seedling',
  ])('flags %j', (prompt) => {
    expect(personIn(prompt)).toBeTruthy();
  });

  it.each([
    'A cube with one orange face',
    'A dodecahedron with twelve faces',
    "The cliff's north face in evening light",
    'Hand-painted tiles on a bench',
    'A tall tower on the left-hand side',
    'A man-made stone arch',
    'A steam engine with no face on the engine',
  ])('passes %j', (prompt) => {
    expect(personIn(prompt)).toBeUndefined();
  });

  it('reports the word through lintTheme', () => {
    const images = kintsugi.theme.images.map((i) => (i.id === 'spot-welcome' ? { ...i, prompt: i.prompt.replace(kintsugi.art.welcome.subject, 'A tea bowl with a smiling face') } : i));
    expect(lintTheme(variant({ images })).find((f) => f.rule === 'prompt-people')?.message).toContain('smiling face');
    const ok = kintsugi.theme.images.map((i) => (i.id === 'spot-welcome' ? { ...i, prompt: i.prompt.replace(kintsugi.art.welcome.subject, 'A small cube with one gold face') } : i));
    expect(lintTheme(variant({ images: ok })).filter((f) => f.rule === 'prompt-people')).toEqual([]);
  });
});

describe('palette-similar', () => {
  it('names the pair and tells you to change the later one', () => {
    const [a, b, c] = kintsugi.theme.palettes;
    const near = { ...c, id: 'near', primary: a.primary, secondary: a.secondary };
    const f = lintTheme(variant({ palettes: [a, b, near] })).filter((x) => x.rule === 'palette-similar');
    expect(f).toHaveLength(1);
    expect(f[0].message).toContain(`colourways ${a.id} and near`);
    expect(f[0].fix).toMatch(/^change near, not raku/);
  });
});

describe('accent', () => {
  // ACCENT_IN_LAYOUT is what the docs and the accent-hidden warning rely on: keep it true to the renderer.
  it.each(LAYOUTS.map((L) => [L.id] as const))('%s draws the accent exactly where ACCENT_IN_LAYOUT says', (layout) => {
    const sentinel = '#13579B';
    const p = { ...kintsugi.theme.palettes[0], accent: sentinel };
    const t = { ...kintsugi.theme, palettes: [p] };
    const drawn = KINDS.some((kind) => {
      const doc = createPage(t, kind, 'en', layout);
      doc.paletteId = p.id;
      return renderPage(doc, t, { assetBase: 'https://x.invalid/assets', courseId: '1' }).toLowerCase().includes(sentinel.toLowerCase());
    });
    expect(drawn).toBe(!!ACCENT_IN_LAYOUT[layout]);
  });

  it('warns when the default layout hides a signature accent', () => {
    expect(lintTheme(variant({ defaultLayout: 'editorial' })).map((f) => f.rule)).toContain('accent-hidden');
    expect(lintTheme(variant({ defaultLayout: 'classic-band' })).map((f) => f.rule)).not.toContain('accent-hidden');
  });
});

describe('theme:new scaffold palettes', () => {
  it.each(['#1F3A5F', '#4A2545', '#24483A', '#7A1F1F'])('%s passes every colour rule with a light secondary', (hex) => {
    const p = seedPalette(hex, 'x', 'X');
    expect(paletteProblems(p)).toEqual([]);
    // spec.md: secondary is a light companion with a dark onSecondary of its own hue.
    expect(p.onSecondary.toLowerCase()).not.toBe('#ffffff');
    expect(lintTheme(variant({ palettes: [p, ...kintsugi.theme.palettes.slice(1)] })).filter((f) => f.severity === 'error')).toEqual([]);
  });
});

describe('image-asset', () => {
  it('finds every theme image and its @2x twin under public/design-assets/themes', () => {
    expect(existsSync(path.join(DESIGN_ASSET_DIR, 'kintsugi', 'banner@2x.webp'))).toBe(true);
    expect(lintTheme(kintsugi).filter((f) => f.rule === 'image-asset')).toEqual([]);
  });
  it('reports missing art, and can be skipped while drafting', () => {
    const draft = variant({ id: 'not-generated-yet' });
    const f = lintTheme(draft).find((x) => x.rule === 'image-asset');
    expect(f?.message).toContain('banner.webp, banner@2x.webp');
    expect(lintTheme(draft, { assetDir: false }).filter((x) => x.rule === 'image-asset')).toEqual([]);
  });
});

describe('palette-dark', () => {
  it('rejects hand-tuned dark colours that are not hex', () => {
    const [a, b, c] = kintsugi.theme.palettes;
    const bad = { ...a, dark: { primary: 'red' } };
    expect(lintTheme(variant({ palettes: [bad, b, c] })).map((x) => x.rule)).toContain('palette-dark-hex');
    const ok = { ...a, dark: { primary: '#e8b04a' } };
    expect(lintTheme(variant({ palettes: [ok, b, c] })).filter((x) => x.rule.startsWith('palette-dark'))).toEqual([]);
  });
});
