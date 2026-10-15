import { defineTheme } from '../../define';

export default defineTheme({
  id: 'earth-day',
  name: 'Earth Day',
  description: 'Torn-paper collage of a deep-rooted tree, bees, seedlings and our blue-green planet. Hopeful and hands-in-the-soil for Earth Day and Earth Month.',
  audiences: ['early-elementary', 'upper-elementary', 'secondary'],
  tags: ['awareness', 'earth-day', 'environment', 'science', 'torn-paper'],
  collection: 'special-days',
  observance: {
    name: 'Earth Day',
    traditions: [],
    months: [4],
    note: 'April 22. Many schools run Earth Month through April.',
  },
  defaultLayout: 'classic-band',
  iconStyle: 'soft-square',
  corners: 'soft',
  style: 'torn recycled-paper collage',
  colourBlind:
    'Leaf green and sky blue are separated by lightness and by the brown of the soil; nothing is coded as red versus green.',
  palettes: [
    { id: 'leaf', name: 'Leaf & Sky', primary: '#1E4D2B', onPrimary: '#FFFFFF', secondary: '#CFE8F5', onSecondary: '#163A52', accent: '#F6C445', surface: '#F1F8EE', onSurface: '#172318', background: '#FFFFFF', text: '#172318', link: '#23603A' },
    { id: 'soil', name: 'Rich Soil', primary: '#4A2E1A', onPrimary: '#FFFFFF', secondary: '#E9D9C0', onSecondary: '#3A2414', accent: '#7FB069', surface: '#F8F3EC', onSurface: '#251A12', background: '#FFFFFF', text: '#251A12', link: '#6B4020' },
    { id: 'planet', name: 'Blue Planet', primary: '#123E6B', onPrimary: '#FFFFFF', secondary: '#C8EBD6', onSecondary: '#133B26', accent: '#F6C445', surface: '#EEF6FC', onSurface: '#142030', background: '#FFFFFF', text: '#142030', link: '#1A4F85' },
  ],
  icons: {
    start: 'sprout', syllabus: 'clipboard-list', modules: 'layers', assignments: 'recycle', discussions: 'message-circle',
    grades: 'leaf', help: 'circle-help', instructor: 'tree-deciduous', calendar: 'calendar-days', resources: 'earth',
  },
  art: {
    medium: 'Torn-paper collage with rough deckled edges on the shapes inside the picture, visible recycled-paper fibres and layered earthy colours; the collage fills the whole canvas to every edge with no torn white paper border around it',
    palette: 'Leaf green, earth brown and clear sky blue with a little sunflower yellow',
    mood: 'Hopeful, alive and caring',
    banner: 'A big leafy tree with deep spreading roots in rich soil, honeybees and wildflowers around its trunk on the right, open pale sky on the left',
    bannerAlt: 'Our round blue-green planet seen from space with soft swirling clouds, ringed by a few green leaves on the right, calm deep sky on the left',
    welcome: { subject: 'A small seedling sprouting from a mound of dark soil', alt: 'A seedling sprouting' },
    instructor: { subject: 'A watering can beside a potted seedling', alt: 'A watering can and a seedling' },
    cards: [
      'A honeybee resting on a clover blossom',
      'A reusable glass bottle beside a cloth tote of fresh vegetables',
      'Clear river water flowing past smooth stones and ferns',
    ],
  },
});
