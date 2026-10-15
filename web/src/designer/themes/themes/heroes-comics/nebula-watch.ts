import { defineTheme } from '../../define';

export default defineTheme({
  id: 'nebula-watch',
  name: 'Nebula Watch',
  description: 'Caped space guardians seen from afar against glowing nebulae, ringed planets and shimmering energy shields.',
  audiences: ['upper-elementary', 'secondary'],
  tags: ['space', 'superhero', 'nebula', 'cosmic', 'glow'],
  defaultLayout: 'split-hero',
  iconStyle: 'solid-circle',
  corners: 'soft',
  collection: 'heroes-comics',
  style: 'luminous space-opera painting',
  colourBlind:
    'Deep violet or navy against pale teal or gold differ strongly in lightness; the pink glow is atmosphere only and never encodes meaning.',
  palettes: [
    { id: 'nebula', name: 'Nebula', primary: '#2A1858', onPrimary: '#FFFFFF', secondary: '#9EE6E0', onSecondary: '#0B3A38', accent: '#FFC93C', surface: '#F3F0FC', onSurface: '#1A1530', background: '#FFFFFF', text: '#1A1530', link: '#4B2BA0' },
    { id: 'supernova', name: 'Supernova', primary: '#5C1440', onPrimary: '#FFFFFF', secondary: '#FFC2A1', onSecondary: '#4A1A00', accent: '#FF5E8A', surface: '#FFF1F3', onSurface: '#26121D', background: '#FFFFFF', text: '#26121D', link: '#7A1D57' },
    { id: 'starfield', name: 'Starfield', primary: '#0E2A4F', onPrimary: '#FFFFFF', secondary: '#FFE08A', onSecondary: '#3A2A00', accent: '#4FC3F7', surface: '#EEF4FB', onSurface: '#0F1A2A', background: '#FFFFFF', text: '#0F1A2A', link: '#154C8A' },
  ],
  icons: {
    start: 'rocket', syllabus: 'scroll', modules: 'orbit', assignments: 'target', discussions: 'radio',
    grades: 'star', help: 'shield-check', instructor: 'shield', calendar: 'calendar-days', resources: 'satellite',
  },
  art: {
    medium:
      'Luminous painted space-opera illustration with glowing airbrushed nebula clouds, crisp star specks and bold cel-shaded shapes',
    palette: 'Deep violet and electric teal with a glowing gold star on midnight blue',
    mood: 'Vast, heroic and calm, guardians keeping watch over the stars',
    banner:
      'A lone caped space hero seen from behind, small in the distance, standing on a little asteroid before a swirling violet nebula and a huge ringed planet, a glowing round energy shield on one arm, on the right, calm deep starry space on the left',
    bannerAlt:
      'A sleek rounded starship wrapped in a glowing hexagon energy shield drifting past a ringed planet on the right, quiet dark starfield on the left',
    welcome: { subject: 'A glowing crystal star with soft rays of light', alt: 'A glowing crystal star' },
    instructor: { subject: 'A glowing crystal compass with a gold star-shaped needle resting on a moon rock', alt: 'A round star shield on a moon rock' },
    cards: [
      'A glowing hexagon energy shield floating in deep space',
      'A comet streaking past a small cratered moon',
      'A caped hero seen from behind on an asteroid, gazing at a spiral galaxy',
    ],
  },
});
