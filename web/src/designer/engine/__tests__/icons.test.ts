import { describe, expect, it } from 'vitest';
import { hasIcon, iconUrl, renderIconSvg, themeImageUrl } from '../index';
import type { IconStyle } from '../types';

describe('renderIconSvg', () => {
  const styles: IconStyle[] = ['solid-circle', 'soft-square', 'outline', 'duotone'];
  for (const style of styles)
    it(`${style} is a standalone SVG with the requested colours`, () => {
      const svg = renderIconSvg('book-open', { style, fg: '#ffffff', bg: '12355b', size: 64 });
      expect(svg).toMatch(/^<svg xmlns="http:\/\/www\.w3\.org\/2000\/svg" width="64" height="64" viewBox="0 0 64 64">/);
      expect(svg).toContain('stroke="#ffffff"');
      expect(svg).toContain('#12355b');
      expect(svg).toContain('<path d="M12 5v16"/>');
      expect(svg.endsWith('</svg>')).toBe(true);
    });
  it('resolves Lucide aliases (smile → face-slightly-smiling)', () => {
    expect(hasIcon('smile')).toBe(true);
    expect(renderIconSvg('smile', { style: 'outline', fg: '#000000', bg: '#ffffff' })).toBe(
      renderIconSvg('face-slightly-smiling', { style: 'outline', fg: '#000000', bg: '#ffffff' }),
    );
  });
  it('unknown icons fall back to a neutral dashed circle', () => {
    expect(hasIcon('definitely-not-an-icon')).toBe(false);
    const svg = renderIconSvg('definitely-not-an-icon', { style: 'solid-circle', fg: '#000000', bg: '#ffffff' });
    expect(svg).toBe(renderIconSvg('circle-dashed', { style: 'solid-circle', fg: '#000000', bg: '#ffffff' }));
  });
  it('rejects bad colours safely', () => {
    const svg = renderIconSvg('flag', { style: 'solid-circle', fg: '"><script>', bg: 'red' });
    expect(svg).not.toContain('<script');
  });
});

describe('urls', () => {
  it('iconUrl matches the contract', () => {
    expect(iconUrl('https://a.org/assets/', 'layers', { style: 'duotone', fg: '#FFFFFF', bg: '#12355b' })).toBe(
      'https://a.org/assets/icons/duotone/layers.svg?fg=ffffff&bg=12355b',
    );
  });
  it('themeImageUrl matches the contract', () => {
    expect(themeImageUrl('https://a.org/assets', 'campus', { id: 'banner', kind: 'banner', file: 'banner.webp', width: 1200, height: 300, alt: '', prompt: '' })).toBe(
      'https://a.org/assets/themes/campus/banner.webp',
    );
  });
});
