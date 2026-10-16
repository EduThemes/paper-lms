import { defineTheme } from '../../define';

export default defineTheme({
  id: 'quantitown',
  name: 'Quantitown',
  description: 'A cheerful isometric pixel city where the towers are plus, times, divide and equals shapes. Math-arcade energy, tidy enough to teach on.',
  audiences: ['upper-elementary', 'secondary'],
  tags: ['pixel-art', 'isometric', 'math', 'city', 'arcade', 'quantitown'],
  defaultLayout: 'card-grid',
  iconStyle: 'soft-square',
  corners: 'square',
  collection: 'arcade',
  style: 'isometric pixel art',
  colourBlind:
    'Sunny yellow on deep indigo carries the structure by lightness; mint and pink appear together only as decoration in the art.',
  palettes: [
    { id: 'sunny-town', name: 'Sunny Town', primary: '#2E2A6E', onPrimary: '#FFFFFF', secondary: '#FFD447', onSecondary: '#2B2300', accent: '#FF7AB6', surface: '#F3F0FF', onSurface: '#1D1A3A', background: '#FFFFFF', text: '#1D1A3A', link: '#4036A8' },
    { id: 'mint-street', name: 'Mint Street', primary: '#0D4D4A', onPrimary: '#FFFFFF', secondary: '#A8F0D4', onSecondary: '#0B3A2C', accent: '#FFD447', surface: '#ECFAF5', onSurface: '#122826', background: '#FFFFFF', text: '#122826', link: '#0E5F5A' },
    { id: 'bubblegum', name: 'Bubblegum Block', primary: '#6B1F55', onPrimary: '#FFFFFF', secondary: '#FFC2DE', onSecondary: '#4A1238', accent: '#5BC0EB', surface: '#FFF0F7', onSurface: '#2B1426', background: '#FFFFFF', text: '#2B1426', link: '#8A2470' },
  ],
  icons: {
    start: 'gamepad-2', syllabus: 'scroll', modules: 'building-2', assignments: 'calculator', discussions: 'messages-square',
    grades: 'trophy', help: 'circle-help', instructor: 'blocks', calendar: 'calendar-days', resources: 'boxes',
  },
  art: {
    medium: 'Cheerful isometric pixel art with chunky clean pixels, bright flat colours and simple two-tone shading',
    palette: 'Sunny yellow, sky blue and bubblegum pink with mint green on soft lavender',
    mood: 'Bright, friendly and buzzing with puzzle-solving fun',
    banner: 'An isometric pixel city block on the right whose towers are shaped like a plus symbol, a multiplication cross, a division symbol and two stacked equal bars, with plain lit square windows and tiny round trees, and no billboards, signs, screens or posters anywhere, open soft lavender pixel sky on the left',
    bannerAlt: 'A tiny friendly round pixel school bus with plain windows trundling along a winding road past plus-shaped houses and a fountain shaped like a multiplication cross on the right, open pastel pixel sky on the left',
    welcome: { subject: 'An isometric pixel tower shaped like a big plus symbol with square windows', alt: 'A pixel tower shaped like a plus symbol' },
    instructor: { subject: 'An isometric pixel toolbox holding colourful building pieces shaped like plus symbols and multiplication crosses', alt: 'A pixel toolbox of plus-shaped building pieces' },
    cards: [
      'An isometric pixel bridge of two equal bars spanning a sparkling river',
      'An isometric pixel park with a round splashing fountain, benches, flower beds and round trees',
      'An isometric pixel running track looping around a grassy field with round trees and a small pond',
    ],
  },
});
