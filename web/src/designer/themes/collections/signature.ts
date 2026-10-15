import type { ThemeEntry } from '../define';
import gildedAtlas from '../themes/signature/gilded-atlas';
import inkAndWash from '../themes/signature/ink-and-wash';
import glasshouse from '../themes/signature/glasshouse';
import decoSkyline from '../themes/signature/deco-skyline';
import midCentury from '../themes/signature/mid-century';
import kintsugi from '../themes/signature/kintsugi';
import marbleAndBrass from '../themes/signature/marble-and-brass';
import observatory from '../themes/signature/observatory';
import mossAndStone from '../themes/signature/moss-and-stone';
import marbledPaper from '../themes/signature/marbled-paper';

/** 'signature' collection. Theme files live in ../themes/signature/. Owned by one design agent; keep the order intentional. */
export const ENTRIES: ThemeEntry[] = [
  gildedAtlas, inkAndWash, decoSkyline, kintsugi, marbleAndBrass,
  observatory, glasshouse, midCentury, mossAndStone, marbledPaper,
];
