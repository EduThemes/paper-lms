/**
 * Light + dark in one page.
 *
 * The page is rendered twice through the same layout code, once per colourway. The two renders must have the
 * same structure (layouts branch on tone roles, never on colours), so they can be walked in parallel:
 *   - inside every style="…", a hex colour that differs becomes `var(--pd-cN, #light)` (one N per
 *     (light, dark) pair), and the stylesheet sets `--pd-cN: #dark` under `prefers-color-scheme: dark`;
 *   - an <img> whose src differs (an icon drawn in the page's colours) gets `data-pd-icon="N"` and the
 *     stylesheet swaps it with `content: url(<dark src>)`.
 * Anything else that differs means the renders diverged; the page then falls back to light only.
 *
 * Paper's stylesheet scoper rewrites `:root` to the page and the colour-scheme media query to its own
 * `html.dark` toggle (docs/page-designer/README.md, "Page stylesheet rules").
 */
import { decodeEntities } from '../html';
import type { Ctx } from './context';

/** The custom property a reader's chosen font is published in. Set only when the reader picked a font. */
export const READER_FONT_VAR = '--reading-font-choice';

export interface Merged {
  html: string;
  vars: Record<string, [string, string]>;
  icons: Record<string, string>;
}

const ATTR = / (style|src)="([^"]*)"/g;
const HEX = /#[0-9a-fA-F]{6}\b/g;

/** Merge a light and a dark render of the same page; null when they differ in anything but colours and icon srcs. */
export function mergeRenders(light: string, dark: string): Merged | null {
  const a = [...light.matchAll(ATTR)];
  const b = [...dark.matchAll(ATTR)];
  if (a.length !== b.length) return null;
  const pairs = new Map<string, string>();
  const vars: Record<string, [string, string]> = {};
  const icons: Record<string, string> = {};
  const varFor = (l: string, d: string) => {
    const key = `${l}|${d}`;
    let name = pairs.get(key);
    if (!name) {
      name = `--pd-c${pairs.size + 1}`;
      pairs.set(key, name);
      vars[name] = [l, d];
    }
    return name;
  };
  let out = '';
  let pa = 0;
  let pb = 0;
  let iconN = 0;
  for (let i = 0; i < a.length; i++) {
    const [ma, mb] = [a[i], b[i]];
    if (light.slice(pa, ma.index) !== dark.slice(pb, mb.index) || ma[1] !== mb[1]) return null;
    out += light.slice(pa, ma.index);
    if (ma[1] === 'style') {
      const style = mergeStyle(ma[2], mb[2], varFor);
      if (style === null) return null;
      out += ` style="${style}"`;
    } else if (ma[2] === mb[2]) out += ma[0];
    else {
      iconN++;
      icons[String(iconN)] = decodeEntities(mb[2]);
      out += ` data-pd-icon="${iconN}" src="${ma[2]}"`;
    }
    pa = ma.index + ma[0].length;
    pb = mb.index + mb[0].length;
  }
  if (light.slice(pa) !== dark.slice(pb)) return null;
  return { html: out + light.slice(pa), vars, icons };
}

function mergeStyle(la: string, lb: string, varFor: (l: string, d: string) => string): string | null {
  const textA = la.split(HEX);
  const textB = lb.split(HEX);
  if (textA.length !== textB.length || textA.some((t, i) => t !== textB[i])) return null;
  const hexA = la.match(HEX) ?? [];
  const hexB = lb.match(HEX) ?? [];
  let out = textA[0];
  for (let i = 0; i < hexA.length; i++) {
    const [l, d] = [hexA[i].toLowerCase(), hexB[i].toLowerCase()];
    out += (l === d ? l : `var(${varFor(l, d)}, ${l})`) + textA[i + 1];
  }
  return out;
}

/** `var(--pd-cN, #hex)` → `#hex`: the light page with no custom properties (what Canvas export does). */
export function flattenDesignVars(html: string): string {
  return html.replace(/var\(--pd-[a-z0-9-]+,\s*(#[0-9a-fA-F]{3,8})\)/g, '$1');
}

/** A font stack safe inside a declaration (theme data, but never let it close a rule). */
const cleanStack = (f: string) => f.replace(/[;{}<>\\]/g, '').replace(/"/g, "'").trim();

/** CSS-safe url("…") for a same-origin or https src. */
const cssUrl = (src: string) => `url("${src.replace(/["\\\n\r]/g, c => `\\${c.charCodeAt(0).toString(16)} `)}")`;

const TILE = ':root a[data-pd-tile]';
const BUTTON = ':root a[data-pd-button]';

/**
 * The designer stylesheet for one page, written against `:root` (the page). Small and deterministic:
 * focus ring, hover lift, reduced motion, the heading font, and the dark-mode block.
 */
export function designCss(light: Ctx, dark: Ctx | null, merged: Merged | null): string {
  const rules: string[] = [];
  const focusLight = light.tones.page.heading;
  rules.push(`:root{--pd-focus:${focusLight}}`);
  if (light.headingFont)
    rules.push(`:root h2,:root h3,:root h4,:root [data-pd-font]{font-family:var(${READER_FONT_VAR}, ${cleanStack(light.headingFont)})}`);
  rules.push(
    `${TILE},${BUTTON}{transition:box-shadow .15s ease,transform .15s ease}`,
    `${TILE}:hover,${BUTTON}:hover{box-shadow:0 6px 16px rgba(0,0,0,.14);transform:translateY(-2px)}`,
    `:root a[data-pd-tile="list"]:hover,:root a[data-pd-tile="index"]:hover{box-shadow:none;transform:none;text-decoration:underline}`,
    `:root a:focus-visible{outline:3px solid var(--pd-focus);outline-offset:3px}`,
    `@media (prefers-reduced-motion: reduce){${TILE},${BUTTON}{transition:none}${TILE}:hover,${BUTTON}:hover{transform:none}}`,
  );
  if (dark && merged) {
    const decl = Object.entries(merged.vars).map(([k, [, d]]) => `${k}:${d}`);
    decl.push(`--pd-focus:${dark.tones.page.heading}`);
    const inner = [`:root{${decl.join(';')}}`];
    for (const [n, src] of Object.entries(merged.icons)) inner.push(`:root img[data-pd-icon="${n}"]{content:${cssUrl(src)}}`);
    inner.push(`:root img[data-pd-art]{filter:brightness(.92)}`, `${TILE}:hover,${BUTTON}:hover{box-shadow:0 6px 18px rgba(0,0,0,.5)}`);
    rules.push(`@media (prefers-color-scheme: dark){${inner.join('')}}`);
  }
  return rules.join('\n') + '\n';
}
