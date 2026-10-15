import { defineTheme, FONTS } from '../define';

export default defineTheme({
  id: 'cut-and-paste',
  name: 'Cut & Paste',
  description: 'Zine-style collage of torn coloured paper, halftone dots and tape strips. Loud in a good way, for teens.',
  audiences: ['secondary'],
  tags: ['collage', 'zine', 'halftone', 'energetic'],
  defaultLayout: 'split-hero',
  iconStyle: 'outline',
  headingFont: FONTS.courier,
  corners: 'square',
  style: 'zine collage',
  colourBlind:
    'Electric blue and tangerine are complementary and safe for colour-blind readers; pink accents never encode meaning.',
  palettes: [
    { id: 'electric', name: 'Electric', primary: '#1C3FAA', onPrimary: '#FFFFFF', secondary: '#FFB38A', onSecondary: '#4A1A00', accent: '#FF5FA2', surface: '#F2F2EE', onSurface: '#161616', background: '#FFFFFF', text: '#161616', link: '#1C3FAA' },
    { id: 'acid', name: 'Acid', primary: '#222222', onPrimary: '#FFFFFF', secondary: '#D7F75B', onSecondary: '#263000', accent: '#7B5CFF', surface: '#F4F4F0', onSurface: '#161616', background: '#FFFFFF', text: '#161616', link: '#4A2FD1' },
    { id: 'sunset', name: 'Sunset', primary: '#5B1E4E', onPrimary: '#FFFFFF', secondary: '#FFD27F', onSecondary: '#3E2800', accent: '#FF7A59', surface: '#FFF1EC', onSurface: '#24161F', background: '#FFFFFF', text: '#24161F', link: '#6E2260' },
  ],
  icons: {
    start: 'zap', syllabus: 'clipboard-list', modules: 'layers', assignments: 'pen-tool', discussions: 'megaphone',
    grades: 'star', help: 'circle-help', instructor: 'mic', calendar: 'calendar-days', resources: 'paperclip',
  },
  art: {
    medium: 'Zine-style collage of torn plain coloured paper with halftone-dot shading, rough scissor edges and strips of masking tape',
    palette: 'Electric blue, tangerine and bubblegum pink with black halftone on newsprint grey',
    mood: 'Loud, scrappy and full of energy, but tidy enough to read around',
    banner: 'A big torn-paper lightning bolt, a halftone star and a paper airplane layered with tape strips on the right, plain newsprint-grey paper on the left',
    bannerAlt: 'A large halftone flower, a torn-paper sun and spiral cut-paper shapes layered on the right, plain newsprint-grey paper on the left',
    welcome: { subject: 'A torn-paper star with halftone shading and a strip of tape', alt: '' },
    instructor: { subject: 'A collage microphone cut from torn paper with halftone shading', alt: 'A collage microphone' },
    cards: [
      'A torn-paper lightning bolt over halftone dots',
      'A halftone paper airplane with a dotted flight path',
      'A torn-paper flower taped onto newsprint-grey paper',
    ],
  },
});
