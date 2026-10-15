import { defineTheme } from '../../define';

export default defineTheme({
  id: 'lunar-new-year',
  name: 'Lunar New Year',
  description: 'Layered red paper-cut with gold foil: plain lanterns, plum and apricot blossom, kumquats, mandarins and plain red envelopes. No zodiac animal, so it never expires.',
  audiences: ['early-elementary', 'upper-elementary', 'secondary'],
  tags: ['holiday', 'cultural', 'chinese', 'korean', 'vietnamese', 'new-year', 'lanterns'],
  collection: 'holidays',
  observance: {
    name: 'Lunar New Year',
    traditions: ['Chinese (Spring Festival)', 'Korean (Seollal)', 'Vietnamese (Tết)', 'other East and Southeast Asian communities'],
    months: [1, 2],
    note: 'Usually the second new moon after the winter solstice, between late January and mid-February. The art has no zodiac animal, so the theme works every year.',
  },
  defaultLayout: 'card-grid',
  iconStyle: 'solid-circle',
  corners: 'soft',
  style: 'layered red paper-cut with gold foil',
  colourBlind:
    'Red and gold differ strongly in lightness, so the palette never depends on red vs green; green appears only as small leaves in the art.',
  palettes: [
    { id: 'lucky-red', name: 'Lucky Red', primary: '#8B1A1A', onPrimary: '#FFFFFF', secondary: '#F6D27A', onSecondary: '#3A2600', accent: '#E58FA0', surface: '#FFF4E8', onSurface: '#2A1414', background: '#FFFFFF', text: '#2A1414', link: '#9E1C1C' },
    { id: 'plum-blossom', name: 'Plum Blossom', primary: '#6B1F3F', onPrimary: '#FFFFFF', secondary: '#FAD4DE', onSecondary: '#4A0F2A', accent: '#E3B341', surface: '#FFF1F4', onSurface: '#2A1520', background: '#FFFFFF', text: '#2A1520', link: '#80244C' },
    { id: 'ink-gold', name: 'Ink & Gold', primary: '#1E2A3A', onPrimary: '#FFFFFF', secondary: '#F2CF73', onSecondary: '#2E2000', accent: '#C8322E', surface: '#FBF6EA', onSurface: '#181F29', background: '#FFFFFF', text: '#181F29', link: '#7A1F1F' },
  ],
  icons: {
    start: 'sparkles', syllabus: 'scroll', modules: 'layers', assignments: 'pencil', discussions: 'messages-square',
    grades: 'star', help: 'circle-help', instructor: 'citrus', calendar: 'calendar-days', resources: 'gift',
  },
  art: {
    medium: 'Layered red paper-cut illustration with fine cut-out details, soft shadows between the paper layers and touches of gold foil',
    palette: 'Lucky red and deep crimson with warm gold, blossom pink and cream',
    mood: 'Festive, warm and hopeful',
    banner: 'Three round red paper lanterns with gold tassels, plain and unmarked, hanging from a branch of pink plum blossom beside a little potted kumquat tree heavy with orange fruit on the right, a calm cream background on the left',
    bannerAlt: 'A branch of yellow apricot blossom and a bowl of mandarin oranges with leaves beside two plain red envelopes with no markings on the right, an open cream background on the left',
    welcome: { subject: 'A round red paper lantern with a gold tassel, plain and unmarked', alt: 'A red lantern with a gold tassel' },
    instructor: { subject: 'A bowl of mandarin oranges with green leaves', alt: 'A bowl of mandarin oranges' },
    cards: [
      'A branch of plum blossom against a soft gold moon',
      'A steaming bowl of rice cake soup beside a square rice cake wrapped in green leaves and tied with string',
      'A silk lucky pouch with flower embroidery only, plain and unmarked, beside two plain red envelopes',
    ],
  },
});
