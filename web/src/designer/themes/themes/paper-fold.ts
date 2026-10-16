import { defineTheme, FONTS } from '../define';

export default defineTheme({
  id: 'paper-fold',
  name: 'Paper Fold',
  description: 'Origami cranes, boats and mountains in crisp folded paper with one saffron accent. Precise and understated.',
  audiences: ['professional', 'higher-ed'],
  tags: ['origami', 'minimal', 'neutral', 'corporate'],
  defaultLayout: 'card-grid',
  iconStyle: 'duotone',
  headingFont: FONTS.lato,
  corners: 'soft',
  style: 'folded-paper origami',
  colourBlind:
    'One warm accent on neutral greys: all the contrast comes from lightness, not hue.',
  palettes: [
    { id: 'charcoal', name: 'Charcoal & Saffron', primary: '#2B2D33', onPrimary: '#FFFFFF', secondary: '#F1E6D3', onSecondary: '#2B2D33', accent: '#E8A33D', surface: '#F5F3EF', onSurface: '#1D1F24', background: '#FFFFFF', text: '#1D1F24', link: '#2C4E86' },
    { id: 'harbor', name: 'Harbor', primary: '#173A5E', onPrimary: '#FFFFFF', secondary: '#DCEAF4', onSecondary: '#173A5E', accent: '#F2A541', surface: '#F1F5F9', onSurface: '#18212C', background: '#FFFFFF', text: '#18212C', link: '#1A4C80' },
    { id: 'moss', name: 'Moss', primary: '#2F3E2C', onPrimary: '#FFFFFF', secondary: '#ECE5D6', onSecondary: '#2F3E2C', accent: '#D97941', surface: '#F4F4EF', onSurface: '#1E231C', background: '#FFFFFF', text: '#1E231C', link: '#2E5A6E' },
  ],
  icons: {
    start: 'send', syllabus: 'file-text', modules: 'layers', assignments: 'clipboard-list', discussions: 'messages-square',
    grades: 'chart-column', help: 'circle-help', instructor: 'briefcase', calendar: 'calendar-days', resources: 'folder-open',
  },
  art: {
    medium: 'Origami-style folded paper art with crisp creases, soft directional light and gentle shadows, minimal and precise',
    palette: 'Charcoal grey and warm white paper with a single saffron accent',
    mood: 'Understated, precise and composed',
    banner: 'A flock of large origami cranes rising over folded-paper mountains on the right, plain smooth warm-white paper on the left',
    bannerAlt: 'A large folded-paper boat on folded-paper waves under a saffron paper sun on the right, plain smooth warm-white paper on the left',
    welcome: { subject: 'An origami crane', alt: 'An origami crane' },
    instructor: { subject: 'A folded-paper potted plant with angular leaves', alt: 'A folded-paper plant' },
    cards: [
      'An origami boat on a smooth paper surface',
      'A folded-paper mountain with a saffron sun',
      'A folded paper airplane in mid-flight',
    ],
  },
});
