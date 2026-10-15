import { defineTheme } from '../../define';

export default defineTheme({
  id: 'aanhpi-heritage-month',
  name: 'AANHPI Heritage Month',
  description: 'Layered landscapes from terraced hills to Pacific islands, a mountain railway and a lotus pond. Many places and peoples, no single stereotype.',
  audiences: ['upper-elementary', 'secondary', 'higher-ed'],
  tags: ['heritage', 'aanhpi', 'asian-american', 'pacific-islander', 'native-hawaiian', 'landscape'],
  collection: 'special-days',
  observance: {
    name: 'Asian American, Native Hawaiian and Pacific Islander Heritage Month',
    traditions: [],
    months: [5],
    note: 'US: May (Canada’s Asian Heritage Month is also May). It spans more than 50 communities; feature specific ones, including Native Hawaiian and Pacific Islander voices.',
  },
  defaultLayout: 'classic-band',
  iconStyle: 'duotone',
  corners: 'soft',
  style: 'layered gradient landscape',
  colourBlind:
    'Ocean blue and jade differ in lightness, and coral is a warm accent against blue, so nothing depends on red versus green.',
  palettes: [
    { id: 'ocean', name: 'Ocean & Jade', primary: '#123F5E', onPrimary: '#FFFFFF', secondary: '#C7EBDD', onSecondary: '#134030', accent: '#F28C6B', surface: '#EEF6F8', onSurface: '#15212A', background: '#FFFFFF', text: '#15212A', link: '#1A5480' },
    { id: 'jade', name: 'Jade Terraces', primary: '#1E4A3C', onPrimary: '#FFFFFF', secondary: '#F6DCA8', onSecondary: '#3F2E0A', accent: '#5DA9C9', surface: '#F0F7F3', onSurface: '#17241F', background: '#FFFFFF', text: '#17241F', link: '#21604D' },
    { id: 'coral', name: 'Coral Dawn', primary: '#6E2E2A', onPrimary: '#FFFFFF', secondary: '#FFDCCB', onSecondary: '#5A221A', accent: '#4FB3A9', surface: '#FFF6F1', onSurface: '#2A1C1A', background: '#FFFFFF', text: '#2A1C1A', link: '#8A3830' },
  ],
  icons: {
    start: 'sunrise', syllabus: 'scroll', modules: 'layers', assignments: 'pen-tool', discussions: 'messages-square',
    grades: 'award', help: 'circle-help', instructor: 'mountain', calendar: 'calendar-days', resources: 'library',
  },
  art: {
    medium: 'Layered gradient landscape illustration with soft silhouettes, gentle atmospheric depth and smooth matte colour',
    palette: 'Deep ocean blue and jade green with warm coral and soft gold light',
    mood: 'Expansive, warm and connected',
    banner: 'Misty terraced green hillsides stepping down to a wide ocean dotted with volcanic islands and palm trees on the right, calm soft morning sky on the left',
    bannerAlt: 'A railway winding through tall mountains and pine forest toward a wide blue ocean on the right, calm soft sky on the left',
    welcome: { subject: 'A flowering branch with a small songbird', alt: 'A songbird on a flowering branch' },
    instructor: { subject: 'A small potted plumeria beside a bowl of ripe mangoes', alt: 'A flowering plant and a bowl of mangoes' },
    cards: [
      'A lotus flower on a calm pond',
      'Ocean waves rolling onto a black-sand beach with palms',
      'Terraced rice fields glowing in early light',
    ],
  },
});
