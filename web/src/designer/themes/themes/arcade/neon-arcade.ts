import { defineTheme } from '../../define';

export default defineTheme({
  id: 'neon-arcade',
  name: 'Neon Arcade',
  description: 'Synthwave neon line art: a glowing joystick, arcade buttons and a striped sunset over a perspective grid. After-hours cool for teens.',
  audiences: ['secondary', 'higher-ed'],
  tags: ['neon', 'synthwave', 'arcade', 'retro', 'line-art', '1980s'],
  defaultLayout: 'split-hero',
  iconStyle: 'duotone',
  corners: 'soft',
  collection: 'arcade',
  style: 'synthwave neon line art',
  colourBlind:
    'Magenta and cyan sit on the blue-red axis and differ in lightness, so red-green colour blindness keeps them apart; neither colour ever carries meaning on its own.',
  palettes: [
    { id: 'magenta-night', name: 'Magenta Night', primary: '#2A0E4A', onPrimary: '#FFFFFF', secondary: '#FF9FE0', onSecondary: '#3A0A30', accent: '#22D3EE', surface: '#F6F0FF', onSurface: '#1B0F2E', background: '#FFFFFF', text: '#1B0F2E', link: '#7A1FA8' },
    { id: 'cyan-drive', name: 'Cyan Drive', primary: '#0A2E44', onPrimary: '#FFFFFF', secondary: '#9FF3FF', onSecondary: '#08303A', accent: '#FF4FB3', surface: '#EDFAFD', onSurface: '#0F1E28', background: '#FFFFFF', text: '#0F1E28', link: '#0B5577' },
    { id: 'sunset-strip', name: 'Sunset Strip', primary: '#4A1030', onPrimary: '#FFFFFF', secondary: '#FFC27A', onSecondary: '#3A1A00', accent: '#B05CFF', surface: '#FFF3EC', onSurface: '#2A0E1E', background: '#FFFFFF', text: '#2A0E1E', link: '#9A2A40' },
  ],
  icons: {
    start: 'joystick', syllabus: 'scroll-text', modules: 'layers', assignments: 'zap', discussions: 'messages-square',
    grades: 'trophy', help: 'circle-help', instructor: 'headphones', calendar: 'calendar-days', resources: 'disc-3',
  },
  art: {
    medium: 'Glowing neon line art in the synthwave style, luminous tube outlines with soft bloom on a deep violet night with a faint perspective grid',
    palette: 'Hot magenta and electric cyan neon with sunset orange on deep violet',
    mood: 'Energetic, sleek and after-hours cool',
    banner: 'A huge glowing neon joystick and a cluster of round arcade buttons in front of a striped setting sun on the right, a quiet dark violet sky over the faint grid on the left',
    bannerAlt: 'An arcade cabinet seen from the side in silhouette, outlined in glowing magenta tubes, beside a cyan grid road running to a striped sun on the right, calm empty violet night on the left',
    welcome: { subject: 'A glowing neon joystick', alt: 'A neon joystick' },
    instructor: { subject: 'A pair of glowing neon headphones', alt: 'Neon headphones' },
    cards: [
      'A neon lightning bolt over a glowing grid horizon',
      'A neon starburst rising above the grid',
      'A neon palm tree beside a striped setting sun',
    ],
  },
});
