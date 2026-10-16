import { defineTheme } from '../../define';

export default defineTheme({
  id: 'holi',
  name: 'Holi',
  description: 'Exuberant powdered pigment and watercolour: clouds of colour over brass bowls of gulal, marigolds, a pichkari and gujiya sweets. The festival of colours, no people.',
  audiences: ['upper-elementary', 'early-elementary', 'secondary'],
  tags: ['holiday', 'religious', 'hindu', 'festival-of-colours', 'spring'],
  collection: 'holidays',
  observance: {
    name: 'Holi',
    traditions: ['Hindu', 'South Asian communities'],
    months: [2, 3],
    note: 'On the full moon of Phalguna (late February or March), with the Holika Dahan bonfire the evening before. Celebrated widely across India, Nepal and the diaspora.',
  },
  defaultLayout: 'playful',
  iconStyle: 'solid-circle',
  corners: 'round',
  style: 'powdered pigment and watercolour',
  colourBlind:
    'Structure uses deep magenta, teal or violet against pale yellow or pink, so every pair differs strongly in lightness; the many festival hues in the art are decoration only.',
  palettes: [
    { id: 'magenta', name: 'Gulal Magenta', primary: '#6E1453', onPrimary: '#FFFFFF', secondary: '#FFD34E', onSecondary: '#3A2A00', accent: '#22B3C9', surface: '#FFF0F8', onSurface: '#2A1424', background: '#FFFFFF', text: '#2A1424', link: '#8A1A68' },
    { id: 'turquoise', name: 'Turquoise Splash', primary: '#0D4B5C', onPrimary: '#FFFFFF', secondary: '#FFC2E0', onSecondary: '#4A0F2E', accent: '#FFB000', surface: '#ECF8FA', onSurface: '#122126', background: '#FFFFFF', text: '#122126', link: '#0D5E73' },
    { id: 'violet', name: 'Violet & Marigold', primary: '#3D1E6D', onPrimary: '#FFFFFF', secondary: '#FFE08A', onSecondary: '#332800', accent: '#F25C9A', surface: '#F5F0FC', onSurface: '#1E1730', background: '#FFFFFF', text: '#1E1730', link: '#4E2A90' },
  ],
  icons: {
    start: 'sparkles', syllabus: 'scroll', modules: 'blocks', assignments: 'paintbrush', discussions: 'message-circle',
    grades: 'star', help: 'circle-help', instructor: 'palette', calendar: 'calendar-days', resources: 'droplets',
  },
  art: {
    medium: 'Vibrant mixed-media painting of powdered pigment clouds and wet watercolour blooms with fine speckled grain',
    palette: 'Hot magenta, marigold yellow, turquoise and violet on clean white',
    mood: 'Joyful, playful and bursting with colour',
    banner: 'Billowing clouds of bright coloured powder bursting above four small brass bowls heaped with pink, yellow, blue and green powder on the right, a calm white ground with faint pastel dust on the left',
    bannerAlt: 'Marigold garlands, a brass water sprayer and a plate of crescent-shaped gujiya sweets among drifting clouds of colour on the right, open soft white space on the left',
    welcome: { subject: 'Four small brass bowls of coloured powder in pink, yellow, blue and green', alt: 'Bowls of coloured powder' },
    instructor: { subject: 'A brass pichkari water sprayer resting on marigold petals', alt: 'A brass water sprayer on marigold petals' },
    cards: [
      'A soft burst of magenta and yellow powder in the air',
      'A plate of crescent-shaped gujiya sweets beside marigolds',
      'Spring flowers dusted with bright coloured powder',
    ],
  },
});
