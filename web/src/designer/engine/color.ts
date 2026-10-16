/** Colour maths: WCAG 2.x relative luminance + contrast, plus small helpers for picking readable pairs. */

/** Normalise "#abc", "abc", "#AABBCC" to "#aabbcc". Returns null when it isn't a hex colour. */
export function normHex(c: string | undefined | null): string | null {
  if (!c) return null;
  let h = c.trim().replace(/^#/, '').toLowerCase();
  if (/^[0-9a-f]{3}$/.test(h)) h = h.split('').map(x => x + x).join('');
  return /^[0-9a-f]{6}$/.test(h) ? `#${h}` : null;
}

export function hexToRgb(c: string): [number, number, number] {
  const h = normHex(c) ?? '#000000';
  return [parseInt(h.slice(1, 3), 16), parseInt(h.slice(3, 5), 16), parseInt(h.slice(5, 7), 16)];
}

export function rgbToHex([r, g, b]: [number, number, number]): string {
  const p = (n: number) => Math.max(0, Math.min(255, Math.round(n))).toString(16).padStart(2, '0');
  return `#${p(r)}${p(g)}${p(b)}`;
}

/** WCAG relative luminance (0..1). */
export function luminance(c: string): number {
  const [r, g, b] = hexToRgb(c).map(v => {
    const s = v / 255;
    return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

/** WCAG contrast ratio between two hex colours (1..21). Invalid input counts as black. */
export function contrastRatio(fg: string, bg: string): number {
  const a = luminance(fg);
  const b = luminance(bg);
  const [hi, lo] = a > b ? [a, b] : [b, a];
  return (hi + 0.05) / (lo + 0.05);
}

/** Linear mix in sRGB: t=0 → a, t=1 → b. */
export function mix(a: string, b: string, t: number): string {
  const x = hexToRgb(a);
  const y = hexToRgb(b);
  return rgbToHex([0, 1, 2].map(i => x[i] + (y[i] - x[i]) * t) as [number, number, number]);
}

/** First candidate reaching `min` on `bg`; otherwise the best of candidates + black/white. */
export function readable(bg: string, candidates: (string | undefined)[], min = 4.5): string {
  const list = candidates.map(normHex).filter((c): c is string => !!c);
  for (const c of list) if (contrastRatio(c, bg) >= min) return c;
  const all = [...list, '#000000', '#ffffff'];
  return all.reduce((best, c) => (contrastRatio(c, bg) > contrastRatio(best, bg) ? c : best), all[0]);
}

/** Push `fg` toward black or white (whichever gives more room) until it reaches `min` on `bg`. */
export function ensureContrast(fg: string, bg: string, min = 4.5): string {
  const f = normHex(fg) ?? '#000000';
  if (contrastRatio(f, bg) >= min) return f;
  const target = luminance(bg) > 0.4 ? '#000000' : '#ffffff';
  // Mid-luminance colours (≈0.18–0.4) can only reach `min` with black, so try the other way before giving up.
  const other = target === '#000000' ? '#ffffff' : '#000000';
  for (const to of [target, other]) {
    for (let t = 0.05; t <= 1.0001; t += 0.05) {
      const c = mix(f, to, t);
      if (contrastRatio(c, bg) >= min) return c;
    }
  }
  return contrastRatio(other, bg) > contrastRatio(target, bg) ? other : target;
}

/** Hex without '#', for URLs. */
export const bare = (c: string) => (normHex(c) ?? '#000000').slice(1);
