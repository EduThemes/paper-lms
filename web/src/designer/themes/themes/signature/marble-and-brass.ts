import { defineTheme, FONTS } from '../../define';

export default defineTheme({
  id: 'marble-and-brass',
  name: 'Marble & Brass',
  description: 'Veined marble columns, polished brass and museum light. Classical gravity for law, humanities, history and leadership courses.',
  audiences: ['higher-ed', 'professional', 'secondary'],
  tags: ['classical', 'marble', 'brass', 'architecture', 'law', 'humanities'],
  defaultLayout: 'sidebar',
  iconStyle: 'outline',
  headingFont: FONTS.georgia,
  corners: 'square',
  collection: 'signature',
  premium: true,
  style: 'marble and brass architectural still-life render',
  colourBlind:
    'White marble, dark slate and brass differ in lightness; the Verde Antico green is never paired with a red to mean anything.',
  palettes: [
    { id: 'carrara', name: 'Carrara', primary: '#2B2F36', onPrimary: '#FFFFFF', secondary: '#E9E4D8', onSecondary: '#2B2F36', accent: '#B08D47', surface: '#F5F4F1', onSurface: '#1C1E22', background: '#FFFFFF', text: '#1C1E22', link: '#3B4F7A' },
    { id: 'nero', name: 'Nero & Brass', primary: '#1A1A1A', onPrimary: '#FFFFFF', secondary: '#E8DCC0', onSecondary: '#2A2416', accent: '#B08D47', surface: '#F4F2EE', onSurface: '#151515', background: '#FFFFFF', text: '#151515', link: '#6B4E16' },
    { id: 'verde-antico', name: 'Verde Antico', primary: '#1F3D36', onPrimary: '#FFFFFF', secondary: '#E2E9E2', onSecondary: '#1F3D36', accent: '#B08D47', surface: '#F1F5F2', onSurface: '#17241F', background: '#FFFFFF', text: '#17241F', link: '#23584B' },
  ],
  icons: {
    start: 'landmark', syllabus: 'scroll-text', modules: 'columns-3', assignments: 'feather', discussions: 'messages-square',
    grades: 'scale', help: 'circle-help', instructor: 'gavel', calendar: 'calendar-days', resources: 'library',
  },
  art: {
    medium: 'Elegant architectural still-life render of white veined marble and polished brass, soft museum lighting, crisp shadows and a calm symmetrical composition',
    palette: 'White Carrara marble, polished brass and deep slate with warm ivory',
    mood: 'Timeless, measured and dignified',
    banner: 'A row of tall fluted marble columns on a stepped marble plinth with a polished brass balance scale standing before them on the right, a smooth plain marble wall continuing to the left',
    bannerAlt: 'A sweeping curved marble staircase with a polished brass handrail spiralling upward on the right, a smooth plain marble wall and floor in soft light on the left',
    welcome: { subject: 'A polished brass balance scale on a marble block', alt: 'A brass balance scale on marble' },
    instructor: { subject: 'A brass inkwell and a quill pen on a marble slab', alt: 'A brass inkwell and quill on marble' },
    cards: [
      'A fallen marble Ionic capital resting on a stone floor',
      'A brass hourglass on a marble ledge',
      'A brass laurel wreath on a marble plinth',
    ],
  },
});
