import { defineTheme } from '../../define';

export default defineTheme({
  id: 'dungeon-quest',
  name: 'Dungeon Quest',
  description: 'Tabletop fantasy in torchlight: painted miniatures, crafted stone terrain, treasure and polished dice. Adventure without the scares.',
  audiences: ['secondary', 'upper-elementary'],
  tags: ['tabletop', 'fantasy', 'rpg', 'miniatures', 'quest', 'gaming'],
  defaultLayout: 'sidebar',
  iconStyle: 'solid-circle',
  corners: 'soft',
  collection: 'arcade',
  style: 'tabletop miniature photography',
  colourBlind:
    'Torchlit amber against deep teal and plum is a lightness and blue-yellow contrast; nothing is coded red versus green.',
  palettes: [
    { id: 'torchlight', name: 'Torchlight', primary: '#3A2414', onPrimary: '#FFFFFF', secondary: '#F2C879', onSecondary: '#3A2400', accent: '#2F7F7A', surface: '#FAF3E6', onSurface: '#22170E', background: '#FFFFFF', text: '#22170E', link: '#7A4012' },
    { id: 'deep-crypt', name: 'Deep Crypt', primary: '#16343A', onPrimary: '#FFFFFF', secondary: '#CFE3E0', onSecondary: '#12302F', accent: '#E0A93B', surface: '#EEF5F4', onSurface: '#12201F', background: '#FFFFFF', text: '#12201F', link: '#1A5560' },
    { id: 'royal-keep', name: 'Royal Keep', primary: '#3B1E4A', onPrimary: '#FFFFFF', secondary: '#E8D7F0', onSecondary: '#2E173A', accent: '#E0A93B', surface: '#F6F1F9', onSurface: '#1F1528', background: '#FFFFFF', text: '#1F1528', link: '#5E2F80' },
  ],
  icons: {
    start: 'castle', syllabus: 'scroll', modules: 'map', assignments: 'swords', discussions: 'messages-square',
    grades: 'crown', help: 'shield', instructor: 'wand-sparkles', calendar: 'calendar-days', resources: 'dices',
  },
  art: {
    medium: 'Atmospheric tabletop photography of carefully painted fantasy miniatures and crafted terrain, warm torchlight, rich soft shadows and a shallow depth of field',
    palette: 'Torchlit amber, mossy stone grey and deep dungeon teal with gold',
    mood: 'Adventurous, cosy and a little mysterious, never scary',
    banner: 'A crafted stone dungeon archway with two flickering torches, a small open treasure chest and a few unmarked gemstone dice with plain polished sides on the stone floor on the right, a smooth shadowy stone wall continuing to the left',
    bannerAlt: 'A miniature castle tower on a mossy tabletop hill with a tiny painted dragon curled asleep on the battlements on the right, a plain dim wooden tabletop fading into soft shadow on the left',
    welcome: { subject: 'A small wooden treasure chest spilling gold coins', alt: 'A treasure chest of gold coins' },
    instructor: { subject: 'A wizard’s staff with a glowing crystal leaning beside a brass lantern', alt: 'A wizard’s staff and a lantern' },
    cards: [
      'A tiny painted dragon asleep on a pile of gold',
      'A miniature stone bridge over a chasm lit by a single torch',
      'Three unmarked polyhedral dice with plain polished sides beside a lit candle',
    ],
  },
});
