/**
 * ENGINE ADAPTER: the only file in the designer UI that imports the engine and the theme catalog.
 *
 * Everything else imports from here, so a change in the engine's shape only touches this file. Render and
 * check calls are wrapped so an engine bug shows up as an empty preview or an empty checklist instead of
 * crashing the editor.
 *
 * Rendering is `renderDesign(doc, theme, opts) → { html, css }` (light + dark in one page). If an engine
 * without it is ever paired with this UI, `renderPage` is used with no stylesheet.
 */
import * as core from '@/designer/engine';
import * as themes from '@/designer/themes';

export const ASSET_BASE = '/design-assets';
export const ICON_BASE = '/api/v1/design/icons';

const ALL_COLLECTIONS = ['essentials', 'signature', 'arcade', 'heroes-comics', 'cartoons', 'holidays', 'special-days'];

// ---- themes ------------------------------------------------------------------------------------

export const THEMES = themes.THEMES;
export const getTheme = (id) => themes.getTheme(id);
/** The theme for a doc, falling back to the first theme so the UI never crashes on a stale id. */
export const themeOf = (doc) => getTheme(doc?.themeId) ?? THEMES[0];
export const COLLECTIONS = (() => {
  const given = themes.COLLECTIONS ?? [];
  const missing = ALL_COLLECTIONS.filter((id) => !given.some((c) => c.id === id)).map((id) => ({ id, name: id, description: '' }));
  return [...given, ...missing];
})();

// ---- core --------------------------------------------------------------------------------------

export const LAYOUTS = core.LAYOUTS.map(({ id, name, description }) => ({ id, name, description }));
export const SECTION_TYPES = core.SECTION_TYPES;
export const SLOT_HREFS = core.SLOT_HREFS;
export const PALETTE_KEYS = core.PALETTE_KEYS;
export const PALETTE_NAME_MAX = core.PALETTE_NAME_MAX ?? 60;
export const HEX_RE = core.HEX_RE;

export const createPage = (theme, kind, locale, layout) => core.createPage(theme, kind, locale, layout || undefined);
export const starterStrings = (locale) => core.strings(locale);
export const sanitizeRich = (html) => core.sanitizeRich(html);
export const iconNames = () => core.iconNames();
export const normHex = (c) => core.normHex(c);
export const derivePalette = (base, name, id) => core.derivePalette(base, name, id);
export const checkPalette = (p) => core.checkPalette(p);
export const fixPalette = (p) => core.fixPalette(p);
export const sanitizePalette = (x) => core.sanitizePalette(x);

/** Render options for Paper: theme art under /design-assets, icons from the icon route, links for this course. */
export function renderOptions(courseId, extra = {}) {
  return { assetBase: ASSET_BASE, iconBase: ICON_BASE, ...(courseId ? { courseId: String(courseId) } : {}), ...extra };
}

/** The palette a doc renders with: its valid custom palette, else the theme colourway it names. */
export function paletteOf(doc) {
  const theme = themeOf(doc);
  return (doc.palette && core.sanitizePalette(doc.palette)) || theme.palettes.find((p) => p.id === doc.paletteId) || theme.palettes[0];
}

/** Placeholder colours for missing theme art, from the doc's colours. */
export function tintOf(doc) {
  const p = paletteOf(doc);
  return { bg: p?.surface ?? '#eef1f3', fg: p?.primary ?? '#2b7abc' };
}

/** `/api/v1/design/icons/<style>/<name>.svg?fg=…&bg=…`. */
export const iconUrl = (name, o) =>
  typeof core.iconSrc === 'function' ? core.iconSrc(ICON_BASE, name, o) : core.iconUrl(ICON_BASE.replace(/\/icons$/, ''), name, o);
export const themeImageUrl = (themeId, image) => core.themeImageUrl(ASSET_BASE, themeId, image);
/** `banner.webp` → `banner@2x.webp`. */
export const retinaFile = (file) => String(file).replace(/(\.[a-z0-9]+)$/i, '@2x$1');
export const themeImageUrl2x = (themeId, image) =>
  typeof core.themeImageUrl2x === 'function'
    ? core.themeImageUrl2x(ASSET_BASE, themeId, image)
    : core.themeImageUrl(ASSET_BASE, themeId, { ...image, file: retinaFile(image.file) });

/** PageDoc → { html, css }. Never throws: an engine error renders as an empty page (and is logged). */
export function renderDesign(doc, opts) {
  if (!doc) return { html: '', css: '' };
  const theme = themeOf(doc);
  try {
    if (typeof core.renderDesign === 'function') {
      const r = core.renderDesign(doc, theme, opts);
      return { html: String(r?.html ?? ''), css: String(r?.css ?? '') };
    }
    return { html: core.renderPage(doc, theme, opts), css: '' };
  } catch (err) {
    console.error('[designer] render failed', err);
    return { html: '', css: '' };
  }
}

/** Accessibility issues for a doc. `opts.courseId` lets "this course" links pass. */
export function checkAccessibility(doc, opts = {}) {
  if (!doc) return [];
  try {
    return core.checkAccessibility(doc, themeOf(doc), opts);
  } catch (err) {
    console.error('[designer] accessibility check failed', err);
    return [];
  }
}

/** The hash the engine writes into the page root (`data-pd-hash`). */
export const docHash = (doc) => core.shortHash(JSON.stringify(doc));

// ---- course look ---------------------------------------------------------------------------------

/**
 * The banner a doc shows, as the course look's `banner`, when it isn't the theme's main banner: an alternate
 * or seasonal theme banner (under /design-assets) or an image uploaded to this course's Files. Undefined means
 * "the theme's banner" (what the engine's courseLook sets) — including banners on other sites, which the
 * server would refuse.
 */
export function lookBanner(doc) {
  const theme = themeOf(doc);
  const section = doc.sections.find((s) => s.type === 'banner');
  if (!section) return undefined;
  const src = String(section.customSrc ?? '').trim();
  if (src) {
    if (/^\/api\/v1\/files\/\d+\/download$/.test(src)) return { src, src_2x: src, alt: section.alt ?? '' };
    if (src.startsWith(`${ASSET_BASE}/`)) return { src, src_2x: retinaFile(src), alt: section.alt ?? '' };
    return undefined;
  }
  const img = section.imageId ? theme.images.find((i) => i.id === section.imageId) : null;
  if (!img || img.id === 'banner') return undefined;
  return { src: themeImageUrl(theme.id, img), src_2x: themeImageUrl2x(theme.id, img), alt: section.alt ?? img.alt ?? '' };
}

/**
 * The course look (`courses.look`) for a doc: the engine's courseLook for its theme and colours, with the
 * doc's own banner when it uses an alternate, seasonal or uploaded one.
 */
export function courseLook(doc) {
  const theme = themeOf(doc);
  const look = core.courseLook(theme, doc.paletteId, doc.palette, { assetBase: ASSET_BASE });
  const banner = lookBanner(doc);
  return banner ? { ...look, banner } : look;
}
