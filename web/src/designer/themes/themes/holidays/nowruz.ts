import { defineTheme } from '../../define';

export default defineTheme({
  id: 'nowruz',
  name: 'Nowruz',
  description: 'Fresh editorial illustration of the haft-sin table, sprouting sabzeh, hyacinths, a goldfish bowl and blossom by a turquoise pool. The new year at the spring equinox.',
  audiences: ['upper-elementary', 'secondary', 'early-elementary'],
  tags: ['holiday', 'cultural', 'persian', 'iranian', 'afghan', 'kurdish', 'central-asian', 'new-year', 'spring'],
  collection: 'holidays',
  observance: {
    name: 'Nowruz',
    traditions: ['Iranian (Persian)', 'Afghan', 'Kurdish (Newroz)', 'Azerbaijani', 'Central Asian', 'Parsi and Zoroastrian', 'Bahá’í (Naw-Rúz)'],
    months: [3],
    note: 'The new year at the March equinox, on March 20 or 21. Celebrated across Iran, Afghanistan, the Kurdish regions, Azerbaijan, Central Asia and the diaspora.',
  },
  defaultLayout: 'split-hero',
  iconStyle: 'duotone',
  corners: 'soft',
  style: 'editorial flat illustration',
  colourBlind:
    'Spring green and rose pink appear together in the art, so the structure never relies on them: deep band colours sit against pale secondaries, a light-dark pair readable in any colour vision.',
  palettes: [
    { id: 'sabzeh', name: 'Sabzeh Green', primary: '#1F4D3A', onPrimary: '#FFFFFF', secondary: '#F7D9E3', onSecondary: '#4A1630', accent: '#2BB3B1', surface: '#F1F7F2', onSurface: '#162019', background: '#FFFFFF', text: '#162019', link: '#1D6045' },
    { id: 'turquoise', name: 'Turquoise Tile', primary: '#0E4A5A', onPrimary: '#FFFFFF', secondary: '#F5DE9E', onSecondary: '#332800', accent: '#E06C8F', surface: '#EDF7F9', onSurface: '#132227', background: '#FFFFFF', text: '#132227', link: '#0F5F73' },
    { id: 'hyacinth', name: 'Hyacinth', primary: '#4A2A6B', onPrimary: '#FFFFFF', secondary: '#D8F0E0', onSecondary: '#173A24', accent: '#F2B84B', surface: '#F6F1FB', onSurface: '#211A2C', background: '#FFFFFF', text: '#211A2C', link: '#5E3A8A' },
  ],
  icons: {
    start: 'sprout', syllabus: 'scroll', modules: 'layers', assignments: 'pencil', discussions: 'messages-square',
    grades: 'star', help: 'circle-help', instructor: 'apple', calendar: 'calendar-days', resources: 'flower',
  },
  art: {
    medium: 'Clean editorial flat illustration with soft grain, gentle gradients and crisp shapes',
    palette: 'Fresh spring green and turquoise with rose pink, warm gold and cream',
    mood: 'Fresh, bright and welcoming',
    banner: 'A haft-sin table on the right: green wheatgrass sprouts tied with a red ribbon, red apples, garlic bulbs, a dish of sumac, a small glass of vinegar, dried oleaster fruit, a bowl of wheat pudding, a pink hyacinth, a goldfish bowl, a plain round mirror reflecting only candlelight and two candles; a calm cream wall on the left',
    bannerAlt: 'A flowering almond tree in pink and white blossom beside a turquoise tiled pool with a small fountain on the right, an open soft spring sky on the left',
    welcome: { subject: 'A dish of bright green wheatgrass sprouts tied with a red ribbon', alt: 'A dish of green sprouts tied with a ribbon' },
    instructor: { subject: 'A pink hyacinth in a small pot beside a red apple', alt: 'A hyacinth and an apple' },
    cards: [
      'A small round goldfish bowl with one orange goldfish beside a pink hyacinth',
      'Painted eggs in soft colours with simple floral patterns on a cream cloth',
      'A blossoming tree in a spring garden beside a turquoise pool',
    ],
  },
});
