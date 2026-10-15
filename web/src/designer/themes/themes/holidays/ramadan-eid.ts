import { defineTheme } from '../../define';

export default defineTheme({
  id: 'ramadan-eid',
  name: 'Ramadan & Eid',
  description: 'Jewel-toned gouache with fine gold line: glowing fanous lanterns, a crescent moon, dates and geometric star patterns. For Ramadan, Eid al-Fitr and Eid al-Adha.',
  audiences: ['upper-elementary', 'secondary', 'early-elementary'],
  tags: ['holiday', 'religious', 'muslim', 'islamic', 'ramadan', 'eid', 'lanterns'],
  collection: 'holidays',
  observance: {
    name: 'Ramadan, Eid al-Fitr & Eid al-Adha',
    traditions: ['Muslim'],
    months: [1, 2, 3, 4, 5],
    note: 'Follows the Islamic lunar calendar, about 11 days earlier each year, and each date waits on a moon sighting. In 2027–2030 Ramadan falls in January–March and Eid al-Adha in April–May.',
  },
  defaultLayout: 'split-hero',
  iconStyle: 'soft-square',
  corners: 'soft',
  style: 'jewel-toned gouache with gold line',
  colourBlind:
    'Deep teal and warm gold differ in hue family (blue-green vs yellow) and strongly in lightness, so nothing depends on red vs green.',
  palettes: [
    { id: 'teal', name: 'Teal & Gold', primary: '#0F3D3E', onPrimary: '#FFFFFF', secondary: '#E9C46A', onSecondary: '#2A1F00', accent: '#2A9D8F', surface: '#EEF6F2', onSurface: '#14211F', background: '#FFFFFF', text: '#14211F', link: '#0E5E57' },
    { id: 'emerald', name: 'Emerald', primary: '#14452F', onPrimary: '#FFFFFF', secondary: '#F3E3B5', onSecondary: '#3A2A05', accent: '#C9A227', surface: '#F1F7F1', onSurface: '#162019', background: '#FFFFFF', text: '#162019', link: '#1C6B44' },
    { id: 'midnight', name: 'Midnight & Gold', primary: '#1B2A4A', onPrimary: '#FFFFFF', secondary: '#F2D58A', onSecondary: '#2B2000', accent: '#6C8EBF', surface: '#F0F3FA', onSurface: '#151B2B', background: '#FFFFFF', text: '#151B2B', link: '#2A4A8A' },
  ],
  icons: {
    start: 'moon-star', syllabus: 'scroll-text', modules: 'layers', assignments: 'pencil', discussions: 'messages-square',
    grades: 'star', help: 'circle-help', instructor: 'lamp', calendar: 'calendar-days', resources: 'book-open',
  },
  art: {
    medium: 'Jewel-toned gouache painting with fine gold linework and crisp eight-pointed geometric star patterns',
    palette: 'Deep midnight teal and emerald with warm gold and soft ivory',
    mood: 'Peaceful, glowing and generous',
    banner: 'Three plain, unmarked brass fanous lanterns with geometric cut-outs glowing at different heights beside a slim crescent moon on the right, a calm deep teal evening sky with a few soft stars on the left',
    bannerAlt: 'A low silhouette of rounded domes and one slender minaret among date palms under a crescent moon at dusk on the right, a calm open dusk sky fading to teal on the left',
    welcome: { subject: 'A single glowing brass lantern with geometric cut-outs, plain and unmarked', alt: 'A glowing brass lantern' },
    instructor: { subject: 'A brass tray with a small bowl of dates, a glass of water and a little teapot', alt: 'A tray of dates, water and tea' },
    cards: [
      'A bowl of dates and dried apricots on a cloth with a geometric star pattern',
      'A slim crescent moon above calm desert dunes at twilight',
      'A tiled courtyard with an eight-pointed star fountain and one glowing lantern',
    ],
  },
});
