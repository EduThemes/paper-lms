import { defineTheme } from '../define';

export default defineTheme({
  id: 'felt-board',
  name: 'Felt Board',
  description: 'Stitched felt appliqué with blanket-stitch edges and soft wool fibres: hedgehogs, owls and sailboats from the craft basket.',
  audiences: ['early-elementary'],
  tags: ['felt', 'craft', 'cosy', 'animals'],
  defaultLayout: 'playful',
  iconStyle: 'soft-square',
  corners: 'round',
  style: 'stitched felt appliqué',
  colourBlind:
    'Denim blue against mustard is a blue-yellow pair that holds for red-green colour blindness; the coral accent never stands alone for meaning.',
  palettes: [
    { id: 'oatmeal', name: 'Oatmeal & Denim', primary: '#2D4A7A', onPrimary: '#FFFFFF', secondary: '#F2C04A', onSecondary: '#3A2A00', accent: '#E9806E', surface: '#F7F1E6', onSurface: '#2A2622', background: '#FFFFFF', text: '#2A2622', link: '#2D4A7A' },
    { id: 'forest', name: 'Forest Felt', primary: '#2F5A3F', onPrimary: '#FFFFFF', secondary: '#F6D7B0', onSecondary: '#4A2E10', accent: '#D9764A', surface: '#F2F5EE', onSurface: '#222222', background: '#FFFFFF', text: '#222222', link: '#2F5A3F' },
    { id: 'plum', name: 'Plum Wool', primary: '#5B2A5E', onPrimary: '#FFFFFF', secondary: '#CFE1F2', onSecondary: '#16365A', accent: '#F2B544', surface: '#F8F0F6', onSurface: '#2A2229', background: '#FFFFFF', text: '#2A2229', link: '#6A2E6E' },
  ],
  icons: {
    start: 'house', syllabus: 'list', modules: 'layout-grid', assignments: 'scissors', discussions: 'messages-square',
    grades: 'star', help: 'hand-helping', instructor: 'apple', calendar: 'calendar', resources: 'shapes',
  },
  art: {
    medium: 'Handmade felt appliqué craft illustration with visible blanket stitching, soft wool fibres and gentle shadows between layers',
    palette: 'Denim blue and mustard yellow with soft coral and oatmeal felt',
    mood: 'Snug, homemade and gentle, like a story told on a classroom felt board',
    banner: 'A large felt hedgehog, two spotted mushrooms and a pine tree on stitched rolling hills on the right, plain oatmeal felt sky on the left',
    bannerAlt: 'A big felt sailboat on stitched waves with a round sun and a seagull on the right, plain oatmeal felt sky on the left',
    welcome: { subject: 'A round felt owl sitting on a stitched branch', alt: 'A felt owl on a branch' },
    instructor: { subject: 'An apple-shaped felt pincushion beside a spool of thread', alt: 'An apple pincushion and a spool of thread' },
    cards: [
      'A felt hedgehog among autumn leaves',
      'A felt rocket with a stitched flame',
      'A felt fish swimming among round bubbles',
    ],
  },
});
