/**
 * Page Designer engine — PageDoc + Theme → { html, css } (light + dark), plus starter content, icons, palette
 * maths and an accessibility checker. See types.ts and docs/page-designer/README.md for the contract.
 */
export * from './types';
export { createPage, makeQuickLink, SLOT_HREFS, SLOT_ICONS, LAYOUT_DEFAULTS } from './starter';
export { renderDesign, renderDesignDetail, renderPage, resolveDarkPalette, type DesignDetail } from './render';
export { READER_FONT_VAR, flattenDesignVars } from './render/design';
export { resolvePalette } from './render/context';
export { courseLook, DESIGN_ASSET_BASE } from './look';
export { checkAccessibility, checkHtmlAccessibility, type A11yOptions } from './a11y';
export { contrastRatio, luminance, mix, normHex, readable, ensureContrast } from './color';
export {
  AA,
  NEAR_WHITE,
  HEX_RE,
  PALETTE_KEYS,
  PALETTE_PAIRS,
  PALETTE_NAME_MAX,
  PAPER_DARK_BACKGROUND,
  PAPER_DARK_CARD,
  checkPalette,
  darkPalette,
  derivePalette,
  fixDarkPalette,
  fixPalette,
  hexToHsl,
  hslToHex,
  isNearWhite,
  nudgeLightness,
  paletteFailures,
  palettePasses,
  sanitizePalette,
  themeDarkPalette,
  type PaletteBase,
  type PaletteKey,
  type PalettePair,
  type PairResult,
} from './palette';
export { sanitizeRich, safeHref } from './sanitize';
export { renderIconSvg, iconSrc, iconUrl, themeImageUrl, themeImageUrl2x, at2x, hasIcon, iconNames, ICON_FALLBACK } from './icons';
export { LAYOUTS } from './layouts';
export { SECTION_TYPES } from './sections-meta';
export { strings } from './strings';
export { shortHash } from './hash';
