import { defineTheme } from '../../define';

export default defineTheme({
  id: 'gem-puzzle',
  name: 'Gem Puzzle',
  description: 'Glossy cut gems lining up and bursting into sparkles on deep velvet. Match-three satisfaction without the clutter.',
  audiences: ['upper-elementary', 'secondary'],
  tags: ['gems', 'puzzle', 'match-three', 'vector', 'sparkle'],
  defaultLayout: 'card-grid',
  iconStyle: 'duotone',
  corners: 'round',
  collection: 'arcade',
  style: 'faceted vector gem art',
  colourBlind:
    'Each gem has its own cut (heart, square, round, teardrop, star), so shape tells them apart, never colour; ruby and emerald are never the only difference between two things.',
  palettes: [
    { id: 'plum-velvet', name: 'Plum Velvet', primary: '#3A1446', onPrimary: '#FFFFFF', secondary: '#F3C6E8', onSecondary: '#3A0E33', accent: '#2FB5A0', surface: '#F7F0FA', onSurface: '#22122A', background: '#FFFFFF', text: '#22122A', link: '#6A2385' },
    { id: 'sapphire', name: 'Sapphire', primary: '#12286A', onPrimary: '#FFFFFF', secondary: '#BFD3FF', onSecondary: '#0F2050', accent: '#FFB703', surface: '#EEF3FF', onSurface: '#111A33', background: '#FFFFFF', text: '#111A33', link: '#1F3FA8' },
    { id: 'emerald', name: 'Emerald', primary: '#0D3F33', onPrimary: '#FFFFFF', secondary: '#B8EBD5', onSecondary: '#0B3328', accent: '#E8505B', surface: '#ECF8F3', onSurface: '#10241E', background: '#FFFFFF', text: '#10241E', link: '#0F5E4A' },
  ],
  icons: {
    start: 'sparkles', syllabus: 'scroll', modules: 'grid-3x3', assignments: 'puzzle', discussions: 'messages-square',
    grades: 'gem', help: 'circle-help', instructor: 'diamond', calendar: 'calendar-days', resources: 'coins',
  },
  art: {
    medium: 'Polished faceted vector game art of glossy cut gemstones with crisp facets, bright glints and a soft inner glow',
    palette: 'Ruby, sapphire, emerald and amber jewel tones on deep plum',
    mood: 'Sparkling, satisfying and calm',
    banner: 'A cascade of large glossy cut gems, a heart-cut ruby, a square sapphire, a round emerald and a teardrop amber, tumbling in a sparkling arc on the right, a smooth dark plum velvet backdrop with a soft glow on the left',
    bannerAlt: 'Three matching round emeralds snapping into a row and bursting into a ring of sparkles among a few scattered gems on the right, smooth deep plum velvet stretching calmly to the left',
    welcome: { subject: 'A single glossy heart-cut ruby with a bright glint', alt: 'A sparkling heart-shaped gem' },
    instructor: { subject: 'A small open velvet pouch spilling glossy cut gems', alt: 'A velvet pouch spilling gems' },
    cards: [
      'A star-cut sapphire glowing with sparkles',
      'Three matching teardrop ambers in a row bursting into light',
      'A cracked-open geode glowing with amber crystals',
    ],
  },
});
