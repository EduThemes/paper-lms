import { defineTheme } from '../../define';

export default defineTheme({
  id: 'ink-and-wash',
  name: 'Ink & Wash',
  description: 'Sumi-e brush painting: misty peaks, plum blossom and cranes in graded ink washes, with one touch of vermilion. Space to think.',
  audiences: ['higher-ed', 'professional', 'secondary'],
  tags: ['ink-wash', 'brush-painting', 'minimal', 'landscape', 'calm'],
  defaultLayout: 'editorial',
  iconStyle: 'outline',
  corners: 'square',
  collection: 'signature',
  premium: true,
  style: 'sumi-e ink wash painting',
  colourBlind:
    'Almost everything is black ink in greys, so the structure is pure lightness; vermilion is a single decorative accent and never carries meaning.',
  palettes: [
    { id: 'sumi', name: 'Sumi', primary: '#222222', onPrimary: '#FFFFFF', secondary: '#E7E3DA', onSecondary: '#222222', accent: '#C8402A', surface: '#F5F3EE', onSurface: '#1A1A1A', background: '#FFFFFF', text: '#1A1A1A', link: '#8C2B1A' },
    { id: 'indigo-wash', name: 'Indigo Wash', primary: '#1F2E4D', onPrimary: '#FFFFFF', secondary: '#DDE2EC', onSecondary: '#1F2E4D', accent: '#C8402A', surface: '#F2F4F8', onSurface: '#161C2A', background: '#FFFFFF', text: '#161C2A', link: '#2C4677' },
    { id: 'celadon', name: 'Celadon', primary: '#2D4A42', onPrimary: '#FFFFFF', secondary: '#D6E6DE', onSecondary: '#20362F', accent: '#B5523B', surface: '#F1F6F2', onSurface: '#1A2622', background: '#FFFFFF', text: '#1A2622', link: '#2D5A4E' },
  ],
  icons: {
    start: 'brush', syllabus: 'scroll-text', modules: 'mountain', assignments: 'pen-line', discussions: 'messages-square',
    grades: 'award', help: 'circle-help', instructor: 'paintbrush', calendar: 'calendar-days', resources: 'library',
  },
  art: {
    medium: 'Traditional sumi-e ink wash painting with expressive brushstrokes, soft graded washes and wet bleeds on pale rice paper that runs off every edge, without any seal or inscription',
    palette: 'Charcoal ink black and soft grey washes with a single touch of vermilion on pale rice paper',
    mood: 'Spare, contemplative and full of quiet space',
    banner: 'Misty mountain peaks rising through layered fog with a twisted pine clinging to a cliff on the right, pale empty mist and paper on the left',
    bannerAlt: 'A large branch of plum blossom arching in from the right with a few vermilion blossoms and a lone crane gliding beneath it, wide empty paper and faint mist on the left',
    welcome: { subject: 'A single bamboo stalk with three leaves', alt: '' },
    instructor: { subject: 'An ink brush resting on a smooth ink stone', alt: 'An ink brush on an ink stone' },
    cards: [
      'A small fishing boat on still water beneath a distant peak',
      'A lone crane standing in shallow reeds',
      'A round vermilion sun over a wash of misty hills',
    ],
  },
});
