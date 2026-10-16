import { defineTheme } from '../../define';

export default defineTheme({
  id: 'navratri-durga-puja',
  name: 'Navratri & Durga Puja',
  description: 'Embroidered mirror-work textiles with glowing garba lamp pots, dandiya sticks, marigolds, kash grass and shiuli flowers. Festive objects only, no deity art.',
  audiences: ['upper-elementary', 'secondary', 'early-elementary'],
  tags: ['holiday', 'religious', 'hindu', 'gujarati', 'bengali', 'dance', 'autumn'],
  collection: 'holidays',
  observance: {
    name: 'Navratri & Durga Puja',
    traditions: ['Hindu', 'Gujarati (garba and dandiya)', 'Bengali (Durga Puja)'],
    months: [9, 10],
    note: 'Sharad Navratri runs nine nights in the lunar month of Ashvin (September or October) and ends with Dussehra / Vijayadashami. Bengali communities celebrate the last days as Durga Puja.',
  },
  defaultLayout: 'card-grid',
  iconStyle: 'duotone',
  corners: 'round',
  style: 'mirror-work embroidered textile',
  colourBlind:
    'Bands are deep vermilion, peacock blue or magenta against pale marigold or pink, so every pair differs strongly in lightness; there is no red-green pairing.',
  palettes: [
    { id: 'vermilion', name: 'Vermilion & Marigold', primary: '#7A1A12', onPrimary: '#FFFFFF', secondary: '#FFC24A', onSecondary: '#2E1A00', accent: '#1F7A8C', surface: '#FFF3E8', onSurface: '#2A1511', background: '#FFFFFF', text: '#2A1511', link: '#951F14' },
    { id: 'peacock', name: 'Peacock', primary: '#0E3F5C', onPrimary: '#FFFFFF', secondary: '#FFD08A', onSecondary: '#3A2200', accent: '#D2336E', surface: '#EDF5F9', onSurface: '#132028', background: '#FFFFFF', text: '#132028', link: '#145878' },
    { id: 'magenta', name: 'Magenta Silk', primary: '#6B1240', onPrimary: '#FFFFFF', secondary: '#FBD2E2', onSecondary: '#4A0F2E', accent: '#F2A03D', surface: '#FDF0F5', onSurface: '#2A1420', background: '#FFFFFF', text: '#2A1420', link: '#851A52' },
  ],
  icons: {
    start: 'sparkles', syllabus: 'scroll', modules: 'layers', assignments: 'pencil', discussions: 'messages-square',
    grades: 'star', help: 'circle-help', instructor: 'lamp', calendar: 'calendar-days', resources: 'music',
  },
  art: {
    medium: 'Rich embroidered textile with tiny round mirror-work, satin-stitched motifs and bright appliqué on cotton',
    palette: 'Vermilion red and marigold orange with deep magenta, peacock blue and gold',
    mood: 'Festive, rhythmic and joyful',
    banner: 'A round clay garba pot pierced with rows of small holes and glowing from a lamp inside, ringed by marigolds, with two separate X shapes side by side beside it, each X made of exactly two crossed decorated dandiya sticks (four sticks in total), the two X shapes spaced apart and not touching, all on the right, calm plain deep peacock blue fabric with no embroidery on the left',
    bannerAlt: 'Tall white feathery kash grass plumes and white shiuli flowers with orange stems beside a clay incense burner trailing soft smoke and a white conch shell on the right, open soft cream fabric on the left',
    welcome: { subject: 'One pair of decorated dandiya sticks crossed over marigold flowers', alt: 'A pair of decorated dandiya sticks' },
    instructor: { subject: 'A round clay garba pot pierced with small holes, glowing from a lamp inside', alt: 'A glowing clay lamp pot' },
    cards: [
      'A garland of marigolds with mirror-work tassels',
      'White shiuli flowers with orange stems scattered on a dark cloth',
      'A large barrel drum topped with white feather plumes',
    ],
  },
});
