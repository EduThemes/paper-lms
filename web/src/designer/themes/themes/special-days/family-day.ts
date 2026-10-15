import { defineTheme } from '../../define';

export default defineTheme({
  id: 'family-day',
  name: 'Family Day',
  description: 'Cosy naive acrylic paintings: a lit house, a nest of birds of every size and mismatched mugs on a picnic blanket. An inclusive stand-in for Mother’s and Father’s Day.',
  audiences: ['early-elementary', 'upper-elementary'],
  tags: ['school-event', 'family-day', 'mothers-day-alternative', 'fathers-day-alternative', 'inclusive', 'acrylic'],
  collection: 'special-days',
  observance: {
    name: 'Family Day',
    traditions: [],
    months: [2, 5, 6, 9],
    regionalMonths: { US: [5, 6, 9], CA: [2, 5, 6], GB: [3, 6], IE: [3, 6], AU: [5, 9], NZ: [5, 9] },
    note: 'Use for Mother’s Day, Father’s Day, Grandparents Day or a family week: not every child has a mother or father at home. Canada: Family Day is in February in several provinces. UK and Ireland: Mothering Sunday is in March. Australia and New Zealand: Father’s Day is in September.',
  },
  defaultLayout: 'playful',
  iconStyle: 'solid-circle',
  corners: 'round',
  style: 'naive acrylic painting',
  colourBlind:
    'Terracotta and teal are a warm-cool pair that differs in lightness, and honey yellow adds contrast; nothing relies on red versus green.',
  palettes: [
    { id: 'terracotta', name: 'Terracotta Home', primary: '#6A2E1C', onPrimary: '#FFFFFF', secondary: '#FFDDA0', onSecondary: '#3F2808', accent: '#4FA3A5', surface: '#FFF6EC', onSurface: '#2A1C16', background: '#FFFFFF', text: '#2A1C16', link: '#8A3C22' },
    { id: 'teal', name: 'Teal Porch', primary: '#154A50', onPrimary: '#FFFFFF', secondary: '#CDEDEC', onSecondary: '#12393D', accent: '#F2A65A', surface: '#EFF8F8', onSurface: '#142426', background: '#FFFFFF', text: '#142426', link: '#17606A' },
    { id: 'honey', name: 'Honey Kitchen', primary: '#4E3610', onPrimary: '#FFFFFF', secondary: '#FBE3A6', onSecondary: '#3F2A08', accent: '#D9775A', surface: '#FFF9EA', onSurface: '#261E12', background: '#FFFFFF', text: '#261E12', link: '#6E4C12' },
  ],
  icons: {
    start: 'house', syllabus: 'clipboard-list', modules: 'layers', assignments: 'pencil', discussions: 'message-circle-heart',
    grades: 'star', help: 'circle-help', instructor: 'heart-handshake', calendar: 'calendar-days', resources: 'users',
  },
  art: {
    medium: 'Warm naive-style acrylic painting with simple shapes, visible brush texture and cosy glowing colour',
    palette: 'Warm terracotta and honey yellow with soft teal on cream',
    mood: 'Cosy, welcoming and loving',
    banner: 'A little house with warmly lit windows beside a big leafy tree hung with a wooden swing on the right, calm soft evening sky on the left',
    bannerAlt: 'A cosy picnic blanket with a basket, mugs of many different sizes and a plate of fruit under a shady tree on the right, calm open meadow and sky on the left',
    welcome: { subject: 'A round nest holding birds of different sizes', alt: 'A nest of birds' },
    instructor: { subject: 'A teapot with mismatched teacups of different sizes', alt: 'A teapot and teacups' },
    cards: [
      'A row of empty rain boots of different sizes by a front door, nobody wearing them',
      'A family of ducks of different sizes swimming across a pond',
      'A cosy sofa with a pile of cushions and a knitted blanket',
    ],
  },
});
