import type { ThemeEntry } from '../define';
import quantitown from '../themes/arcade/quantitown';
import neonArcade from '../themes/arcade/neon-arcade';
import voxelWorld from '../themes/arcade/voxel-world';
import dungeonQuest from '../themes/arcade/dungeon-quest';
import starShooter from '../themes/arcade/star-shooter';
import kartRally from '../themes/arcade/kart-rally';
import gemPuzzle from '../themes/arcade/gem-puzzle';
import platformHop from '../themes/arcade/platform-hop';
import boardGameNight from '../themes/arcade/board-game-night';
import bossBattle from '../themes/arcade/boss-battle';

/** 'arcade' collection. Theme files live in ../themes/arcade/. Owned by one design agent; keep the order intentional. */
export const ENTRIES: ThemeEntry[] = [
  quantitown, starShooter, neonArcade, voxelWorld, gemPuzzle,
  bossBattle, dungeonQuest, platformHop, kartRally, boardGameNight,
];
