import { defineTheme } from '../../define';

export default defineTheme({
  id: 'juneteenth',
  name: 'Juneteenth',
  description: 'Luminous oil-painted dawn over a quiet bay, a great oak and red wildflowers. A dignified, hopeful celebration of freedom.',
  audiences: ['upper-elementary', 'secondary', 'higher-ed'],
  tags: ['civic', 'heritage', 'juneteenth', 'emancipation', 'oil-painting'],
  collection: 'special-days',
  observance: {
    name: 'Juneteenth',
    traditions: [],
    months: [6],
    note: 'June 19. A US federal holiday since 2021, marking the day in 1865 when news of emancipation reached Galveston, Texas. Red foods and drinks are a long tradition.',
  },
  defaultLayout: 'editorial',
  iconStyle: 'outline',
  corners: 'soft',
  style: 'luminous oil painting',
  colourBlind:
    'Deep red and gold differ strongly in lightness, and indigo carries the cool side; no meaning depends on red versus green.',
  palettes: [
    { id: 'dawn', name: 'Red & Gold Dawn', primary: '#6E1A1A', onPrimary: '#FFFFFF', secondary: '#F3D27A', onSecondary: '#3A2A00', accent: '#2E3B7A', surface: '#FBF5EA', onSurface: '#261A18', background: '#FFFFFF', text: '#261A18', link: '#8A2222' },
    { id: 'indigo', name: 'Indigo Evening', primary: '#1F2A55', onPrimary: '#FFFFFF', secondary: '#F1DCA8', onSecondary: '#3A2A08', accent: '#B83A3A', surface: '#F3F3F8', onSurface: '#1A1C2A', background: '#FFFFFF', text: '#1A1C2A', link: '#2C3C80' },
    { id: 'ember', name: 'Earth & Ember', primary: '#4A2A16', onPrimary: '#FFFFFF', secondary: '#F2D3B8', onSecondary: '#4A2414', accent: '#C0392B', surface: '#FAF4EE', onSurface: '#261A12', background: '#FFFFFF', text: '#261A12', link: '#7A3A1A' },
  ],
  icons: {
    start: 'sunrise', syllabus: 'scroll', modules: 'layers', assignments: 'pen-tool', discussions: 'messages-square',
    grades: 'award', help: 'circle-help', instructor: 'tree-deciduous', calendar: 'calendar-days', resources: 'library',
  },
  art: {
    medium: 'Luminous oil painting with soft impasto brushwork, warm glazes and gentle light',
    palette: 'Deep red, warm gold and indigo blue with soft dawn cream',
    mood: 'Quiet, dignified and hopeful',
    banner: 'Dawn breaking over a calm coastal bay with tall grass and red wildflowers, golden light spreading across the water on the right, calm soft sky on the left',
    bannerAlt: 'A great old oak tree with spreading branches standing alone in an open field at sunrise on the right, calm soft dawn sky on the left',
    welcome: { subject: 'A glass pitcher of deep red hibiscus tea with ice', alt: 'A pitcher of red hibiscus tea' },
    instructor: { subject: 'A bowl of fresh red strawberries beside a small vase of wildflowers', alt: 'Strawberries and wildflowers' },
    cards: [
      'A long open road through golden fields at sunrise',
      'A sunlit park picnic table under an old live oak, spread with a red-checked cloth, sliced watermelon, a red velvet cake and a pitcher of red drink, ready for a community celebration',
      'A bright morning star above a quiet shoreline',
    ],
  },
});
