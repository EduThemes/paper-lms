import { describe, expect, it } from 'vitest';
import { audienceForUiMode, comingUp, filterThemes, matchesQuery, seasonalPatch, seasonalThemeOf, seasonalThemes, switchBackPatch } from '../lib/catalog';
import { detectRegion } from '../lib/region';

/** Made-up themes only. */
function theme(id, o = {}) {
  return {
    id,
    name: o.name ?? id,
    description: `${id} description`,
    audiences: o.audiences ?? ['secondary'],
    tags: o.tags ?? [],
    defaultLayout: 'classic-band',
    palettes: [],
    iconStyle: 'outline',
    corners: 'soft',
    images: [{ id: 'banner', kind: 'banner', file: 'banner.webp', width: 1600, height: 500, alt: `${id} banner`, prompt: '' }],
    collection: o.collection,
    observance: o.observance,
  };
}
const obs = (name, months, traditions = []) => ({ name, months, traditions, note: `${name} note` });

const CATALOG = [
  theme('meadow', { audiences: ['early-elementary'], tags: ['nature'] }),
  theme('lab', { audiences: ['secondary', 'higher-ed'], tags: ['science', 'space'] }),
  theme('pixel', { collection: 'arcade', audiences: ['upper-elementary', 'secondary'], tags: ['pixel', 'games'] }),
  theme('lamps', { collection: 'holidays', name: 'Festival of Lights', observance: obs('Diwali', [10, 11], ['Hindu', 'Sikh', 'Jain']) }),
  theme('lamps-kids', { collection: 'holidays', audiences: ['early-elementary'], observance: obs('Diwali', [10, 11], ['Hindu']) }),
  theme('muertos', { collection: 'holidays', observance: obs('Día de Muertos', [11]) }),
  theme('newyear', { collection: 'holidays', observance: obs('Lunar New Year', [1, 2]) }),
  theme('book', { collection: 'special-days', observance: obs('Book Week', [10]) }),
];

describe('shelves, audience and search', () => {
  it('filters by shelf; unset collection means Essentials', () => {
    expect(filterThemes(CATALOG, { shelf: 'essentials', audience: 'all', query: '' }).map((t) => t.id)).toEqual(['meadow', 'lab']);
    expect(filterThemes(CATALOG, { shelf: 'all', audience: 'all', query: '' })).toHaveLength(CATALOG.length);
  });

  it('applies the audience within a shelf', () => {
    expect(filterThemes(CATALOG, { shelf: 'holidays', audience: 'early-elementary', query: '' }).map((t) => t.id)).toEqual(['lamps-kids']);
  });

  it('searches name, tags, description and observance, ignoring case and accents', () => {
    const q = (query) => filterThemes(CATALOG, { shelf: 'all', audience: 'all', query }).map((t) => t.id);
    expect(q('SPACE')).toEqual(['lab']);
    expect(q('diwali')).toEqual(['lamps', 'lamps-kids']);
    expect(q('dia de muertos')).toEqual(['muertos']);
    expect(q('pix')).toEqual(['pixel']);
    expect(q('ixel')).toEqual([]);
    expect(matchesQuery(CATALOG[0], '   ')).toBe(true);
  });
});

describe('comingUp', () => {
  it('lists this month first, then next month, one entry per observance', () => {
    const list = comingUp(CATALOG, 10);
    expect(list.map((c) => [c.name, c.thisMonth])).toEqual([
      ['Book Week', true],
      ['Diwali', true],
      ['Día de Muertos', false],
    ]);
    expect(list.find((c) => c.name === 'Diwali').themes.map((t) => t.id)).toEqual(['lamps', 'lamps-kids']);
  });

  it('wraps December to January and can be limited to shelves', () => {
    expect(comingUp(CATALOG, 12).map((c) => c.name)).toEqual(['Lunar New Year']);
    expect(comingUp(CATALOG, 10, ['special-days']).map((c) => c.name)).toEqual(['Book Week']);
  });

  it("follows one country's calendar where an observance moves", () => {
    const heritage = theme('heritage', { collection: 'special-days', observance: { name: 'Heritage Month', traditions: [], months: [2, 10], regionalMonths: { US: [2], GB: [10] } } });
    expect(comingUp([heritage], 10, undefined, 'US')).toEqual([]);
    expect(comingUp([heritage], 10, undefined, 'GB')).toHaveLength(1);
  });
});

describe('detectRegion and K-12 defaults', () => {
  it('reads the country from the language, then the browser, else the US', () => {
    expect(detectRegion('en-GB', ['en-US'])).toBe('GB');
    expect(detectRegion('es', ['en-AU'])).toBe('AU');
    expect(detectRegion('en-UK', [])).toBe('GB');
    expect(detectRegion('fr-FR', ['de'])).toBe('US');
  });

  it('defaults the audience filter from the course grade band', () => {
    expect(audienceForUiMode('k2')).toBe('early-elementary');
    expect(audienceForUiMode('3-5')).toBe('upper-elementary');
    expect(audienceForUiMode('standard')).toBe('all');
  });
});

describe('seasonal banner', () => {
  const url = (t, img) => `/design-assets/themes/${t.id}/${img.file}`;
  it('swaps only the banner and reads the seasonal theme back from the URL', () => {
    expect(seasonalThemes(CATALOG).map((t) => t.id)).toEqual(['lamps', 'lamps-kids', 'muertos', 'newyear', 'book']);
    const patch = seasonalPatch(CATALOG[3], url);
    expect(patch).toEqual({ imageId: undefined, customSrc: '/design-assets/themes/lamps/banner.webp', alt: 'lamps banner' });
    expect(seasonalThemeOf({ id: 'b', type: 'banner', alt: '', ...patch }, CATALOG).id).toBe('lamps');
    expect(switchBackPatch(CATALOG[1])).toEqual({ imageId: 'banner', customSrc: undefined, alt: 'lab banner' });
  });
});
