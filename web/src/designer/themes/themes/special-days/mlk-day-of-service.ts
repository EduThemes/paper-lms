import { defineTheme } from '../../define';

export default defineTheme({
  id: 'mlk-day-of-service',
  name: 'MLK Day of Service',
  description: 'Soft chalk pastel of a community garden, donation boxes and a newly planted tree. Service and community, quietly told, with no likeness.',
  audiences: ['upper-elementary', 'secondary', 'early-elementary'],
  tags: ['civic', 'mlk-day', 'service', 'community', 'chalk-pastel'],
  collection: 'special-days',
  observance: {
    name: 'Martin Luther King Jr. Day of Service',
    traditions: [],
    months: [1],
    note: 'Third Monday in January, a national day of service. Use real photographs and speeches (Library of Congress, The King Center) rather than AI images of Dr. King.',
  },
  defaultLayout: 'classic-band',
  iconStyle: 'solid-circle',
  corners: 'soft',
  style: 'soft chalk pastel on toned paper',
  colourBlind:
    'Plum and amber differ strongly in lightness; sage is scenery only, so nothing relies on telling red from green.',
  palettes: [
    { id: 'amber', name: 'Amber & Plum', primary: '#4A2048', onPrimary: '#FFFFFF', secondary: '#F6D89A', onSecondary: '#3F2A06', accent: '#8BA888', surface: '#FBF6EC', onSurface: '#241A22', background: '#FFFFFF', text: '#241A22', link: '#6A2E66' },
    { id: 'slate', name: 'Slate & Sage', primary: '#2A3A4E', onPrimary: '#FFFFFF', secondary: '#D8E6D4', onSecondary: '#22382A', accent: '#E0A458', surface: '#F2F5F7', onSurface: '#1A2028', background: '#FFFFFF', text: '#1A2028', link: '#2F4F72' },
    { id: 'plum', name: 'Deep Plum', primary: '#3A1F44', onPrimary: '#FFFFFF', secondary: '#E9D8EE', onSecondary: '#3A1F44', accent: '#E0A458', surface: '#F7F3F8', onSurface: '#221A26', background: '#FFFFFF', text: '#221A26', link: '#5A3368' },
  ],
  icons: {
    start: 'hand-helping', syllabus: 'clipboard-list', modules: 'layers', assignments: 'shovel', discussions: 'messages-square',
    grades: 'award', help: 'circle-help', instructor: 'heart-handshake', calendar: 'calendar-days', resources: 'package',
  },
  art: {
    medium: 'Warm soft chalk-pastel illustration on toned paper with blended colour and gentle grain',
    palette: 'Warm amber and deep plum with calm slate blue and soft sage',
    mood: 'Hopeful, quiet and community-minded',
    banner: 'A community garden with raised beds of young seedlings, a watering can and a freshly planted young tree tied to a stake on the right, calm toned paper sky on the left',
    bannerAlt: 'Plain cardboard donation boxes filled with unmarked tins, apples and bread beside a neat pile of folded warm coats on the right, calm plain toned paper on the left',
    welcome: { subject: 'A watering can pouring water onto a small seedling', alt: 'A watering can and a seedling' },
    instructor: { subject: 'A sturdy garden spade standing in fresh soil beside a young tree', alt: 'A spade beside a young tree' },
    cards: [
      'A neat stack of donated books with plain covers tied with string',
      'A bird feeder hanging from a winter branch with small birds',
      'A freshly painted park bench beside a tidy flowerbed',
    ],
  },
});
