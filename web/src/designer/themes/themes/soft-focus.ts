import { defineTheme, FONTS } from '../define';

export default defineTheme({
  id: 'soft-focus',
  name: 'Soft Focus',
  description: 'Frosted-glass shapes over a blurred pastel gradient. Polished and quiet, for professional learning and onboarding.',
  audiences: ['professional', 'higher-ed'],
  tags: ['glass', '3d', 'gradient', 'corporate'],
  defaultLayout: 'split-hero',
  iconStyle: 'soft-square',
  headingFont: FONTS.lato,
  corners: 'round',
  style: 'frosted glass and soft gradients',
  colourBlind:
    'Periwinkle and peach differ in lightness; the scheme contains no red/green distinctions.',
  palettes: [
    { id: 'periwinkle', name: 'Periwinkle', primary: '#2E3A73', onPrimary: '#FFFFFF', secondary: '#DCE1FF', onSecondary: '#26306A', accent: '#FFB59A', surface: '#F3F4FB', onSurface: '#1C2033', background: '#FFFFFF', text: '#1C2033', link: '#3640A0' },
    { id: 'peach', name: 'Peach', primary: '#6B2F3A', onPrimary: '#FFFFFF', secondary: '#FFE1D3', onSecondary: '#5A2A20', accent: '#9AD0C2', surface: '#FFF5F1', onSurface: '#2A1C1F', background: '#FFFFFF', text: '#2A1C1F', link: '#8A2F44' },
    { id: 'sage', name: 'Sage Glass', primary: '#1F4A44', onPrimary: '#FFFFFF', secondary: '#D7EDE6', onSecondary: '#1F4A44', accent: '#C8B6FF', surface: '#F2F7F5', onSurface: '#18241F', background: '#FFFFFF', text: '#18241F', link: '#1F5A52' },
  ],
  icons: {
    start: 'sparkles', syllabus: 'file-text', modules: 'layers', assignments: 'clipboard-check', discussions: 'messages-square',
    grades: 'chart-column', help: 'circle-help', instructor: 'user-round', calendar: 'calendar-days', resources: 'folder-open',
  },
  art: {
    medium: 'Soft abstract 3D render of frosted-glass shapes floating over a blurred mesh gradient, gentle studio light and minimal composition',
    palette: 'Periwinkle and deep indigo with soft peach and a hint of mint',
    mood: 'Polished, calm and quietly premium',
    banner: 'A cluster of large floating frosted-glass spheres, a ring and a pill-shaped capsule on the right, a smooth empty pastel gradient on the left',
    bannerAlt: 'Wide translucent glass ribbons curving and overlapping on the right, a smooth empty pastel gradient on the left',
    welcome: { subject: 'A frosted glass sphere resting on a soft round pedestal', alt: '' },
    instructor: { subject: 'A frosted glass coffee cup on a small saucer', alt: 'A glass coffee cup' },
    cards: [
      'A single frosted glass ring standing upright',
      'A stack of three frosted glass discs',
      'A frosted glass arch over a soft gradient floor',
    ],
  },
});
