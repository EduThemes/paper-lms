import { defineTheme } from '../define';

export default defineTheme({
  id: 'game-day',
  name: 'Game Day',
  description: 'Dynamic flat sports art with diagonal speed lines and screen-print grain. For athletics, PE and coaching.',
  audiences: ['specialty', 'secondary', 'upper-elementary'],
  tags: ['athletics', 'sports', 'pe', 'energetic'],
  defaultLayout: 'card-grid',
  iconStyle: 'solid-circle',
  corners: 'soft',
  style: 'screen-printed sports graphics',
  colourBlind:
    'Blue and gold is the safest school-colour pairing for colour-blind athletes; results and standings never use red versus green.',
  palettes: [
    { id: 'royal', name: 'Royal & Gold', primary: '#183A8A', onPrimary: '#FFFFFF', secondary: '#FFC93C', onSecondary: '#2A2000', accent: '#2EC4F1', surface: '#EFF3FB', onSurface: '#161C2A', background: '#FFFFFF', text: '#161C2A', link: '#183A8A' },
    { id: 'crimson', name: 'Crimson & Gold', primary: '#7A1E2C', onPrimary: '#FFFFFF', secondary: '#F2D16B', onSecondary: '#3A2A00', accent: '#9AA5B1', surface: '#F8F1F2', onSurface: '#261A1C', background: '#FFFFFF', text: '#261A1C', link: '#8A2232' },
    { id: 'forest', name: 'Forest & Silver', primary: '#1E4A33', onPrimary: '#FFFFFF', secondary: '#DDE3E8', onSecondary: '#1E2A33', accent: '#F2A93B', surface: '#F1F5F2', onSurface: '#18221C', background: '#FFFFFF', text: '#18221C', link: '#1E5A3E' },
  ],
  icons: {
    start: 'play', syllabus: 'clipboard-list', modules: 'layers', assignments: 'dumbbell', discussions: 'megaphone',
    grades: 'trophy', help: 'circle-help', instructor: 'medal', calendar: 'calendar-days', resources: 'timer',
  },
  art: {
    medium: 'Dynamic flat vector sports illustration with bold diagonal speed lines and a grainy screen-print texture',
    palette: 'Royal blue and bright gold with white and charcoal',
    mood: 'Energetic, team-spirited and upbeat',
    banner: 'A large basketball, a soccer ball and a stopwatch flying along curved running-track lanes on the right, clean open space with a few faint speed lines on the left',
    bannerAlt: 'Bright stadium floodlights and a gold starburst over the curve of an empty running track on the right, clean open space with a few faint speed lines on the left',
    welcome: { subject: 'A gold trophy cup with a star', alt: 'A trophy' },
    instructor: { subject: "A coach's whistle on a lanyard beside a stopwatch", alt: 'A whistle and a stopwatch' },
    cards: [
      'A basketball dropping through a hoop',
      'A soccer ball in mid-flight with speed lines',
      'A pair of plain running shoes on a track lane',
    ],
  },
});
