import { defineTheme } from '../../define';

export default defineTheme({
  id: 'winter-celebrations',
  name: 'Winter Celebrations',
  description: 'Coloured pencil and pastel on toned paper: snowy evergreens strung with lights, cocoa, mittens and snowflakes. Seasonal and inclusive, tied to no one holiday.',
  audiences: ['upper-elementary', 'early-elementary', 'secondary'],
  tags: ['holiday', 'cultural', 'inclusive', 'secular', 'winter', 'seasonal'],
  collection: 'holidays',
  observance: {
    name: 'Winter Celebrations',
    traditions: [],
    months: [12, 1, 2],
    note: 'Not tied to any one holiday: snow, lights and warm drinks for the winter season. Northern-hemisphere winter; southern-hemisphere schools may prefer it for June–August.',
  },
  defaultLayout: 'classic-band',
  iconStyle: 'outline',
  corners: 'soft',
  style: 'coloured pencil and pastel on toned paper',
  colourBlind:
    'Built on winter blue and warm amber (a blue-yellow pair) with strong light-dark contrast; there is no red-green distinction anywhere.',
  palettes: [
    { id: 'frost', name: 'Frost', primary: '#1B3A5C', onPrimary: '#FFFFFF', secondary: '#D6E6F5', onSecondary: '#10253D', accent: '#F2A541', surface: '#EEF4FA', onSurface: '#14202E', background: '#FFFFFF', text: '#14202E', link: '#1F4F82' },
    { id: 'cocoa', name: 'Cocoa', primary: '#4A2C21', onPrimary: '#FFFFFF', secondary: '#F0DCC6', onSecondary: '#33200F', accent: '#7FB3D5', surface: '#FAF4EE', onSurface: '#241812', background: '#FFFFFF', text: '#241812', link: '#7A3F22' },
    { id: 'aurora', name: 'Aurora', primary: '#1E3B44', onPrimary: '#FFFFFF', secondary: '#CFEDE8', onSecondary: '#10302C', accent: '#9D7AD9', surface: '#EEF7F6', onSurface: '#152225', background: '#FFFFFF', text: '#152225', link: '#1A5E63' },
  ],
  icons: {
    start: 'snowflake', syllabus: 'scroll', modules: 'layers', assignments: 'pencil', discussions: 'messages-square',
    grades: 'star', help: 'circle-help', instructor: 'coffee', calendar: 'calendar-days', resources: 'mountain-snow',
  },
  art: {
    medium: 'Coloured-pencil and soft pastel drawing on toned blue-grey paper with white highlights and gentle grain',
    palette: 'Deep winter blue and frosty sky blue with snow white and warm amber light',
    mood: 'Quiet, cosy and bright',
    banner: 'Snow-covered evergreen trees strung with small warm lights beside a little cabin with glowing windows on the right, a calm open snowy field under a pale winter sky on the left',
    bannerAlt: 'Two steaming mugs of cocoa beside an empty pair of knitted mittens lying flat on their own, nobody wearing them, and a few pine cones on the right, a soft open blue-grey background on the left',
    welcome: { subject: 'A single large snowflake crystal with exactly six arms', alt: 'A snowflake' },
    instructor: { subject: 'A steaming mug of cocoa on a knitted woollen scarf', alt: 'A mug of cocoa on a scarf' },
    cards: [
      'A red fox walking across fresh snow among birch trees',
      'A snowy pine forest at blue twilight with soft warm lights',
      'A frozen pond between gentle hills under a pale winter sun',
    ],
  },
});
