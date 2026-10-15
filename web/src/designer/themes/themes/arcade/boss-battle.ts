import { defineTheme } from '../../define';

export default defineTheme({
  id: 'boss-battle',
  name: 'Boss Battle',
  description: 'A friendly giant robot and a cartoon dragon guard the final arena. Cel-shaded drama for review weeks, test prep and big finales.',
  audiences: ['upper-elementary', 'secondary'],
  tags: ['cel-shaded', 'cartoon', 'robot', 'dragon', 'review', 'test-prep', 'gaming'],
  defaultLayout: 'classic-band',
  iconStyle: 'solid-circle',
  corners: 'soft',
  collection: 'arcade',
  style: 'cel-shaded 3D cartoon game art',
  colourBlind:
    'Purple, orange and steel blue differ in lightness and sit off the red-green axis; yellow sparks are decoration only.',
  palettes: [
    { id: 'purple-arena', name: 'Purple Arena', primary: '#2E1458', onPrimary: '#FFFFFF', secondary: '#FFD447', onSecondary: '#2B2300', accent: '#FF7A1A', surface: '#F4F0FC', onSurface: '#1C1230', background: '#FFFFFF', text: '#1C1230', link: '#5A2AA8' },
    { id: 'molten', name: 'Molten Core', primary: '#5A1E0E', onPrimary: '#FFFFFF', secondary: '#FFC27A', onSecondary: '#3A1A00', accent: '#7A5CFF', surface: '#FFF3EA', onSurface: '#2A140A', background: '#FFFFFF', text: '#2A140A', link: '#9A3A10' },
    { id: 'steel', name: 'Steel Blue', primary: '#1A3550', onPrimary: '#FFFFFF', secondary: '#C7DDF0', onSecondary: '#12263A', accent: '#FFD447', surface: '#EFF4F9', onSurface: '#142030', background: '#FFFFFF', text: '#142030', link: '#22507E' },
  ],
  icons: {
    start: 'swords', syllabus: 'scroll', modules: 'castle', assignments: 'target', discussions: 'messages-square',
    grades: 'crown', help: 'shield', instructor: 'bot', calendar: 'calendar-days', resources: 'heart',
  },
  art: {
    medium: 'Bold cel-shaded 3D cartoon game art with thick clean outlines, saturated colour and dramatic rim lighting',
    palette: 'Electric purple, molten orange and steel blue with bright yellow sparks',
    mood: 'Epic and exciting but friendly, a big final challenge that can be beaten',
    banner: 'A giant friendly round robot with stubby arms and two glowing round eyes standing in a stone arena amid crackling yellow sparks on the right, a broad empty arena floor under a soft purple sky on the left',
    bannerAlt: 'A big friendly cartoon dragon with tiny wings and a curled tail puffing a harmless smoke ring from the top of a castle arena wall on the right, open dusky purple sky on the left',
    welcome: { subject: 'A glowing crystal shield with a star emblem', alt: 'A glowing shield with a star' },
    instructor: { subject: 'A golden crown resting on a velvet cushion', alt: 'A golden crown on a cushion' },
    cards: [
      'A treasure chest unlocking with a burst of light',
      'A small round robot sidekick hovering beside a glowing star',
      'A heart-shaped power-up crystal on a stone pedestal',
    ],
  },
});
