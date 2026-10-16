import { describe, expect, it } from 'vitest';
import {
  AA,
  checkAccessibility,
  checkPalette,
  contrastRatio,
  createPage,
  derivePalette,
  fixPalette,
  hexToHsl,
  hslToHex,
  isNearWhite,
  LAYOUTS,
  luminance,
  PALETTE_PAIRS,
  paletteFailures,
  palettePasses,
  renderPage,
  sanitizePalette,
} from '../index';
import type { Palette } from '../types';
import { canvasViolations } from './canvas-rules';
import { THEMES } from './fixtures/themes';
import { everySection } from './helpers';

/** Seeded PRNG (mulberry32) so property-style runs are reproducible. */
function rng(seed: number) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
const randomHex = (r: () => number) => `#${Math.floor(r() * 0x1000000).toString(16).padStart(6, '0')}`;
const hueOf = (c: string) => hexToHsl(c)[0];
const hueDistance = (a: number, b: number) => Math.min(Math.abs(a - b), 360 - Math.abs(a - b));

const bad: Palette = {
  id: 'mine',
  name: 'Too pale',
  primary: '#9fc6d0',
  onPrimary: '#ffffff',
  secondary: '#f2d04b',
  onSecondary: '#ffffff',
  accent: '#ff00ff',
  surface: '#f3f6f8',
  onSurface: '#a0a0a0',
  background: '#ffffff',
  text: '#999999',
  link: '#7fb3ff',
};

describe('HSL helpers', () => {
  it('round-trips hex → HSL → hex', () => {
    const r = rng(1);
    for (let i = 0; i < 300; i++) {
      const c = randomHex(r);
      const [h, s, l] = hexToHsl(c);
      const back = hslToHex(h, s, l);
      // ±1 per channel from rounding
      const d = [1, 3, 5].map(j => Math.abs(parseInt(c.slice(j, j + 2), 16) - parseInt(back.slice(j, j + 2), 16)));
      expect(Math.max(...d)).toBeLessThanOrEqual(1);
    }
  });
  it('knows the primaries', () => {
    expect(hexToHsl('#ff0000')).toEqual([0, 1, 0.5]);
    expect(hslToHex(120, 1, 0.25)).toBe('#008000');
    expect(hslToHex(0, 0, 1)).toBe('#ffffff');
  });
});

describe('checkPalette', () => {
  it('reports every required pair with its ratio', () => {
    const res = checkPalette(bad);
    expect(res.map(r => r.id)).toEqual(PALETTE_PAIRS.map(p => p.id));
    const text = res.find(r => r.id === 'text-background')!;
    expect(text.ratio).toBeCloseTo(contrastRatio('#999999', '#ffffff'), 6);
    expect(text.pass).toBe(false);
    expect(paletteFailures(bad).length).toBeGreaterThan(3);
    expect(palettePasses(bad)).toBe(false);
  });
  it('built-in fixture palettes pass', () => {
    for (const th of THEMES) for (const p of th.palettes) expect(paletteFailures(p)).toEqual([]);
  });
  it('accent is never checked (decorative only)', () => {
    expect(PALETTE_PAIRS.some(p => p.fg === 'accent' || p.bg === 'accent')).toBe(false);
  });
});

describe('fixPalette', () => {
  it('fixes every failing pair, keeps brand colours and hues', () => {
    const fixed = fixPalette(bad);
    expect(paletteFailures(fixed)).toEqual([]);
    expect(palettePasses(fixed)).toBe(true);
    expect(fixed.primary).toBe(bad.primary);
    expect(fixed.secondary).toBe(bad.secondary);
    expect(fixed.accent).toBe(bad.accent);
    expect(fixed.id).toBe('mine');
    expect(fixed.name).toBe('Too pale');
    // link stays blue
    expect(hueDistance(hueOf(fixed.link), hueOf(bad.link))).toBeLessThan(6);
  });
  it('leaves passing colours alone', () => {
    const ok = THEMES[0].palettes[0];
    expect(fixPalette(ok)).toEqual(ok);
  });
  it('lightens a dark background to near-white and a dark panel until text fits', () => {
    const dark = { ...bad, background: '#202040', surface: '#303030' };
    const fixed = fixPalette(dark);
    expect(isNearWhite(fixed.background)).toBe(true);
    expect(hueDistance(hueOf(fixed.background), hueOf('#202040'))).toBeLessThan(6);
    expect(paletteFailures(fixed)).toEqual([]);
  });
  it('replaces invalid colours', () => {
    const fixed = fixPalette({ ...bad, text: 'nope', link: '' });
    expect(paletteFailures(fixed)).toEqual([]);
  });
  it('property: any random palette is fixed to AA', () => {
    const r = rng(42);
    for (let i = 0; i < 400; i++) {
      const p: Palette = {
        id: 'r',
        name: 'Random',
        primary: randomHex(r),
        onPrimary: randomHex(r),
        secondary: randomHex(r),
        onSecondary: randomHex(r),
        accent: randomHex(r),
        surface: randomHex(r),
        onSurface: randomHex(r),
        background: randomHex(r),
        text: randomHex(r),
        link: randomHex(r),
      };
      const fixed = fixPalette(p);
      expect(paletteFailures(fixed), JSON.stringify(p)).toEqual([]);
      expect(isNearWhite(fixed.background), JSON.stringify(p)).toBe(true);
      expect(fixed.primary).toBe(p.primary);
    }
  }, 60_000); // 400 random palettes; slow CPUs need more than the 5 s default
});

describe('derivePalette', () => {
  it('builds a complete palette from one colour', () => {
    const p = derivePalette({ primary: '#ffcc00' }, 'Go Gold');
    expect(p.name).toBe('Go Gold');
    expect(p.primary).toBe('#ffcc00');
    expect(p.background).toBe('#ffffff');
    expect(palettePasses(p)).toBe(true);
    // gold is light: dark text on it, and a darkened gold for links
    expect(luminance(p.onPrimary)).toBeLessThan(0.1);
    expect(hueDistance(hueOf(p.link), hueOf('#ffcc00'))).toBeLessThan(8);
    expect(contrastRatio(p.link, '#ffffff')).toBeGreaterThanOrEqual(AA);
  });
  it('keeps the brand colours it is given', () => {
    const p = derivePalette({ primary: '#003366', secondary: '#cc0000', accent: '#ffd700' }, 'School');
    expect([p.primary, p.secondary, p.accent]).toEqual(['#003366', '#cc0000', '#ffd700']);
    expect(p.onPrimary).toBe('#ffffff');
    expect(p.link).toBe('#003366'); // already readable: used as is
  });
  it('accepts shorthand hex and falls back for a missing name', () => {
    const p = derivePalette({ primary: '#0a0' }, '   ');
    expect(p.primary).toBe('#00aa00');
    expect(p.name).toBe('My colours');
  });
  it('property: every derived palette passes AA (1, 2 and 3 brand colours)', () => {
    const r = rng(7);
    for (let i = 0; i < 600; i++) {
      const n = (i % 3) + 1;
      const base = { primary: randomHex(r), ...(n > 1 ? { secondary: randomHex(r) } : {}), ...(n > 2 ? { accent: randomHex(r) } : {}) };
      const p = derivePalette(base, `Random ${i}`);
      expect(checkPalette(p).filter(x => !x.pass), JSON.stringify(base)).toEqual([]);
      expect(isNearWhite(p.background)).toBe(true);
      expect(sanitizePalette(p)).toEqual(p);
      expect(p.primary).toBe(base.primary);
    }
  });
  it('property: derived palettes render without Canvas violations or contrast errors', () => {
    const r = rng(99);
    const theme = THEMES[0];
    for (let i = 0; i < 25; i++) {
      const palette = derivePalette({ primary: randomHex(r) }, 'R');
      const L = LAYOUTS[i % LAYOUTS.length];
      const doc = { ...everySection(theme, L.id), palette };
      const html = renderPage(doc, theme, { assetBase: 'https://a.test/assets', courseId: '1' });
      expect(canvasViolations(html)).toEqual([]);
      const issues = checkAccessibility(doc, theme, { courseId: '1' }).filter(x => x.severity !== 'suggestion');
      expect(issues, `${palette.primary} / ${L.id}`).toEqual([]);
    }
  });
});

describe('sanitizePalette', () => {
  const good = derivePalette({ primary: '#123456' }, 'Ok', 'scheme-1');
  it('accepts a valid palette and normalises hex', () => {
    expect(sanitizePalette({ ...good, primary: '#ABC' })?.primary).toBe('#aabbcc');
  });
  it('rejects anything that is not strictly hex, a bad id or a bad name', () => {
    expect(sanitizePalette({ ...good, primary: 'red' })).toBeNull();
    expect(sanitizePalette({ ...good, primary: '123456' })).toBeNull();
    expect(sanitizePalette({ ...good, text: '#123456;background:url(x)' })).toBeNull();
    expect(sanitizePalette({ ...good, link: undefined })).toBeNull();
    expect(sanitizePalette({ ...good, id: '../x' })).toBeNull();
    expect(sanitizePalette({ ...good, name: '' })).toBeNull();
    expect(sanitizePalette({ ...good, name: 'x'.repeat(61) })).toBeNull();
    expect(sanitizePalette(null)).toBeNull();
    expect(sanitizePalette([])).toBeNull();
  });
});

describe('render with a custom palette', () => {
  const theme = THEMES[0];
  const opts = { assetBase: 'https://rmt.example.org/assets', courseId: '4821' };

  it('a doc without a palette renders byte-identically to before (palette key absent or undefined)', () => {
    const doc = createPage(theme, 'home', 'en', 'classic-band');
    expect('palette' in doc).toBe(false);
    expect(renderPage({ ...doc, palette: undefined }, theme, opts)).toBe(renderPage(doc, theme, opts));
  });
  it('uses the custom colours instead of the theme palette', () => {
    const doc = createPage(theme, 'home', 'en', 'classic-band');
    const palette = derivePalette({ primary: '#7a1fa2', secondary: '#0b6e4f' }, 'Purple');
    const html = renderPage({ ...doc, palette }, theme, opts);
    expect(html).toContain('#7a1fa2');
    expect(html).not.toContain(theme.palettes[0].primary);
  });
  it('an invalid custom palette falls back to the theme palette and never reaches a style', () => {
    const doc = createPage(theme, 'home', 'en', 'classic-band');
    const evil = { ...derivePalette({ primary: '#7a1fa2' }, 'x'), primary: 'red;background:url(https://evil.test/x)' };
    const html = renderPage({ ...doc, palette: evil as Palette }, theme, opts);
    expect(html).not.toContain('evil.test');
    const noHash = (h: string) => h.replace(/data-pd-hash="[^"]+"/, '');
    expect(noHash(html)).toBe(noHash(renderPage(doc, theme, opts)));
  });
});

describe('a11y: custom palette contrast', () => {
  const theme = THEMES[0];
  const doc = () => createPage(theme, 'home', 'en', 'classic-band');

  it('flags a failing custom palette as an error with a one-click fix that makes it pass', () => {
    const issues = checkAccessibility({ ...doc(), palette: bad }, theme, { courseId: '1' });
    const issue = issues.find(i => i.id === 'palette-contrast');
    expect(issue?.severity).toBe('error');
    expect(issue?.sectionId).toBeUndefined();
    expect(issue?.message).toMatch(/page text \(2\.85:1\)/);
    const fixed = issue!.fix!.docPatch!.palette!;
    expect(palettePasses(fixed)).toBe(true);
    const after = checkAccessibility({ ...doc(), palette: fixed }, theme, { courseId: '1' });
    expect(after.filter(i => i.id.startsWith('palette'))).toEqual([]);
  });
  it('warns about a dark background', () => {
    const p = { ...derivePalette({ primary: '#003366' }, 'x'), background: '#333333', text: '#ffffff', link: '#ffffff' };
    const issues = checkAccessibility({ ...doc(), palette: p }, theme, { courseId: '1' });
    expect(issues.find(i => i.id === 'palette-background')?.severity).toBe('warning');
  });
  it('offers to go back to the theme colours when the palette is invalid', () => {
    const issues = checkAccessibility({ ...doc(), palette: { id: 'x' } as unknown as Palette }, theme, { courseId: '1' });
    expect(issues.find(i => i.id === 'palette-invalid')?.fix?.docPatch).toEqual({ palette: null });
  });
  it('says nothing for docs without a custom palette', () => {
    expect(checkAccessibility(doc(), theme, { courseId: '1' }).some(i => i.id.startsWith('palette'))).toBe(false);
  });
});
