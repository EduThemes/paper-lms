import { defineTheme, FONTS } from '../define';

export default defineTheme({
  id: 'isometric-lab',
  name: 'Isometric Lab',
  description: 'Clean isometric 3D vector scenes of lab benches, glassware and gears. Precise and modern for STEM courses.',
  audiences: ['secondary', 'higher-ed'],
  tags: ['isometric', 'stem', 'science', 'modern'],
  defaultLayout: 'card-grid',
  iconStyle: 'soft-square',
  headingFont: FONTS.lato,
  corners: 'soft',
  style: 'isometric 3D vector',
  colourBlind:
    'Mint and amber are separated by blue-violet primaries and by lightness; nothing depends on a red/green difference.',
  palettes: [
    { id: 'cobalt', name: 'Cobalt & Mint', primary: '#23408E', onPrimary: '#FFFFFF', secondary: '#C8F1E1', onSecondary: '#0B3B2C', accent: '#FFB020', surface: '#F1F4FA', onSurface: '#1A2233', background: '#FFFFFF', text: '#1A2233', link: '#23408E' },
    { id: 'ultraviolet', name: 'Ultraviolet', primary: '#4B2A99', onPrimary: '#FFFFFF', secondary: '#FFE3A3', onSecondary: '#3D2900', accent: '#22C3B5', surface: '#F4F1FC', onSurface: '#1F1A33', background: '#FFFFFF', text: '#1F1A33', link: '#4B2A99' },
    { id: 'graphite', name: 'Graphite', primary: '#253040', onPrimary: '#FFFFFF', secondary: '#FFC9B0', onSecondary: '#4A1E0A', accent: '#3DA5F4', surface: '#F2F4F6', onSurface: '#1A1F26', background: '#FFFFFF', text: '#1A1F26', link: '#1F4F8A' },
  ],
  icons: {
    start: 'flask-conical', syllabus: 'clipboard-list', modules: 'boxes', assignments: 'notebook-pen', discussions: 'messages-square',
    grades: 'chart-column', help: 'circle-help', instructor: 'microscope', calendar: 'calendar-days', resources: 'atom',
  },
  art: {
    medium: 'Clean isometric 3D vector illustration with soft gradients, crisp edges and gentle ambient shadows',
    palette: 'Cobalt blue and fresh mint with amber highlights on cool light grey',
    mood: 'Precise, optimistic and modern, like a well-organised maker lab',
    banner: 'A large isometric lab bench with conical flasks, a microscope and a cluster of interlocking gears on the right, a plain empty pale floor plane on the left',
    bannerAlt: 'A large isometric floating platform holding a DNA double helix, a horseshoe magnet and a light bulb on the right, a plain empty pale floor plane on the left',
    welcome: { subject: 'An isometric conical flask with bubbling liquid', alt: 'A bubbling flask' },
    instructor: { subject: 'An isometric microscope on a small platform', alt: 'A microscope' },
    cards: [
      'An isometric light bulb standing on a small square platform',
      'An isometric stack of interlocking cubes',
      'An isometric horseshoe magnet attracting small metal spheres',
    ],
  },
});
