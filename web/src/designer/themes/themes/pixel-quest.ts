import { defineTheme } from '../define';

export default defineTheme({
  id: 'pixel-quest',
  name: 'Pixel Quest',
  description: 'Crisp 16-bit pixel art: floating islands, treasure chests and power-ups. Retro-game energy without the noise.',
  audiences: ['upper-elementary', 'secondary'],
  tags: ['pixel-art', 'retro', 'gaming', 'quest'],
  defaultLayout: 'card-grid',
  iconStyle: 'soft-square',
  corners: 'square',
  style: '16-bit pixel art',
  colourBlind:
    'Coin gold on midnight indigo works by lightness alone; leaf green sits next to gold only in the art, never as a status colour.',
  palettes: [
    { id: 'midnight', name: 'Midnight Run', primary: '#2B2D6E', onPrimary: '#FFFFFF', secondary: '#FFD447', onSecondary: '#2B2300', accent: '#4CC9F0', surface: '#EEF0FF', onSurface: '#1B1B2F', background: '#FFFFFF', text: '#1B1B2F', link: '#3236A0' },
    { id: 'lava', name: 'Lava Cave', primary: '#4A1D3F', onPrimary: '#FFFFFF', secondary: '#FF9F5A', onSecondary: '#3A1600', accent: '#FFD447', surface: '#FBEFF3', onSurface: '#24151F', background: '#FFFFFF', text: '#24151F', link: '#6B2358' },
    { id: 'sky-island', name: 'Sky Island', primary: '#0B4F6C', onPrimary: '#FFFFFF', secondary: '#B6F0C8', onSecondary: '#0B3A22', accent: '#FFB703', surface: '#EAF7FB', onSurface: '#132430', background: '#FFFFFF', text: '#132430', link: '#0B4F6C' },
  ],
  icons: {
    start: 'gamepad-2', syllabus: 'scroll', modules: 'map', assignments: 'target', discussions: 'messages-square',
    grades: 'trophy', help: 'heart-handshake', instructor: 'wand-sparkles', calendar: 'calendar-clock', resources: 'gem',
  },
  art: {
    medium: 'Crisp 16-bit pixel art with a limited colour palette, clean square pixel edges and simple two-step shading',
    palette: 'Midnight indigo, coin gold and leaf green with bright sky cyan',
    mood: 'Playful and heroic like a classic adventure game, tidy rather than busy',
    banner: 'A large pixel-art floating island with a waterfall, a treasure chest and a round tree on the right, a plain flat pixel sky gradient on the left',
    bannerAlt: 'A big pixel-art castle on a green hill under a crescent moon and twinkling stars on the right, a plain dark pixel sky on the left',
    welcome: { subject: 'A pixel-art treasure chest with a few gold coins', alt: 'A pixel treasure chest with coins' },
    instructor: { subject: "A pixel-art wizard's hat beside a glowing potion bottle", alt: "A pixel wizard's hat and a potion" },
    cards: [
      'A pixel-art golden key floating over grass',
      'A pixel-art heart and star power-up hovering above a stone platform',
      'A pixel-art rocket lifting off from a grassy hill',
    ],
  },
});
