import { defineTheme } from '../../define';

export default defineTheme({
  id: 'sidekick-squad',
  name: 'Sidekick Squad',
  description: 'A caped puppy, a masked owl and a turtle in a tiny cape: little animal heroes in chunky picture-book shapes for K-2.',
  audiences: ['early-elementary', 'upper-elementary'],
  tags: ['superhero', 'animals', 'picture-first', 'friendly', 'bright'],
  defaultLayout: 'playful',
  iconStyle: 'solid-circle',
  corners: 'round',
  collection: 'heroes-comics',
  style: 'chunky picture-book cartoon',
  colourBlind:
    'Structure rests on deep blue or deep red against sunny yellow, which differ strongly in lightness; grass green is background only and never paired with red to carry meaning.',
  palettes: [
    { id: 'sky-cape', name: 'Sky Cape', primary: '#1D4E89', onPrimary: '#FFFFFF', secondary: '#FFCF5C', onSecondary: '#3A2A00', accent: '#FF6B6B', surface: '#EDF5FF', onSurface: '#172233', background: '#FFFFFF', text: '#172233', link: '#1D4E89' },
    { id: 'hero-red', name: 'Hero Red', primary: '#A51C30', onPrimary: '#FFFFFF', secondary: '#FFD447', onSecondary: '#3A2A00', accent: '#3FA9F5', surface: '#FFF7E0', onSurface: '#22201C', background: '#FFFFFF', text: '#22201C', link: '#A51C30' },
    { id: 'leafy-shell', name: 'Leafy Shell', primary: '#1F5C45', onPrimary: '#FFFFFF', secondary: '#FFE08A', onSecondary: '#3A2A00', accent: '#FF9447', surface: '#EEF7F1', onSurface: '#1B2620', background: '#FFFFFF', text: '#1B2620', link: '#1F5C45' },
  ],
  icons: {
    start: 'paw-print', syllabus: 'list', modules: 'shapes', assignments: 'pencil', discussions: 'message-circle',
    grades: 'star', help: 'hand-helping', instructor: 'shield', calendar: 'calendar', resources: 'backpack',
  },
  art: {
    medium:
      'Chunky rounded cartoon illustration with thick friendly outlines, soft grainy shading and simple bold shapes, like a modern picture book',
    palette: 'Sky blue and sunny yellow with cape red and fresh grass green on a soft cream ground',
    mood: 'Brave, silly and kind, the smallest heroes around',
    banner:
      'A puppy in a flowing red cape, a round owl in a little eye mask and a turtle with a tiny cape striking brave poses together on a grassy hilltop on the right, open sunny sky over soft green hills on the left',
    bannerAlt:
      'A caped puppy flying past puffy clouds with a masked owl gliding beside it on the right, wide open pale blue sky on the left',
    welcome: { subject: 'A happy puppy in a flowing plain red cape', alt: 'A puppy in a red cape' },
    instructor: { subject: 'A plain star-shaped badge resting on a neatly folded little cape on a round cushion', alt: 'A star badge on a folded cape' },
    cards: [
      'A caped puppy leaping over a puddle',
      'A round owl in a little eye mask perched on a branch under a crescent moon',
      'A turtle in a tiny cape carrying a shiny acorn up a garden hill',
    ],
  },
});
