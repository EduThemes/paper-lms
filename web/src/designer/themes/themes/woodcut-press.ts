import { defineTheme, FONTS } from '../define';

export default defineTheme({
  id: 'woodcut-press',
  name: 'Woodcut Press',
  description: 'Relief woodcut prints with carved lines and visible grain: oak trees, windmills and sailing ships. Made for the humanities.',
  audiences: ['higher-ed', 'secondary'],
  tags: ['woodcut', 'print', 'humanities', 'classic'],
  defaultLayout: 'editorial',
  iconStyle: 'solid-circle',
  headingFont: FONTS.georgia,
  corners: 'square',
  style: 'relief woodcut print',
  colourBlind:
    'Indigo and ochre are a blue-yellow pair, so they remain distinct for red-green colour blindness.',
  palettes: [
    { id: 'indigo', name: 'Indigo & Ochre', primary: '#22284A', onPrimary: '#FFFFFF', secondary: '#EBCB8B', onSecondary: '#3A2A08', accent: '#B5462E', surface: '#F7F2E7', onSurface: '#1E1E24', background: '#FFFFFF', text: '#1E1E24', link: '#2D3A7A' },
    { id: 'oxblood', name: 'Oxblood', primary: '#5E1F24', onPrimary: '#FFFFFF', secondary: '#E9DCC7', onSecondary: '#3A2A1A', accent: '#2E5E6E', surface: '#F8F3EC', onSurface: '#241A18', background: '#FFFFFF', text: '#241A18', link: '#7A2530' },
    { id: 'sea-ink', name: 'Sea Ink', primary: '#164B4A', onPrimary: '#FFFFFF', secondary: '#F2D7B5', onSecondary: '#4A2A10', accent: '#E07B39', surface: '#F2F5F2', onSurface: '#182322', background: '#FFFFFF', text: '#182322', link: '#155553' },
  ],
  icons: {
    start: 'feather', syllabus: 'scroll', modules: 'book-copy', assignments: 'pen-tool', discussions: 'messages-square',
    grades: 'award', help: 'circle-help', instructor: 'lamp-desk', calendar: 'calendar-days', resources: 'library',
  },
  art: {
    medium: 'Relief woodcut print with bold carved lines, visible wood grain and two inks pressed on warm paper',
    palette: 'Deep indigo ink and warm ochre on cream paper with a small touch of brick red',
    mood: 'Timeless, crafted and thoughtful',
    banner: 'A rolling landscape of carved hills with a winding river and a large old oak tree on the right, plain cream paper sky on the left',
    bannerAlt: 'A large windmill among ploughed fields with carved furrows and a few birds on the right, plain cream paper sky on the left',
    welcome: { subject: 'A quill feather standing in an inkwell', alt: 'A quill in an inkwell' },
    instructor: { subject: 'An old oil lamp with a glowing flame', alt: 'An oil lamp' },
    cards: [
      'A broad oak tree on a small hill. The carved image runs off all four edges: no block outline, plate edge or paper margin',
      'A sailing ship on carved waves. The carved image runs off all four edges: no block outline, plate edge or paper margin',
      'A mountain range with a rising sun',
    ],
  },
});
