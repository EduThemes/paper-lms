import { defineTheme } from '../../define';

export default defineTheme({
  id: 'kart-rally',
  name: 'Kart Rally',
  description: 'Glossy plastic toy go-karts zipping round a looping tabletop track. Chunky, bright and picture-first for young racers.',
  audiences: ['early-elementary', 'upper-elementary'],
  tags: ['toys', '3d', 'racing', 'cars', 'picture-first'],
  defaultLayout: 'playful',
  iconStyle: 'solid-circle',
  corners: 'round',
  collection: 'arcade',
  style: 'glossy 3D toy render',
  colourBlind:
    'Cherry red and grass green appear together only in the toy art; the interface uses deep blue, red or green bands with light lemon or cream, so meaning never rests on red versus green.',
  palettes: [
    { id: 'raceway', name: 'Raceway', primary: '#17346E', onPrimary: '#FFFFFF', secondary: '#FFD447', onSecondary: '#2B2300', accent: '#E63946', surface: '#EEF3FD', onSurface: '#141C30', background: '#FFFFFF', text: '#141C30', link: '#1E4A9A' },
    { id: 'cherry', name: 'Cherry Kart', primary: '#7A1A22', onPrimary: '#FFFFFF', secondary: '#FFD9A0', onSecondary: '#3A2000', accent: '#3A86FF', surface: '#FFF1EE', onSurface: '#2A1214', background: '#FFFFFF', text: '#2A1214', link: '#A01E2A' },
    { id: 'lime-lap', name: 'Lime Lap', primary: '#1E4620', onPrimary: '#FFFFFF', secondary: '#D8F5A2', onSecondary: '#213A0A', accent: '#FFB703', surface: '#F1FAE8', onSurface: '#182418', background: '#FFFFFF', text: '#182418', link: '#2A6A2A' },
  ],
  icons: {
    start: 'car-front', syllabus: 'route', modules: 'milestone', assignments: 'pencil', discussions: 'message-circle',
    grades: 'trophy', help: 'circle-help', instructor: 'gauge', calendar: 'calendar-days', resources: 'backpack',
  },
  art: {
    medium: 'Glossy 3D toy render of chunky rounded plastic toys with soft studio lighting and gentle reflections, like a tabletop playset',
    palette: 'Cherry red, lemon yellow and bright blue with grass green on soft cream',
    mood: 'Zippy, joyful and safe',
    banner: 'Two chunky plain unmarked toy go-karts racing round a looping toy track with a little arched bridge over rounded green hills on the right, a smooth soft cream tabletop and pale sky on the left',
    bannerAlt: 'A big chunky blue toy kart sailing over a small ramp with a puff of round toy smoke above rounded toy hills on the right, soft open pale sky on the left',
    welcome: { subject: 'A chunky plain red toy go-kart with no markings', alt: 'A red toy go-kart' },
    instructor: { subject: 'An empty shiny toy racing helmet, nobody wearing it, beside a small gold trophy cup', alt: 'A toy racing helmet and a trophy cup' },
    cards: [
      'A toy traffic light beside a curving toy road',
      'A toy kart driving into a tunnel through a rounded green hill',
      'A row of toy traffic cones on a racetrack curve',
    ],
  },
});
