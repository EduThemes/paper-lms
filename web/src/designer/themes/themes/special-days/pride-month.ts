import { defineTheme, FONTS } from '../../define';

export default defineTheme({
  id: 'pride-month',
  name: 'Pride Month',
  description: 'Abstract translucent ribbons, arcs and a prism in rainbow and progress colours. Joyful and welcoming, with no slogans or symbols to read.',
  audiences: ['secondary', 'higher-ed', 'professional'],
  tags: ['heritage', 'pride', 'lgbtq', 'awareness', 'abstract'],
  collection: 'special-days',
  observance: {
    name: 'Pride Month',
    traditions: [],
    months: [6],
    regionalMonths: { GB: [2, 6] },
    note: 'US: June. UK: LGBT+ History Month is February. US LGBTQ+ History Month is October.',
  },
  defaultLayout: 'split-hero',
  iconStyle: 'solid-circle',
  headingFont: FONTS.helvetica,
  corners: 'round',
  style: 'abstract translucent geometry',
  colourBlind:
    'The rainbow is decorative and always in spectral order, so it reads by position; structure uses deep purple or charcoal on pale panels.',
  palettes: [
    { id: 'spectrum', name: 'Spectrum', primary: '#3B1F6B', onPrimary: '#FFFFFF', secondary: '#FFD84D', onSecondary: '#2E2200', accent: '#E8467C', surface: '#F6F2FC', onSurface: '#1F1A2E', background: '#FFFFFF', text: '#1F1A2E', link: '#5A2FA0' },
    { id: 'progress', name: 'Progress', primary: '#2B2B2B', onPrimary: '#FFFFFF', secondary: '#F7C6D9', onSecondary: '#4A1A30', accent: '#7FD1EA', surface: '#EEF8FC', onSurface: '#1C2228', background: '#FFFFFF', text: '#1C2228', link: '#1F5D86' },
    { id: 'sunrise', name: 'Sunrise', primary: '#8A2A12', onPrimary: '#FFFFFF', secondary: '#FFE08A', onSecondary: '#3A2A00', accent: '#3BA0E0', surface: '#FFF5EC', onSurface: '#2A1A14', background: '#FFFFFF', text: '#2A1A14', link: '#9A3416' },
  ],
  icons: {
    start: 'rainbow', syllabus: 'scroll', modules: 'layers', assignments: 'pen-tool', discussions: 'messages-square',
    grades: 'star', help: 'circle-help', instructor: 'heart-handshake', calendar: 'calendar-days', resources: 'library',
  },
  art: {
    medium: 'Bold abstract geometric illustration with overlapping translucent shapes, smooth gradients and crisp edges',
    palette: 'Full rainbow spectrum with soft pink, light blue, brown and black accents on a clean white ground',
    mood: 'Joyful, welcoming and proud',
    banner: 'Broad translucent ribbons in rainbow order sweeping in a wide arc across the right, crossed by a chevron of pink, light blue, white, brown and black stripes, calm plain white on the left',
    bannerAlt: 'Large overlapping translucent circles in rainbow colours gathered on the right like a soft bouquet of light, calm plain white on the left',
    welcome: { subject: 'A small rainbow made of rounded arcs', alt: 'A rainbow' },
    instructor: { subject: 'A paper heart in rainbow stripes', alt: 'A rainbow-striped heart' },
    cards: [
      'Concentric rainbow arcs over soft rolling shapes',
      'A field of overlapping translucent triangles in rainbow colours',
      'A glass prism splitting a beam of light into a rainbow',
    ],
  },
});
