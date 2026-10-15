/**
 * Gallery logic: shelves (collections), audience filter, search, "Coming up" observances and seasonal
 * banners. Pure functions over theme lists, so they are easy to test.
 */

export const collectionOf = (t) => t.collection ?? 'essentials';
export const SEASONAL_COLLECTIONS = ['holidays', 'special-days'];

/** Lower-case words, accents removed: "Día de Muertos" → ["dia", "de", "muertos"]. */
function words(s) {
  return String(s)
    .toLowerCase()
    .normalize('NFKD')
    .replace(/[̀-ͯ]/g, '')
    .split(/[^a-z0-9]+/)
    .filter(Boolean);
}

function haystack(t) {
  return words([t.name, t.description, ...(t.tags ?? []), t.observance?.name ?? '', ...(t.observance?.traditions ?? [])].join(' ')).join(' ');
}

/** Every word of the query must appear (as a word prefix) in the theme's name, description, tags or observance. */
export function matchesQuery(t, query) {
  const q = words(query);
  if (!q.length) return true;
  const hay = ` ${haystack(t)}`;
  return q.every((w) => hay.includes(` ${w}`));
}

export function filterThemes(themes, { shelf, audience, query }) {
  return themes.filter(
    (t) =>
      (shelf === 'all' || collectionOf(t) === shelf) &&
      (audience === 'all' || t.audiences.includes(audience)) &&
      matchesQuery(t, query),
  );
}

/** The months an observance falls in for `region`, or its general months when the region isn't listed. */
export const monthsFor = (o, region) => (region && o.regionalMonths?.[region]) || o.months;

/**
 * Observances in `month` (1-12) or the month after, one entry per observance (several themes can share
 * one): this month first, then next month, then alphabetical. Optionally limited to shelves, and using one
 * country's calendar where observances differ by country.
 */
export function comingUp(themes, month, shelves, region) {
  const next = (month % 12) + 1;
  const byName = new Map();
  for (const t of themes) {
    const o = t.observance;
    if (!o || (shelves && !shelves.includes(collectionOf(t)))) continue;
    const months = monthsFor(o, region);
    const now = months.includes(month);
    if (!now && !months.includes(next)) continue;
    const key = o.name.trim().toLowerCase();
    const hit = byName.get(key);
    if (hit) {
      hit.themes.push(t);
      hit.thisMonth ||= now;
    } else {
      byName.set(key, { name: o.name, note: o.note, traditions: o.traditions, months, thisMonth: now, themes: [t] });
    }
  }
  return [...byName.values()].sort((a, b) => Number(b.thisMonth) - Number(a.thisMonth) || a.name.localeCompare(b.name));
}

// ---- seasonal banners ---------------------------------------------------------------------------

export const bannerOf = (t) => t.images.find((i) => i.id === 'banner') ?? t.images.find((i) => i.kind === 'banner');

/** Themes whose banner can be borrowed: holidays and special days. */
export function seasonalThemes(themes) {
  return themes.filter((t) => SEASONAL_COLLECTIONS.includes(collectionOf(t)) && !!bannerOf(t));
}

/** The patch that swaps a banner section to another theme's banner (theme and colours stay). */
export function seasonalPatch(theme, imageUrl) {
  const img = bannerOf(theme);
  return { imageId: undefined, customSrc: imageUrl(theme, img), alt: img.alt };
}

/**
 * Which seasonal theme a banner is borrowing, read back from its customSrc (".../themes/<themeId>/<file>"),
 * so it survives reloads without extra fields.
 */
export function seasonalThemeOf(section, themes) {
  const m = section.customSrc && /\/themes\/([a-z0-9-]+)\/[^/]+$/i.exec(section.customSrc.split('?')[0]);
  if (!m) return undefined;
  return seasonalThemes(themes).find((t) => t.id === m[1]);
}

/** Back to how it was before the swap, or to the page theme's own banner. */
export function switchBackPatch(own, previous) {
  if (previous) return { imageId: previous.imageId, customSrc: previous.customSrc, alt: previous.alt };
  const img = bannerOf(own);
  return { imageId: img?.id, customSrc: undefined, alt: img?.alt ?? '' };
}

/** Audience filter default for a course's K-12 mode. */
export function audienceForUiMode(uiMode) {
  if (uiMode === 'k2') return 'early-elementary';
  if (uiMode === '3-5') return 'upper-elementary';
  return 'all';
}
