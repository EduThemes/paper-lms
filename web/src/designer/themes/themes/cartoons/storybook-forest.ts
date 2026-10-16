import { defineTheme } from '../../define';

export default defineTheme({
  id: 'storybook-forest',
  name: 'Storybook Forest',
  description: 'Classic gouache picture-book woods: a hollow oak with a little round door, a sleeping fox and a stone cottage down the path.',
  audiences: ['early-elementary', 'upper-elementary'],
  tags: ['gouache', 'picture-book', 'forest', 'animals', 'cosy', 'classic'],
  defaultLayout: 'classic-band',
  iconStyle: 'duotone',
  corners: 'round',
  collection: 'cartoons',
  style: 'classic gouache picture book',
  colourBlind:
    'Moss green is always paired with light parchment or apricot, never with the fox orange, so nothing depends on telling green from red-orange.',
  palettes: [
    { id: 'mossy-glade', name: 'Mossy Glade', primary: '#2E4A2E', onPrimary: '#FFFFFF', secondary: '#F2C38B', onSecondary: '#4A2A08', accent: '#D9692F', surface: '#F8F3E6', onSurface: '#24231C', background: '#FFFFFF', text: '#24231C', link: '#2E5A2E' },
    { id: 'bluebell', name: 'Bluebell Dusk', primary: '#2D3466', onPrimary: '#FFFFFF', secondary: '#D9D2F2', onSecondary: '#2A2450', accent: '#F2B544', surface: '#F3F1FA', onSurface: '#1C1D33', background: '#FFFFFF', text: '#1C1D33', link: '#3A428A' },
    { id: 'autumn', name: 'Autumn Hollow', primary: '#6B2E1A', onPrimary: '#FFFFFF', secondary: '#F5D48A', onSecondary: '#3D2A00', accent: '#B0412E', surface: '#FBF2E6', onSurface: '#2A1A12', background: '#FFFFFF', text: '#2A1A12', link: '#8A3A1C' },
  ],
  icons: {
    start: 'door-open', syllabus: 'book-open', modules: 'trees', assignments: 'pencil', discussions: 'message-circle',
    grades: 'star', help: 'circle-help', instructor: 'lamp', calendar: 'calendar', resources: 'leaf',
  },
  art: {
    medium:
      'Classic gouache picture-book painting with soft dry-brush edges, layered opaque washes and gentle golden light',
    palette: 'Moss green and fox orange with berry red and warm parchment',
    mood: 'Cosy, gentle and a little magical, the hush of the woods at golden hour',
    banner:
      'A cosy hollow oak tree with a little round door and a glowing lantern, red-capped mushrooms and a fox curled asleep beside it on the right, soft open golden woodland light across the left',
    bannerAlt:
      'A winding forest path through tall birches to a tiny stone cottage with a curl of chimney smoke on the right, a calm misty glade of soft parchment light across the left',
    welcome: { subject: 'A fox cub curled up asleep among autumn leaves', alt: 'A sleeping fox cub' },
    instructor: { subject: 'A glowing lantern and a pair of round spectacles resting on a mossy tree stump', alt: 'A lantern and spectacles on a tree stump' },
    cards: [
      'A family of rabbits sheltering under a big toadstool in the rain',
      'A deer drinking at a lily pond at dusk',
      'A red squirrel holding an acorn on a mossy log',
    ],
  },
});
