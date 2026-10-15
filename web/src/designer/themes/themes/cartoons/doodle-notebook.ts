import { defineTheme } from '../../define';

export default defineTheme({
  id: 'doodle-notebook',
  name: 'Doodle Notebook',
  description: 'Ballpoint-pen doodles straight onto lined paper: rockets, cubes in perspective and a curling dragon, with a swipe of highlighter.',
  audiences: ['secondary', 'upper-elementary'],
  tags: ['doodle', 'ballpoint', 'lined-paper', 'sketch', 'casual'],
  defaultLayout: 'card-grid',
  iconStyle: 'outline',
  corners: 'soft',
  collection: 'cartoons',
  style: 'ballpoint doodles on lined paper',
  colourBlind:
    'Ballpoint blue against highlighter yellow is a blue-yellow pair with a big lightness gap; pink is an accent mark only.',
  palettes: [
    { id: 'ballpoint', name: 'Ballpoint', primary: '#1F3A93', onPrimary: '#FFFFFF', secondary: '#FFF27A', onSecondary: '#333000', accent: '#E85A70', surface: '#F3F6FD', onSurface: '#161B2E', background: '#FFFFFF', text: '#161B2E', link: '#1F3A93' },
    { id: 'gel-pen', name: 'Gel Pen', primary: '#4B1F7A', onPrimary: '#FFFFFF', secondary: '#B6F2D6', onSecondary: '#0E3D2A', accent: '#FF7AC8', surface: '#F6F1FC', onSurface: '#1F1630', background: '#FFFFFF', text: '#1F1630', link: '#5B2A94' },
    { id: 'pencil-case', name: 'Pencil Case', primary: '#2E2E2E', onPrimary: '#FFFFFF', secondary: '#FFC68A', onSecondary: '#4A2400', accent: '#4FA3E0', surface: '#F4F4F2', onSurface: '#1A1A1A', background: '#FFFFFF', text: '#1A1A1A', link: '#1F4E8C' },
  ],
  icons: {
    start: 'pen-line', syllabus: 'notebook', modules: 'box', assignments: 'pencil', discussions: 'message-square',
    grades: 'star', help: 'circle-help', instructor: 'pen', calendar: 'calendar-days', resources: 'paperclip',
  },
  art: {
    medium:
      'Blue ballpoint-pen doodle illustration drawn straight onto lined notebook paper, with crisp pen strokes, cross-hatched shading and a few swipes of yellow highlighter; the ruled paper fills the whole image and runs off every edge, with no page edge, holes or spiral binding',
    palette: 'Ballpoint blue ink with highlighter yellow and a touch of pink gel pen on white paper with pale blue ruled lines',
    mood: 'Curious, restless and creative, the good kind of distracted',
    banner:
      'A sprawling doodle of a rocket looping around a spiral galaxy, a stack of cubes in perspective and a little dragon curling through them on the right, only plain pale ruled lines across the left',
    bannerAlt:
      'A doodled city of boxy towers in perspective with a hot-air balloon and swirling clouds above it on the right, only plain pale ruled lines across the left',
    welcome: { subject: 'A doodled star with cross-hatched shading and little sparkle marks', alt: '' },
    instructor: { subject: 'A ballpoint pen lying across a doodled spiral', alt: 'A ballpoint pen on a doodled spiral' },
    cards: [
      'A doodled light bulb with a little dragon curled around it',
      'A cluster of doodled cubes, cones and spheres in perspective',
      'A doodled paper airplane looping through swirly clouds',
    ],
  },
});
