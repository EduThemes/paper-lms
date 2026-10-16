import { defineTheme } from '../../define';

export default defineTheme({
  id: 'field-day',
  name: 'Field Day',
  description: 'Sunny gouache playing fields with tug-of-war rope, sack-race sacks, cones and water balloons. Outdoor games and long summer shadows.',
  audiences: ['early-elementary', 'upper-elementary'],
  tags: ['school-event', 'field-day', 'sports-day', 'outdoors', 'gouache'],
  collection: 'special-days',
  observance: {
    name: 'Field Day',
    traditions: [],
    months: [5, 6],
    note: 'Late spring in the US; called Sports Day in the UK, Ireland and Australia.',
  },
  defaultLayout: 'playful',
  iconStyle: 'solid-circle',
  corners: 'round',
  style: 'sunny flat gouache',
  colourBlind:
    'Sky blue and sunshine yellow carry the structure; grass green and tomato red only ever appear as scenery, never as a pair that signals anything.',
  palettes: [
    { id: 'sunny', name: 'Sunny Field', primary: '#1D4E89', onPrimary: '#FFFFFF', secondary: '#FFD447', onSecondary: '#3A2C00', accent: '#E85D3F', surface: '#FFF8DC', onSurface: '#1D2430', background: '#FFFFFF', text: '#1D2430', link: '#1D4E89' },
    { id: 'grass', name: 'Fresh Grass', primary: '#1F5130', onPrimary: '#FFFFFF', secondary: '#CDEFB8', onSecondary: '#1A3A20', accent: '#FFB347', surface: '#F2FAEC', onSurface: '#17241A', background: '#FFFFFF', text: '#17241A', link: '#23633A' },
    { id: 'tomato', name: 'Tomato Relay', primary: '#8C2B14', onPrimary: '#FFFFFF', secondary: '#FFE08A', onSecondary: '#3A2A00', accent: '#3A86FF', surface: '#FFF5EA', onSurface: '#2A1B14', background: '#FFFFFF', text: '#2A1B14', link: '#9C3418' },
  ],
  icons: {
    start: 'footprints', syllabus: 'clipboard-list', modules: 'layers', assignments: 'target', discussions: 'message-circle',
    grades: 'medal', help: 'circle-help', instructor: 'volleyball', calendar: 'calendar-days', resources: 'sun',
  },
  art: {
    medium: 'Sunny flat gouache illustration with matte paint texture, simple rounded shapes and long soft summer shadows',
    palette: 'Sky blue and sunshine yellow with fresh grass green and a little tomato red',
    mood: 'Energetic, sunny and full of play',
    banner: 'A big coil of tug-of-war rope, bright orange cones, a striped relay baton and a bucket of water balloons on a sunny playing field on the right, a wide calm stretch of open grass and pale sky on the left',
    bannerAlt: 'A row of burlap sack-race sacks standing beside a stretched finish ribbon under a big round sun on the right, open field and soft sky on the left',
    welcome: { subject: 'A plain ribbon rosette with streaming tails and no markings', alt: 'A prize ribbon' },
    instructor: { subject: 'A shiny silver whistle on a lanyard beside a water bottle', alt: 'A whistle and a water bottle' },
    cards: [
      'A hula hoop and a bright ball resting on the grass',
      'An egg balanced on a spoon on a sunny lawn',
      'A big rainbow play parachute spread out on the grass',
    ],
  },
});
