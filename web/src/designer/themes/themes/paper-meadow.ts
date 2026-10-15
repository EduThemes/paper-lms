import { defineTheme } from '../define';

export default defineTheme({
  id: 'paper-meadow',
  name: 'Paper Meadow',
  description: 'Layered paper-cut hills, daisies and ladybirds in sunny picture-book colours. Big, warm and picture-first for K-2.',
  audiences: ['early-elementary'],
  tags: ['paper-cut', 'nature', 'warm', 'picture-first'],
  defaultLayout: 'playful',
  iconStyle: 'solid-circle',
  corners: 'round',
  style: 'layered paper-cut collage',
  colourBlind:
    'Deep sky blue and sunflower yellow carry the structure and differ strongly in lightness; grass green is decoration only and never set against the tangerine to mean anything.',
  palettes: [
    { id: 'sunny', name: 'Sunny Meadow', primary: '#1E5A8A', onPrimary: '#FFFFFF', secondary: '#FFD25E', onSecondary: '#3A2A00', accent: '#F2994A', surface: '#FFF6DA', onSurface: '#2B2B2B', background: '#FFFFFF', text: '#2B2B2B', link: '#1D5B8C' },
    { id: 'berry', name: 'Berry Patch', primary: '#7A2E5C', onPrimary: '#FFFFFF', secondary: '#FFC8D6', onSecondary: '#4A1235', accent: '#8BC34A', surface: '#FFF0F4', onSurface: '#2B2430', background: '#FFFFFF', text: '#2B2430', link: '#8A2A62' },
    { id: 'high-noon', name: 'High Noon', primary: '#14365C', onPrimary: '#FFFFFF', secondary: '#F7B32B', onSecondary: '#1A1A1A', accent: '#2E86DE', surface: '#EAF3FB', onSurface: '#111827', background: '#FFFFFF', text: '#111827', link: '#14457A' },
  ],
  icons: {
    start: 'footprints', syllabus: 'map', modules: 'blocks', assignments: 'pencil', discussions: 'message-circle',
    grades: 'star', help: 'circle-help', instructor: 'apple', calendar: 'calendar-days', resources: 'backpack',
  },
  art: {
    medium: 'Layered paper-cut collage illustration with soft shadows between the paper layers and visible paper-fibre texture',
    palette: 'Cheerful palette of sunflower yellow, deep sky blue and tangerine with fresh grass green on a warm cream ground',
    mood: 'Warm, friendly and picture-book simple',
    banner: 'A sunny paper meadow with rounded hills, tall daisies, a big ladybird and a round sun among puffy clouds gathered on the right, a wide stretch of pale open sky over low hills on the left',
    bannerAlt: 'A winding paper path through tulips and a small wooden fence to a big round hilltop tree on the right, gentle open cream sky and soft hills on the left',
    welcome: { subject: 'A cheerful paper sunflower in a small clay pot', alt: 'A paper-cut sunflower in a pot' },
    instructor: { subject: 'A red apple resting on a small stack of closed books with plain blank covers', alt: 'An apple on a stack of books' },
    cards: [
      'A ladybird resting on a big rounded leaf',
      'A little paper snail beside a spotted mushroom in tall grass',
      'A butterfly floating over three tulips on a soft hill',
    ],
  },
});
