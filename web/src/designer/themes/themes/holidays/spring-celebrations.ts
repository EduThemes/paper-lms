import { defineTheme } from '../../define';

export default defineTheme({
  id: 'spring-celebrations',
  name: 'Spring Celebrations',
  description: 'Fresh risograph prints of cherry blossom, nests, seedlings in pots and spring rain. Seasonal and inclusive, a stand-in for any spring holiday.',
  audiences: ['early-elementary', 'upper-elementary', 'secondary'],
  tags: ['holiday', 'cultural', 'inclusive', 'secular', 'spring', 'seasonal'],
  collection: 'holidays',
  observance: {
    name: 'Spring Celebrations',
    traditions: [],
    months: [3, 4, 5],
    note: 'Not tied to any one holiday: blossom, new growth and longer days. Northern-hemisphere spring; southern-hemisphere schools may prefer it for September–November.',
  },
  defaultLayout: 'card-grid',
  iconStyle: 'duotone',
  corners: 'round',
  style: 'three-ink risograph print',
  colourBlind:
    'Blossom pink and mint sit against deep band colours, so every pair differs strongly in lightness; no red-green pairing carries meaning.',
  palettes: [
    { id: 'blossom', name: 'Blossom', primary: '#7A1F4E', onPrimary: '#FFFFFF', secondary: '#FFD6E5', onSecondary: '#4A0F2E', accent: '#7FD1B9', surface: '#FFF1F6', onSurface: '#2B1622', background: '#FFFFFF', text: '#2B1622', link: '#8E1F5A' },
    { id: 'mint', name: 'Mint Shoots', primary: '#0F4C4A', onPrimary: '#FFFFFF', secondary: '#CDEFE3', onSecondary: '#0F3A30', accent: '#F7C548', surface: '#EFF8F5', onSurface: '#132322', background: '#FFFFFF', text: '#132322', link: '#0E5E5A' },
    { id: 'sunshine', name: 'Sunshower', primary: '#3B2A6B', onPrimary: '#FFFFFF', secondary: '#FBE38A', onSecondary: '#332800', accent: '#F28AB2', surface: '#FBF8EC', onSurface: '#1D1A2B', background: '#FFFFFF', text: '#1D1A2B', link: '#4A3590' },
  ],
  icons: {
    start: 'sprout', syllabus: 'scroll', modules: 'layers', assignments: 'pencil', discussions: 'messages-square',
    grades: 'star', help: 'circle-help', instructor: 'flower', calendar: 'calendar-days', resources: 'bird',
  },
  art: {
    medium: 'Fresh risograph print in three overprinted inks with soft grain and slight misregistration',
    palette: 'Blossom pink and fresh mint with sunny yellow and deep teal on off-white',
    mood: 'Light, cheerful and full of new growth',
    banner: 'A cherry tree in full pink blossom with a bird’s nest of three pale blue eggs on a branch and petals drifting on the right, an open pale sky on the left',
    bannerAlt: 'Seedlings sprouting in a row of small terracotta pots beside a watering can and a few tulips on the right, a calm open off-white background on the left',
    welcome: { subject: 'A single green seedling with two leaves sprouting from soil', alt: 'A sprouting seedling' },
    instructor: { subject: 'A watering can beside a pot of tulips', alt: 'A watering can and tulips' },
    cards: [
      'A butterfly resting on a blossom branch',
      'A gentle spring rain shower over a field of tulips',
      'A kite drifting over a hill of wildflowers',
    ],
  },
});
