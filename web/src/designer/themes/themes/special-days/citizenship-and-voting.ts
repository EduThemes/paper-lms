import { defineTheme } from '../../define';

export default defineTheme({
  id: 'citizenship-and-voting',
  name: 'Citizenship & Voting',
  description: 'Fine engraving-style civic architecture, an unmarked ballot box, balance scales and a quill. Nonpartisan navy and gold for Constitution Day and elections.',
  audiences: ['upper-elementary', 'secondary', 'higher-ed'],
  tags: ['civic', 'constitution-day', 'election-day', 'government', 'nonpartisan', 'engraving'],
  collection: 'special-days',
  observance: {
    name: 'Constitution and Citizenship Day, Election Day',
    traditions: [],
    months: [9, 11],
    note: 'US Constitution and Citizenship Day: September 17. Election Day: the Tuesday after the first Monday in November. Navy and gold keep it nonpartisan.',
  },
  defaultLayout: 'sidebar',
  iconStyle: 'outline',
  corners: 'square',
  style: 'fine-line engraving',
  colourBlind:
    'Navy and gold are a blue-yellow pair with strong lightness contrast; red is left out on purpose, which also keeps the theme nonpartisan.',
  palettes: [
    { id: 'navy', name: 'Navy & Gold', primary: '#1A2A4A', onPrimary: '#FFFFFF', secondary: '#EAD69A', onSecondary: '#3A2E08', accent: '#B8963E', surface: '#F7F4EA', onSurface: '#181E2A', background: '#FFFFFF', text: '#181E2A', link: '#24407A' },
    { id: 'slate', name: 'Slate & Ivory', primary: '#2E3A44', onPrimary: '#FFFFFF', secondary: '#DDE3E8', onSecondary: '#2E3A44', accent: '#A88B4A', surface: '#F5F5F2', onSurface: '#1C2228', background: '#FFFFFF', text: '#1C2228', link: '#34536E' },
    { id: 'evergreen', name: 'Evergreen', primary: '#1F3A30', onPrimary: '#FFFFFF', secondary: '#E8DDB8', onSecondary: '#3A3010', accent: '#B8963E', surface: '#F3F6F2', onSurface: '#18221E', background: '#FFFFFF', text: '#18221E', link: '#255844' },
  ],
  icons: {
    start: 'landmark', syllabus: 'scroll', modules: 'layers', assignments: 'vote', discussions: 'messages-square',
    grades: 'scale', help: 'circle-help', instructor: 'feather', calendar: 'calendar-days', resources: 'library',
  },
  art: {
    medium: 'Fine-line engraving-style illustration with crisp cross-hatching printed in two inks on warm paper; the image runs off every edge with no border, frame or margin',
    palette: 'Deep navy and antique gold on warm ivory',
    mood: 'Steady, fair and civic-minded',
    banner: 'A grand columned civic building with a shallow dome and wide stone steps under a clear sky on the right, calm plain ivory sky on the left',
    bannerAlt: 'A plain wooden ballot box with an empty slot and no markings beside a quill and a rolled parchment scroll tied with ribbon on the right, calm plain ivory on the left',
    welcome: { subject: 'A plain wooden ballot box with an empty slot and no markings', alt: 'A ballot box' },
    instructor: { subject: 'A quill resting in an inkwell beside a rolled scroll tied with ribbon', alt: 'A quill and a rolled scroll' },
    cards: [
      'A pair of brass balance scales',
      'A row of tall stone columns in soft light',
      'A large old bell hanging from a heavy wooden beam',
    ],
  },
});
