import type { Palette } from '@/designer/engine';
import type { ThemeEntry } from './define';

const esc = (s: string) =>
  s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

function swatch(p: Palette): string {
  const chip = (bg: string, fg: string, label: string) =>
    `<div style="flex:1 1 0;min-width:84px;padding:8px;background:${bg};color:${fg};font:12px/1.3 system-ui,sans-serif">${label}<br><code>${bg}</code></div>`;
  return `<div style="margin:8px 0;border:1px solid #d0d5dc;border-radius:8px;overflow:hidden;background:${p.background};color:${p.text}">
  <div style="padding:6px 10px;font:600 13px system-ui,sans-serif;background:${p.background};color:${p.text}">${esc(p.name)} <span style="color:${p.link};text-decoration:underline">link</span></div>
  <div style="display:flex;flex-wrap:wrap">${chip(p.primary, p.onPrimary, 'primary')}${chip(p.secondary, p.onSecondary, 'secondary')}${chip(p.surface, p.onSurface, 'surface')}${chip(p.accent, '#000000', 'accent')}${chip(p.background, p.text, 'background')}</div>
</div>`;
}

/**
 * A self-contained HTML swatch sheet of every theme and colourway, for design review.
 * Pass `assetBase` (e.g. "http://localhost:8787/assets") to show each theme's banner too.
 */
export function previewHtml(entries: ThemeEntry[], assetBase?: string): string {
  const sections = entries
    .map(({ theme: t, style, colourBlind }) => {
      const banner = assetBase
        ? `<img src="${esc(`${assetBase}/themes/${t.id}/banner.webp`)}" alt="" width="1200" height="300" style="display:block;width:100%;height:auto;border-radius:6px">`
        : '';
      return `<section style="margin:0 0 32px">
  <h2 style="margin:0 0 4px;font-family:${esc(t.headingFont ?? 'system-ui, sans-serif')}">${esc(t.name)} <small style="font:13px system-ui;color:#555">${esc(t.id)}</small></h2>
  <p style="margin:0 0 6px;font:14px/1.4 system-ui,sans-serif">${esc(t.description)}</p>
  <p style="margin:0 0 6px;font:12px/1.4 system-ui,sans-serif;color:#444"><strong>${esc(style)}</strong> · ${esc(t.audiences.join(', '))} · layout ${t.defaultLayout} · icons ${t.iconStyle} · corners ${t.corners}<br>Colour-blind: ${esc(colourBlind)}</p>
  ${banner}
  ${t.palettes.map(swatch).join('\n')}
</section>`;
    })
    .join('\n');
  return `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>RMT Studio themes</title></head>
<body style="margin:0;padding:24px 16px;background:#ffffff;color:#1f2328;max-width:1100px;margin:0 auto;font-family:system-ui,sans-serif">
<h1 style="font-size:24px">RMT Studio themes (${entries.length})</h1>
${sections}
</body></html>`;
}
