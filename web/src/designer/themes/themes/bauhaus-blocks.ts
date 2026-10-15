import { defineTheme, FONTS } from '../define';

export default defineTheme({
  id: 'bauhaus-blocks',
  name: 'Bauhaus Blocks',
  description: 'Circles, semicircles and bars balanced like a design-school study. Graphic, bold and subject-neutral.',
  audiences: ['secondary', 'higher-ed'],
  tags: ['bauhaus', 'geometric', 'graphic', 'modernist'],
  defaultLayout: 'card-grid',
  iconStyle: 'solid-circle',
  headingFont: FONTS.helvetica,
  corners: 'square',
  style: 'Bauhaus geometric shapes',
  colourBlind:
    'Cobalt and mustard carry the structure; brick red is decorative and always differs from its neighbours in lightness.',
  palettes: [
    { id: 'primary', name: 'Primaries', primary: '#1F3A93', onPrimary: '#FFFFFF', secondary: '#F2B705', onSecondary: '#2A2000', accent: '#D9462B', surface: '#F5F1E8', onSurface: '#161616', background: '#FFFFFF', text: '#161616', link: '#1F3A93' },
    { id: 'muted', name: 'Muted Studio', primary: '#3B4A5A', onPrimary: '#FFFFFF', secondary: '#E7C8A0', onSecondary: '#3D2A10', accent: '#C76B4A', surface: '#F4F2EE', onSurface: '#1C1F22', background: '#FFFFFF', text: '#1C1F22', link: '#2F4A6E' },
    { id: 'mono', name: 'Mono + Yellow', primary: '#111111', onPrimary: '#FFFFFF', secondary: '#FFD23F', onSecondary: '#1A1A1A', accent: '#2D5BFF', surface: '#F2F2F2', onSurface: '#111111', background: '#FFFFFF', text: '#111111', link: '#1B3FB8' },
  ],
  icons: {
    start: 'circle-play', syllabus: 'list', modules: 'shapes', assignments: 'square-pen', discussions: 'messages-square',
    grades: 'chart-pie', help: 'circle-help', instructor: 'user-round', calendar: 'calendar-days', resources: 'layers',
  },
  art: {
    medium: 'Bauhaus-inspired flat geometric composition of circles, semicircles, quarter-circles and bars with subtle screen-print grain',
    palette: 'Cobalt blue, mustard yellow and brick red with black on warm off-white',
    mood: 'Balanced, confident and graphic',
    banner: 'A large balanced stack of circles, semicircles and thick bars on the right, a plain flat warm off-white field on the left',
    bannerAlt: 'A bold rhythmic row of quarter-circles and stripes like an abstract skyline on the right, a plain flat warm off-white field on the left',
    welcome: { subject: 'A single cobalt circle balanced on a mustard triangle', alt: '' },
    instructor: { subject: 'An abstract desk lamp built from a black bar, a semicircle shade and a yellow circle of light', alt: 'An abstract desk lamp' },
    cards: [
      'A large cobalt circle crossed by a black bar',
      'Three overlapping semicircles in mustard, brick red and cobalt',
      'A staircase of quarter-circles stepping upward',
    ],
  },
});
