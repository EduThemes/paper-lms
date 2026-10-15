import { defineTheme } from '../../define';

export default defineTheme({
  id: 'pi-day',
  name: 'Pi Day',
  description: 'Mid-century geometry: a lattice fruit pie, a compass tracing circles and ripples on a pond. Circles everywhere, digits nowhere.',
  audiences: ['upper-elementary', 'secondary'],
  tags: ['awareness', 'pi-day', 'math', 'geometry', 'mid-century'],
  collection: 'special-days',
  observance: {
    name: 'Pi Day',
    traditions: [],
    months: [3],
    note: 'March 14 (3/14). The pi symbol isn’t in the art, so put it in your title.',
  },
  defaultLayout: 'card-grid',
  iconStyle: 'solid-circle',
  corners: 'round',
  style: 'mid-century modern geometric',
  colourBlind:
    'Cherry and teal differ clearly in lightness, and mustard sits between them; no meaning depends on telling red from green.',
  palettes: [
    { id: 'cherry', name: 'Cherry Pie', primary: '#7A1F2B', onPrimary: '#FFFFFF', secondary: '#F6D57A', onSecondary: '#3A2A00', accent: '#2A9D8F', surface: '#FFF6E6', onSurface: '#2A1C1E', background: '#FFFFFF', text: '#2A1C1E', link: '#8E2433' },
    { id: 'teal', name: 'Mid-Century Teal', primary: '#164E52', onPrimary: '#FFFFFF', secondary: '#F2C14E', onSecondary: '#2E2000', accent: '#E76F51', surface: '#F0F7F5', onSurface: '#15241F', background: '#FFFFFF', text: '#15241F', link: '#17595E' },
    { id: 'mustard', name: 'Mustard & Ink', primary: '#2B2D42', onPrimary: '#FFFFFF', secondary: '#F4D35E', onSecondary: '#2B2D42', accent: '#EE964B', surface: '#FAF6E8', onSurface: '#1E1F2E', background: '#FFFFFF', text: '#1E1F2E', link: '#3A3D78' },
  ],
  icons: {
    start: 'circle', syllabus: 'list-checks', modules: 'chart-pie', assignments: 'compass', discussions: 'messages-square',
    grades: 'star', help: 'circle-help', instructor: 'cake-slice', calendar: 'calendar-days', resources: 'shapes',
  },
  art: {
    medium: 'Mid-century modern flat illustration with crisp geometric shapes, subtle grain and slightly offset print colour',
    palette: 'Warm cherry red, mustard yellow and deep teal on a creamy ground',
    mood: 'Clever, playful and tidy',
    banner: 'A large round fruit pie with a woven lattice crust and one slice lifted out, a drafting compass tracing perfect circles around it on the right, calm plain cream on the left',
    bannerAlt: 'Large concentric circles, a spoked wheel and a round ringed planet arranged in a playful orbit on the right, calm plain cream on the left',
    welcome: { subject: 'A round cherry pie with a lattice crust', alt: 'A cherry pie' },
    instructor: { subject: 'A drafting compass standing beside a round wooden wheel', alt: 'A drawing compass and a wheel' },
    cards: [
      'Ripples spreading in perfect circles on a still pond',
      'A sliced round orange showing its segments',
      'A bicycle wheel with fine spokes',
    ],
  },
});
