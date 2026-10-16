import type { Theme } from '../../types';

/** Three deliberately different fixture themes (until @rmt/themes ships its catalogue). */
export const campusTerracotta: Theme = {
  id: 'campus-terracotta',
  name: 'Campus Terracotta',
  description: 'Paper-cut campus arches in deep teal and terracotta.',
  audiences: ['higher-ed', 'secondary'],
  tags: ['campus', 'warm'],
  defaultLayout: 'classic-band',
  palettes: [
    {
      id: 'teal',
      name: 'Teal & terracotta',
      primary: '#164c59',
      onPrimary: '#ffffff',
      secondary: '#a8442c',
      onSecondary: '#ffffff',
      accent: '#e8896b',
      surface: '#f8f1e7',
      onSurface: '#2a2522',
      background: '#ffffff',
      text: '#222b2f',
      link: '#0f5a69',
    },
  ],
  iconStyle: 'solid-circle',
  corners: 'soft',
  images: [
    { id: 'banner', kind: 'banner', file: 'banner.jpg', width: 1200, height: 300, alt: '', prompt: 'Paper-cut campus arches and stairs, teal and terracotta, no text' },
    { id: 'spot-books', kind: 'spot', file: 'spot-books.jpg', width: 600, height: 600, alt: 'A stack of books with a potted plant on top', prompt: 'Stack of books, no text' },
  ],
  icons: { modules: 'layers', help: 'life-buoy' },
};

export const sunnyStorybook: Theme = {
  id: 'sunny-storybook',
  name: 'Sunny Storybook',
  description: 'Paper-craft sunshine, clouds and crayons for the youngest learners.',
  audiences: ['early-elementary', 'upper-elementary'],
  tags: ['primary', 'bright'],
  defaultLayout: 'playful',
  palettes: [
    {
      id: 'sky',
      name: 'Sky & meadow',
      primary: '#1c5aa6',
      onPrimary: '#ffffff',
      secondary: '#2b7a2f',
      onSecondary: '#ffffff',
      accent: '#f4c02f',
      surface: '#fff4d6',
      onSurface: '#3a2d0c',
      background: '#ffffff',
      text: '#1d2939',
      link: '#1a55a0',
    },
  ],
  iconStyle: 'duotone',
  corners: 'round',
  images: [
    { id: 'banner', kind: 'banner', file: 'banner.jpg', width: 1200, height: 300, alt: 'An open storybook with crayons, stars and a smiling sun over green hills', prompt: 'Paper-craft storybook, no text' },
    { id: 'card-schoolhouse', kind: 'card', file: 'card-schoolhouse.jpg', width: 900, height: 604, alt: 'A little red schoolhouse on a hill', prompt: 'Red schoolhouse, no text' },
  ],
  icons: { modules: 'book-open', assignments: 'pencil', grades: 'star', discussions: 'smile', start: 'rocket', help: 'hand-helping' },
};

export const slateProfessional: Theme = {
  id: 'slate-professional',
  name: 'Slate Professional',
  description: 'Quiet slate and plum with a serif display face, for PD and graduate programmes.',
  audiences: ['professional', 'higher-ed'],
  tags: ['calm', 'serif'],
  defaultLayout: 'editorial',
  palettes: [
    {
      id: 'slate',
      name: 'Slate & plum',
      primary: '#2e3d4f',
      onPrimary: '#ffffff',
      secondary: '#5c3d7a',
      onSecondary: '#ffffff',
      accent: '#c9a227',
      surface: '#f1f3f6',
      onSurface: '#1f2933',
      background: '#ffffff',
      text: '#1f2328',
      link: '#2457a6',
    },
  ],
  iconStyle: 'outline',
  corners: 'square',
  headingFont: "Georgia, 'Times New Roman', serif",
  images: [
    { id: 'banner', kind: 'banner', file: 'banner.jpg', width: 1200, height: 300, alt: 'A paper-cut university town with a clock tower and wide stone steps', prompt: 'Campus town, no text' },
  ],
};

export const THEMES: Theme[] = [campusTerracotta, sunnyStorybook, slateProfessional];
