import { defineTheme, FONTS } from '../../define';

export default defineTheme({
  id: 'nordic-knit',
  name: 'Nordic Knit',
  description: 'Quiet Scandinavian still life: red and cream star-pattern knitwear, woven straw ornaments, spruce and beeswax candlelight on pale linen. Hygge for the dark season.',
  audiences: ['secondary', 'higher-ed', 'professional'],
  tags: ['holiday', 'cultural', 'christmas', 'winter', 'nordic', 'scandinavian', 'knit', 'hygge', 'still-life'],
  collection: 'holidays',
  premium: true,
  observance: {
    name: 'Christmas',
    traditions: ['Christian', 'Nordic and Scandinavian (Jul)', 'secular Christmas'],
    months: [12, 1],
    note: 'Nordic Jul: Advent candles from late November, Lucia on December 13 and the main celebration on Christmas Eve. No religious imagery, and the knitwear and candlelight suit the whole dark season.',
  },
  defaultLayout: 'editorial',
  iconStyle: 'outline',
  headingFont: FONTS.georgia,
  corners: 'soft',
  style: 'Scandinavian knitwear still-life photography',
  colourBlind:
    'Berry red, charcoal and fjord blue all sit against pale cream or birch grey with strong lightness contrast; red never has to be told apart from green.',
  palettes: [
    { id: 'jul', name: 'Jul Red', primary: '#7E1D24', onPrimary: '#FFFFFF', secondary: '#F1E6D2', onSecondary: '#3B2618', accent: '#C9A24A', surface: '#F8F3EA', onSurface: '#241816', background: '#FFFFFF', text: '#241816', link: '#8E2129' },
    { id: 'birch', name: 'Birch & Charcoal', primary: '#2E3A3F', onPrimary: '#FFFFFF', secondary: '#E6E2D8', onSecondary: '#2B2A26', accent: '#A8282F', surface: '#F4F3EF', onSurface: '#1C2123', background: '#FFFFFF', text: '#1C2123', link: '#3D5560' },
    { id: 'fjord', name: 'Fjord', primary: '#1E3448', onPrimary: '#FFFFFF', secondary: '#DCE6EE', onSecondary: '#162A3A', accent: '#B5303A', surface: '#F1F5F8', onSurface: '#162330', background: '#FFFFFF', text: '#162330', link: '#25507A' },
  ],
  icons: {
    start: 'snowflake', syllabus: 'scroll-text', modules: 'layers', assignments: 'pen-line', discussions: 'messages-square',
    grades: 'star', help: 'circle-help', instructor: 'coffee', calendar: 'calendar-days', resources: 'flame',
  },
  art: {
    medium: 'Refined Scandinavian still-life photography of homemade knitted wool in red and cream stranded colourwork, natural straw ornaments and beeswax candlelight, soft north-window light, shallow depth of field and a pale linen backdrop',
    palette: 'Deep berry red and warm cream wool with natural straw gold and soft birch grey',
    mood: 'Calm, cosy and quietly festive',
    banner: 'A neatly folded stack of knitted red and cream sweaters with eight-point star and snowflake colourwork, two lit beeswax candles in brass holders and a sprig of spruce on a pale wooden table on the right; the same plain linen backdrop and tabletop continuing calmly to the left',
    bannerAlt: 'A delicate geometric straw mobile and three woven straw stars hanging in soft window light above a bowl of red apples and a single lit candle on the right; the same pale linen backdrop continuing smooth, plain and empty across the left, with no separate wall, panel or seam',
    welcome: { subject: 'A single knitted eight-point star ornament in red and cream wool hanging from a fine thread', alt: 'A knitted star ornament' },
    instructor: { subject: 'A stoneware mug of warm spiced drink resting on a folded red and cream knitted blanket beside a lit candle', alt: 'A warm mug on a knitted blanket' },
    cards: [
      'A small goat figure woven from golden straw and tied with red thread, standing on a linen cloth',
      'A close detail of red and cream knitted colourwork in an eight-point star pattern, softly lit',
      'Lit white candles in simple brass holders among spruce sprigs and red berries on a wooden windowsill, with soft snow outside',
    ],
  },
});
