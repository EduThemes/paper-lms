import type { LayoutId } from '../types';
import { cardGrid } from './card-grid';
import { classicBand } from './classic-band';
import { editorial } from './editorial';
import { playful } from './playful';
import type { LayoutModule } from './shared';
import { sidebar } from './sidebar';
import { splitHero } from './split-hero';
import { timeline } from './timeline';

export const LAYOUT_MODULES: Record<LayoutId, LayoutModule> = {
  'classic-band': classicBand,
  'split-hero': splitHero,
  'card-grid': cardGrid,
  sidebar,
  timeline,
  editorial,
  playful,
};

export const LAYOUTS: { id: LayoutId; name: string; description: string }[] = Object.values(LAYOUT_MODULES).map(m => ({ ...m.meta }));

export type { LayoutModule };
