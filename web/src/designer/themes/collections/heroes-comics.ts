import type { ThemeEntry } from '../define';
import comicPop from '../themes/heroes-comics/comic-pop';
import rooftopHeroes from '../themes/heroes-comics/rooftop-heroes';
import sidekickSquad from '../themes/heroes-comics/sidekick-squad';
import nebulaWatch from '../themes/heroes-comics/nebula-watch';
import gadgetLair from '../themes/heroes-comics/gadget-lair';
import inkNoir from '../themes/heroes-comics/ink-noir';
import mangaSketch from '../themes/heroes-comics/manga-sketch';

/** 'heroes-comics' collection. Theme files live in ../themes/heroes-comics/. Owned by one design agent; keep the order intentional. */
// Order: the flagship comic look first, then youngest to oldest audience.
export const ENTRIES: ThemeEntry[] = [comicPop, sidekickSquad, rooftopHeroes, nebulaWatch, gadgetLair, mangaSketch, inkNoir];
