/**
 * The whole catalogue in Paper: every theme × colourway has a passing dark twin that looks like it, every
 * theme × layout × kind renders the same structure in light and dark (so the page gets its dark mode), and
 * every theme gives a valid course look.
 */
import { existsSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import {
  checkPalette,
  courseLook,
  createPage,
  hexToHsl,
  LAYOUTS,
  luminance,
  renderDesignDetail,
  themeDarkPalette,
  type PageKind,
} from '@/designer/engine';
import { paperViolations } from '@/designer/engine/__tests__/canvas-rules';
import { THEMES } from '../index';
import { DESIGN_ASSET_DIR } from '../lint';

const KINDS: PageKind[] = ['home', 'start-here', 'overview', 'module', 'instructor'];
const opts = { assetBase: '/design-assets', iconBase: '/api/v1/design/icons', courseId: '7' };
const hueDistance = (a: number, b: number) => Math.min(Math.abs(a - b), 360 - Math.abs(a - b));
const HEX = /^#[0-9a-f]{6}$/;

describe('dark colourways', () => {
  it.each(THEMES.map(t => [t.id, t] as const))('%s: every colourway has a dark twin that passes and keeps its hues', (_id, theme) => {
    for (const p of theme.palettes) {
      const d = themeDarkPalette(p);
      expect(checkPalette(d).filter(r => !r.pass).map(r => `${p.id} ${r.id} ${r.ratio.toFixed(2)}`)).toEqual([]);
      expect(luminance(d.background), p.id).toBeLessThan(0.03);
      for (const k of ['primary', 'secondary', 'accent', 'link'] as const) {
        const [h0, s0] = hexToHsl(p[k]);
        const [h1, s1] = hexToHsl(d[k]);
        if (s0 >= 0.2 && s1 >= 0.2) expect(hueDistance(h0, h1), `${p.id}.${k} ${p[k]} → ${d[k]}`).toBeLessThanOrEqual(12);
      }
      // the sheet is tinted toward the primary's hue (greys stay grey)
      const [hp, sp] = hexToHsl(p.primary);
      const [hb, sb] = hexToHsl(d.background);
      if (sp >= 0.2 && sb >= 0.05) expect(hueDistance(hp, hb), `${p.id} background`).toBeLessThanOrEqual(15);
    }
  });
});

describe('light and dark renders never diverge', () => {
  // 122 themes × 7 layouts × 5 kinds; one test per theme keeps the report readable.
  it.each(THEMES.map(t => [t.id, t] as const))('%s: all layouts × kinds', (_id, theme) => {
    for (const L of LAYOUTS)
      for (const kind of KINDS) {
        const doc = createPage(theme, kind, 'en', L.id);
        const d = renderDesignDetail(doc, theme, opts);
        expect(d.diverged, `${L.id}/${kind}`).toBe(false);
        expect(Object.keys(d.vars).length, `${L.id}/${kind}`).toBeGreaterThan(0);
        expect(paperViolations(d.html), `${L.id}/${kind}`).toEqual([]);
      }
  });
});

describe('courseLook', () => {
  it.each(THEMES.map(t => [t.id, t] as const))('%s: validation-ready', (_id, theme) => {
    for (const p of theme.palettes) {
      const look = courseLook(theme, p.id);
      expect(look.version).toBe(1);
      expect(look.theme_id).toMatch(/^[a-z0-9-]{1,64}$/);
      expect(look.palette_id).toBe(p.id);
      for (const colors of [look.colors, look.dark_colors]) {
        expect(Object.keys(colors).sort()).toEqual(['accent', 'background', 'link', 'on_primary', 'on_secondary', 'on_surface', 'primary', 'secondary', 'surface', 'text']);
        for (const v of Object.values(colors)) expect(v).toMatch(HEX);
      }
      expect(look.colors.primary).toBe(p.primary.toLowerCase());
      expect(look.banner).toBeDefined();
      expect(look.banner!.src).toBe(`/design-assets/themes/${theme.id}/banner.webp`);
      expect(look.banner!.src_2x).toBe(`/design-assets/themes/${theme.id}/banner@2x.webp`);
      for (const src of [look.banner!.src, look.banner!.src_2x]) expect(existsSync(path.join(DESIGN_ASSET_DIR, src.replace('/design-assets/themes/', '')))).toBe(true);
      expect(JSON.stringify(look).length).toBeLessThanOrEqual(16 * 1024);
    }
  });

  it('uses a teacher’s own colours when given, and falls back to the first colourway for an unknown id', () => {
    const theme = THEMES[0];
    const custom = { ...theme.palettes[1], id: 'mine', name: 'Mine', primary: '#123456' };
    const look = courseLook(theme, 'mine', custom);
    expect(look.palette_id).toBe('mine');
    expect(look.colors.primary).toBe('#123456');
    expect(courseLook(theme, 'nope').palette_id).toBe(theme.palettes[0].id);
  });
});
