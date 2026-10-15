import { defineTheme, FONTS } from '../../define';

export default defineTheme({
  id: 'saturday-morning',
  name: 'Saturday Morning',
  description: '1930s rubber-hose animation with noodle limbs, pie-cut eyes and film grain, starring an original cast of dancing teapots, toasters and a jolly goose.',
  audiences: ['secondary', 'higher-ed'],
  tags: ['rubber-hose', 'vintage-cartoon', '1930s', 'retro', 'playful', 'film-grain'],
  defaultLayout: 'classic-band',
  iconStyle: 'solid-circle',
  headingFont: FONTS.georgia,
  corners: 'round',
  collection: 'cartoons',
  style: '1930s rubber-hose animation',
  colourBlind:
    'Built on ink black and cream, a pure lightness contrast; tomato red and mustard are decorative fills and are never set against each other to carry meaning.',
  palettes: [
    { id: 'matinee', name: 'Matinee', primary: '#1E1A17', onPrimary: '#FFFFFF', secondary: '#F4C95D', onSecondary: '#3A2A00', accent: '#D94A38', surface: '#FBF3E2', onSurface: '#1E1A17', background: '#FFFFFF', text: '#1E1A17', link: '#8C2A1C' },
    { id: 'sepia', name: 'Sepia Reel', primary: '#4A2E1B', onPrimary: '#FFFFFF', secondary: '#EAD7B5', onSecondary: '#3A2412', accent: '#C97B3B', surface: '#FAF4EA', onSurface: '#2A1C12', background: '#FFFFFF', text: '#2A1C12', link: '#7A3E14' },
    { id: 'two-strip', name: 'Two-Strip', primary: '#134B4E', onPrimary: '#FFFFFF', secondary: '#F6B3A3', onSecondary: '#4A1610', accent: '#E85D4A', surface: '#EFF6F5', onSurface: '#142224', background: '#FFFFFF', text: '#142224', link: '#134B4E' },
  ],
  icons: {
    start: 'clapperboard', syllabus: 'scroll', modules: 'film', assignments: 'pencil', discussions: 'messages-square',
    grades: 'star', help: 'life-buoy', instructor: 'projector', calendar: 'calendar-days', resources: 'music',
  },
  art: {
    medium:
      '1930s rubber-hose cartoon illustration with bouncy noodle limbs, pie-cut eyes, thick black ink lines, flat colour and soft vintage film grain',
    palette: 'Inky black and warm cream with tomato red and mustard yellow',
    mood: 'Bouncy, jazzy and good-humoured, an old cartoon with a brand-new cast',
    banner:
      'A jolly teapot and a round-bellied goose dancing on bare noodle legs beside a grinning sun with springy rays on the right, a calm plain cream ground with faint film grain across the left',
    bannerAlt:
      'A chugging little steam train with bouncy rubber-hose wheels and no face on the engine, puffing round clouds over a rolling hill on the right, plain calm cream sky across the left',
    welcome: { subject: 'A cheerful sunflower swaying with its leaves held out like noodle arms', alt: 'A dancing cartoon sunflower' },
    instructor: { subject: 'An old brass film projector with two round reels', alt: 'A vintage film projector' },
    cards: [
      'A hopping toaster popping out two slices of toast',
      'A flower pot bobbing along to music on springy legs',
      'A grand piano dancing on bouncy noodle legs',
    ],
  },
});
