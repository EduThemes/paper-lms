import { defineTheme, FONTS } from '../define';

export default defineTheme({
  id: 'campus-modern',
  name: 'Campus Modern',
  description: 'Editorial flat architecture of arches, colonnades and broad steps in teal and coral. A warm, contemporary university look.',
  audiences: ['higher-ed'],
  tags: ['architecture', 'editorial', 'university', 'warm'],
  defaultLayout: 'split-hero',
  iconStyle: 'solid-circle',
  headingFont: FONTS.georgia,
  corners: 'soft',
  style: 'editorial flat architecture',
  colourBlind:
    'Teal and coral differ in hue family and in lightness, which keeps them apart for red-green colour blindness.',
  palettes: [
    { id: 'teal-coral', name: 'Teal & Coral', primary: '#134E5E', onPrimary: '#FFFFFF', secondary: '#F6D2C0', onSecondary: '#5A2414', accent: '#E9785D', surface: '#F8F1E7', onSurface: '#1E2A2E', background: '#FFFFFF', text: '#1E2A2E', link: '#135266' },
    { id: 'plum-gold', name: 'Plum & Gold', primary: '#3E2A5C', onPrimary: '#FFFFFF', secondary: '#F5DFA8', onSecondary: '#3E2A00', accent: '#D9A441', surface: '#F6F2F8', onSurface: '#221A2E', background: '#FFFFFF', text: '#221A2E', link: '#4E3475' },
    { id: 'navy-stone', name: 'Navy & Stone', primary: '#1C2E4A', onPrimary: '#FFFFFF', secondary: '#E3DDD3', onSecondary: '#1C2E4A', accent: '#C98B5B', surface: '#F4F2EE', onSurface: '#1A2230', background: '#FFFFFF', text: '#1A2230', link: '#234A7A' },
  ],
  icons: {
    start: 'compass', syllabus: 'scroll-text', modules: 'library-big', assignments: 'notebook-pen', discussions: 'messages-square',
    grades: 'chart-column', help: 'circle-help', instructor: 'landmark', calendar: 'calendar-days', resources: 'book-open',
  },
  art: {
    medium: 'Editorial flat illustration of abstract architecture built from simple geometric planes with subtle paper grain',
    palette: 'Deep teal and warm coral with cream and soft sand neutrals',
    mood: 'Calm, sophisticated and welcoming',
    banner: 'Large arches, columns, broad steps and a bell-tower silhouette with two rounded trees on the right, generous calm flat cream space on the left',
    bannerAlt: 'A large modern library facade with a sweeping staircase and tall arched windows beside rounded trees on the right, generous calm flat cream space on the left',
    welcome: { subject: 'An arched doorway with a potted tree beside it', alt: '' },
    instructor: { subject: 'A reading armchair beside a tall arched window', alt: 'An armchair beside an arched window' },
    cards: [
      'A single grand arch over broad steps',
      'A colonnade casting long shadows',
      'A courtyard tree framed by arches',
    ],
  },
});
