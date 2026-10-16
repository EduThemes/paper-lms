import { defineTheme, FONTS } from '../define';

export default defineTheme({
  id: 'swiss-grid',
  name: 'Swiss Grid',
  description: 'International-style modernism: flat shapes locked to a strict grid, black and white with one vermilion signal.',
  audiences: ['higher-ed', 'professional'],
  tags: ['swiss', 'editorial', 'minimal', 'modernist'],
  defaultLayout: 'editorial',
  iconStyle: 'outline',
  headingFont: FONTS.helvetica,
  corners: 'square',
  style: 'Swiss editorial modernism',
  colourBlind:
    'Vermilion is only ever a decorative accent; links are blue and the structure relies on black/white lightness, so red-green colour blindness loses nothing.',
  palettes: [
    { id: 'signal', name: 'Signal', primary: '#1A1A1A', onPrimary: '#FFFFFF', secondary: '#E6E8EB', onSecondary: '#1A1A1A', accent: '#E63B2E', surface: '#F3F4F6', onSurface: '#111111', background: '#FFFFFF', text: '#111111', link: '#1E4FC2' },
    { id: 'cobalt', name: 'Cobalt', primary: '#0B3D91', onPrimary: '#FFFFFF', secondary: '#FFE14D', onSecondary: '#222222', accent: '#FF5A36', surface: '#F1F4FA', onSurface: '#121826', background: '#FFFFFF', text: '#121826', link: '#0B3D91' },
    { id: 'sand', name: 'Sand', primary: '#3A3226', onPrimary: '#FFFFFF', secondary: '#E9DCC4', onSecondary: '#3A3226', accent: '#D2552B', surface: '#F7F3EC', onSurface: '#221D16', background: '#FFFFFF', text: '#221D16', link: '#8A3A14' },
  ],
  icons: {
    start: 'circle-arrow-right', syllabus: 'file-text', modules: 'layout-grid', assignments: 'square-pen', discussions: 'messages-square',
    grades: 'chart-column', help: 'circle-help', instructor: 'user-round', calendar: 'calendar-days', resources: 'library',
  },
  art: {
    medium: 'International Style modernist graphic composition of flat geometric shapes aligned to a strict invisible grid, crisp edges and a faint offset-print texture',
    palette: 'Ink black, cool grey and a single signal vermilion on clean white',
    mood: 'Rational, calm and confident',
    banner: 'A large vermilion circle, a black diagonal bar and stacked grey rectangles locked to the grid on the right, a plain white field on the left',
    bannerAlt: 'A rhythmic sequence of bold black vertical lines with one vermilion square stepping across the right, a plain white field on the left',
    welcome: { subject: 'A vermilion circle overlapping a black square', alt: '' },
    instructor: { subject: 'A minimalist desk lamp made of a black arc and a vermilion disc', alt: 'A minimalist desk lamp' },
    cards: [
      'A vermilion circle cut by a black horizontal bar',
      'Overlapping grey rectangles with one small vermilion square',
      'A black triangle pointing up through a grey circle',
    ],
  },
});
