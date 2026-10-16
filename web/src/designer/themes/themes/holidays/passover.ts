import { defineTheme } from '../../define';

export default defineTheme({
  id: 'passover',
  name: 'Passover',
  description: 'Two-ink linocut of the seder table: a plain seder plate, stacked matzah, parsley and salt water, and spring barley. Warm, reflective and springlike.',
  audiences: ['upper-elementary', 'secondary', 'early-elementary'],
  tags: ['holiday', 'religious', 'jewish', 'pesach', 'spring'],
  collection: 'holidays',
  observance: {
    name: 'Passover (Pesach)',
    traditions: ['Jewish'],
    months: [3, 4],
    note: 'Begins at sundown on 15 Nisan on the Hebrew calendar, in March or April, and lasts seven days in Israel and eight in many diaspora communities. The first night is the seder.',
  },
  defaultLayout: 'split-hero',
  iconStyle: 'outline',
  corners: 'square',
  style: 'two-ink linocut print',
  colourBlind:
    'Wine red and warm gold differ strongly in lightness; the sage green is a small decorative accent and never sits beside red to carry meaning.',
  palettes: [
    { id: 'wine', name: 'Wine & Gold', primary: '#5C1A2B', onPrimary: '#FFFFFF', secondary: '#EED9A8', onSecondary: '#3A2A05', accent: '#7A9A6B', surface: '#FAF4EA', onSurface: '#2A1A1D', background: '#FFFFFF', text: '#2A1A1D', link: '#7A2337' },
    { id: 'indigo', name: 'Indigo Ink', primary: '#1F2D52', onPrimary: '#FFFFFF', secondary: '#D7E0EE', onSecondary: '#17213B', accent: '#B5462E', surface: '#F3F5F9', onSurface: '#161C2B', background: '#FFFFFF', text: '#161C2B', link: '#2B4785' },
    { id: 'barley', name: 'Spring Barley', primary: '#3B3A1E', onPrimary: '#FFFFFF', secondary: '#EBDDAE', onSecondary: '#2E2608', accent: '#C26A3D', surface: '#F8F5EA', onSurface: '#22200F', background: '#FFFFFF', text: '#22200F', link: '#5E5A18' },
  ],
  icons: {
    start: 'sprout', syllabus: 'scroll', modules: 'layers', assignments: 'pencil', discussions: 'messages-square',
    grades: 'star', help: 'circle-help', instructor: 'door-open', calendar: 'calendar-days', resources: 'book-open',
  },
  art: {
    medium: 'Two-ink linocut print with bold carved lines and soft paper texture, the carved image running off every edge with no plate edge or margin',
    palette: 'Deep wine red and warm gold on cream with a little soft sage',
    mood: 'Warm, reflective and springlike',
    banner: 'A seder table on the right: a round seder plate, plain and unmarked, holding exactly six small dishes, beside a stack of three square matzah sheets, a plain, unmarked stemmed silver cup and a vase of spring flowers; a calm cream wall on the left',
    bannerAlt: 'A stack of square matzah sheets under a plain white cloth beside a small dish of salt water and sprigs of parsley on the right, an open cream background on the left',
    welcome: { subject: 'A stack of three square matzah sheets with dotted rows of tiny holes', alt: 'A stack of matzah' },
    instructor: { subject: 'A plain, unmarked silver cup beside a small dish of charoset', alt: 'A silver cup and a dish of charoset' },
    cards: [
      'A round seder plate, plain and unmarked, with exactly six small dishes set in a ring around the rim and nothing in the centre of the plate',
      'Sprigs of parsley beside a small bowl of salt water',
      'A spring barley field under a rising sun',
    ],
  },
});
