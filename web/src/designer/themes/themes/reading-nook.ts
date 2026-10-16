import { defineTheme } from '../define';

export default defineTheme({
  id: 'reading-nook',
  name: 'Reading Nook',
  description: 'Cosy lamplit gouache: tall shelves, window seats and armchairs. For the library media centre and reading programmes.',
  audiences: ['specialty', 'upper-elementary', 'secondary'],
  tags: ['library', 'reading', 'cosy', 'gouache'],
  defaultLayout: 'card-grid',
  iconStyle: 'duotone',
  corners: 'soft',
  style: 'lamplit gouache interiors',
  colourBlind:
    'Deep teal against mustard is a blue-yellow contrast; the warm red accent never marks anything on its own.',
  palettes: [
    { id: 'lamplight', name: 'Lamplight', primary: '#1D4D4F', onPrimary: '#FFFFFF', secondary: '#F3D27A', onSecondary: '#3A2A00', accent: '#B5543C', surface: '#F7F1E4', onSurface: '#24211C', background: '#FFFFFF', text: '#24211C', link: '#1D5355' },
    { id: 'walnut', name: 'Walnut', primary: '#4A2E1F', onPrimary: '#FFFFFF', secondary: '#E4EDE6', onSecondary: '#1E3A30', accent: '#E5A93B', surface: '#FAF4EC', onSurface: '#2A1F18', background: '#FFFFFF', text: '#2A1F18', link: '#6A3A1E' },
    { id: 'midnight', name: 'Midnight Stacks', primary: '#1F2547', onPrimary: '#FFFFFF', secondary: '#F6D9C4', onSecondary: '#4A2414', accent: '#F0B347', surface: '#F2F2F8', onSurface: '#1A1C2B', background: '#FFFFFF', text: '#1A1C2B', link: '#2E3A80' },
  ],
  icons: {
    start: 'door-open', syllabus: 'scroll-text', modules: 'library', assignments: 'book-open-check', discussions: 'messages-square',
    grades: 'bookmark-check', help: 'circle-help', instructor: 'lamp', calendar: 'calendar-days', resources: 'book-marked',
  },
  art: {
    medium: 'Cosy gouache illustration with warm lamplight, soft textured brushwork and rich gentle shadows',
    palette: 'Deep teal and mustard with warm walnut brown and a touch of brick red',
    mood: 'Warm, hushed and inviting, the best seat in the library',
    banner: 'Tall wooden shelves of closed books with plain blank spines, a large reading armchair and a glowing floor lamp on the right, a plain warm wall in soft shadow on the left',
    bannerAlt: 'A cushioned window seat with a sleeping cat and a stack of closed books with plain blank covers, rain on the window panes, on the right, a plain warm wall in soft shadow on the left',
    welcome: { subject: 'A stack of closed books with plain blank covers topped by a cup of cocoa', alt: 'A stack of books with a cup of cocoa' },
    instructor: { subject: 'A glowing brass reading lamp', alt: 'A reading lamp' },
    cards: [
      'An open book with blank pages and a ribbon bookmark',
      'A reading armchair beside a glowing lamp',
      'A potted plant on a shelf of books with plain blank spines',
    ],
  },
});
