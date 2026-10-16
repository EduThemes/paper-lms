import { defineTheme } from '../define';

export default defineTheme({
  id: 'toy-town',
  name: 'Toy Town',
  description: 'Painted wooden toy blocks in a tidy isometric world: block houses, toy trains and stacking arches.',
  audiences: ['early-elementary'],
  tags: ['isometric', 'wooden-toys', 'building', 'bright'],
  defaultLayout: 'card-grid',
  iconStyle: 'soft-square',
  corners: 'soft',
  style: 'isometric painted wooden toys',
  colourBlind:
    'Cobalt and yellow are the working pair; red-orange is decoration and never sits beside green to make a distinction.',
  palettes: [
    { id: 'classic', name: 'Toy Box', primary: '#1D3F91', onPrimary: '#FFFFFF', secondary: '#FFCF3F', onSecondary: '#2E2400', accent: '#F26B3A', surface: '#FBF3E4', onSurface: '#1F2430', background: '#FFFFFF', text: '#1F2430', link: '#1D3F91' },
    { id: 'nursery', name: 'Pastel Nursery', primary: '#3C4F76', onPrimary: '#FFFFFF', secondary: '#F9D6E0', onSecondary: '#5A1F35', accent: '#8FD3C1', surface: '#F3F7FB', onSurface: '#1F2430', background: '#FFFFFF', text: '#1F2430', link: '#3C4F76' },
    { id: 'brick', name: 'Brick & Sky', primary: '#A33510', onPrimary: '#FFFFFF', secondary: '#9FD4F5', onSecondary: '#0B2E4A', accent: '#FFC93C', surface: '#FFF4EC', onSurface: '#2A211C', background: '#FFFFFF', text: '#2A211C', link: '#A33510' },
  ],
  icons: {
    start: 'blocks', syllabus: 'clipboard-list', modules: 'boxes', assignments: 'pencil', discussions: 'message-circle',
    grades: 'trophy', help: 'circle-help', instructor: 'school', calendar: 'calendar-days', resources: 'toy-brick',
  },
  art: {
    medium: 'Isometric illustration of painted wooden toy blocks with soft rounded edges, visible wood grain and matte paint',
    palette: 'Cobalt blue, sunshine yellow and red-orange with natural pale wood',
    mood: 'Tidy, bright and inviting, like a toy shelf at the start of the day',
    banner: 'A little wooden toy town of big block houses with triangle roofs, a round lollipop tree and a toy train on the right, a plain pale wooden tabletop on the left',
    bannerAlt: 'A tall bridge of wooden arches and cylinders with a toy car crossing it on the right, a plain warm wooden floor on the left',
    welcome: { subject: 'A wooden toy train engine', alt: 'A wooden toy train' },
    instructor: { subject: 'A wooden abacus with coloured beads', alt: 'A wooden abacus' },
    cards: [
      'A balanced tower of wooden blocks',
      'A wooden toy sailboat',
      'A wooden shape-sorter cube with a star block beside it',
    ],
  },
});
