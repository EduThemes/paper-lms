import { defineTheme } from '../../define';

export default defineTheme({
  id: 'rosh-hashanah',
  name: 'Rosh Hashanah',
  description: 'Loose botanical watercolour of apples and honey, round raisin challah, pomegranates and a ram’s-horn shofar. A sweet, hopeful Jewish New Year.',
  audiences: ['upper-elementary', 'early-elementary', 'secondary'],
  tags: ['holiday', 'religious', 'jewish', 'new-year', 'autumn'],
  collection: 'holidays',
  observance: {
    name: 'Rosh Hashanah',
    traditions: ['Jewish'],
    months: [9, 10],
    note: 'The Jewish New Year, 1–2 Tishrei, begins at sundown in September or early October. Yom Kippur follows ten days later as a solemn fast, so this theme is for the New Year only.',
  },
  defaultLayout: 'classic-band',
  iconStyle: 'soft-square',
  corners: 'soft',
  style: 'botanical watercolour and ink',
  colourBlind:
    'Honey gold and pomegranate red differ strongly in lightness; the leaf green in the art is decoration only and is never set against red to carry meaning.',
  palettes: [
    { id: 'honey', name: 'Apples & Honey', primary: '#5A2A12', onPrimary: '#FFFFFF', secondary: '#F6C453', onSecondary: '#2E1D00', accent: '#B3263A', surface: '#FFF6E0', onSurface: '#2A1D12', background: '#FFFFFF', text: '#2A1D12', link: '#8A3A10' },
    { id: 'pomegranate', name: 'Pomegranate', primary: '#7A1F2B', onPrimary: '#FFFFFF', secondary: '#F7D3D3', onSecondary: '#4A1018', accent: '#E0A526', surface: '#FDF0EF', onSurface: '#2A1517', background: '#FFFFFF', text: '#2A1517', link: '#8E2433' },
    { id: 'plum', name: 'Autumn Plum', primary: '#3E2148', onPrimary: '#FFFFFF', secondary: '#EBD9F0', onSecondary: '#2A1530', accent: '#E3A93B', surface: '#F7F0F9', onSurface: '#221828', background: '#FFFFFF', text: '#221828', link: '#5E2F75' },
  ],
  icons: {
    start: 'sunrise', syllabus: 'scroll', modules: 'layers', assignments: 'pencil', discussions: 'messages-square',
    grades: 'star', help: 'circle-help', instructor: 'apple', calendar: 'calendar-days', resources: 'book-open',
  },
  art: {
    medium: 'Loose botanical watercolour with fine brown ink lines and soft blooms of colour on white paper',
    palette: 'Honey gold, pomegranate red and deep plum on warm cream',
    mood: 'Sweet, hopeful and gentle',
    banner: 'A plain glass bowl of golden honey with a wooden dipper, sliced red apples and two whole pomegranates on the right, a soft wash of warm cream paper on the left',
    bannerAlt: 'A round braided challah studded with raisins beside a plain curved ram’s horn shofar with no markings and one pomegranate on the right, an open pale cream wash on the left',
    welcome: { subject: 'A red apple beside a small plain, unmarked honey jar with a wooden dipper', alt: 'An apple and a jar of honey' },
    instructor: { subject: 'A plain curved ram’s horn shofar with no markings resting on a folded cloth', alt: 'A ram’s horn shofar' },
    cards: [
      'An apple branch heavy with ripe red apples',
      'A halved pomegranate showing its jewel-red seeds',
      'Honeybees around a golden honeycomb in soft sunlight',
    ],
  },
});
