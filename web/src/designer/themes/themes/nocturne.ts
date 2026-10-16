import { defineTheme, FONTS } from '../define';

export default defineTheme({
  id: 'nocturne',
  name: 'Nocturne',
  description: 'Atmospheric gouache night scenes: aurora over a still lake, lanterns and comets. Calm and a little bit wondrous.',
  audiences: ['secondary'],
  tags: ['night-sky', 'gouache', 'atmospheric', 'calm'],
  defaultLayout: 'split-hero',
  iconStyle: 'duotone',
  headingFont: FONTS.lato,
  corners: 'soft',
  style: 'night-sky gouache',
  colourBlind:
    'The scheme is blue-violet with gold, which separates by lightness for protan, deutan and most tritan viewers.',
  palettes: [
    { id: 'midnight', name: 'Midnight', primary: '#1B2450', onPrimary: '#FFFFFF', secondary: '#B9E4E0', onSecondary: '#0D3532', accent: '#F2C14E', surface: '#EEF0F8', onSurface: '#161A2E', background: '#FFFFFF', text: '#161A2E', link: '#2F3D8C' },
    { id: 'dawn', name: 'First Light', primary: '#4A2557', onPrimary: '#FFFFFF', secondary: '#FFD6A8', onSecondary: '#4A2600', accent: '#F28C6B', surface: '#FBF0F2', onSurface: '#24172A', background: '#FFFFFF', text: '#24172A', link: '#6A2C78' },
    { id: 'polar', name: 'Polar', primary: '#0E3B4A', onPrimary: '#FFFFFF', secondary: '#D6E7FF', onSecondary: '#102C57', accent: '#7FE0C5', surface: '#EDF5F7', onSurface: '#13232A', background: '#FFFFFF', text: '#13232A', link: '#0E4F63' },
  ],
  icons: {
    start: 'moon-star', syllabus: 'scroll-text', modules: 'layers', assignments: 'feather', discussions: 'messages-square',
    grades: 'star', help: 'lamp', instructor: 'telescope', calendar: 'calendar-days', resources: 'sparkles',
  },
  art: {
    medium: 'Atmospheric gouache painting with soft dry-brush texture and small glowing highlights',
    palette: 'Midnight blue and violet with aurora teal and warm lantern gold',
    mood: 'Calm, spacious and quietly wondrous',
    banner: 'A wide green-teal aurora rippling over a still lake with tall pine silhouettes and a small cabin with one warm lit window on the right, calm deep starry sky fading to quiet blue on the left',
    bannerAlt: 'A large brass telescope on a hilltop beneath a crescent moon and a sweep of stars on the right, calm dark blue sky on the left',
    welcome: { subject: 'A glowing paper lantern', alt: 'A glowing lantern' },
    instructor: { subject: 'A brass telescope on a wooden tripod', alt: 'A telescope on a tripod' },
    cards: [
      'A crescent moon rising over dark pine trees',
      'Fireflies glowing above a meadow at night',
      'A comet crossing a starry sky above a hill',
    ],
  },
});
