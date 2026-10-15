import { defineTheme } from '../define';

export default defineTheme({
  id: 'clay-corner',
  name: 'Clay Corner',
  description: 'Chunky handmade plasticine shapes with fingerprint texture: rainbows, snails and toy boats that feel squishable.',
  audiences: ['early-elementary'],
  tags: ['clay', '3d', 'tactile', 'playful'],
  defaultLayout: 'playful',
  iconStyle: 'soft-square',
  corners: 'round',
  style: 'handmade plasticine claymation',
  colourBlind:
    'Teal and tomato red are told apart by lightness (deep teal band, pale tiles), and no state is ever shown as red versus green.',
  palettes: [
    { id: 'peach', name: 'Peach Dough', primary: '#0F5E63', onPrimary: '#FFFFFF', secondary: '#FFE08A', onSecondary: '#3B2F00', accent: '#F0644B', surface: '#FFF1E8', onSurface: '#2A2A2A', background: '#FFFFFF', text: '#2A2A2A', link: '#0E5C61' },
    { id: 'grape', name: 'Grape Jelly', primary: '#4B2C82', onPrimary: '#FFFFFF', secondary: '#FFD1A6', onSecondary: '#4A2400', accent: '#F59E0B', surface: '#F4EEFF', onSurface: '#2A2433', background: '#FFFFFF', text: '#2A2433', link: '#4B2C82' },
    { id: 'blueberry', name: 'Blueberry', primary: '#1F3F8F', onPrimary: '#FFFFFF', secondary: '#BFE6D8', onSecondary: '#0F3B2E', accent: '#FF8A65', surface: '#EEF6FF', onSurface: '#1E2633', background: '#FFFFFF', text: '#1E2633', link: '#1F3F8F' },
  ],
  icons: {
    start: 'play', syllabus: 'clipboard-list', modules: 'shapes', assignments: 'palette', discussions: 'messages-square',
    grades: 'award', help: 'hand-helping', instructor: 'apple', calendar: 'calendar-days', resources: 'puzzle',
  },
  art: {
    medium: 'Handmade plasticine claymation illustration with chunky rounded shapes, gentle fingerprint texture and soft studio lighting',
    palette: 'Deep teal, tomato red and butter yellow with soft lilac on a pale peach ground',
    mood: 'Cosy, tactile and gently funny',
    banner: 'A big chunky clay rainbow arching over two round clay trees and a clay snail on the right, smooth plain peach clay ground on the left',
    bannerAlt: 'A large clay toy boat, a striped ball and a stack of round clay blocks on the right, a smooth plain peach backdrop on the left',
    welcome: { subject: 'A round clay hot-air balloon with a little basket', alt: 'A clay hot-air balloon' },
    instructor: { subject: 'A clay coffee mug filled with chunky clay pencils', alt: 'A clay mug full of pencils' },
    cards: [
      'A clay snail resting on a spotted clay mushroom',
      'A clay paint palette with round dabs of colour and a brush',
      'A wobbly stack of rounded clay building blocks',
    ],
  },
});
