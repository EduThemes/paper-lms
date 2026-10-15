import { shortHash } from '../hash';
import { esc } from '../html';
import { LAYOUT_MODULES } from '../layouts';
import type { PageDoc, RenderOptions, Theme } from '../types';
import { makeCtx } from './context';
import { paint } from './parts';

/**
 * PageDoc + Theme → one Canvas-safe HTML fragment.
 * The root carries the re-edit markers (id, data-rmt-doc, data-rmt-v, data-rmt-hash) and an explicit
 * background + text colour so it reads as a white sheet in the mobile apps' dark mode.
 */
export function renderPage(doc: PageDoc, theme: Theme, opts: RenderOptions): string {
  const mod = LAYOUT_MODULES[doc.layout] ?? LAYOUT_MODULES['classic-band'];
  const ctx = makeCtx({ ...doc, layout: mod.meta.id }, theme, opts, mod.type, { forceRound: mod.forceRound });
  const body = mod.render(ctx);
  const id = String(doc.id).replace(/\s+/g, '');
  const style = [paint(ctx.tones.page), 'max-width:1100px', 'margin:0 auto', 'padding:0', 'line-height:1.6', mod.rootStyle].filter(Boolean).join('; ');
  return (
    `<div id="rmt-${esc(id)}" class="rmt-page rmt-layout-${mod.meta.id}" data-rmt-doc="${esc(id)}" data-rmt-v="1" data-rmt-hash="${shortHash(JSON.stringify(doc))}" ` +
    `data-rmt-theme="${esc(theme.id)}" data-rmt-layout="${mod.meta.id}" data-rmt-kind="${esc(doc.kind)}" lang="${esc(doc.locale)}" style="${esc(style)};">` +
    body +
    `</div>`
  );
}
