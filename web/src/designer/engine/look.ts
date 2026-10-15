/**
 * The course look: the `courses.look` JSON (docs/page-designer/README.md, "Data model"), denormalized so the
 * course header never loads the theme catalog. Colours are 6-digit lower-case hex; the banner is the theme's
 * own art under /design-assets/.
 */
import { normHex } from './color';
import { themeImageUrl, themeImageUrl2x } from './icons';
import { darkPalette, sanitizePalette, themeDarkPalette } from './palette';
import type { CourseLook, LookColors, Palette, Theme } from './types';

/** Where theme art is served (Paper's static design assets). */
export const DESIGN_ASSET_BASE = '/design-assets';

const lookColors = (p: Palette): LookColors => {
  const hex = (c: string) => normHex(c) ?? '#000000';
  return {
    primary: hex(p.primary),
    on_primary: hex(p.onPrimary),
    secondary: hex(p.secondary),
    on_secondary: hex(p.onSecondary),
    accent: hex(p.accent),
    surface: hex(p.surface),
    on_surface: hex(p.onSurface),
    background: hex(p.background),
    text: hex(p.text),
    link: hex(p.link),
  };
};

/**
 * `courses.look` for a theme colourway. `palette` (a teacher's own colours) replaces the theme colourway when
 * it is valid; `paletteId` is kept either way. The banner is the theme's "banner" art (or its first banner).
 */
export function courseLook(theme: Theme, paletteId: string, palette?: Palette, opts: { assetBase?: string } = {}): CourseLook {
  const custom = palette ? sanitizePalette(palette) : null;
  const themed = theme.palettes.find(p => p.id === paletteId) ?? theme.palettes[0];
  const light = custom ?? themed;
  const dark = custom ? darkPalette(custom) : themeDarkPalette(themed);
  const look: CourseLook = {
    version: 1,
    theme_id: theme.id,
    palette_id: custom ? paletteId : themed.id,
    colors: lookColors(light),
    dark_colors: lookColors(dark),
  };
  const base = opts.assetBase ?? DESIGN_ASSET_BASE;
  const img = theme.images?.find(i => i.id === 'banner') ?? theme.images?.find(i => i.kind === 'banner');
  if (img) look.banner = { src: themeImageUrl(base, theme.id, img), src_2x: themeImageUrl2x(base, theme.id, img), alt: img.alt ?? '' };
  return look;
}
