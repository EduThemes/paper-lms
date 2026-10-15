// Guards the colour tokens in index.css: every text token and every
// "on a fill" token must meet WCAG 2.1 AA (4.5:1) against the surfaces and
// fills it is used on, in both the light and the dark theme.
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

const css = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), '../../index.css'), 'utf8');

function tokens(selector) {
  const start = css.indexOf(`${selector} {`);
  const body = css.slice(start, css.indexOf('}', start));
  const out = {};
  for (const m of body.matchAll(/--color-([a-z0-9-]+):\s*(\d+)\s+(\d+)\s+(\d+)\s*;/g)) {
    out[m[1]] = [Number(m[2]), Number(m[3]), Number(m[4])];
  }
  return out;
}

const lin = (c) => {
  const s = c / 255;
  return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
};
const lum = ([r, g, b]) => 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
const ratio = (a, b) => {
  const [x, y] = [lum(a), lum(b)].sort((p, q) => q - p);
  return (x + 0.05) / (y + 0.05);
};
// a tint (e.g. bg-accent-success/20) composited over a surface
const over = (fg, bg, alpha) => fg.map((c, i) => Math.round(c * alpha + bg[i] * (1 - alpha)));

const THEMES = { light: tokens(':root'), dark: tokens('.dark') };
const SURFACES = ['surface-0', 'surface-1', 'surface-2', 'surface-raised'];

describe.each(Object.entries(THEMES))('%s theme tokens', (name, t) => {
  it('parses the palette', () => {
    expect(t['surface-0']).toBeDefined();
    expect(t['on-brand']).toBeDefined();
  });

  it.each(['text-primary', 'text-secondary', 'text-tertiary'])('%s is AA on every surface', (text) => {
    for (const s of SURFACES) expect(ratio(t[text], t[s])).toBeGreaterThanOrEqual(4.5);
  });

  it('text-tertiary and brand-700 are AA on the brand-50 highlight', () => {
    expect(ratio(t['text-tertiary'], t['brand-50'])).toBeGreaterThanOrEqual(4.5);
    expect(ratio(t['brand-700'], t['brand-50'])).toBeGreaterThanOrEqual(4.5);
  });

  it.each(['brand-500', 'brand-600', 'brand-700'])('on-brand text is AA on bg-%s', (fill) => {
    expect(ratio(t['on-brand'], t[fill])).toBeGreaterThanOrEqual(4.5);
  });

  it('brand-600 as link/text colour is AA on the page surfaces', () => {
    for (const s of ['surface-0', 'surface-1', 'surface-raised']) expect(ratio(t['brand-600'], t[s])).toBeGreaterThanOrEqual(4.5);
  });

  it.each(['accent-success', 'accent-warning', 'accent-danger', 'accent-info'])('%s: on-accent fill text, plain text and /10 /20 tint badges are AA', (accent) => {
    expect(ratio(t['on-accent'], t[accent])).toBeGreaterThanOrEqual(4.5);
    expect(ratio(t[accent], t['surface-0'])).toBeGreaterThanOrEqual(4.5);
    for (const a of [0.1, 0.2]) expect(ratio(t[accent], over(t[accent], t['surface-0'], a))).toBeGreaterThanOrEqual(4.5);
  });
});
