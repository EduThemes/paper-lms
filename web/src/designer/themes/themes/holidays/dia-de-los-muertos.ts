import { defineTheme } from '../../define';

export default defineTheme({
  id: 'dia-de-los-muertos',
  name: 'Día de los Muertos',
  description: 'Bright Mexican folk-art painting of cempasúchil marigolds, candles, pan de muerto, papel picado and monarch butterflies. Joyful remembrance, never spooky.',
  audiences: ['upper-elementary', 'secondary', 'early-elementary'],
  tags: ['holiday', 'cultural', 'mexican', 'latino', 'remembrance', 'folk-art'],
  collection: 'holidays',
  observance: {
    name: 'Día de los Muertos (Day of the Dead)',
    traditions: ['Mexican', 'Mexican American', 'Central American communities'],
    months: [10, 11],
    note: 'November 1–2 (some families begin on October 31 or earlier). A joyful remembrance of loved ones, distinct from Halloween. Also called Día de Muertos.',
  },
  defaultLayout: 'card-grid',
  iconStyle: 'solid-circle',
  corners: 'round',
  style: 'Mexican folk-art painting',
  colourBlind:
    'Purple, magenta and teal bands sit against pale marigold or pink, so every pair differs strongly in lightness; there is no red-green pairing.',
  palettes: [
    { id: 'cempasuchil', name: 'Cempasúchil', primary: '#4B1D5C', onPrimary: '#FFFFFF', secondary: '#FFB443', onSecondary: '#2E1A00', accent: '#E0457B', surface: '#FFF4E6', onSurface: '#251A20', background: '#FFFFFF', text: '#251A20', link: '#6A2A82' },
    { id: 'rosa', name: 'Rosa Mexicano', primary: '#7A1650', onPrimary: '#FFFFFF', secondary: '#FFD6E6', onSecondary: '#4A0F2E', accent: '#1FB5A8', surface: '#FFF0F6', onSurface: '#2A1424', background: '#FFFFFF', text: '#2A1424', link: '#8E1A5E' },
    { id: 'turquesa', name: 'Turquesa', primary: '#0D4A57', onPrimary: '#FFFFFF', secondary: '#FFD08A', onSecondary: '#3A2200', accent: '#E0457B', surface: '#EAF7F7', onSurface: '#132326', background: '#FFFFFF', text: '#132326', link: '#0D5F6E' },
  ],
  icons: {
    start: 'flame', syllabus: 'scroll', modules: 'layers', assignments: 'pencil', discussions: 'messages-square',
    grades: 'star', help: 'circle-help', instructor: 'flower', calendar: 'calendar-days', resources: 'heart',
  },
  art: {
    medium: 'Bright Mexican folk-art painting with flat saturated colour, dotted patterns and fine linework, like painted wood',
    palette: 'Marigold orange and magenta pink with turquoise and deep purple on cream',
    mood: 'Joyful, loving and vibrant',
    banner: 'Mounds of orange marigold flowers, tall lit candles and a round loaf of pan de muerto on a festive cloth, under strings of papel picado cut only in abstract flower patterns, plain and unmarked, on the right; a calm cream background on the left',
    bannerAlt: 'Orange monarch butterflies drifting over a field of marigolds at golden dusk on the right, an open calm violet sky on the left',
    welcome: { subject: 'A cheerful folk-art sugar skull decorated with flowers and dots, with a plain, unmarked brow', alt: 'A decorated sugar skull' },
    instructor: { subject: 'A round loaf of pan de muerto with a sugar topping beside a lit candle and marigolds', alt: 'Pan de muerto with a candle and marigolds' },
    cards: [
      'A cluster of orange marigolds in full bloom',
      'A string of papel picado cut only in abstract flower patterns, plain and unmarked',
      'A monarch butterfly resting on a marigold',
    ],
  },
});
