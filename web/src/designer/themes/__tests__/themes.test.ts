import { describe, expect, it } from 'vitest';
import { THEMES, THEME_ENTRIES, getTheme, previewHtml, themesFor } from '../index';
import type { Palette } from '@/designer/engine';
import { formatFinding, lintCatalogue, lintTheme, paletteProblems, suggestPalette, type LintFinding } from '../lint';

// The rules live in src/lint.ts, shared with `pnpm theme:check`. A failure lists every error with its fix.
const errors = (findings: LintFinding[]) => findings.filter((f) => f.severity === 'error').map(formatFinding);

describe('catalogue', () => {
  it('passes lintCatalogue (unique ids and names, tier coverage, no Instructure names)', () => {
    expect(errors(lintCatalogue(THEME_ENTRIES))).toEqual([]);
  });

  it('getTheme and themesFor work', () => {
    expect(getTheme('campus-modern')?.name).toBe('Campus Modern');
    expect(getTheme('nope')).toBeUndefined();
    const k2 = themesFor('early-elementary');
    expect(k2.every((t) => t.audiences.includes('early-elementary'))).toBe(true);
    expect(k2[0].audiences[0]).toBe('early-elementary');
  });

  it('previewHtml lists every palette', () => {
    const html = previewHtml(THEME_ENTRIES);
    for (const t of THEMES) for (const p of t.palettes) expect(html).toContain(p.name.replace(/&/g, '&amp;'));
  });
});

describe('themes', () => {
  it.each(THEME_ENTRIES.map((e) => [e.theme.id, e] as const))('%s passes lintTheme', (_id, entry) => {
    expect(errors(lintTheme(entry))).toEqual([]);
  });
});

describe('suggestPalette', () => {
  // Deliberately bad: mid-tone primary, grey page, dark panel, light text, pale link.
  const bad: Palette[] = [
    { id: 'a', name: 'A', primary: '#5F9A84', onPrimary: '#FFFFFF', secondary: '#F9B233', onSecondary: '#FFFFFF', accent: '#D6336C', surface: '#B0B8C0', onSurface: '#666666', background: '#D0D0D0', text: '#777777', link: '#C9A23A' },
    { id: 'b', name: 'B', primary: '#FFD400', onPrimary: '#FFFFFF', secondary: '#00A0FF', onSecondary: '#00A0FF', accent: '#00FF00', surface: '#333333', onSurface: '#444444', background: '#222222', text: '#FFFFFF', link: '#FFFF00' },
  ];
  it.each(bad.map((p) => [p.id, p] as const))('fixes %s so every theme colour rule passes', (_id, p) => {
    expect(paletteProblems(p).length).toBeGreaterThan(0);
    const s = suggestPalette(p);
    expect(s).not.toBeNull();
    expect(paletteProblems(s!).map((x) => x.message)).toEqual([]);
  });
});
