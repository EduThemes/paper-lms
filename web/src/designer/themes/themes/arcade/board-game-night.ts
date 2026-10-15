import { defineTheme, FONTS } from '../../define';

export default defineTheme({
  id: 'board-game-night',
  name: 'Board Game Night',
  description: 'Wooden pawns, hex tiles, spinners and sand timers on teal felt under a warm lamp. Social, strategic and grown-up enough for seminars.',
  audiences: ['higher-ed', 'secondary', 'professional'],
  tags: ['board-games', 'tabletop', 'photography', 'strategy', 'social', 'gamification'],
  defaultLayout: 'card-grid',
  iconStyle: 'soft-square',
  headingFont: FONTS.trebuchet,
  corners: 'soft',
  collection: 'arcade',
  style: 'tabletop board game photography',
  colourBlind:
    'Teal felt and brick-red pieces differ in lightness as well as hue; game pieces are told apart by shape (pawn, cube, hexagon), not colour.',
  palettes: [
    { id: 'teal-felt', name: 'Teal Felt', primary: '#12403F', onPrimary: '#FFFFFF', secondary: '#F2CF7A', onSecondary: '#3A2A00', accent: '#C0533A', surface: '#EEF6F4', onSurface: '#142624', background: '#FFFFFF', text: '#142624', link: '#145A57' },
    { id: 'walnut', name: 'Walnut Table', primary: '#3E2A1A', onPrimary: '#FFFFFF', secondary: '#EED9B8', onSecondary: '#3A2410', accent: '#2F8F83', surface: '#FAF3EA', onSurface: '#24180F', background: '#FFFFFF', text: '#24180F', link: '#7A4518' },
    { id: 'brick', name: 'Brick & Mustard', primary: '#6A2620', onPrimary: '#FFFFFF', secondary: '#F5D8C8', onSecondary: '#4A1A14', accent: '#E0B03A', surface: '#FBF0EC', onSurface: '#2A1512', background: '#FFFFFF', text: '#2A1512', link: '#8A3226' },
  ],
  icons: {
    start: 'dices', syllabus: 'scroll-text', modules: 'hexagon', assignments: 'target', discussions: 'messages-square',
    grades: 'medal', help: 'circle-help', instructor: 'hourglass', calendar: 'calendar-days', resources: 'puzzle',
  },
  art: {
    medium: 'Warm overhead tabletop photography of wooden board game pieces on a felt-covered table, soft lamplight and a gentle depth of field',
    palette: 'Deep teal felt, honey wood and brick red with mustard yellow',
    mood: 'Social, relaxed and quietly competitive',
    banner: 'A scatter of wooden pawns, plain hexagonal tiles, a spinner with coloured wedges and two dice showing simple dotted pips in a pool of lamplight on the right, smooth empty teal felt on the left',
    bannerAlt: 'A leaning tower of plain wooden blocks beside a stack of round wooden tokens and a little cloth bag spilling glass beads on the right, quiet plain teal felt on the left',
    welcome: { subject: 'A wooden pawn beside two dice with simple dotted pips', alt: 'A wooden pawn and two dice' },
    instructor: { subject: 'A wooden sand timer beside a cloth pouch of tokens', alt: 'A sand timer and a pouch of tokens' },
    cards: [
      'A spinner with coloured wedges and a single arrow',
      'A small pile of glass gem tokens on felt',
      'A wooden spinning top and three plain wooden pawns on felt',
    ],
  },
});
