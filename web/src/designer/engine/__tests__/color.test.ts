import { describe, expect, it } from 'vitest';
import { contrastRatio, ensureContrast, mix, normHex, readable } from '../index';

describe('contrastRatio (WCAG 2.x)', () => {
  const known: [string, string, number][] = [
    ['#000000', '#ffffff', 21],
    ['#ffffff', '#ffffff', 1],
    ['#767676', '#ffffff', 4.54], // the classic "lightest AA grey on white"
    ['#777777', '#ffffff', 4.48], // …and the one that just fails
    ['#0000ff', '#ffffff', 8.59],
    ['#ff0000', '#ffffff', 4.0],
    ['#008000', '#ffffff', 5.14],
    ['#0374b5', '#ffffff', 5.0], // Canvas brand blue
  ];
  for (const [fg, bg, r] of known) it(`${fg} on ${bg} ≈ ${r}`, () => expect(contrastRatio(fg, bg)).toBeCloseTo(r, 1));
  it('is symmetric and accepts shorthand/uppercase', () => {
    expect(contrastRatio('#FFF', '000')).toBeCloseTo(21, 5);
    expect(contrastRatio('#123456', '#abcdef')).toBeCloseTo(contrastRatio('#abcdef', '#123456'), 10);
  });
});

describe('helpers', () => {
  it('normHex', () => {
    expect(normHex('#ABC')).toBe('#aabbcc');
    expect(normHex('nope')).toBeNull();
  });
  it('mix', () => expect(mix('#000000', '#ffffff', 0.5)).toBe('#808080'));
  it('readable picks the first passing candidate', () => {
    expect(readable('#ffffff', ['#eeeeee', '#333333'])).toBe('#333333');
  });
  it('ensureContrast reaches the target', () => {
    const c = ensureContrast('#88aaff', '#ffffff', 4.5);
    expect(contrastRatio(c, '#ffffff')).toBeGreaterThanOrEqual(4.5);
  });
  it('ensureContrast works on mid-luminance backgrounds (needs black, not white)', () => {
    // #42ade3 has luminance ≈0.37: white only reaches 2.5:1, so the text must go darker.
    const c = ensureContrast('#3a6f8c', '#42ade3', 4.6);
    expect(contrastRatio(c, '#42ade3')).toBeGreaterThanOrEqual(4.6);
  });
});
