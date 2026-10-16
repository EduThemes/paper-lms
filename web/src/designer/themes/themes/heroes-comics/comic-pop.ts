import { defineTheme } from '../../define';

export default defineTheme({
  id: 'comic-pop',
  name: 'Comic Pop',
  description: 'Vintage comic-book print: bold black ink, flat primaries, Ben-Day dots and empty starbursts. Big energy without a single sound effect.',
  audiences: ['upper-elementary', 'secondary'],
  tags: ['comic', 'ben-day-dots', 'halftone', 'superhero', 'bold', 'retro'],
  defaultLayout: 'split-hero',
  iconStyle: 'solid-circle',
  corners: 'square',
  collection: 'heroes-comics',
  style: 'Ben-Day dot comic print',
  colourBlind:
    'Deep comic blue and lemon yellow carry the structure and differ hugely in lightness; the red accent is decoration only and never sits beside green to mean anything.',
  palettes: [
    { id: 'classic', name: 'Classic Pop', primary: '#1B3A8C', onPrimary: '#FFFFFF', secondary: '#FFD93B', onSecondary: '#2A2000', accent: '#E63946', surface: '#FFF8DC', onSurface: '#1A1A1A', background: '#FFFFFF', text: '#1A1A1A', link: '#1B3A8C' },
    { id: 'crimson', name: 'Crimson Ink', primary: '#8E1B2A', onPrimary: '#FFFFFF', secondary: '#9FD8F0', onSecondary: '#0F3247', accent: '#FFC43D', surface: '#FFF1EF', onSurface: '#1F1416', background: '#FFFFFF', text: '#1F1416', link: '#8E1B2A' },
    { id: 'night', name: 'Night Edition', primary: '#241B4F', onPrimary: '#FFFFFF', secondary: '#FFB067', onSecondary: '#3A1800', accent: '#29C4E0', surface: '#F2F0FB', onSurface: '#17142B', background: '#FFFFFF', text: '#17142B', link: '#3B2C8F' },
  ],
  icons: {
    start: 'zap', syllabus: 'clipboard-list', modules: 'layers', assignments: 'pen-tool', discussions: 'messages-square',
    grades: 'star', help: 'life-buoy', instructor: 'shield', calendar: 'calendar-days', resources: 'sparkles',
  },
  art: {
    medium:
      'Vintage comic-book print illustration with bold black ink outlines, flat primary colour fills and Ben-Day halftone dots on slightly yellowed newsprint, drawn as one single unframed scene that runs off every edge with no panel borders or gutters',
    palette: 'Deep comic blue, lemon yellow and fire-engine red with black ink and newsprint cream',
    mood: 'Punchy, bright and heroic, with all the action and none of the noise',
    banner:
      'A huge empty jagged starburst filled only with Ben-Day dots, a crackling lightning bolt and bold speed lines bursting outward on the right, a calm field of soft pale halftone dots across the left',
    bannerAlt:
      'A small hero in a lemon-yellow suit with a flowing deep-blue cape and no chest emblem, seen from behind, small in the distance, soaring over a chunky city skyline toward a big empty starburst on the right, the same calm pale dotted sky continuing across the left',
    welcome: { subject: 'A big empty jagged starburst shape shaded with Ben-Day dots', alt: '' },
    instructor: { subject: 'A plain red cape draped over the back of a wooden chair', alt: 'A red cape draped over a chair' },
    cards: [
      'An empty jagged starburst with radiating speed lines',
      'A lightning bolt striking past a rooftop water tower, the stormy sky and rooftops filling the picture right to all four edges with no white margin and no black frame line',
      'A plain domino eye mask resting on a brick rooftop ledge',
    ],
  },
});
