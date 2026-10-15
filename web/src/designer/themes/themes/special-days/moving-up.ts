import { defineTheme } from '../../define';

export default defineTheme({
  id: 'moving-up',
  name: 'Graduation & Moving Up',
  description: 'Gold foil on midnight navy: tossed mortarboards, diploma scrolls and laurel. Works for graduation, promotion and moving-up ceremonies.',
  audiences: ['secondary', 'higher-ed', 'upper-elementary'],
  tags: ['school-event', 'graduation', 'moving-up', 'commencement', 'end-of-year', 'gold-foil'],
  collection: 'special-days',
  observance: {
    name: 'Graduation and Moving Up',
    traditions: [],
    months: [5, 6, 12],
    regionalMonths: { US: [5, 6, 12], CA: [5, 6, 12], GB: [6, 7], IE: [5, 6], AU: [11, 12], NZ: [11, 12] },
    note: 'Mostly May and June; many colleges also hold December ceremonies. Swap the colourway for school colours.',
  },
  defaultLayout: 'split-hero',
  iconStyle: 'outline',
  corners: 'square',
  style: 'gold foil on midnight paper',
  colourBlind:
    'Gold on deep navy is a strong lightness contrast that holds for every type of colour blindness; maroon and forest colourways are never shown side by side.',
  palettes: [
    { id: 'midnight', name: 'Midnight & Gold', primary: '#14213D', onPrimary: '#FFFFFF', secondary: '#F2D27A', onSecondary: '#3A2A00', accent: '#C9A227', surface: '#F7F4EA', onSurface: '#161C2B', background: '#FFFFFF', text: '#161C2B', link: '#1F3A75' },
    { id: 'maroon', name: 'Maroon & Gold', primary: '#5C1A2B', onPrimary: '#FFFFFF', secondary: '#F2D9A0', onSecondary: '#3F2A0A', accent: '#C9A227', surface: '#FAF4EC', onSurface: '#261820', background: '#FFFFFF', text: '#261820', link: '#7A2338' },
    { id: 'forest', name: 'Forest & Silver', primary: '#1C3F2E', onPrimary: '#FFFFFF', secondary: '#DDE5E1', onSecondary: '#1C3F2E', accent: '#B8C4BF', surface: '#F2F6F4', onSurface: '#17221D', background: '#FFFFFF', text: '#17221D', link: '#22593F' },
  ],
  icons: {
    start: 'graduation-cap', syllabus: 'scroll', modules: 'layers', assignments: 'notebook-pen', discussions: 'messages-square',
    grades: 'award', help: 'circle-help', instructor: 'medal', calendar: 'calendar-days', resources: 'sparkles',
  },
  art: {
    medium: 'Foil-stamped illustration in metallic gold on deep midnight navy paper with fine line detail; the image runs off every edge with no border, frame or margin',
    palette: 'Midnight navy and burnished gold with soft ivory highlights',
    mood: 'Proud, bright and full of possibility',
    banner: 'A burst of mortarboard caps with swinging tassels tossed high among small gold stars and curling streamers on the right, calm deep navy sky on the left',
    bannerAlt: 'An open doorway spilling warm light down a short flight of steps, with a laurel branch curling beside it on the right, calm deep navy on the left',
    welcome: { subject: 'A mortarboard cap with a swinging tassel', alt: 'A graduation cap' },
    instructor: { subject: 'A rolled diploma scroll tied with a ribbon bow', alt: 'A rolled diploma' },
    cards: [
      'A staircase of gold steps rising toward a bright star',
      'A paper airplane flying out of an open window toward the sky',
      'A laurel wreath around a shining star',
    ],
  },
});
