import { defineTheme } from '../../define';

export default defineTheme({
  id: 'back-to-school',
  name: 'Back to School',
  description: 'Crayon-and-cut-paper backpacks, sharpened pencils and a shiny apple. A bright, welcoming first week for any grade.',
  audiences: ['early-elementary', 'upper-elementary', 'secondary'],
  tags: ['school-event', 'back-to-school', 'first-day', 'welcome', 'cut-paper'],
  collection: 'special-days',
  observance: {
    name: 'Back to School',
    traditions: [],
    months: [8, 9, 1],
    regionalMonths: { US: [8, 9], CA: [8, 9], GB: [9], IE: [8, 9], AU: [1, 2], NZ: [1, 2] },
    note: 'US and UK: August or September. Australia and New Zealand: late January or February.',
  },
  defaultLayout: 'playful',
  iconStyle: 'solid-circle',
  corners: 'round',
  style: 'cut-paper and wax-crayon collage',
  colourBlind:
    'Cobalt blue and school-bus yellow carry the structure and differ strongly in lightness; apple red is decoration only and never paired with green to mean anything.',
  palettes: [
    { id: 'crayon', name: 'Crayon Box', primary: '#1F4E9A', onPrimary: '#FFFFFF', secondary: '#FFD23F', onSecondary: '#3A2A00', accent: '#E4572E', surface: '#FFF7DD', onSurface: '#1F2430', background: '#FFFFFF', text: '#1F2430', link: '#1F4E9A' },
    { id: 'apple', name: 'Apple Red', primary: '#8E1F1F', onPrimary: '#FFFFFF', secondary: '#FFE3D6', onSecondary: '#5A1414', accent: '#4CAF50', surface: '#FFF4F0', onSurface: '#2A1A1A', background: '#FFFFFF', text: '#2A1A1A', link: '#9E2A2A' },
    { id: 'notebook', name: 'Notebook Blue', primary: '#23395B', onPrimary: '#FFFFFF', secondary: '#CDE7F7', onSecondary: '#23395B', accent: '#F2A541', surface: '#F1F7FC', onSurface: '#18222E', background: '#FFFFFF', text: '#18222E', link: '#1F5482' },
  ],
  icons: {
    start: 'backpack', syllabus: 'clipboard-list', modules: 'blocks', assignments: 'pencil', discussions: 'message-circle',
    grades: 'star', help: 'circle-help', instructor: 'apple', calendar: 'calendar-days', resources: 'library',
  },
  art: {
    medium: 'Bright cut-paper and wax-crayon collage illustration with chunky shapes, crayon-textured fills and soft shadows between the paper layers',
    palette: 'School-bus yellow and cobalt blue with apple red and fresh green on a warm cream ground',
    mood: 'Cheerful, fresh and welcoming',
    banner: 'A big open backpack spilling sharpened coloured pencils, chunky crayons and a shiny red apple on the right, a wide stretch of calm plain cream paper on the left',
    bannerAlt: 'A cheerful little schoolhouse with a bell tower, a plain blank facade and open red doors among round paper trees on the right, open pale sky and soft hills on the left',
    welcome: { subject: 'A sharpened yellow pencil leaning on a box of chunky crayons', alt: 'A pencil and a box of crayons' },
    instructor: { subject: 'A shiny red apple resting on a small stack of closed books with plain blank covers', alt: 'An apple on a stack of books' },
    cards: [
      'A cup full of colourful sharpened pencils',
      'A small globe on a stand beside a potted plant',
      'A paper airplane gliding over soft clouds',
    ],
  },
});
