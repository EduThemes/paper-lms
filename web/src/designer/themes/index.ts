import type { Audience, CollectionId, Observance, Region, Theme } from '@/designer/engine';
import type { ThemeEntry } from './define';

// Early elementary (K-2)
import paperMeadow from './themes/paper-meadow';
import clayCorner from './themes/clay-corner';
import puddleJump from './themes/puddle-jump';
import feltBoard from './themes/felt-board';
import toyTown from './themes/toy-town';
// Upper elementary (3-5)
import stickerTrail from './themes/sticker-trail';
import pixelQuest from './themes/pixel-quest';
import fieldNotes from './themes/field-notes';
import risoRocket from './themes/riso-rocket';
import tidePool from './themes/tide-pool';
// Secondary (6-12)
import isometricLab from './themes/isometric-lab';
import chalkAndSlate from './themes/chalk-and-slate';
import nocturne from './themes/nocturne';
import bauhausBlocks from './themes/bauhaus-blocks';
import cutAndPaste from './themes/cut-and-paste';
// Higher ed
import swissGrid from './themes/swiss-grid';
import botanicalLinework from './themes/botanical-linework';
import draftingTable from './themes/drafting-table';
import campusModern from './themes/campus-modern';
import woodcutPress from './themes/woodcut-press';
import contour from './themes/contour';
// Professional
import softFocus from './themes/soft-focus';
import paperFold from './themes/paper-fold';
import terrazzo from './themes/terrazzo';
// Specialty
import stillWater from './themes/still-water';
import readingNook from './themes/reading-nook';
import gameDay from './themes/game-day';
import shopFloor from './themes/shop-floor';
import careLine from './themes/care-line';
import circuitGarden from './themes/circuit-garden';

import { ENTRIES as signature } from './collections/signature';
import { ENTRIES as arcade } from './collections/arcade';
import { ENTRIES as heroesComics } from './collections/heroes-comics';
import { ENTRIES as cartoons } from './collections/cartoons';
import { ENTRIES as holidays } from './collections/holidays';
import { ENTRIES as specialDays } from './collections/special-days';

export { FONTS, defineTheme, buildImages } from './define';
export type { ArtDirection, SpotSpec, ThemeEntry, ThemeSpec } from './define';
export { previewHtml } from './preview';

/** Every theme with its design notes (art direction, colour-blind consideration). */
export const THEME_ENTRIES: ThemeEntry[] = [
  paperMeadow, clayCorner, puddleJump, feltBoard, toyTown,
  stickerTrail, pixelQuest, fieldNotes, risoRocket, tidePool,
  isometricLab, chalkAndSlate, nocturne, bauhausBlocks, cutAndPaste,
  swissGrid, botanicalLinework, draftingTable, campusModern, woodcutPress, contour,
  softFocus, paperFold, terrazzo,
  stillWater, readingNook, gameDay, shopFloor, careLine, circuitGarden,
  ...signature, ...arcade, ...heroesComics, ...cartoons, ...holidays, ...specialDays,
];

export const THEMES: Theme[] = THEME_ENTRIES.map((e) => e.theme);

const byId = new Map(THEMES.map((t) => [t.id, t]));

export function getTheme(id: string): Theme | undefined {
  return byId.get(id);
}

/** Themes that list `audience`, those made primarily for it (listed first) ahead of crossovers. */
export function themesFor(audience: Audience): Theme[] {
  const matches = THEMES.filter((t) => t.audiences.includes(audience));
  return [...matches.filter((t) => t.audiences[0] === audience), ...matches.filter((t) => t.audiences[0] !== audience)];
}

/** Design notes for a theme: the named art style and its colour-blind consideration. */
export function themeNotes(id: string): Pick<ThemeEntry, 'style' | 'colourBlind' | 'art'> | undefined {
  const e = THEME_ENTRIES.find((x) => x.theme.id === id);
  return e && { style: e.style, colourBlind: e.colourBlind, art: e.art };
}

export const COLLECTIONS: { id: CollectionId; name: string; description: string }[] = [
  { id: 'essentials', name: 'Essentials', description: 'Everyday looks for every grade, from K-2 to professional learning.' },
  { id: 'signature', name: 'Signature', description: 'Premium, high-craft designs for courses that should feel special.' },
  { id: 'arcade', name: 'Arcade & Games', description: 'Pixel worlds, retro arcades and quest maps. Turn a course into a game.' },
  { id: 'heroes-comics', name: 'Heroes & Comics', description: 'Original superheroes, comic panels and graphic-novel ink.' },
  { id: 'cartoons', name: 'Cartoons & Storybooks', description: 'Friendly monsters, dinosaurs, robots and storybook worlds.' },
  { id: 'holidays', name: 'Holidays & Festivals', description: 'Celebrations from many faiths and cultures, made with care.' },
  { id: 'special-days', name: 'Special Days', description: 'School events, heritage months and awareness days through the year.' },
];

/** Collection a theme belongs to ('essentials' when unset). */
export function collectionOf(theme: Theme): CollectionId {
  return theme.collection ?? 'essentials';
}

export function themesIn(collection: CollectionId): Theme[] {
  return THEMES.filter((t) => collectionOf(t) === collection);
}

/** Holiday and special-day themes whose observance falls in `month` (1-12) or the next month, soonest first. */
/** The months an observance falls in for `region`, or its general months when the region isn't listed. */
export function observanceMonths(o: Observance, region?: Region): number[] {
  return (region && o.regionalMonths?.[region]) || o.months;
}

export function upcomingObservances(month: number, region?: Region): Theme[] {
  const next = (month % 12) + 1;
  const months = (t: Theme) => observanceMonths(t.observance!, region);
  const rank = (t: Theme) => (months(t).includes(month) ? 0 : 1);
  return THEMES.filter((t) => t.observance && (months(t).includes(month) || months(t).includes(next))).sort(
    (a, b) => rank(a) - rank(b),
  );
}
