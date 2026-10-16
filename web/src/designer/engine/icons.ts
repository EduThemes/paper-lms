import iconNodesJson from 'lucide-static/icon-nodes.json';
import aliasesJson from './icon-aliases.json';
import { bare, mix, normHex } from './color';
import { esc } from './html';
import type { IconName, IconStyle, ThemeImage } from './types';

/** Lucide (ISC) icon geometry: name → [tag, attrs][] in a 24×24 box. */
type IconNode = [string, Record<string, string | number>][];
const ICONS = iconNodesJson as unknown as Record<string, IconNode>;
/** Older Lucide names (e.g. "smile", "help-circle") → current names. Regenerate with scripts/gen-icon-aliases.ts. */
const ALIASES = aliasesJson as Record<string, string>;
const own = (o: object, k: string) => Object.prototype.hasOwnProperty.call(o, k);
/** Canonical Lucide name for a name or alias, or undefined. */
export const resolveIcon = (name: string): string | undefined => {
  const k = String(name).toLowerCase().replace(/[^a-z0-9-]/g, '');
  return own(ICONS, k) ? k : own(ALIASES, k) ? ALIASES[k] : undefined;
};

/** Shown for unknown names: a dashed circle reads as "placeholder", not as a wrong meaning. */
export const ICON_FALLBACK: IconName = 'circle-dashed';

export const hasIcon = (name: string): boolean => resolveIcon(name) !== undefined;

/** Every available Lucide icon name (for the builder's icon picker). */
export const iconNames = (): IconName[] => Object.keys(ICONS);

const ICON_STYLES: IconStyle[] = ['solid-circle', 'soft-square', 'outline', 'duotone'];
const cleanName = (name: string) => String(name).toLowerCase().replace(/[^a-z0-9-]/g, '');
const fmt = (n: number) => String(Math.round(n * 1000) / 1000);

/**
 * A standalone SVG file for one icon in one of the four styles. Used via <img src>, so it may use
 * anything SVG supports (opacity etc.) — Canvas never sanitizes the file itself.
 */
export function renderIconSvg(name: IconName, o: { style: IconStyle; fg: string; bg: string; size?: number }): string {
  const size = Math.max(8, Math.min(1024, Math.round(o.size ?? 96)));
  const style: IconStyle = ICON_STYLES.includes(o.style) ? o.style : 'solid-circle';
  const fg = normHex(o.fg) ?? '#1f2328';
  const bg = normHex(o.bg) ?? '#ffffff';
  const nodes = ICONS[resolveIcon(name) ?? ICON_FALLBACK] ?? [];
  const S = size;
  let back = '';
  let glyph = S * 0.5;
  switch (style) {
    case 'solid-circle':
      back = `<circle cx="${fmt(S / 2)}" cy="${fmt(S / 2)}" r="${fmt(S / 2)}" fill="${bg}"/>`;
      break;
    case 'soft-square':
      back = `<rect width="${S}" height="${S}" rx="${fmt(S * 0.24)}" fill="${bg}"/>`;
      break;
    case 'outline': {
      const sw = Math.max(1.5, S * 0.045);
      back = `<circle cx="${fmt(S / 2)}" cy="${fmt(S / 2)}" r="${fmt(S / 2 - sw / 2)}" fill="${bg}" stroke="${fg}" stroke-width="${fmt(sw)}"/>`;
      glyph = S * 0.46;
      break;
    }
    case 'duotone':
      back =
        `<rect width="${S}" height="${S}" rx="${fmt(S * 0.3)}" fill="${bg}"/>` +
        `<circle cx="${fmt(S * 0.6)}" cy="${fmt(S * 0.42)}" r="${fmt(S * 0.22)}" fill="${mix(fg, bg, 0.72)}"/>`;
      break;
  }
  const off = (S - glyph) / 2;
  const k = glyph / 24;
  const inner = nodes
    .map(([tag, attrs]) => {
      const a = Object.entries(attrs)
        .filter(([key]) => key !== 'key')
        .map(([key, v]) => ` ${key}="${esc(String(v))}"`)
        .join('');
      return `<${tag}${a}/>`;
    })
    .join('');
  return (
    `<svg xmlns="http://www.w3.org/2000/svg" width="${S}" height="${S}" viewBox="0 0 ${S} ${S}">` +
    back +
    `<g transform="translate(${fmt(off)} ${fmt(off)}) scale(${fmt(k)})" fill="none" stroke="${fg}" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">${inner}</g>` +
    `</svg>`
  );
}

/** `${iconBase}/${style}/${name}.svg?fg=<hex>&bg=<hex>` (colours without '#'), e.g. iconBase "/api/v1/design/icons". */
export function iconSrc(iconBase: string, name: IconName, o: { style: IconStyle; fg: string; bg: string }): string {
  return `${trimBase(iconBase)}/${o.style}/${encodeURIComponent(cleanName(name) || ICON_FALLBACK)}.svg?fg=${bare(o.fg)}&bg=${bare(o.bg)}`;
}

/** `${assetBase}/icons/${style}/${name}.svg?fg=<hex>&bg=<hex>` (colours without '#'). */
export function iconUrl(assetBase: string, name: IconName, o: { style: IconStyle; fg: string; bg: string }): string {
  return iconSrc(`${trimBase(assetBase)}/icons`, name, o);
}

/** `${assetBase}/themes/${themeId}/${image.file}` */
export function themeImageUrl(assetBase: string, themeId: string, image: ThemeImage | Pick<ThemeImage, 'file'>): string {
  return `${trimBase(assetBase)}/themes/${encodeURIComponent(themeId)}/${image.file.split('/').map(seg => encodeURIComponent(seg).replace(/%40/g, '@')).join('/')}`;
}

/** The 2x twin of a theme image: `banner.webp` → `…/banner@2x.webp`. */
export function themeImageUrl2x(assetBase: string, themeId: string, image: ThemeImage | Pick<ThemeImage, 'file'>): string {
  return themeImageUrl(assetBase, themeId, { file: at2x(image.file) });
}

/** `name.ext` → `name@2x.ext` (no extension: `name@2x`). */
export const at2x = (file: string): string => (/\.[a-z0-9]+$/i.test(file) ? file.replace(/(\.[a-z0-9]+)$/i, '@2x$1') : `${file}@2x`);

const trimBase = (b: string) => String(b ?? '').replace(/\/+$/, '');
