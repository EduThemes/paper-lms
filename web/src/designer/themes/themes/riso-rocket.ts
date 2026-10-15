import { defineTheme } from '../define';

export default defineTheme({
  id: 'riso-rocket',
  name: 'Riso Rocket',
  description: 'Two-ink risograph print with grain and slight misregistration: rockets, ringed planets and comets.',
  audiences: ['upper-elementary', 'secondary'],
  tags: ['risograph', 'space', 'print', 'bold'],
  defaultLayout: 'split-hero',
  iconStyle: 'duotone',
  corners: 'soft',
  style: 'risograph two-ink print',
  colourBlind:
    'Pink and blue differ in lightness and remain distinct for protan, deutan and tritan viewers.',
  palettes: [
    { id: 'pink-blue', name: 'Pink & Blue', primary: '#2A2F8F', onPrimary: '#FFFFFF', secondary: '#FFC2DD', onSecondary: '#5A0F38', accent: '#FF48B0', surface: '#F3F1FF', onSurface: '#1E1B3A', background: '#FFFFFF', text: '#1E1B3A', link: '#2A2F8F' },
    { id: 'teal-yellow', name: 'Teal & Yellow', primary: '#0D5257', onPrimary: '#FFFFFF', secondary: '#FFE45C', onSecondary: '#332B00', accent: '#00A6A6', surface: '#EEF8F7', onSurface: '#152526', background: '#FFFFFF', text: '#152526', link: '#0D5257' },
    { id: 'orange-navy', name: 'Orange & Navy', primary: '#1D2B53', onPrimary: '#FFFFFF', secondary: '#FFC9A3', onSecondary: '#4A1F00', accent: '#FF6C2F', surface: '#FFF3EB', onSurface: '#1B1F2E', background: '#FFFFFF', text: '#1B1F2E', link: '#1D2B53' },
  ],
  icons: {
    start: 'rocket', syllabus: 'clipboard-list', modules: 'orbit', assignments: 'pencil', discussions: 'messages-square',
    grades: 'star', help: 'life-buoy', instructor: 'telescope', calendar: 'calendar-days', resources: 'satellite',
  },
  art: {
    medium: 'Risograph print illustration in two overprinted spot inks with visible grain, slight misregistration and halftone shading',
    palette: 'Fluorescent pink and medium blue overprinting into violet on off-white paper',
    mood: 'Energetic, handmade and a little bit cosmic',
    banner: 'A big chunky rocket lifting off past two ringed planets and a scatter of stars on the right, quiet open grainy paper sky on the left',
    bannerAlt: 'A large astronaut helmet floating beside a crescent moon and a streaking comet on the right, quiet open grainy paper sky on the left',
    welcome: { subject: 'A ringed planet with a small moon', alt: 'A ringed planet' },
    instructor: { subject: 'An empty astronaut helmet resting on the ground', alt: 'An astronaut helmet' },
    cards: [
      'A rocket with round portholes',
      'A crescent moon surrounded by stars',
      'A satellite with wide solar panels',
    ],
  },
});
