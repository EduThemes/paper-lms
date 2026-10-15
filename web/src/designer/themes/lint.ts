/**
 * The rules every theme follows (README "Rules every theme follows"), as data. The tests and the
 * `theme:check` CLI both call these, so they can never disagree.
 *
 *   lintTheme(entry)        one theme on its own: metadata, palettes, images, prompts, icons, fonts
 *   lintCatalogue(entries)  rules across themes: unique ids and names, tier coverage, near-duplicate palettes
 *
 * Errors break the tests. Warnings are heuristics for a designer to look at; they never fail anything.
 * Not exported from index.ts: it reads lucide-static from disk, so it is for Node (tests, scripts) only.
 */
import { existsSync } from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  at2x,
  checkPalette,
  contrastRatio,
  fixPalette,
  hexToHsl,
  hslToHex,
  luminance,
  nudgeLightness,
  PALETTE_KEYS,
  themeDarkPalette,
  type Audience,
  type LayoutId,
  type Palette,
  type PaletteKey,
  type QuickLinkSlot,
} from '@/designer/engine';
import { FONTS, type ThemeEntry } from './define';

export interface LintFinding {
  /** Stable rule id, e.g. "palette-contrast". */
  rule: string;
  severity: 'error' | 'warn';
  message: string;
  /** What to change, in words. For colour failures it carries the exact suggested hex values. */
  fix?: string;
  /** Machine-readable colour fix: set these keys on palette `palette`. */
  palettePatch?: { palette: string; set: Partial<Record<PaletteKey, string>> };
  /** Catalogue findings: the theme ids involved. */
  themes?: string[];
}

export interface LintOptions {
  /** Whether a Lucide icon exists. Defaults to looking in lucide-static/icons. */
  iconExists?: (name: string) => boolean;
  /**
   * Where theme art lives (`<dir>/<themeId>/<file>` plus its `@2x` twin). Defaults to DESIGN_ASSET_DIR;
   * false skips the check (a theme whose art isn't generated yet).
   */
  assetDir?: string | false;
}

/** web/public/design-assets/themes: theme art, WebP at 1x and @2x, served at /design-assets/themes/. */
export const DESIGN_ASSET_DIR = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../public/design-assets/themes');

/* ------------------------------------------------------------------ thresholds */

export const AUDIENCES: Audience[] = ['early-elementary', 'upper-elementary', 'secondary', 'higher-ed', 'professional', 'specialty'];
export const SLOTS: QuickLinkSlot[] = ['start', 'syllabus', 'modules', 'assignments', 'discussions', 'grades', 'help', 'instructor', 'calendar', 'resources'];
export const IMAGE_SPEC = {
  banner: ['banner', 1200, 300],
  'banner-alt': ['banner', 1200, 300],
  'spot-welcome': ['spot', 600, 600],
  'spot-instructor': ['spot', 600, 600],
  'card-1': ['card', 600, 400],
  'card-2': ['card', 600, 400],
  'card-3': ['card', 600, 400],
} as const;

/** AA for every text pair; body text and panel text must also be dark enough to survive dark mode. */
const AA = 4.5;
const DARK_TEXT_ON_WHITE = 7;
/** Canvas pages are white: background near-white, surface a light tinted panel, primary a deep band colour. */
const MIN_BACKGROUND_LUM = 0.9;
const MIN_SURFACE_LUM = 0.75;
const MAX_PRIMARY_LUM = 0.18;
const MAX_PROMPT = 900;
/** Below this link-to-text ratio links read as body text (every catalogue palette but one sits at 1.6+). */
const LINK_VS_TEXT = 1.4;
const KEBAB = /^[a-z0-9]+(-[a-z0-9]+)*$/;
const HEX6 = /^#[0-9A-Fa-f]{6}$/;
/**
 * Mean ΔE (CIE76) below which two colourways look alike. Inside a theme only primary and secondary are
 * compared (accent is often shared on purpose); across themes all four visible colours, and the bar is lower.
 */
const SIMILAR_IN_THEME = 12;
const SIMILAR_ACROSS_THEMES = 3;
const MAIN_KEYS: PaletteKey[] = ['primary', 'secondary'];
/** An accent at least this far (ΔE) from primary, secondary and link is a colour only the accent brings. */
const ACCENT_DISTINCT = 20;
/**
 * Where each layout draws `accent` on its starter pages; '' = nowhere. test/lint.test.ts renders every layout to
 * keep this true. Outside the starter pages, a teacher-added short divider (every layout but card-grid and sidebar)
 * and the builder's "underline" heading style also draw it.
 */
export const ACCENT_IN_LAYOUT: Record<LayoutId, string> = {
  'classic-band': '6px rule under the title band',
  'split-hero': '3px rule left of the welcome text (beside the banner) and 3px underline under page-level h2s',
  'card-grid': '',
  sidebar: '3px underline under page-level h2s',
  timeline: '',
  editorial: '',
  playful: 'every third quick-link tile: 3px border (darkened to 3:1 if pale) and a pale tint of it',
};
const VISIBLE_KEYS: PaletteKey[] = ['primary', 'secondary', 'accent', 'surface'];
/** Image subjects shorter than this many words tend to come back generic. */
const MIN_SUBJECT_WORDS = 4;
const MIN_BANNER_WORDS = 12;

/* ------------------------------------------------------------------ prompt word checks */

/**
 * Drops short negated lists ("no billboards, signs or posters", "no face on the engine") so the word
 * checks below only catch things a prompt asks FOR. Each listed item may be at most three words.
 */
const ITEM = String.raw`(?:[\w-]+\s+){0,2}[\w-]+`;
const NEGATED = new RegExp(String.raw`\b(?:no|without)\s+${ITEM}(?:,\s+${ITEM})*(?:,?\s+or\s+${ITEM})?`, 'gi');
export const asked = (prompt: string) => prompt.replace(NEGATED, '');

/** Lettering requests, and restating the pipeline's own suffix. */
const LETTERING = /\b(text|texts|letter|letters|lettering|words?|typography|font|caption|captions|label|labels|title|titles|headline|slogan|logo|logos|signage|watermark|writing|written|handwriting|numbers?|digits?)\b/i;
/** Objects that invite lettering (README rule 3). */
const LETTERING_OBJECTS = /\b(sign|signs|signpost|chalkboard|blackboard|whiteboard|screens?(?!-print)|monitor|map|maps|calendar|flag|flags|jersey|jerseys|poster|posters|newspaper|keyboard|book cover|clock face)\b/i;
/** Words that always mean a person. "man-made" is a material, not a man. */
const PEOPLE = /\b(person|people|portrait|teacher|student|child|children|man(?!-made)|woman|kids?|[a-z]+-year-olds?)\b/i;
/** A hand, but not a technique ("hand-painted", "hands-on") or a side ("left-hand side"). */
const HAND = /(?<!\b(?:left|right)-)\bhands?\b(?!-)/i;
/** Someone or something alive that has a face. */
const BEING = String.raw`human|animal|person|people|child|children|kids?|bab(?:y|ies)|girls?|boys?|wom[ae]n|m[ae]n|students?|teachers?|dolls?|teddy|bears?|cats?|dogs?|pupp(?:y|ies)|kittens?|owls?|fox(?:es)?|bunn(?:y|ies)|rabbits?|lions?|tigers?|monkeys?|pandas?|robots?|clowns?|monsters?|snowm[ae]n`;
const EXPRESSION = String.raw`smiling|smiley|happy|sad|friendly|cute|grinning|laughing|cheerful|kind|kindly|beaming|winking|sleepy|sleeping|surprised|funny|silly|joyful|angry|crying|worried|freckled|rosy|chubby|kawaii|cartoon|expressive|young|old|little`;
/**
 * "face" only when it is a living face, so "a cube with one orange face", "the cliff's north face" and
 * "a die with six faces" pass, while "a smiling face", "a cat's face", "her face", "a sun with a face",
 * "faces of children" and "a face with big eyes" are caught.
 */
const FACE = new RegExp(
  [
    String.raw`(?:\b(?:${BEING})(?:['’]s|s['’])|\b(?:his|her|their|my|your|our))\s+(?:[\w-]+\s+)?faces?\b`,
    String.raw`\b(?:${EXPRESSION}|${BEING})\s+faces?\b`,
    String.raw`\bfaces?\s+of\s+(?:(?:a|an|the|some|young|old|little|smiling|happy)\s+)*(?:${BEING}|crowds?|friends?|famil(?:y|ies))\b`,
    String.raw`\bwith\s+(?:a\s+)?faces?\b`,
    String.raw`\bfaces?\s+(?:with|and)\s+(?:[\w-]+\s+){0,2}(?:eyes|smiles?|noses?|mouths?|cheeks|ears)\b`,
  ].join('|'),
  'i',
);
/** The person a prompt asks for, if any (after negated lists are dropped). Exported for the tests. */
export function personIn(prompt: string): string | undefined {
  const p = asked(prompt);
  return (PEOPLE.exec(p) ?? FACE.exec(p) ?? HAND.exec(p))?.[0];
}
const HEX_IN_PROMPT = /#[0-9a-f]{3,6}\b/i;
const INSTRUCTURE_NAMES = /\b(4D|5E|waves of blue|crayons|superheroes)\b/i;

/* ------------------------------------------------------------------ colour helpers */

const up = (c: string) => c.toUpperCase();

/** CIE L*a*b* (D65) of a hex colour, for perceptual distance. */
function lab(hex: string): [number, number, number] {
  const n = parseInt(hex.slice(1), 16);
  const [r, g, b] = [(n >> 16) & 255, (n >> 8) & 255, n & 255].map((v) => {
    const c = v / 255;
    return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  });
  const xyz = [
    (0.4124 * r + 0.3576 * g + 0.1805 * b) / 0.95047,
    0.2126 * r + 0.7152 * g + 0.0722 * b,
    (0.0193 * r + 0.1192 * g + 0.9505 * b) / 1.08883,
  ].map((t) => (t > 0.008856 ? Math.cbrt(t) : 7.787 * t + 16 / 116));
  return [116 * xyz[1] - 16, 500 * (xyz[0] - xyz[1]), 200 * (xyz[1] - xyz[2])];
}
const deltaE = (a: string, b: string) => {
  const [x, y] = [lab(a), lab(b)];
  return Math.hypot(x[0] - y[0], x[1] - y[1], x[2] - y[2]);
};
/** How alike two colourways look: mean ΔE over `keys`. */
export function paletteDistance(a: Palette, b: Palette, keys: PaletteKey[] = VISIBLE_KEYS): number {
  if (!keys.every((k) => HEX6.test(a[k] ?? '') && HEX6.test(b[k] ?? ''))) return Infinity;
  return keys.reduce((s, k) => s + deltaE(a[k], b[k]), 0) / keys.length;
}

/** Moves a colour's HSL lightness (hue kept) until `ok` holds, closest first. */
function shiftLightness(c: string, ok: (x: string) => boolean): string {
  if (ok(c)) return c;
  const [h, s, l] = hexToHsl(c);
  const steps = Array.from({ length: 401 }, (_, i) => i / 400).sort((a, b) => Math.abs(a - l) - Math.abs(b - l));
  for (const L of steps) {
    const x = hslToHex(h, s, L);
    if (ok(x)) return x;
  }
  return c;
}

interface PaletteProblem {
  rule: string;
  message: string;
  /** The colours a fix for this problem would change. */
  keys: PaletteKey[];
}

/** Every theme colour rule a (valid-hex) palette breaks: the AA pairs plus the theme-only thresholds. */
export function paletteProblems(p: Palette): PaletteProblem[] {
  const out: PaletteProblem[] = [];
  const lum = (k: PaletteKey) => luminance(p[k]).toFixed(3);
  for (const r of checkPalette(p).filter((r) => !r.pass))
    out.push({ rule: 'palette-contrast', message: `${p.id}: ${r.fg} ${up(p[r.fg])} on ${r.bg} ${up(p[r.bg])} = ${r.ratio.toFixed(2)}:1, needs ${AA}`, keys: [r.fg, r.bg] });
  if (luminance(p.background) <= MIN_BACKGROUND_LUM)
    out.push({ rule: 'palette-background', message: `${p.id}: background ${up(p.background)} luminance ${lum('background')} must be > ${MIN_BACKGROUND_LUM} (Canvas pages are white)`, keys: ['background'] });
  if (luminance(p.surface) <= MIN_SURFACE_LUM)
    out.push({ rule: 'palette-surface', message: `${p.id}: surface ${up(p.surface)} luminance ${lum('surface')} must be > ${MIN_SURFACE_LUM} (a light tinted panel)`, keys: ['surface'] });
  for (const k of ['text', 'onSurface'] as const) {
    const r = contrastRatio(p[k], '#ffffff');
    if (r < DARK_TEXT_ON_WHITE) out.push({ rule: 'palette-dark-text', message: `${p.id}: ${k} ${up(p[k])} is ${r.toFixed(2)}:1 on white, needs ${DARK_TEXT_ON_WHITE} (dark-mode tolerance)`, keys: [k] });
  }
  if (luminance(p.primary) >= MAX_PRIMARY_LUM)
    out.push({ rule: 'palette-primary-deep', message: `${p.id}: primary ${up(p.primary)} luminance ${lum('primary')} must be < ${MAX_PRIMARY_LUM} (a deep band colour, not a mid-tone)`, keys: ['primary'] });
  return out;
}

/**
 * The palette with every theme colour rule met, changing as little as possible. A theme-level step around
 * core's fixPalette (which only knows AA and a 0.85 near-white, the bar for teachers' own colours): deepen
 * primary, lighten background/surface to the theme thresholds, fixPalette for the AA pairs, then darken
 * text/onSurface to 7:1 on white. Repeats until paletteProblems() is empty; null if it never gets there.
 */
export function suggestPalette(p: Palette): Palette | null {
  let q: Palette = { ...p };
  for (let round = 0; round < 4; round++) {
    if (!paletteProblems(q).length) return q;
    q.primary = shiftLightness(q.primary, (c) => luminance(c) < MAX_PRIMARY_LUM);
    q.background = shiftLightness(q.background, (c) => luminance(c) > MIN_BACKGROUND_LUM);
    q.surface = shiftLightness(q.surface, (c) => luminance(c) > MIN_SURFACE_LUM);
    q = fixPalette(q);
    q.text = nudgeLightness(q.text, ['#ffffff', q.background, q.surface], DARK_TEXT_ON_WHITE) ?? q.text;
    q.onSurface = nudgeLightness(q.onSurface, ['#ffffff', q.surface], DARK_TEXT_ON_WHITE) ?? q.onSurface;
  }
  return paletteProblems(q).length ? null : q;
}

/** The keys `suggestPalette` changed, upper-case like the theme files; null when no passing fix was found. */
function paletteChanges(p: Palette): Partial<Record<PaletteKey, string>> | null {
  const s = suggestPalette(p);
  if (!s) return null;
  const set: Partial<Record<PaletteKey, string>> = {};
  for (const k of PALETTE_KEYS) if (s[k].toLowerCase() !== p[k].toLowerCase()) set[k] = up(s[k]);
  return set;
}

/* ------------------------------------------------------------------ lintTheme */

const lucideDir = () => path.join(path.dirname(createRequire(import.meta.url).resolve('lucide-static/package.json')), 'icons');
const lucideExists = (name: string) => existsSync(path.join(lucideDir(), `${name}.svg`));
const words = (s: string) => s.trim().split(/\s+/).filter(Boolean).length;

/** Every string in the spec that still says TODO (from `theme:new`). */
function todos(x: unknown, at: string, out: string[]) {
  if (typeof x === 'string') {
    if (/\bTODO\b/.test(x)) out.push(at);
  } else if (Array.isArray(x)) x.forEach((v, i) => todos(v, `${at}[${i}]`, out));
  else if (x && typeof x === 'object') for (const [k, v] of Object.entries(x)) todos(v, at ? `${at}.${k}` : k, out);
}

export function lintTheme(entry: ThemeEntry, opts: LintOptions = {}): LintFinding[] {
  const out: LintFinding[] = [];
  const t = entry.theme;
  const err = (rule: string, message: string, extra: Partial<LintFinding> = {}) => out.push({ rule, severity: 'error', message, ...extra });
  const warn = (rule: string, message: string, extra: Partial<LintFinding> = {}) => out.push({ rule, severity: 'warn', message, ...extra });
  const iconExists = opts.iconExists ?? lucideExists;

  // ---- metadata
  if (!KEBAB.test(t.id)) err('id-kebab', `id "${t.id}" is not kebab-case (it becomes the asset folder name)`);
  const left: string[] = [];
  todos({ ...entry, theme: { ...t, images: [] } }, '', left); // image prompts repeat art.*, so skip them
  if (left.length) err('todo', `still has TODO in ${left.slice(0, 8).join(', ')}${left.length > 8 ? ` and ${left.length - 8} more` : ''}`);
  if (t.description.length <= 20) err('metadata', 'description is 20 characters or fewer');
  if (!t.tags.length) err('metadata', 'no tags');
  if (!t.audiences.length) err('metadata', 'no audiences');
  if (new Set(t.audiences).size !== t.audiences.length) err('metadata', `audiences repeat: ${t.audiences.join(', ')}`);
  if (entry.style.length <= 3) err('metadata', 'style (the named art direction) is missing');
  if (entry.colourBlind.length <= 20) err('metadata', 'colourBlind note is missing or too short to say how red vs green is avoided');

  // ---- palettes
  if (t.palettes.length !== 3) err('palette-count', `has ${t.palettes.length} palettes, needs 3`);
  if (new Set(t.palettes.map((p) => p.id)).size !== t.palettes.length) err('palette-ids', 'palette ids repeat');
  for (const p of t.palettes) {
    const bad = PALETTE_KEYS.filter((k) => !HEX6.test(p[k] ?? ''));
    if (bad.length) {
      err('palette-hex', `${p.id}: not a 6-digit hex: ${bad.map((k) => `${k} "${p[k]}"`).join(', ')}`);
      continue;
    }
    const set = paletteChanges(p);
    // The fix is the whole re-linted suggestion, led by the keys this rule is about, so applying it passes.
    const fixFor = (keys: PaletteKey[]): Partial<LintFinding> => {
      if (!set) return { fix: `no automatic fix keeps ${p.id}'s hues; pick a deeper primary or darker text by hand` };
      const all = Object.keys(set) as PaletteKey[];
      const use = [...keys.filter((k) => set[k]), ...all.filter((k) => !keys.includes(k))];
      if (!use.length) return {};
      return {
        fix: `in ${p.id} use ${use.map((k) => `${k} ${set[k]} (was ${up(p[k])})`).join(', ')}`,
        palettePatch: { palette: p.id, set },
      };
    };
    for (const pr of paletteProblems(p)) err(pr.rule, pr.message, fixFor(pr.keys));
    // Dark mode: hand-tuned colours must be hex, and the dark twin must pass (it is fixed, so this guards the fixer).
    const badDark = Object.entries(p.dark ?? {}).filter(([k, v]) => !(PALETTE_KEYS as readonly string[]).includes(k) || !HEX6.test(v ?? ''));
    if (badDark.length) err('palette-dark-hex', `${p.id}: dark colours must be 6-digit hex palette keys: ${badDark.map(([k, v]) => `${k} "${v}"`).join(', ')}`);
    const darkFails = checkPalette(themeDarkPalette(p)).filter((r) => !r.pass);
    if (darkFails.length) err('palette-dark', `${p.id}: dark colourway fails ${darkFails.map((r) => `${r.id} ${r.ratio.toFixed(2)}:1`).join(', ')}`, { fix: 'hand-tune the palette\'s `dark` colours' });
    // Links underlined by Canvas still need to stand apart from body text by more than hue alone.
    const lt = contrastRatio(p.link, p.text);
    if (lt < LINK_VS_TEXT)
      warn('link-like-text', `${p.id}: link ${up(p.link)} vs body text ${up(p.text)} is ${lt.toFixed(2)}:1; links are hard to tell from text`, {
        fix: 'give link a lighter, more saturated shade of the brand hue (it still needs 4.5:1 on background and surface)',
      });
  }
  for (let i = 0; i < t.palettes.length; i++)
    for (let j = i + 1; j < t.palettes.length; j++) {
      const [a, b] = [t.palettes[i], t.palettes[j]];
      const d = paletteDistance(a, b, MAIN_KEYS);
      if (d < SIMILAR_IN_THEME) {
        const each = MAIN_KEYS.map((k) => `${k} ${up(a[k])} vs ${up(b[k])} ΔE ${HEX6.test(a[k]) && HEX6.test(b[k]) ? deltaE(a[k], b[k]).toFixed(1) : '?'}`).join(', ');
        // Change the later one: [0] matches the art, and taste.md's order is signature, quieter, other hue family.
        warn('palette-similar', `colourways ${a.id} and ${b.id} look alike: ${each} (mean ΔE ${d.toFixed(1)}, warning below ${SIMILAR_IN_THEME})`, {
          fix: `change ${b.id}, not ${a.id}${i === 0 ? ' (it matches the art)' : ''}: move its primary to another hue family (or make it clearly quieter) and rebuild its secondary, surface, text and link from the new primary until the mean ΔE of primary and secondary is ${SIMILAR_IN_THEME}+`,
        });
      }
    }
  // Some layouts never draw the accent (ACCENT_IN_LAYOUT). That matters when the theme relies on it: the same
  // accent in every colourway (taste.md's signature spark) that no other page colour stands in for.
  const p0 = t.palettes[0];
  const signatureAccent = t.palettes.length > 1 && new Set(t.palettes.map((p) => p.accent?.toUpperCase())).size === 1;
  if (p0 && signatureAccent && !ACCENT_IN_LAYOUT[t.defaultLayout] && [p0.accent, p0.primary, p0.secondary, p0.link].every((c) => HEX6.test(c ?? ''))) {
    const nearest = Math.min(...(['primary', 'secondary', 'link'] as const).map((k) => deltaE(p0.accent, p0[k])));
    if (nearest >= ACCENT_DISTINCT)
      warn('accent-hidden', `defaultLayout ${t.defaultLayout} never draws the signature accent ${up(p0.accent)} (the same in every colourway), and no other page colour is close to it (nearest ΔE ${nearest.toFixed(0)}), so the spark only shows in the art`, {
        fix: `fine if the art carries it; otherwise use a layout that draws it (${Object.entries(ACCENT_IN_LAYOUT).filter(([, v]) => v).map(([k]) => k).join(', ')})`,
      });
  }

  // ---- images
  const ids = t.images.map((i) => i.id).sort();
  const want = Object.keys(IMAGE_SPEC).sort();
  if (ids.join() !== want.join()) err('images-set', `image ids are [${ids.join(', ')}], need [${want.join(', ')}]`);
  for (const img of t.images) {
    const spec = IMAGE_SPEC[img.id as keyof typeof IMAGE_SPEC];
    if (!spec) continue;
    const [kind, w, h] = spec;
    if (img.kind !== kind) err('image-kind', `${img.id}: kind ${img.kind}, needs ${kind}`);
    if (img.width !== w || img.height !== h) err('image-size', `${img.id}: ${img.width}×${img.height}, needs ${w}×${h}`);
    if (img.file !== `${img.id}.webp`) err('image-file', `${img.id}: file "${img.file}", needs "${img.id}.webp"`);
    if (img.kind !== 'spot' && img.alt !== '') err('image-alt', `${img.id} is decorative, so alt must be ''`);
    if (img.alt.length >= 120) err('image-alt', `${img.id}: alt text is ${img.alt.length} characters, keep it under 120`);
  }
  if (t.images.find((i) => i.id === 'spot-instructor')?.alt === '') err('image-alt', 'spot-instructor needs alt text (it stands in for the teacher)');
  const assetDir = opts.assetDir ?? DESIGN_ASSET_DIR;
  if (assetDir) {
    const missing = t.images.flatMap((img) => [img.file, at2x(img.file)]).filter((f) => !existsSync(path.join(assetDir, t.id, f)));
    if (missing.length)
      err('image-asset', `art missing under public/design-assets/themes/${t.id}/: ${missing.join(', ')}`, { fix: 'generate the art (WebP at 1x and @2x) or pass assetDir: false while drafting' });
  }

  // ---- prompts
  const medium = entry.art.medium.replace(/\.$/, '');
  const palette = entry.art.palette.replace(/\.$/, '');
  const mood = entry.art.mood.replace(/\.$/, '');
  for (const img of t.images) {
    const p = asked(img.prompt);
    if (!img.prompt.startsWith(medium)) err('prompt-medium', `${img.id}: prompt does not start with the medium sentence`);
    if (!img.prompt.includes(palette)) err('prompt-palette', `${img.id}: prompt is missing the palette sentence`);
    if (!img.prompt.includes(mood)) err('prompt-mood', `${img.id}: prompt is missing the mood sentence`);
    const m1 = LETTERING.exec(p);
    if (m1) err('prompt-lettering', `${img.id}: asks for lettering ("${m1[0]}"); the pipeline already adds "no text"`, { fix: 'describe the object without words, numbers or labels' });
    const m2 = LETTERING_OBJECTS.exec(p);
    if (m2) err('prompt-lettering-object', `${img.id}: "${m2[0]}" invites lettering (README rule 3)`, { fix: 'pick an object that carries no writing' });
    if (HEX_IN_PROMPT.test(p)) err('prompt-hex', `${img.id}: hex code in the prompt gets painted as a swatch; use colour words`);
    if (p.length >= MAX_PROMPT) err('prompt-length', `${img.id}: prompt is ${p.length} characters, keep it under ${MAX_PROMPT}`);
    const m3 = personIn(img.prompt);
    if (m3) err('prompt-people', `${img.id}: asks for a person ("${m3}"); spots and banners never show people or faces`, { fix: 'show an object or scene that stands in for the person' });
  }
  for (const img of t.images.filter((i) => i.kind === 'banner'))
    if (!/\bleft\b/.test(img.prompt)) err('banner-left', `${img.id}: banner prompt must say what sits on the left (the calm area under the title)`);
  for (const [id, s] of [['banner', entry.art.banner], ['banner-alt', entry.art.bannerAlt]] as const) {
    if (!/\bright\b/.test(s)) warn('banner-right', `${id}: subject doesn't say "right"; Canvas titles sit on the left, so the subject belongs on the right`);
    if (words(s) < MIN_BANNER_WORDS) warn('prompt-vague', `${id}: subject is ${words(s)} words; name the subject, where it sits and what fills the calm left side`);
  }
  const spots = [['spot-welcome', entry.art.welcome.subject], ['spot-instructor', entry.art.instructor.subject], ...entry.art.cards.map((c, i) => [`card-${i + 1}`, c])];
  for (const [id, s] of spots)
    if (words(s) < MIN_SUBJECT_WORDS) warn('prompt-vague', `${id}: subject "${s}" is ${words(s)} words; one concrete object with a detail reads better`);
  if (new Set(entry.art.cards.map((c) => c.toLowerCase().trim())).size < entry.art.cards.length) warn('cards-repeat', 'two card subjects are the same');

  // ---- icons
  for (const slot of SLOTS) {
    const name = t.icons?.[slot];
    if (!name) err('icon-missing', `no icon for the ${slot} slot`);
    else if (!iconExists(name)) err('icon-missing', `${slot}: Lucide icon "${name}" does not exist`, { fix: 'browse node_modules/lucide-static/icons' });
  }

  // ---- fonts
  const elementary = t.audiences.some((a) => a === 'early-elementary' || a === 'upper-elementary');
  if (elementary && t.headingFont !== undefined)
    err('heading-font-elementary', 'K-5 themes leave headingFont undefined (young readers get the default face, or their own reading font)');
  else if (t.headingFont !== undefined) {
    if (!(Object.values(FONTS) as string[]).includes(t.headingFont)) err('heading-font', `headingFont is not one of FONTS: ${t.headingFont}`);
    if (t.headingFont.includes('"')) err('heading-font', 'headingFont uses double quotes; it must survive inside style="…"');
  }
  return out;
}

/* ------------------------------------------------------------------ lintCatalogue */

const ESSENTIALS = [26, 32] as const;
const TIER_MINIMUM: Record<Audience, number> = {
  'early-elementary': 5,
  'upper-elementary': 5,
  secondary: 5,
  'higher-ed': 6,
  professional: 3,
  specialty: 4,
};
const ANY_TIER_MINIMUM = 6;
const SPECIALTY_AREAS = ['counseling', 'sel', 'library', 'athletics', 'cte', 'healthcare', 'digital-literacy'];

export function lintCatalogue(entries: ThemeEntry[]): LintFinding[] {
  const out: LintFinding[] = [];
  const themes = entries.map((e) => e.theme);
  const err = (rule: string, message: string, ids: string[]) => out.push({ rule, severity: 'error', message, themes: ids });

  const dupes = <K>(key: (t: (typeof themes)[number]) => K) => {
    const by = new Map<K, string[]>();
    for (const t of themes) by.set(key(t), [...(by.get(key(t)) ?? []), t.id]);
    return [...by].filter(([, ids]) => ids.length > 1);
  };
  for (const [id, ids] of dupes((t) => t.id)) err('id-unique', `id "${id}" is used ${ids.length} times`, ids);
  for (const [name, ids] of dupes((t) => t.name)) err('name-unique', `name "${name}" is used by ${ids.join(', ')}`, ids);
  for (const t of themes) if (INSTRUCTURE_NAMES.test(t.name)) err('name-instructure', `"${t.name}" borrows an Instructure theme name`, [t.id]);

  const essentials = themes.filter((t) => (t.collection ?? 'essentials') === 'essentials');
  if (essentials.length < ESSENTIALS[0] || essentials.length > ESSENTIALS[1])
    err('essentials-count', `Essentials has ${essentials.length} themes; keep it between ${ESSENTIALS[0]} and ${ESSENTIALS[1]}`, essentials.map((t) => t.id));
  for (const a of AUDIENCES) {
    const made = themes.filter((t) => t.audiences[0] === a).length;
    if (made < TIER_MINIMUM[a]) err('tier-coverage', `${made} themes are made first for ${a}; needs ${TIER_MINIMUM[a]}`, []);
    const any = themes.filter((t) => t.audiences.includes(a)).length;
    if (any < ANY_TIER_MINIMUM) err('tier-coverage', `${any} themes list ${a}; needs ${ANY_TIER_MINIMUM}`, []);
  }
  const specialtyTags = new Set(themes.filter((t) => t.audiences.includes('specialty')).flatMap((t) => t.tags));
  for (const area of SPECIALTY_AREAS) if (!specialtyTags.has(area)) err('specialty-coverage', `no specialty theme is tagged "${area}"`, []);

  // Near-identical colourways in two different themes: the gallery shows them side by side.
  const all = themes.flatMap((t) => t.palettes.map((p) => ({ t, p })));
  for (let i = 0; i < all.length; i++)
    for (let j = i + 1; j < all.length; j++) {
      const [a, b] = [all[i], all[j]];
      if (a.t.id === b.t.id) continue;
      const d = paletteDistance(a.p, b.p);
      if (d < SIMILAR_ACROSS_THEMES)
        out.push({
          rule: 'palette-duplicate',
          severity: 'warn',
          message: `${a.t.id}/${a.p.id} and ${b.t.id}/${b.p.id} are nearly the same colourway (mean ΔE ${d.toFixed(1)})`,
          fix: 'shift the hue or depth of one of them',
          themes: [a.t.id, b.t.id],
        });
    }
  return out;
}

/** "error  palette-contrast  message\n        fix: …" — one finding, for test failures and the CLI. */
export function formatFinding(f: LintFinding): string {
  return `${f.severity === 'error' ? 'error' : 'warn '}  ${f.rule}  ${f.message}${f.fix ? `\n         fix: ${f.fix}` : ''}`;
}
