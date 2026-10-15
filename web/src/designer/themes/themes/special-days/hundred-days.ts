import { defineTheme } from '../../define';

export default defineTheme({
  id: 'hundred-days',
  name: '100th Day of School',
  description: 'A hundred round dot stickers in candy colours: jars of buttons, rows of dots and balloon bunches. Counting fun with no numerals in the art.',
  audiences: ['early-elementary', 'upper-elementary'],
  tags: ['school-event', '100th-day', 'counting', 'math', 'dot-stickers'],
  collection: 'special-days',
  observance: {
    name: '100th Day of School',
    traditions: [],
    months: [1, 2],
    note: 'Usually late January or February, on the hundredth day your school is in session. Add the number in your title; the art has none.',
  },
  defaultLayout: 'playful',
  iconStyle: 'soft-square',
  corners: 'round',
  style: 'round dot-sticker collage',
  colourBlind:
    'Many colours appear only as decoration; counting relies on shape and position, and the structural pair (deep purple and warm yellow) differs strongly in lightness.',
  palettes: [
    { id: 'gumball', name: 'Gumball', primary: '#5B2A86', onPrimary: '#FFFFFF', secondary: '#FFC94A', onSecondary: '#3A2600', accent: '#2EC4B6', surface: '#FFF5E1', onSurface: '#241B30', background: '#FFFFFF', text: '#241B30', link: '#5B2A86' },
    { id: 'mint', name: 'Mint Chip', primary: '#135E55', onPrimary: '#FFFFFF', secondary: '#BDEBDF', onSecondary: '#0F3D37', accent: '#FF8C69', surface: '#EFFAF6', onSurface: '#13241F', background: '#FFFFFF', text: '#13241F', link: '#136158' },
    { id: 'berry', name: 'Berry Pop', primary: '#8A1C4A', onPrimary: '#FFFFFF', secondary: '#FFD1E0', onSecondary: '#5A0F2E', accent: '#FFB703', surface: '#FFF2F6', onSurface: '#2B1720', background: '#FFFFFF', text: '#2B1720', link: '#9A1F52' },
  ],
  icons: {
    start: 'party-popper', syllabus: 'list-checks', modules: 'grid-3x3', assignments: 'pencil-line', discussions: 'message-circle',
    grades: 'star', help: 'circle-help', instructor: 'apple', calendar: 'calendar-days', resources: 'boxes',
  },
  art: {
    medium: 'Playful collage made of round vinyl dot stickers in many colours, layered on a soft cream ground with crisp sticker edges and tiny shadows',
    palette: 'Candy colours of cherry, tangerine, lemon, mint and sky blue with a little deep purple on warm cream',
    mood: 'Bouncy, festive and full of counting fun',
    banner: 'A big glass jar brimming with a hundred colourful round buttons and gumballs on the right, a few stray dots rolling toward a wide calm cream area on the left',
    bannerAlt: 'Ten neat rows of ten round coloured dots forming a bright square grid on the right, a few small paper stars sprinkled around it, plain calm cream on the left',
    welcome: { subject: 'A cupcake topped with lots of round rainbow sprinkles', alt: 'A cupcake covered in sprinkles' },
    instructor: { subject: 'A glass jar full of colourful round buttons', alt: 'A jar of buttons' },
    cards: [
      'A long caterpillar made of many round coloured dots crawling along a leaf',
      'A big bunch of round balloons floating upward',
      'A tall stack of colourful counting cubes',
    ],
  },
});
