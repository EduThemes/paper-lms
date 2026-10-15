import { defineTheme } from '../../define';

export default defineTheme({
  id: 'vesak',
  name: 'Vesak',
  description: 'Serene ink-wash and watercolour of lotus ponds under a full moon, glowing paper lanterns, bodhi leaves and oil lamps. No statues, just light and flowers.',
  audiences: ['upper-elementary', 'secondary', 'early-elementary'],
  tags: ['holiday', 'religious', 'buddhist', 'lanterns', 'spring'],
  collection: 'holidays',
  observance: {
    name: 'Vesak (Buddha Day)',
    traditions: ['Buddhist'],
    months: [4, 5, 6],
    note: 'Theravada communities keep the full moon of Vesākha, usually in May. East Asian communities mark the Buddha’s birthday on the 8th day of the 4th lunar month (April–May), and Japan on April 8.',
  },
  defaultLayout: 'split-hero',
  iconStyle: 'outline',
  corners: 'soft',
  style: 'ink-wash and watercolour',
  colourBlind:
    'Night blue against pale lotus pink and saffron against pale gold are light-dark pairs; nothing relies on red vs green.',
  palettes: [
    { id: 'lotus-night', name: 'Lotus Night', primary: '#1C2A55', onPrimary: '#FFFFFF', secondary: '#F8C8D8', onSecondary: '#4A1630', accent: '#F2A03D', surface: '#F2F2FA', onSurface: '#171B2E', background: '#FFFFFF', text: '#171B2E', link: '#2D3F8A' },
    { id: 'saffron', name: 'Saffron Robe', primary: '#6B3410', onPrimary: '#FFFFFF', secondary: '#FFD9A0', onSecondary: '#3A2200', accent: '#E58FB0', surface: '#FFF6EA', onSurface: '#2A1A0E', background: '#FFFFFF', text: '#2A1A0E', link: '#8A3E0E' },
    { id: 'lotus-pink', name: 'Lotus Pink', primary: '#6E2148', onPrimary: '#FFFFFF', secondary: '#F9E1A8', onSecondary: '#3A2A05', accent: '#3B5BA5', surface: '#FDF1F5', onSurface: '#2A1621', background: '#FFFFFF', text: '#2A1621', link: '#83285A' },
  ],
  icons: {
    start: 'sunrise', syllabus: 'scroll', modules: 'layers', assignments: 'pencil', discussions: 'messages-square',
    grades: 'star', help: 'circle-help', instructor: 'lamp', calendar: 'calendar-days', resources: 'leaf',
  },
  art: {
    medium: 'Serene ink-wash and watercolour painting with soft glowing light and gentle paper texture',
    palette: 'Deep night blue and lotus pink with saffron orange and soft gold',
    mood: 'Calm, peaceful and luminous',
    banner: 'Pink lotus flowers and broad lotus leaves on still water under a glowing full moon, with a few floating lotus-shaped paper lanterns on the right, a calm deep blue night sky reflected in the water on the left',
    bannerAlt: 'Several glowing eight-sided paper lanterns, plain and unmarked, with long paper tassels hanging from a bodhi tree branch with heart-shaped leaves on the right, an open calm night sky on the left',
    welcome: { subject: 'A single pink lotus flower in bloom on a lotus leaf', alt: 'A pink lotus flower' },
    instructor: { subject: 'A small lit clay oil lamp beside a white lotus bud', alt: 'A lit oil lamp and a lotus bud' },
    cards: [
      'A heart-shaped bodhi leaf with a long pointed tip',
      'A full moon rising over a quiet lake',
      'A row of small oil lamps glowing beside lotus flowers',
    ],
  },
});
