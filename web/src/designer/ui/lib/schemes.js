/**
 * "My colour schemes" ↔ palettes. A saved scheme becomes a palette whose id is `scheme-<server id>`, so a
 * page that uses it can tell which scheme it came from (and the id is a valid palette id).
 */
import { PALETTE_NAME_MAX, sanitizePalette } from './engine';

export const MAX_SCHEMES = 50;
export const schemePaletteId = (id) => `scheme-${String(id).replace(/[^A-Za-z0-9_-]/g, '')}`;
export const schemeIdOf = (paletteId) => (typeof paletteId === 'string' && paletteId.startsWith('scheme-') ? paletteId.slice(7) : null);

/** Name: collapsed whitespace, no angle brackets or control characters, 1..60 characters. */
export function cleanSchemeName(name) {
  return String(name ?? '')
    .replace(/[\u0000-\u001f\u007f<>]/g, '') // eslint-disable-line no-control-regex
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, PALETTE_NAME_MAX);
}

/** A server scheme → { id, name, palette } with a valid palette, or null. */
export function toScheme(s) {
  const palette = sanitizePalette({ ...(s?.palette ?? {}), id: schemePaletteId(s?.id), name: cleanSchemeName(s?.name) || 'My colours' });
  return palette ? { id: String(s.id), name: palette.name, palette, updatedAt: s.updatedAt ?? null } : null;
}

export function toSchemes(list) {
  return (Array.isArray(list) ? list : []).map(toScheme).filter(Boolean);
}
