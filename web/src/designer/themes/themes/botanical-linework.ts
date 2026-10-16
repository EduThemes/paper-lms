import { defineTheme, FONTS } from '../define';

export default defineTheme({
  id: 'botanical-linework',
  name: 'Botanical Linework',
  description: 'Fine ink botanical drawings with a sage watercolour wash. Quiet, literate and unhurried.',
  audiences: ['higher-ed', 'professional'],
  tags: ['botanical', 'line-art', 'serif', 'calm'],
  defaultLayout: 'editorial',
  iconStyle: 'outline',
  headingFont: FONTS.georgia,
  corners: 'soft',
  style: 'botanical ink linework',
  colourBlind:
    'Sage and terracotta can look alike to red-green colour-blind readers, so they never distinguish two things; the ink line carries the shapes.',
  palettes: [
    { id: 'sage', name: 'Sage', primary: '#2F4A3F', onPrimary: '#FFFFFF', secondary: '#DCE5D3', onSecondary: '#233A2E', accent: '#C7704B', surface: '#F4F6EF', onSurface: '#1F2A24', background: '#FFFFFF', text: '#1F2A24', link: '#2C5546' },
    { id: 'terracotta', name: 'Terracotta', primary: '#7A3B24', onPrimary: '#FFFFFF', secondary: '#F2DCCB', onSecondary: '#4A2414', accent: '#8FA98C', surface: '#FAF3EE', onSurface: '#2A1E18', background: '#FFFFFF', text: '#2A1E18', link: '#8A3F22' },
    { id: 'indigo', name: 'Ink & Indigo', primary: '#233159', onPrimary: '#FFFFFF', secondary: '#DDE3F0', onSecondary: '#233159', accent: '#B08D57', surface: '#F3F4F8', onSurface: '#1A1F2E', background: '#FFFFFF', text: '#1A1F2E', link: '#2F4687' },
  ],
  icons: {
    start: 'sprout', syllabus: 'scroll-text', modules: 'layers', assignments: 'pen-line', discussions: 'messages-square',
    grades: 'award', help: 'circle-help', instructor: 'flower-2', calendar: 'calendar-days', resources: 'library',
  },
  art: {
    medium: 'Fine-line botanical ink illustration with delicate cross-hatching and a soft sage watercolour wash on warm off-white paper',
    palette: 'Deep forest ink and sage green with touches of terracotta on cream',
    mood: 'Quiet, scholarly and unhurried',
    banner: 'A generous spray of eucalyptus, olive branches and a fern drawn large on the right, open blank paper on the left',
    bannerAlt: 'A tall foxglove stem, dried seed heads and a curling vine drawn large on the right, open blank paper on the left',
    welcome: { subject: 'A sprig of olive leaves', alt: '' },
    instructor: { subject: 'A glass bottle vase holding a single flowering branch', alt: 'A flowering branch in a glass vase' },
    cards: [
      'A single ginkgo leaf',
      'A poppy seed head on a curved stem',
      'An unfurling fern fiddlehead',
    ],
  },
});
