/**
 * @rmt/core — PageDoc + Theme → Canvas-safe HTML, plus starter content, icons and an accessibility checker.
 * See types.ts for the contract and docs/research/canvas-html-constraints.md for the rules the output follows.
 */
export * from './types';
export { createPage, makeQuickLink, SLOT_HREFS, SLOT_ICONS, LAYOUT_DEFAULTS } from './starter';
export { renderPage } from './render';
export { resolvePalette } from './render/context';
export { checkAccessibility, checkHtmlAccessibility, type A11yOptions } from './a11y';
export { contrastRatio, luminance, mix, normHex, readable, ensureContrast } from './color';
export {
  AA,
  NEAR_WHITE,
  HEX_RE,
  PALETTE_KEYS,
  PALETTE_PAIRS,
  PALETTE_NAME_MAX,
  checkPalette,
  derivePalette,
  fixPalette,
  hexToHsl,
  hslToHex,
  isNearWhite,
  nudgeLightness,
  paletteFailures,
  palettePasses,
  sanitizePalette,
  type PaletteBase,
  type PaletteKey,
  type PalettePair,
  type PairResult,
} from './palette';
export { sanitizeRich, safeHref } from './sanitize';
export { renderIconSvg, iconUrl, themeImageUrl, hasIcon, iconNames, ICON_FALLBACK } from './icons';
export { LAYOUTS } from './layouts';
export { SECTION_TYPES } from './sections-meta';
export { strings } from './strings';
export { shortHash } from './hash';
