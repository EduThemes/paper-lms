import { defineTheme } from '../../define';

export default defineTheme({
  id: 'read-aloud-day',
  name: 'Book Day & Read-Aloud',
  description: 'Storybook pen-and-ink: a giant pop-up book, a blanket reading tent and a dragon asleep on a pile of books. For World Book Day and read-aloud events.',
  audiences: ['early-elementary', 'upper-elementary'],
  tags: ['awareness', 'world-book-day', 'read-aloud', 'literacy', 'library', 'pen-and-ink'],
  collection: 'special-days',
  observance: {
    name: 'World Book Day and Read-Aloud Day',
    traditions: [],
    months: [2, 3, 4],
    note: 'World Book Day: April 23 (UNESCO), or the first Thursday of March in the UK and Ireland. US Read Across America: early March. World Read Aloud Day: early February.',
  },
  defaultLayout: 'playful',
  iconStyle: 'solid-circle',
  corners: 'round',
  style: 'storybook pen-and-ink with watercolour',
  colourBlind:
    'Teal, plum and marigold differ in lightness as well as hue, so they stay distinct for red-green colour blindness.',
  palettes: [
    { id: 'teal', name: 'Storybook Teal', primary: '#164B55', onPrimary: '#FFFFFF', secondary: '#F7D488', onSecondary: '#3A2A00', accent: '#8E4C8A', surface: '#FBF7EE', onSurface: '#1A2426', background: '#FFFFFF', text: '#1A2426', link: '#17606D' },
    { id: 'plum', name: 'Plum Pages', primary: '#4E2350', onPrimary: '#FFFFFF', secondary: '#F1D9EF', onSecondary: '#3A1A3C', accent: '#E9A93B', surface: '#FAF4F9', onSurface: '#24182A', background: '#FFFFFF', text: '#24182A', link: '#6A2F6C' },
    { id: 'marigold', name: 'Marigold', primary: '#5A3410', onPrimary: '#FFFFFF', secondary: '#FFDF9E', onSecondary: '#3F2608', accent: '#3E8E9A', surface: '#FFF8EA', onSurface: '#2A1F14', background: '#FFFFFF', text: '#2A1F14', link: '#7A4A12' },
  ],
  icons: {
    start: 'book-open', syllabus: 'bookmark', modules: 'library', assignments: 'pencil', discussions: 'message-circle',
    grades: 'star', help: 'circle-help', instructor: 'book-heart', calendar: 'calendar-days', resources: 'book-copy',
  },
  art: {
    medium: 'Whimsical pen-and-ink illustration with loose watercolour washes and lively scratchy linework, like a classic storybook',
    palette: 'Storybook teal and plum with warm marigold on soft ivory',
    mood: 'Cosy, curious and full of wonder',
    banner: 'A giant open pop-up book with blank pages on the right, a little castle, curly trees and a hot-air balloon rising out of it, calm ivory sky on the left',
    bannerAlt: 'A cosy blanket reading tent strung with fairy lights, piled with cushions and closed books with plain blank covers on the right, calm ivory space on the left',
    welcome: { subject: 'An open book with blank pages and a little paper boat sailing out of it', alt: 'A paper boat sailing out of a book' },
    instructor: { subject: 'A comfy armchair beside a reading lamp with a closed book on the seat', alt: 'An armchair and a reading lamp' },
    cards: [
      'A small friendly dragon curled up asleep on a pile of closed books',
      'A paper boat sailing on a sea of open blank pages',
      'An owl perched on a stack of closed books with plain covers',
    ],
  },
});
