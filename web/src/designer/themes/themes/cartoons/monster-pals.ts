import { defineTheme } from '../../define';

export default defineTheme({
  id: 'monster-pals',
  name: 'Monster Pals',
  description: 'Fuzzy, round, many-horned plush monsters who blow bubbles and plant seeds. Cuddly rather than scary, for K-5.',
  audiences: ['early-elementary', 'upper-elementary'],
  tags: ['monsters', 'plush', 'cute', 'friendly', 'picture-first'],
  defaultLayout: 'playful',
  iconStyle: 'soft-square',
  corners: 'round',
  collection: 'cartoons',
  style: 'plush-toy 3D render',
  colourBlind:
    'Grape purple against tangerine and teal against lemon both pair a dark colour with a light one; the monsters are told apart by shape (horns, spots, antenna), not colour alone.',
  palettes: [
    { id: 'grape', name: 'Grape Fuzz', primary: '#4A2380', onPrimary: '#FFFFFF', secondary: '#FFC27A', onSecondary: '#4A2400', accent: '#2EC4B6', surface: '#F6F0FF', onSurface: '#221A30', background: '#FFFFFF', text: '#221A30', link: '#5A2D99' },
    { id: 'teal', name: 'Teal Fluff', primary: '#0F5257', onPrimary: '#FFFFFF', secondary: '#FFE07A', onSecondary: '#3A2A00', accent: '#FF8A5B', surface: '#EAF7F6', onSurface: '#142526', background: '#FFFFFF', text: '#142526', link: '#0F5257' },
    { id: 'bubblegum', name: 'Bubblegum', primary: '#7A1F55', onPrimary: '#FFFFFF', secondary: '#BDE7FF', onSecondary: '#0D3550', accent: '#FFB0D0', surface: '#FFF0F7', onSurface: '#2A1824', background: '#FFFFFF', text: '#2A1824', link: '#8A2462' },
  ],
  icons: {
    start: 'smile', syllabus: 'list', modules: 'blocks', assignments: 'pencil', discussions: 'message-circle',
    grades: 'star', help: 'heart-handshake', instructor: 'apple', calendar: 'calendar', resources: 'backpack',
  },
  art: {
    medium:
      'Soft 3D render of plush-toy monsters with fuzzy fur, rounded chunky bodies, little stitched details and gentle studio lighting',
    palette: 'Grape purple and tangerine orange with teal and soft lemon on a pale lilac ground',
    mood: 'Cuddly, silly and kind, monsters who would make the best of friends',
    banner:
      'Three friendly fuzzy monsters peeking over a soft grassy hill on the right: a round tangerine one with two little horns, a tall grape-purple one with polka dots and a tiny teal one with a curly antenna, calm open lilac sky across the left',
    bannerAlt:
      'A fuzzy tangerine monster blowing a stream of giant soap bubbles past a small pink cloud on the right, calm open lilac sky across the left',
    welcome: { subject: 'A round fuzzy purple monster with two little horns waving one arm', alt: 'A friendly fuzzy monster waving' },
    instructor: { subject: 'A pencil with a fuzzy monster pom-pom topper standing in a round pencil cup beside an apple', alt: 'A monster pencil topper and an apple' },
    cards: [
      'A fuzzy purple monster stacking plain wooden blocks',
      'A small teal monster planting a seedling in a pot',
      'Two fuzzy monsters sharing an umbrella in a gentle rain',
    ],
  },
});
