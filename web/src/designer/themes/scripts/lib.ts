/** Shared by the theme:* scripts: repo paths, and finding a theme by id or by file path. */
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { contrastRatio, createPage, derivePalette, hexToHsl, hslToHex, renderPage, type LayoutId, type PageDoc, type PageKind, type Palette } from '@/designer/engine';
import { suggestPalette } from '../lint';
import type { ThemeEntry } from '../define';
import { THEME_ENTRIES } from '../index';

const here = path.dirname(fileURLToPath(import.meta.url));
/** web/src/designer/themes */
export const PKG_DIR = path.resolve(here, '..');
/** web/ */
export const WEB_DIR = path.resolve(PKG_DIR, '../../..');
export const THEMES_DIR = path.join(PKG_DIR, 'themes');
export const ASSET_DIR = path.join(WEB_DIR, 'public', 'design-assets', 'themes');
export const KINDS: PageKind[] = ['home', 'start-here', 'overview', 'module', 'instructor'];

/** Paths on the command line are relative to where the user ran `pnpm`, not this package. */
export const fromUser = (p: string) => path.resolve(process.env.INIT_CWD ?? process.cwd(), p);
/** Short path for printing: relative to where the user ran pnpm, absolute when that would climb out. */
export const rel = (p: string) => {
  const r = path.relative(process.env.INIT_CWD ?? process.cwd(), p);
  return r.startsWith('..') ? p : r || '.';
};

export interface Loaded {
  entry: ThemeEntry;
  /** Source file, when found. */
  file?: string;
  /** Listed in THEME_ENTRIES (a collection or index.ts). */
  registered: boolean;
}

/** Source file of a registered theme: src/themes/<id>.ts or src/themes/<collection>/<id>.ts. */
export function themeFile(id: string): string | undefined {
  const flat = path.join(THEMES_DIR, `${id}.ts`);
  if (fs.existsSync(flat)) return flat;
  for (const d of fs.readdirSync(THEMES_DIR, { withFileTypes: true }))
    if (d.isDirectory() && fs.existsSync(path.join(THEMES_DIR, d.name, `${id}.ts`))) return path.join(THEMES_DIR, d.name, `${id}.ts`);
  return undefined;
}

/** A theme id, or a path to a theme file (registered or not) whose default export is defineTheme(...). */
export async function loadTheme(arg: string): Promise<Loaded> {
  if (/[\\/]|\.ts$/.test(arg)) {
    const file = fromUser(arg);
    if (!fs.existsSync(file)) throw new Error(`no such file: ${arg}`);
    let mod: { default?: ThemeEntry };
    try {
      mod = (await import(pathToFileURL(file).href)) as typeof mod;
    } catch (e) {
      throw new Error(`${arg} did not load: ${(e as Error).message.split('\n')[0]}`);
    }
    const entry = mod.default;
    if (!entry?.theme?.id || !entry.art) throw new Error(`${arg}: default export is not a defineTheme(...) entry`);
    return { entry, file, registered: THEME_ENTRIES.some((e) => e.theme.id === entry.theme.id && themeFile(e.theme.id) === file) };
  }
  const entry = THEME_ENTRIES.find((e) => e.theme.id === arg);
  if (entry) return { entry, file: themeFile(arg), registered: true };
  const file = themeFile(arg);
  if (file) return loadTheme(file);
  throw new Error(`no theme "${arg}" (not registered, and no src/themes/**/${arg}.ts)`);
}

/** The catalogue with this entry in it: replacing the registered theme of the same id, or added. */
export function catalogueWith(entry: ThemeEntry): ThemeEntry[] {
  const i = THEME_ENTRIES.findIndex((e) => e.theme.id === entry.theme.id);
  return i < 0 ? [...THEME_ENTRIES, entry] : THEME_ENTRIES.map((e, j) => (j === i ? entry : e));
}

export interface RenderedPage {
  palette: Palette;
  layout: LayoutId;
  kind: PageKind;
  doc: PageDoc;
  html: string;
}

/** The theme in one layout × kind × palette, with stable ids so outputs diff cleanly. */
export function renderOne(entry: ThemeEntry, palette: Palette, layout: LayoutId, kind: PageKind, assetBase: string): RenderedPage {
  const t = entry.theme;
  const doc = createPage(t, kind, 'en', layout);
  doc.id = `${t.id}-${layout}-${kind}`;
  doc.paletteId = palette.id;
  return { palette, layout, kind, doc, html: renderPage(doc, t, { assetBase, courseId: '101' }) };
}

/**
 * A starting colourway for `theme:new` that passes every colour rule and follows spec.md: deep primary with white
 * text, a light secondary (a pale tint of the primary's hue) with a dark shade of that hue on it, near-black text
 * tinted toward the primary, and a link in the primary's hue that stands apart from the text.
 */
export function seedPalette(hex: string, id: string, name: string): Palette {
  const [h, s] = hexToHsl(hex);
  const secondary = hslToHex(h, Math.min(s, 0.45), 0.88);
  const base = derivePalette({ primary: hex, secondary }, name, id);
  const ink = hslToHex(h, Math.min(s, 0.3), 0.12);
  const p = suggestPalette({ ...base, secondary, onSecondary: hslToHex(h, Math.min(Math.max(s, 0.35), 0.6), 0.18), text: ink, onSurface: ink })!;
  for (let L = 0.6; L > 0.1; L -= 0.01) {
    const c = hslToHex(h, Math.max(s, 0.55), L);
    if (contrastRatio(c, p.background) >= 4.6 && contrastRatio(c, p.surface) >= 4.6) return { ...p, link: c };
  }
  return p;
}

/** Tiny flag parser: positionals plus --flag / --flag value. */
export function args(argv = process.argv.slice(2)): { pos: string[]; flags: Record<string, string | true> } {
  const pos: string[] = [];
  const flags: Record<string, string | true> = {};
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (!a.startsWith('--')) pos.push(a);
    else if (argv[i + 1] !== undefined && !argv[i + 1].startsWith('--') && !['json', 'dry-run', 'no-shots', 'help', 'force'].includes(a.slice(2))) flags[a.slice(2)] = argv[++i];
    else flags[a.slice(2)] = true;
  }
  return { pos, flags };
}

/** Prints a failure as one line (exit 2) instead of a stack: `await loadTheme(x).catch(die)`. */
export function die(e: unknown): never {
  console.error(`✗ ${e instanceof Error ? e.message : String(e)}`);
  process.exit(2);
}
