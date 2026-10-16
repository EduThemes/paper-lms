import { shortHash } from '../hash';
import { esc } from '../html';
import { LAYOUT_MODULES } from '../layouts';
import { darkPalette, themeDarkPalette } from '../palette';
import type { PageDoc, Palette, RenderOptions, RenderResult, Theme, ThemePalette } from '../types';
import { makeCtx, resolvePalette, type Ctx } from './context';
import { designCss, mergeRenders, type Merged } from './design';
import { paint } from './parts';

/**
 * One render of a doc in one palette: the page body. The root carries the re-edit markers (id, data-pd-doc,
 * data-pd-v, data-pd-hash) and an explicit background + text colour, so it reads as its own sheet.
 */
export function renderBody(doc: PageDoc, theme: Theme, opts: RenderOptions, palette?: Palette): { html: string; ctx: Ctx } {
  const mod = LAYOUT_MODULES[doc.layout] ?? LAYOUT_MODULES['classic-band'];
  const ctx = makeCtx({ ...doc, layout: mod.meta.id }, theme, opts, mod.type, { forceRound: mod.forceRound, palette });
  const body = mod.render(ctx);
  const id = String(doc.id).replace(/\s+/g, '');
  const style = [paint(ctx.tones.page), 'max-width:1100px', 'margin:0 auto', 'padding:0', 'line-height:1.6', mod.rootStyle].filter(Boolean).join('; ');
  const html =
    `<div id="pd-${esc(id)}" class="paper-design paper-design--${mod.meta.id}" data-pd-doc="${esc(id)}" data-pd-v="1" data-pd-hash="${shortHash(JSON.stringify(doc))}" ` +
    `data-pd-theme="${esc(theme.id)}" data-pd-layout="${mod.meta.id}" data-pd-kind="${esc(doc.kind)}" lang="${esc(doc.locale)}" style="${esc(style)};">` +
    body +
    `</div>`;
  return { html, ctx };
}

/** The dark colourway a doc renders with: its custom palette's dark twin, or the theme colourway's (with its hand-tuning). */
export function resolveDarkPalette(doc: Pick<PageDoc, 'paletteId' | 'palette'>, theme: Theme): Palette {
  const light = resolvePalette(doc, theme);
  const fromTheme = theme.palettes.find(p => p === light) as ThemePalette | undefined;
  return fromTheme ? themeDarkPalette(fromTheme) : darkPalette(light);
}

export interface DesignDetail extends RenderResult {
  /** True when the light and dark renders differed in structure, so the page fell back to light only. */
  diverged: boolean;
  /** Custom properties emitted: name → [light, dark]. */
  vars: Record<string, [string, string]>;
  /** Icons swapped in dark mode: data-pd-icon id → dark src. */
  icons: Record<string, string>;
}

/** renderDesign plus what the dark merge did (for tests and the designer's diagnostics). */
export function renderDesignDetail(doc: PageDoc, theme: Theme, opts: RenderOptions): DesignDetail {
  const light = renderBody(doc, theme, opts);
  if (opts.dark === false) return { html: light.html, css: designCss(light.ctx, null, null), diverged: false, vars: {}, icons: {} };
  const dark = renderBody(doc, theme, opts, resolveDarkPalette(doc, theme));
  const merged: Merged | null = mergeRenders(light.html, dark.html);
  if (!merged) return { html: light.html, css: designCss(light.ctx, null, null), diverged: true, vars: {}, icons: {} };
  return { html: merged.html, css: designCss(light.ctx, dark.ctx, merged), diverged: false, vars: merged.vars, icons: merged.icons };
}

/**
 * PageDoc + Theme → `{ html, css }`. `html` is the page body; colours that differ between the light and dark
 * colourways are `var(--pd-cN, #light)`, so the HTML alone renders the light page (and is what Canvas sees,
 * with the vars flattened on export). `css` is the designer stylesheet, written against `:root` = the page.
 */
export function renderDesign(doc: PageDoc, theme: Theme, opts: RenderOptions): RenderResult {
  const { html, css } = renderDesignDetail(doc, theme, opts);
  return { html, css };
}

/** The page body only (renderDesign(...).html), for callers that predate the stylesheet. */
export function renderPage(doc: PageDoc, theme: Theme, opts: RenderOptions): string {
  return renderDesign(doc, theme, opts).html;
}
