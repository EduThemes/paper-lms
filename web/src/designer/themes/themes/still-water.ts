import { defineTheme } from '../define';

export default defineTheme({
  id: 'still-water',
  name: 'Still Water',
  description: 'Soft pastel watercolour lakes, balanced pebbles and slow sunrises. A calming look for counselling and SEL.',
  audiences: ['specialty', 'upper-elementary', 'secondary'],
  tags: ['counseling', 'sel', 'calm', 'watercolour'],
  defaultLayout: 'classic-band',
  iconStyle: 'solid-circle',
  corners: 'round',
  style: 'pastel watercolour calm',
  colourBlind:
    'Lavender and seafoam differ in lightness, and feelings or check-in states are never shown as red versus green.',
  palettes: [
    { id: 'lavender', name: 'Lavender Lake', primary: '#3E3266', onPrimary: '#FFFFFF', secondary: '#DDF0EA', onSecondary: '#1E4A40', accent: '#F7C59F', surface: '#F5F2FB', onSurface: '#252233', background: '#FFFFFF', text: '#252233', link: '#4A3C80' },
    { id: 'sunrise', name: 'Slow Sunrise', primary: '#5E3A2A', onPrimary: '#FFFFFF', secondary: '#FCE3CF', onSecondary: '#5E3A2A', accent: '#A8D5C9', surface: '#FFF6EF', onSurface: '#2A1E18', background: '#FFFFFF', text: '#2A1E18', link: '#7A4430' },
    { id: 'sky', name: 'Open Sky', primary: '#1F4466', onPrimary: '#FFFFFF', secondary: '#E2EEFA', onSecondary: '#1F4466', accent: '#F5D0A9', surface: '#F2F7FB', onSurface: '#18222C', background: '#FFFFFF', text: '#18222C', link: '#1F5482' },
  ],
  icons: {
    start: 'heart', syllabus: 'clipboard-list', modules: 'layers', assignments: 'notebook-pen', discussions: 'message-circle-heart',
    grades: 'sun', help: 'hand-heart', instructor: 'heart-handshake', calendar: 'calendar-days', resources: 'life-buoy',
  },
  art: {
    medium: 'Soft pastel watercolour illustration with gentle gradients, rounded forms and lots of breathing room',
    palette: 'Lavender and soft seafoam with pale apricot and a little deep plum',
    mood: 'Quiet, safe and reassuring',
    banner: 'A calm lake with a small rowboat, round soft hills and a large gentle rising sun on the right, wide pale misty water and sky on the left',
    bannerAlt: 'A winding path through soft rounded hills to a big lone tree with a rope swing on the right, wide pale misty sky on the left',
    welcome: { subject: 'A smooth stack of balanced pebbles', alt: 'A stack of balanced pebbles' },
    instructor: { subject: 'A steaming cup of tea beside a small potted plant', alt: 'A cup of tea beside a small plant' },
    cards: [
      'A hot-air balloon floating among soft clouds',
      'A paper boat drifting on calm water',
      'A seedling sprouting from dark soil',
    ],
  },
});
