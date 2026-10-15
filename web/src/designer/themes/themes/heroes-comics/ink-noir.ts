import { defineTheme, FONTS } from '../../define';

export default defineTheme({
  id: 'ink-noir',
  name: 'Ink Noir',
  description: 'Graphic-novel brush ink with heavy shadows and one spot colour: rain on rooftops, fire escapes and streetlamps. Moody, grown-up and calm.',
  audiences: ['secondary', 'higher-ed'],
  tags: ['graphic-novel', 'ink', 'noir', 'city', 'rain', 'moody'],
  defaultLayout: 'editorial',
  iconStyle: 'outline',
  headingFont: FONTS.georgia,
  corners: 'square',
  collection: 'heroes-comics',
  style: 'graphic-novel brush ink',
  colourBlind:
    'The look is near-monochrome and runs on lightness; the single red spot colour is decoration, and links are a dark red that still differs from body text in lightness.',
  palettes: [
    { id: 'midnight', name: 'Midnight Ink', primary: '#1A1A1F', onPrimary: '#FFFFFF', secondary: '#E9D8C4', onSecondary: '#2A2018', accent: '#C1272D', surface: '#F2F1EF', onSurface: '#141414', background: '#FFFFFF', text: '#141414', link: '#9B1C24' },
    { id: 'crimson', name: 'Crimson Spot', primary: '#6E1219', onPrimary: '#FFFFFF', secondary: '#D9D9D9', onSecondary: '#1A1A1A', accent: '#E63B2E', surface: '#F7EFEF', onSurface: '#1C1416', background: '#FFFFFF', text: '#1C1416', link: '#8C1820' },
    { id: 'teal-rain', name: 'Teal Rain', primary: '#0E3A40', onPrimary: '#FFFFFF', secondary: '#CFE6E4', onSecondary: '#0B2E33', accent: '#F2C14E', surface: '#EFF5F5', onSurface: '#121D1F', background: '#FFFFFF', text: '#121D1F', link: '#0E5560' },
  ],
  icons: {
    start: 'lamp', syllabus: 'scroll-text', modules: 'building', assignments: 'pen-line', discussions: 'messages-square',
    grades: 'award', help: 'search', instructor: 'lamp-desk', calendar: 'calendar-days', resources: 'umbrella',
  },
  art: {
    medium:
      'Graphic-novel brush-and-ink illustration with heavy black shadows, dry-brush texture and a single red spot colour, drawn as one single unframed scene that runs off every edge with no panel borders or gutters',
    palette: 'Deep black ink and crimson red on smoky grey paper',
    mood: 'Moody and cinematic, quiet rather than menacing',
    banner:
      'Rain-soaked city rooftops with a tall water tower, zigzag fire escapes and a single glowing red window on the right, the same soft grey rain wash fading into a calm, nearly empty sky on the left',
    bannerAlt:
      'An elevated train crossing a steel bridge through slanting rain at night, its windows glowing, on the right, a calm misty grey sky over the river on the left',
    welcome: { subject: 'An open black umbrella standing on wet cobblestones with rain falling around it', alt: 'An umbrella in the rain' },
    instructor: { subject: 'An empty fedora hat resting on a desk beside a fountain pen and a desk lamp, nobody wearing it', alt: 'A fedora, a pen and a desk lamp' },
    cards: [
      'A dripping fire escape lit by a single streetlamp',
      'A magnifying glass lying on an empty, bare wooden desk in a pool of lamplight, with no papers, letters, notes or books',
      'A lone streetlamp glowing on a rainy corner with reflections in the puddles',
    ],
  },
});
