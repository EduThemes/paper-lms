import { defineTheme } from '../../define';

export default defineTheme({
  id: 'voxel-world',
  name: 'Voxel World',
  description: 'A cosy 3D world built from smooth natural cubes: blocky trees, a teal river and a little cube cottage. Made for builders and explorers.',
  audiences: ['upper-elementary', 'early-elementary'],
  tags: ['voxel', '3d', 'cubes', 'building', 'nature', 'gaming'],
  defaultLayout: 'playful',
  iconStyle: 'soft-square',
  corners: 'square',
  collection: 'arcade',
  style: '3D voxel render',
  colourBlind:
    'Leafy green and sandstone differ strongly in lightness, and deep primaries carry all text; no colour alone marks a state.',
  palettes: [
    { id: 'meadow', name: 'Meadow', primary: '#1F4A2E', onPrimary: '#FFFFFF', secondary: '#BFE8A6', onSecondary: '#1A3A12', accent: '#3FB7C4', surface: '#EFF8EA', onSurface: '#172617', background: '#FFFFFF', text: '#172617', link: '#24603A' },
    { id: 'river', name: 'River', primary: '#0C4258', onPrimary: '#FFFFFF', secondary: '#A9E4F0', onSecondary: '#0A3442', accent: '#F2C14E', surface: '#EAF7FA', onSurface: '#112631', background: '#FFFFFF', text: '#112631', link: '#0C5A78' },
    { id: 'sandstone', name: 'Sandstone', primary: '#5A3414', onPrimary: '#FFFFFF', secondary: '#F5D9A8', onSecondary: '#3E2408', accent: '#4CAF7A', surface: '#FBF3E6', onSurface: '#2A1A0C', background: '#FFFFFF', text: '#2A1A0C', link: '#8A4A14' },
  ],
  icons: {
    start: 'box', syllabus: 'scroll', modules: 'boxes', assignments: 'hammer', discussions: 'message-circle',
    grades: 'star', help: 'circle-help', instructor: 'lamp', calendar: 'calendar-days', resources: 'backpack',
  },
  art: {
    medium: 'Bright 3D voxel art render built from small smooth plain cubes in natural colours, soft ambient-occlusion shading and a gentle tilt-shift focus',
    palette: 'Leafy green, river teal and warm sand with sky blue',
    mood: 'Cosy, curious and ready to build',
    banner: 'A cube-built valley with round-topped blocky trees, a winding teal river, a little cube cottage and a waterfall on the right, a wide soft sky with a few cube clouds on the left',
    bannerAlt: 'A tall cube-built treehouse in a blocky oak beside a pond of teal cubes and a little wooden jetty on the right, gentle open blue sky over low cube hills on the left',
    welcome: { subject: 'A small cube-built sapling in a clay pot', alt: 'A voxel sapling in a pot' },
    instructor: { subject: 'A cube-built lantern beside a neat stack of smooth wooden cubes', alt: 'A voxel lantern beside stacked wooden cubes' },
    cards: [
      'A cube-built sailboat on teal cube water',
      'A blocky fox made of orange cubes sitting in a meadow',
      'A cube-built hot-air balloon over rolling cube hills',
    ],
  },
});
