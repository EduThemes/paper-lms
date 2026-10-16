import { defineTheme } from '../../define';

export default defineTheme({
  id: 'science-fair',
  name: 'Science Fair',
  description: 'Soft 3D renders of a bubbling model volcano, glowing flasks and a lemon battery. Curious and hands-on for STEM nights and science fairs.',
  audiences: ['upper-elementary', 'secondary'],
  tags: ['school-event', 'science-fair', 'stem', 'science', '3d-render'],
  collection: 'special-days',
  observance: {
    name: 'Science Fair',
    traditions: [],
    months: [1, 2, 3],
    note: 'Most school and regional fairs run January to March. Also works for STEM night.',
  },
  defaultLayout: 'card-grid',
  iconStyle: 'duotone',
  corners: 'round',
  style: 'soft matte 3D render',
  colourBlind:
    'Teal and lemon yellow differ strongly in lightness and hue; experiment results are never coded as red versus green.',
  palettes: [
    { id: 'teal', name: 'Lab Teal', primary: '#0B4F5C', onPrimary: '#FFFFFF', secondary: '#FFE45C', onSecondary: '#3A3000', accent: '#9B7BFF', surface: '#EEFAF7', onSurface: '#12262A', background: '#FFFFFF', text: '#12262A', link: '#0B5D6C' },
    { id: 'violet', name: 'Ultraviolet', primary: '#3E2A7A', onPrimary: '#FFFFFF', secondary: '#E2D9FF', onSecondary: '#2E1F5E', accent: '#3DDC97', surface: '#F5F2FF', onSurface: '#1F1A33', background: '#FFFFFF', text: '#1F1A33', link: '#4F36A0' },
    { id: 'lemon', name: 'Lemon Battery', primary: '#3A4A12', onPrimary: '#FFFFFF', secondary: '#F4F19C', onSecondary: '#3A3A08', accent: '#3BA6D9', surface: '#FBFBE8', onSurface: '#22261A', background: '#FFFFFF', text: '#22261A', link: '#4A5E14' },
  ],
  icons: {
    start: 'rocket', syllabus: 'clipboard-list', modules: 'atom', assignments: 'flask-conical', discussions: 'messages-square',
    grades: 'award', help: 'circle-help', instructor: 'microscope', calendar: 'calendar-days', resources: 'lightbulb',
  },
  art: {
    medium: 'Soft 3D render with matte clay-like plastic materials, rounded edges and soft studio lighting',
    palette: 'Bright teal and lemon yellow with soft violet on a pale mint ground',
    mood: 'Curious, inventive and fun',
    banner: 'A bubbling model volcano, round glass flasks of glowing coloured liquid and a big magnifying glass on a tabletop on the right, a wide calm pale mint backdrop on the left',
    bannerAlt: 'A potted bean plant growing beside a small solar panel, a horseshoe magnet and a glowing light bulb wired to a lemon on the right, calm open pale backdrop on the left',
    welcome: { subject: 'A round-bottomed glass flask full of fizzy bubbles', alt: 'A bubbling glass flask' },
    instructor: { subject: 'A big magnifying glass resting on a plain prize ribbon rosette', alt: 'A magnifying glass and a prize ribbon' },
    cards: [
      'A model of the planets on thin curved rods',
      'A crystal geode split open to show sparkling purple crystals',
      'A small robot arm built from cardboard and bottle caps',
    ],
  },
});
