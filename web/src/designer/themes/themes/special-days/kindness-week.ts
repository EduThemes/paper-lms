import { defineTheme } from '../../define';

export default defineTheme({
  id: 'kindness-week',
  name: 'Kindness Week',
  description: 'Paper quilling hearts, flowers and envelopes sealed with little hearts. A warm, inclusive alternative to Valentine’s Day.',
  audiences: ['early-elementary', 'upper-elementary'],
  tags: ['awareness', 'kindness', 'sel', 'valentines-alternative', 'paper-quilling'],
  collection: 'special-days',
  observance: {
    name: 'Kindness Week',
    traditions: [],
    months: [2, 11],
    note: 'Random Acts of Kindness Week falls in mid-February, a gentle alternative to Valentine’s Day. World Kindness Day is November 13.',
  },
  defaultLayout: 'playful',
  iconStyle: 'solid-circle',
  corners: 'round',
  style: 'paper quilling',
  colourBlind:
    'Rose, coral and lavender are separated by lightness, and yellow carries contrast; nothing relies on telling red from green.',
  palettes: [
    { id: 'rose', name: 'Rose Quill', primary: '#7A1E48', onPrimary: '#FFFFFF', secondary: '#FFD6E2', onSecondary: '#5A1234', accent: '#FFC857', surface: '#FFF3F7', onSurface: '#2B1822', background: '#FFFFFF', text: '#2B1822', link: '#8E2455' },
    { id: 'coral', name: 'Coral Kind', primary: '#7E2E1F', onPrimary: '#FFFFFF', secondary: '#FFDCCB', onSecondary: '#5A1E12', accent: '#9B8CE0', surface: '#FFF5EF', onSurface: '#2A1B16', background: '#FFFFFF', text: '#2A1B16', link: '#93361F' },
    { id: 'lavender', name: 'Lavender Note', primary: '#463070', onPrimary: '#FFFFFF', secondary: '#E6DDFA', onSecondary: '#34245A', accent: '#FFB4A2', surface: '#F7F4FD', onSurface: '#211B30', background: '#FFFFFF', text: '#211B30', link: '#573C8C' },
  ],
  icons: {
    start: 'heart', syllabus: 'clipboard-list', modules: 'layers', assignments: 'pencil', discussions: 'message-circle-heart',
    grades: 'star', help: 'hand-helping', instructor: 'heart-handshake', calendar: 'calendar-days', resources: 'gift',
  },
  art: {
    medium: 'Cheerful paper-quilling illustration made of rolled and coiled strips of coloured paper with crisp curled edges and soft shadows',
    palette: 'Rose pink and warm coral with sunny yellow and soft lavender on cream',
    mood: 'Warm, gentle and generous',
    banner: 'A big quilled heart surrounded by curling flowers and little hearts on the right, calm plain cream on the left',
    bannerAlt: 'A plain mailbox overflowing with envelopes sealed with little hearts, quilled flowers curling around it on the right, calm plain cream on the left',
    welcome: { subject: 'A quilled heart with a little flower inside', alt: 'A paper heart with a flower' },
    instructor: { subject: 'A potted flower with a ribbon bow tied around its pot', alt: 'A potted flower with a bow' },
    cards: [
      'Two little birds sharing a branch',
      'An umbrella sheltering a small flower from the rain',
      'A gift box with a heart-shaped bow',
    ],
  },
});
