import { defineTheme } from '../../define';

export default defineTheme({
  id: 'teacher-appreciation',
  name: 'Teacher Appreciation',
  description: 'Stitched embroidery on linen: wildflowers in a pencil cup, a shiny apple and a steaming mug. A warm thank-you for staff and families.',
  audiences: ['upper-elementary', 'early-elementary', 'secondary', 'professional'],
  tags: ['school-event', 'teacher-appreciation', 'staff', 'gratitude', 'embroidery'],
  collection: 'special-days',
  observance: {
    name: 'Teacher Appreciation Week',
    traditions: [],
    months: [5, 10],
    note: 'US: first full week of May. World Teachers’ Day is October 5.',
  },
  defaultLayout: 'card-grid',
  iconStyle: 'solid-circle',
  corners: 'soft',
  style: 'embroidery on linen',
  colourBlind:
    'Rose and sage threads are decorative; structure comes from the deep primary against light linen, so nothing depends on telling red from green.',
  palettes: [
    { id: 'rose', name: 'Rose & Linen', primary: '#6E2639', onPrimary: '#FFFFFF', secondary: '#F6D6DC', onSecondary: '#4A1726', accent: '#D9A441', surface: '#FBF5EE', onSurface: '#2A1C20', background: '#FFFFFF', text: '#2A1C20', link: '#8A2E46' },
    { id: 'sage', name: 'Sage Thread', primary: '#2E4A3A', onPrimary: '#FFFFFF', secondary: '#DCE8D8', onSecondary: '#233A2C', accent: '#E6A57E', surface: '#F4F7F1', onSurface: '#1C2620', background: '#FFFFFF', text: '#1C2620', link: '#2F6045' },
    { id: 'honey', name: 'Honey Gold', primary: '#5A3E10', onPrimary: '#FFFFFF', secondary: '#F7E2A8', onSecondary: '#44300A', accent: '#C76B6B', surface: '#FDF8EC', onSurface: '#2A2216', background: '#FFFFFF', text: '#2A2216', link: '#7A5410' },
  ],
  icons: {
    start: 'heart', syllabus: 'clipboard-list', modules: 'layers', assignments: 'notebook-pen', discussions: 'message-circle-heart',
    grades: 'award', help: 'hand-heart', instructor: 'apple', calendar: 'calendar-days', resources: 'gift',
  },
  art: {
    medium: 'Embroidery on natural linen with satin stitch, French knots and visible thread texture; the stitching fills the whole linen with no hoop, frame or edge',
    palette: 'Warm rose, honey gold and sage green threads on oatmeal linen',
    mood: 'Grateful, warm and heartfelt',
    banner: 'A big bouquet of wildflowers spilling from a pencil cup beside a shiny apple and a steaming mug on the right, plain calm linen on the left',
    bannerAlt: 'A large stitched heart wreathed in leaves and small flowers on the right, plain calm linen on the left',
    welcome: { subject: 'A stitched red apple with a single leaf', alt: 'An embroidered apple' },
    instructor: { subject: 'A steaming mug of coffee beside a small vase of flowers', alt: 'A mug of coffee and a vase of flowers' },
    cards: [
      'A small potted plant with heart-shaped leaves',
      'A stack of closed books with plain blank covers tied with a bow',
      'A gift box wrapped with a ribbon bow',
    ],
  },
});
