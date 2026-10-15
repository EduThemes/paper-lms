// A course's "look" (courses.look, docs/page-designer/README.md "Data
// model"): banner art and colours chosen in the Page Designer, applied to
// the course header and dashboard card. Denormalized so the header never
// loads the theme catalog. null means Paper's generated header.
//
// {
//   version: 1, theme_id, palette_id,
//   colors:      { primary, on_primary, ... },   // #rrggbb
//   dark_colors: { primary, on_primary, ... },
//   banner: { src, src_2x, alt }
// }

const HEX_RE = /^#[0-9a-f]{6}$/i;
// The server only stores banners under /design-assets/ or a course file
// (/api/v1/files/<id>/download); anything else is ignored here too.
const BANNER_RE = /^\/(?:design-assets\/|api\/v1\/files\/\d+\/download(?:\?|$))/;

const hex = (v) => (typeof v === 'string' && HEX_RE.test(v.trim()) ? v.trim().toLowerCase() : null);
const bannerUrl = (v) => (typeof v === 'string' && BANNER_RE.test(v) ? v : null);

// hexToRgbTriplet('#12355b') -> '18 53 91' (the app's rgb(var(--x) / a) style).
export function hexToRgbTriplet(value) {
  const h = hex(value);
  if (!h) return null;
  const n = parseInt(h.slice(1), 16);
  return `${(n >> 16) & 255} ${(n >> 8) & 255} ${n & 255}`;
}

// Relative luminance (WCAG 2.x) of a #rrggbb colour.
export function luminance(value) {
  const h = hex(value);
  if (!h) return 0;
  const n = parseInt(h.slice(1), 16);
  const ch = [(n >> 16) & 255, (n >> 8) & 255, n & 255].map((c) => {
    const s = c / 255;
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * ch[0] + 0.7152 * ch[1] + 0.0722 * ch[2];
}

// normalizeLook returns the parts of a look the UI uses, or null when the
// course has no usable look (no primary colour).
export function normalizeLook(look) {
  if (!look || typeof look !== 'object') return null;
  const light = look.colors || {};
  const dark = look.dark_colors || {};
  const primary = hex(light.primary);
  if (!primary) return null;
  const onPrimary = hex(light.on_primary) || (luminance(primary) > 0.4 ? '#111111' : '#ffffff');
  const darkPrimary = hex(dark.primary) || primary;
  const darkOnPrimary = hex(dark.on_primary)
    || (hex(dark.primary) ? (luminance(darkPrimary) > 0.4 ? '#111111' : '#ffffff') : onPrimary);
  const src = bannerUrl(look.banner?.src);
  const src2x = src ? bannerUrl(look.banner?.src_2x) : null;
  return {
    themeId: typeof look.theme_id === 'string' ? look.theme_id : null,
    primary,
    onPrimary,
    darkPrimary,
    darkOnPrimary,
    banner: src
      ? { src, srcSet: src2x ? `${src} 1x, ${src2x} 2x` : undefined, alt: typeof look.banner?.alt === 'string' ? look.banner.alt : '' }
      : null,
    // Palette swatches for settings (light colourway, in a stable order).
    swatches: ['primary', 'secondary', 'accent', 'surface', 'background', 'text', 'link']
      .map((key) => ({ key, color: hex(light[key]) }))
      .filter((s) => s.color),
  };
}

// lookCssVars: custom properties consumed by the .course-look rules in
// index.css (light values, and dark ones that apply under html.dark).
export function lookCssVars(look) {
  if (!look) return undefined;
  const shadow = (fg) => (luminance(fg) > 0.5 ? '0 1px 2px rgb(0 0 0 / 0.45)' : '0 1px 2px rgb(255 255 255 / 0.5)');
  return {
    '--look-primary': look.primary,
    '--look-primary-rgb': hexToRgbTriplet(look.primary),
    '--look-on-primary': look.onPrimary,
    '--look-text-shadow': shadow(look.onPrimary),
    '--look-primary-dark': look.darkPrimary,
    '--look-primary-dark-rgb': hexToRgbTriplet(look.darkPrimary),
    '--look-on-primary-dark': look.darkOnPrimary,
    '--look-text-shadow-dark': shadow(look.darkOnPrimary),
  };
}
