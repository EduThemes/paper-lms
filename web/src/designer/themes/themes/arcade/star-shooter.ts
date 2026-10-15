import { defineTheme } from '../../define';

export default defineTheme({
  id: 'star-shooter',
  name: 'Star Shooter',
  description: 'Chunky 8-bit space shooter art: a zippy starship, ringed planets and tumbling asteroids. Big pixels, bright lasers, calm layout.',
  audiences: ['upper-elementary', 'secondary'],
  tags: ['pixel-art', '8-bit', 'space', 'arcade', 'retro'],
  defaultLayout: 'classic-band',
  iconStyle: 'soft-square',
  corners: 'square',
  collection: 'arcade',
  style: '8-bit pixel art',
  colourBlind:
    'Laser cyan and hot pink on deep navy are told apart by lightness and the blue-red axis; Plasma Green pairs green with pink only as decoration.',
  palettes: [
    { id: 'deep-space', name: 'Deep Space', primary: '#141A46', onPrimary: '#FFFFFF', secondary: '#7FF0FF', onSecondary: '#08303A', accent: '#FF5FA2', surface: '#EEF0FF', onSurface: '#12152E', background: '#FFFFFF', text: '#12152E', link: '#3A3FB0' },
    { id: 'nova-pink', name: 'Nova Pink', primary: '#4A0F3E', onPrimary: '#FFFFFF', secondary: '#FFB3D9', onSecondary: '#3E0A30', accent: '#4FE3FF', surface: '#FFF0F7', onSurface: '#26101F', background: '#FFFFFF', text: '#26101F', link: '#8A1F6E' },
    { id: 'plasma', name: 'Plasma Green', primary: '#0E3A2A', onPrimary: '#FFFFFF', secondary: '#B9F7A6', onSecondary: '#12350A', accent: '#FF5FA2', surface: '#EEFAEA', onSurface: '#12241A', background: '#FFFFFF', text: '#12241A', link: '#1A5E3A' },
  ],
  icons: {
    start: 'rocket', syllabus: 'scroll', modules: 'orbit', assignments: 'crosshair', discussions: 'messages-square',
    grades: 'trophy', help: 'life-buoy', instructor: 'satellite', calendar: 'calendar-days', resources: 'sparkles',
  },
  art: {
    medium: 'Chunky low-resolution 8-bit pixel art with big square pixels, a tiny limited colour set and hard dark outlines',
    palette: 'Laser cyan, starship white and hot pink on deep space navy',
    mood: 'Fast, bright and upbeat like a classic arcade shooter',
    banner: 'A chunky pixel starship with a glowing engine trail zooming past a big ringed planet and a few tumbling asteroids on the right, plain dark space with tiny scattered pixel stars on the left',
    bannerAlt: 'A big round friendly pixel mothership with blinking lights hovering over a small cratered moon on the right, calm dark pixel space with a few faint stars on the left',
    welcome: { subject: 'A chunky pixel starship with a cyan engine glow', alt: 'A pixel starship' },
    instructor: { subject: 'An empty pixel space helmet with a closed shiny visor beside a small star', alt: 'A pixel space helmet and a star' },
    cards: [
      'A pixel ringed planet with two small moons',
      'Pixel asteroids drifting around a glowing star',
      'A pixel comet streaking past a crescent moon',
    ],
  },
});
