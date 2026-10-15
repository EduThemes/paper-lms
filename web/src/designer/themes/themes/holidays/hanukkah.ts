import { defineTheme } from '../../define';

export default defineTheme({
  id: 'hanukkah',
  name: 'Hanukkah',
  description: 'Mid-century flat illustration of a nine-branch hanukkiah, plain dreidels, latkes, doughnuts and an oil jug in royal blue, silver and candle gold.',
  audiences: ['upper-elementary', 'early-elementary', 'secondary'],
  tags: ['holiday', 'religious', 'jewish', 'festival-of-lights', 'winter'],
  collection: 'holidays',
  observance: {
    name: 'Hanukkah',
    traditions: ['Jewish'],
    months: [11, 12],
    note: 'Eight nights from 25 Kislev on the Hebrew calendar, starting at sundown in late November or December and sometimes running into early January. Also spelled Chanukah.',
  },
  defaultLayout: 'classic-band',
  iconStyle: 'solid-circle',
  corners: 'soft',
  style: 'mid-century flat illustration',
  colourBlind:
    'A blue, white, silver and gold palette with no red-green pairing at all; blue against gold also differs strongly in lightness.',
  palettes: [
    { id: 'royal', name: 'Royal Blue', primary: '#1C3F8C', onPrimary: '#FFFFFF', secondary: '#CFE2F7', onSecondary: '#0F2550', accent: '#C9A227', surface: '#EEF4FC', onSurface: '#14213D', background: '#FFFFFF', text: '#14213D', link: '#1D4FA0' },
    { id: 'silver', name: 'Silver', primary: '#2B3A55', onPrimary: '#FFFFFF', secondary: '#DDE3EA', onSecondary: '#1E2838', accent: '#7FA7D8', surface: '#F2F5F8', onSurface: '#1A2230', background: '#FFFFFF', text: '#1A2230', link: '#2F5590' },
    { id: 'candle', name: 'Candle Gold', primary: '#13294B', onPrimary: '#FFFFFF', secondary: '#F4D58D', onSecondary: '#2B2000', accent: '#4A90D9', surface: '#FBF6E8', onSurface: '#17202E', background: '#FFFFFF', text: '#17202E', link: '#1F4E8C' },
  ],
  icons: {
    start: 'sparkles', syllabus: 'scroll', modules: 'layers', assignments: 'pencil', discussions: 'messages-square',
    grades: 'star', help: 'circle-help', instructor: 'flame', calendar: 'calendar-days', resources: 'book-open',
  },
  art: {
    medium: 'Mid-century modern flat illustration with clean geometric shapes, soft grain and a gentle glow',
    palette: 'Deep royal blue and soft sky blue with silver and warm candle gold on white',
    mood: 'Bright, warm and festive',
    banner: 'A plain, unmarked silver hanukkiah with exactly nine candle holders, eight in a straight even row and one raised in the centre, all nine candles lit, standing on a low table on the right, a calm deep blue evening sky with a few soft stars on the left',
    bannerAlt: 'Two plain dreidels with no markings on any side, a scatter of plain, unmarked gold foil-wrapped chocolate coins and a plate of jam-filled doughnuts on the right, an open soft sky-blue background on the left',
    welcome: { subject: 'A plain wooden dreidel with no markings on any side', alt: 'A spinning dreidel' },
    instructor: { subject: 'A small clay oil jug beside a plate of golden potato latkes', alt: 'An oil jug and a plate of latkes' },
    cards: [
      'An unlit plain, unmarked silver hanukkiah with exactly nine branches, eight in a straight row and one raised in the centre',
      'A plate of golden potato latkes with a bowl of applesauce',
      'A small glowing olive oil lamp beside olive branches',
    ],
  },
});
