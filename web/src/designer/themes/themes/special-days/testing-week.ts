import { defineTheme } from '../../define';

export default defineTheme({
  id: 'testing-week',
  name: 'Testing Week',
  description: 'Calm coloured-pencil scenes of sharpened pencils, a glass of water and a steady hot-air balloon. Encouraging and low-stress for test season.',
  audiences: ['upper-elementary', 'secondary'],
  tags: ['school-event', 'testing', 'calm', 'encouraging', 'coloured-pencil'],
  collection: 'special-days',
  observance: {
    name: 'Testing Week',
    traditions: [],
    months: [3, 4, 5],
    regionalMonths: { GB: [5, 6], IE: [6], AU: [10, 11], NZ: [10, 11] },
    note: 'For state, end-of-course or exam weeks, usually spring (UK and Ireland: May and June; Australia and New Zealand: October and November). Pair it with a short calm-down routine and a reminder to sleep and eat well.',
  },
  defaultLayout: 'classic-band',
  iconStyle: 'outline',
  corners: 'soft',
  style: 'soft coloured pencil on toothy paper',
  colourBlind:
    'Sage, sky blue and honey are separated by lightness, and readiness or progress is never shown as red versus green.',
  palettes: [
    { id: 'sage', name: 'Sage Morning', primary: '#2F4F45', onPrimary: '#FFFFFF', secondary: '#D9EBDD', onSecondary: '#23402F', accent: '#E9B949', surface: '#F3F8F3', onSurface: '#1C2622', background: '#FFFFFF', text: '#1C2622', link: '#2E6150' },
    { id: 'sky', name: 'Clear Sky', primary: '#1F3F66', onPrimary: '#FFFFFF', secondary: '#DCEAF7', onSecondary: '#1F3F66', accent: '#F4C77B', surface: '#F2F7FC', onSurface: '#17212C', background: '#FFFFFF', text: '#17212C', link: '#22558A' },
    { id: 'honey', name: 'Honey', primary: '#5A4214', onPrimary: '#FFFFFF', secondary: '#FBE7B5', onSecondary: '#4A3410', accent: '#8DB7A6', surface: '#FFF9EC', onSurface: '#2A2216', background: '#FFFFFF', text: '#2A2216', link: '#7A5410' },
  ],
  icons: {
    start: 'sunrise', syllabus: 'clipboard-check', modules: 'layers', assignments: 'pencil', discussions: 'message-circle',
    grades: 'badge-check', help: 'life-buoy', instructor: 'lamp-desk', calendar: 'calendar-days', resources: 'library',
  },
  art: {
    medium: 'Soft coloured-pencil illustration on warm toothy paper with gentle hatching, rounded forms and plenty of open space',
    palette: 'Calm sage green and soft sky blue with warm honey and a little deep navy',
    mood: 'Calm, steady and encouraging',
    banner: 'A neat cup of freshly sharpened pencils beside a glass of water and a crisp apple in warm morning light on the right, a calm open stretch of pale light on the left',
    bannerAlt: 'A small hot-air balloon rising steadily over gentle green hills toward a clear sky on the right, wide soft pale sky on the left',
    welcome: { subject: 'A small sailboat gliding on calm water', alt: 'A small sailboat on calm water' },
    instructor: { subject: 'A thermos of warm cocoa beside a peeled orange in segments', alt: 'A thermos and an orange' },
    cards: [
      'A winding path climbing a gentle hill to a sunny summit',
      'A sleeping cat curled up on soft cushions',
      'A bright sunrise over a calm lake',
    ],
  },
});
