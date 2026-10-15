import { defineTheme } from '../../define';

export default defineTheme({
  id: 'dino-dig',
  name: 'Dino Dig',
  description: 'Friendly dinosaurs, puffing volcanoes and fossils in layered rock, in grainy mid-century picture-book print.',
  audiences: ['early-elementary', 'upper-elementary'],
  tags: ['dinosaurs', 'fossils', 'science', 'mid-century', 'prehistoric'],
  defaultLayout: 'playful',
  iconStyle: 'solid-circle',
  corners: 'round',
  collection: 'cartoons',
  style: 'mid-century print cartoon',
  colourBlind:
    'Deep fern green is paired with sandy ochre, a strong lightness contrast; volcanic orange is decorative and never placed against the green to mean something.',
  palettes: [
    { id: 'fern', name: 'Fern Valley', primary: '#24503A', onPrimary: '#FFFFFF', secondary: '#F7C873', onSecondary: '#3D2600', accent: '#E8743B', surface: '#F5F1E4', onSurface: '#1F241C', background: '#FFFFFF', text: '#1F241C', link: '#24503A' },
    { id: 'volcano', name: 'Volcano', primary: '#6B2A12', onPrimary: '#FFFFFF', secondary: '#FFD08A', onSecondary: '#3D2200', accent: '#4CB8A8', surface: '#FFF3E8', onSurface: '#2A1810', background: '#FFFFFF', text: '#2A1810', link: '#8A3416' },
    { id: 'fossil', name: 'Fossil Bed', primary: '#3D3A52', onPrimary: '#FFFFFF', secondary: '#E8DCC2', onSecondary: '#3A2E18', accent: '#F08A4B', surface: '#F5F3EE', onSurface: '#1F1E28', background: '#FFFFFF', text: '#1F1E28', link: '#3D3A72' },
  ],
  icons: {
    start: 'footprints', syllabus: 'list', modules: 'mountain', assignments: 'shovel', discussions: 'message-circle',
    grades: 'star', help: 'circle-help', instructor: 'search', calendar: 'calendar', resources: 'bone',
  },
  art: {
    medium:
      'Mid-century cartoon illustration with flat grainy colour, offset-print texture, simple geometric shapes and loose brush outlines',
    palette: 'Fern green and volcanic orange with sandy ochre and soft sky teal',
    mood: 'Friendly, curious and sunny, a gentle prehistoric world',
    banner:
      'A big friendly long-necked dinosaur and a small triceratops among tall ferns beside a volcano puffing a little round cloud on the right, calm open pale sky over low sandy hills on the left',
    bannerAlt:
      'A cross-section of layered rock with a curled fossil skeleton of a small dinosaur, spiral ammonite shells and a little shovel and brush on the right, plain calm sandy strata continuing across the left',
    welcome: { subject: 'A happy baby dinosaur hatching from a speckled egg', alt: 'A baby dinosaur hatching from an egg' },
    instructor: { subject: 'An empty wide-brimmed explorer hat beside a trowel, a soft brush and a magnifying glass on a rock, nobody wearing the hat', alt: 'An explorer hat and digging tools' },
    cards: [
      'A speckled dinosaur egg in a nest of ferns',
      'A spiral ammonite fossil in layered rock',
      'A stegosaurus strolling past a puffing volcano',
    ],
  },
});
