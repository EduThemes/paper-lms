import { defineTheme } from '../../define';

export default defineTheme({
  id: 'native-american-heritage-month',
  name: 'Native American Heritage Month',
  description: 'Quiet watercolour land, water and sky: a prairie river, sandstone mesas and a cedar coast. Deliberately landscape-only, so Native artists’ own work can lead.',
  audiences: ['upper-elementary', 'secondary', 'higher-ed'],
  tags: ['heritage', 'native-american-heritage-month', 'indigenous', 'landscape', 'watercolour'],
  collection: 'special-days',
  observance: {
    name: 'Native American Heritage Month',
    traditions: [],
    months: [11],
    note: 'US: November. The art is land, water and sky only, by design: feature work by Native artists and the nations whose land your school is on. Canada: National Indigenous History Month is June.',
  },
  defaultLayout: 'editorial',
  iconStyle: 'outline',
  corners: 'soft',
  style: 'atmospheric watercolour landscape',
  colourBlind:
    'Dusk blue and sandstone are a blue-orange pair that stays distinct; pine green is scenery only and never carries meaning.',
  palettes: [
    { id: 'dusk', name: 'Dusk Sky', primary: '#1F3A5A', onPrimary: '#FFFFFF', secondary: '#EBD4B4', onSecondary: '#4A2E14', accent: '#D9A441', surface: '#F3F6F8', onSurface: '#18212B', background: '#FFFFFF', text: '#18212B', link: '#234F7A' },
    { id: 'pine', name: 'Pine & Stone', primary: '#1F3D2E', onPrimary: '#FFFFFF', secondary: '#E3DCCB', onSecondary: '#3A3020', accent: '#C98B4A', surface: '#F3F5F1', onSurface: '#18221C', background: '#FFFFFF', text: '#18221C', link: '#26573F' },
    { id: 'sandstone', name: 'Sandstone', primary: '#5E2E1C', onPrimary: '#FFFFFF', secondary: '#F3D9BF', onSecondary: '#4A2414', accent: '#6D9DC5', surface: '#FBF4ED', onSurface: '#2A1C16', background: '#FFFFFF', text: '#2A1C16', link: '#7E3D22' },
  ],
  icons: {
    start: 'sunrise', syllabus: 'scroll', modules: 'layers', assignments: 'pen-tool', discussions: 'messages-square',
    grades: 'award', help: 'circle-help', instructor: 'mountain', calendar: 'calendar-days', resources: 'library',
  },
  art: {
    medium: 'Calm atmospheric watercolour landscape with wide skies, soft layered washes and gentle light',
    palette: 'Dusky sky blue, warm sandstone and deep pine green with soft sunset gold',
    mood: 'Quiet, grounded and respectful',
    banner: 'A wide river winding through tall prairie grass toward distant blue mountains under a big open sky on the right, calm pale sky on the left',
    bannerAlt: 'Red sandstone mesas glowing at sunset above a quiet desert valley on the right, calm soft evening sky on the left',
    welcome: { subject: 'A single pine tree on a rocky shore by a still lake', alt: 'A pine tree by a lake' },
    instructor: { subject: 'A smooth river stone beside a small green fern', alt: 'A river stone and a fern' },
    cards: [
      'Snow-capped mountains above a still lake',
      'Waves against a misty rocky coast lined with tall cedars',
      'Autumn trees beside a clear stream',
    ],
  },
});
