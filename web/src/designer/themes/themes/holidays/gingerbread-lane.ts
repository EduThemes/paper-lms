import { defineTheme } from '../../define';

export default defineTheme({
  id: 'gingerbread-lane',
  name: 'Gingerbread Lane',
  description: 'Glossy close-up photography of a real gingerbread village: icing-snow roofs, candy-cane fences, gumdrops and boiled-sweet windows glowing in fairy lights.',
  audiences: ['early-elementary', 'upper-elementary'],
  tags: ['holiday', 'cultural', 'christmas', 'gingerbread', 'winter', 'baking', 'secular'],
  collection: 'holidays',
  observance: {
    name: 'Christmas',
    traditions: ['Christian', 'secular Christmas'],
    months: [12, 1],
    note: 'A secular Christmas-season look: gingerbread, candy and icing snow with no religious imagery. Also suits a December gingerbread-house STEM or baking project.',
  },
  defaultLayout: 'playful',
  iconStyle: 'solid-circle',
  corners: 'round',
  style: 'glossy gingerbread-village food photography',
  colourBlind:
    'The palettes rest on gingerbread brown, teal or berry against pale icing cream (strong light-dark contrast); candy red and mint only decorate the art.',
  palettes: [
    { id: 'gingerbread', name: 'Gingerbread', primary: '#5C3218', onPrimary: '#FFFFFF', secondary: '#F6E2C4', onSecondary: '#3E2410', accent: '#D94A5A', surface: '#FBF4EA', onSurface: '#2B1A10', background: '#FFFFFF', text: '#2B1A10', link: '#8A4518' },
    { id: 'candy-cane', name: 'Candy Cane', primary: '#8C1D2C', onPrimary: '#FFFFFF', secondary: '#FBE0E4', onSecondary: '#5A1020', accent: '#2F8F6B', surface: '#FFF4F5', onSurface: '#2A1418', background: '#FFFFFF', text: '#2A1418', link: '#A0213A' },
    { id: 'gumdrop', name: 'Mint Gumdrop', primary: '#1F4D5A', onPrimary: '#FFFFFF', secondary: '#D9F0EE', onSecondary: '#123A40', accent: '#F2A6C0', surface: '#F0F8F8', onSurface: '#14262A', background: '#FFFFFF', text: '#14262A', link: '#1D5F6C' },
  ],
  icons: {
    start: 'candy-cane', syllabus: 'scroll', modules: 'house', assignments: 'pencil', discussions: 'message-circle',
    grades: 'star', help: 'circle-help', instructor: 'cookie', calendar: 'calendar-days', resources: 'candy',
  },
  art: {
    medium: 'Glossy close-up food photography of a homemade gingerbread village of real baked gingerbread, piped white royal icing, sparkling sugar and bright candies, with warm fairy-light bokeh and a shallow depth of field',
    palette: 'Toasty gingerbread brown and snowy white icing with candy-cane red, mint green and gumdrop pink',
    mood: 'Sweet, playful and cosy',
    banner: 'A row of little gingerbread houses with thick icing-snow roofs, gumdrop and candy-cane trim and glowing boiled-sweet windows along a sugar-dusted lane on the right; a soft, out-of-focus white icing snowfield with gentle warm bokeh on the left',
    bannerAlt: 'A gingerbread cottage with an icing-covered roof and a candy-cane fence, beside little trees made of stacked green icing stars and a scatter of gumdrops, on the right; a smooth, softly blurred cream background on the left',
    welcome: { subject: 'A single gingerbread cottage with a snowy icing roof and gumdrop trim', alt: 'A gingerbread house' },
    instructor: { subject: 'A plate of star-shaped and tree-shaped gingerbread cookies with white icing beside a glass of milk', alt: 'Gingerbread cookies and a glass of milk' },
    cards: [
      'Striped candy canes and colourful gumdrops on a bed of sparkling sugar snow',
      'A tiny gingerbread bridge over a frozen pond of clear boiled sweets between icing snowbanks, with only houses, trees and candies around it and no gingerbread people, figures or faces anywhere',
      'A gingerbread roof tiled with rows of round candies and dripping icing icicles',
    ],
  },
});
