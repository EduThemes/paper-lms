// Generates golden.json for internal/designassets: what the TypeScript
// engine's renderIconSvg returns, so the Go port can be checked byte for
// byte. Run through `make designer-icons` (esbuild bundles this file
// against web/node_modules, then node runs it and the output is written
// to testdata/golden.json).
import { createHash } from 'node:crypto';
import { iconNames, renderIconSvg } from '../../../web/src/designer/engine/icons';
import type { IconStyle } from '../../../web/src/designer/engine/types';

const styles: IconStyle[] = ['solid-circle', 'soft-square', 'outline', 'duotone'];

// Names: plain, alias, upper case, punctuation, unknown, empty.
const names = [
  'star', 'book-open', 'graduation-cap', 'calendar-days', 'circle-dashed', 'pencil',
  'smile', 'help-circle', 'alert-triangle', 'Star', 'Book Open!', 'not-an-icon', '',
];

// fg / bg as the route receives them (no '#'), plus odd input.
const colours: [string, string][] = [
  ['1f2328', 'ffffff'],
  ['12355b', 'f5c518'],
  ['ABC', 'fff'],
  ['#0a0b0c', ' #FEDCBA '],
  ['zzz', ''],
];

const samples: { name: string; style: string; fg: string; bg: string; svg: string }[] = [];
for (const name of names) {
  for (const style of styles) {
    for (const [fg, bg] of colours) {
      samples.push({ name, style, fg, bg, svg: renderIconSvg(name, { style, fg, bg }) });
    }
  }
}

// Every icon in every style for one colour pair, as a digest.
const all = iconNames().sort();
const digests: Record<string, string> = {};
for (const style of styles) {
  const h = createHash('sha256');
  for (const name of all) h.update(renderIconSvg(name, { style, fg: '2a6f97', bg: 'fdf6e3' }) + '\n');
  digests[style] = h.digest('hex');
}

process.stdout.write(JSON.stringify({ icon_count: all.length, digest_colours: ['2a6f97', 'fdf6e3'], digests, samples }, null, 1) + '\n');
