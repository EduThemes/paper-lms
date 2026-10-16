import { defineTheme } from '../../define';

export default defineTheme({
  id: 'jungle-safari',
  name: 'Jungle Safari',
  description: 'Naive folk-art jungle of layered leaves, with a friendly elephant, a peeking giraffe, toucans and a sleepy sloth.',
  audiences: ['early-elementary', 'upper-elementary'],
  tags: ['jungle', 'animals', 'folk-art', 'lush', 'adventure'],
  defaultLayout: 'classic-band',
  iconStyle: 'solid-circle',
  corners: 'round',
  collection: 'cartoons',
  style: 'naive folk-art jungle painting',
  colourBlind:
    'Deep jungle green is paired with toucan yellow, a light-dark contrast; mango orange is decoration and never sits against the green to carry meaning.',
  palettes: [
    { id: 'canopy', name: 'Canopy', primary: '#134A35', onPrimary: '#FFFFFF', secondary: '#FFC94A', onSecondary: '#3A2A00', accent: '#FF7B39', surface: '#EEF7EF', onSurface: '#142019', background: '#FFFFFF', text: '#142019', link: '#13573D' },
    { id: 'river', name: 'River Turquoise', primary: '#0B4A5C', onPrimary: '#FFFFFF', secondary: '#FFB88A', onSecondary: '#4A1F00', accent: '#2EC4B6', surface: '#ECF7F8', onSurface: '#102227', background: '#FFFFFF', text: '#102227', link: '#0B4A5C' },
    { id: 'savanna', name: 'Savanna', primary: '#5A3410', onPrimary: '#FFFFFF', secondary: '#F4DC8C', onSecondary: '#3A2A00', accent: '#E0742B', surface: '#FBF5E6', onSurface: '#2A1D10', background: '#FFFFFF', text: '#2A1D10', link: '#7A4210' },
  ],
  icons: {
    start: 'footprints', syllabus: 'list', modules: 'trees', assignments: 'pencil', discussions: 'message-circle',
    grades: 'star', help: 'circle-help', instructor: 'binoculars', calendar: 'calendar', resources: 'backpack',
  },
  art: {
    medium:
      'Naive folk-art jungle painting with dense layered leaves, flat stylised shapes and fine brush detail',
    palette: 'Deep jungle green and mango orange with toucan yellow and turquoise',
    mood: 'Lush, warm and sunny, with friendly animals tucked among the leaves',
    banner:
      'A big friendly elephant, a tall giraffe and a toucan peeking through dense layered jungle leaves and flowers on the right, calm open pale sky glimpsed through the canopy across the left',
    bannerAlt:
      'A winding turquoise river through the jungle with a tall waterfall and a flock of flamingos wading on the right, calm still water and pale sky across the left',
    welcome: { subject: 'A smiling toucan on a leafy branch', alt: 'A toucan on a branch' },
    instructor: { subject: 'A pair of binoculars and a canvas satchel on a mossy rock', alt: 'Binoculars and a satchel on a rock' },
    cards: [
      'A sleepy sloth hanging from a flowering vine',
      'A tiger cub peeking through tall leaves',
      'A hippo wallowing among lily pads with a little bird on its back',
    ],
  },
});
