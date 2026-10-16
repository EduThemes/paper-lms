import { defineTheme } from '../../define';

export default defineTheme({
  id: 'hour-of-code',
  name: 'Hour of Code',
  description: 'Crisp geometric vector art: a little robot rolling along a path of tiles, turning gears and stacked blocks. For Computer Science Education Week.',
  audiences: ['upper-elementary', 'secondary'],
  tags: ['awareness', 'hour-of-code', 'cs-education-week', 'coding', 'stem', 'digital-literacy', 'geometric'],
  collection: 'special-days',
  observance: {
    name: 'Computer Science Education Week',
    traditions: [],
    months: [12],
    note: 'Early to mid December. Hour of Code activities run all year.',
  },
  defaultLayout: 'card-grid',
  iconStyle: 'soft-square',
  corners: 'soft',
  style: 'bold geometric vector on a dotted grid',
  colourBlind:
    'Deep blue and mint differ strongly in lightness; pass and fail states are never shown as red versus green.',
  palettes: [
    { id: 'byte', name: 'Byte Blue', primary: '#172E6E', onPrimary: '#FFFFFF', secondary: '#7FE3C8', onSecondary: '#0F2A3A', accent: '#FF6B9A', surface: '#EFF4FF', onSurface: '#151C30', background: '#FFFFFF', text: '#151C30', link: '#2340A0' },
    { id: 'magenta', name: 'Magenta Loop', primary: '#5E1A5A', onPrimary: '#FFFFFF', secondary: '#FFD1F0', onSecondary: '#4A1046', accent: '#3DDCFF', surface: '#FDF2FA', onSurface: '#26162A', background: '#FFFFFF', text: '#26162A', link: '#7A2276' },
    { id: 'lime', name: 'Lime Logic', primary: '#24402A', onPrimary: '#FFFFFF', secondary: '#D6F57A', onSecondary: '#2A3A08', accent: '#6B5BFF', surface: '#F4FBE8', onSurface: '#1A2418', background: '#FFFFFF', text: '#1A2418', link: '#2E5E30' },
  ],
  icons: {
    start: 'code-xml', syllabus: 'list-checks', modules: 'blocks', assignments: 'puzzle', discussions: 'messages-square',
    grades: 'trophy', help: 'circle-help', instructor: 'bot', calendar: 'calendar-days', resources: 'cpu',
  },
  art: {
    medium: 'Bold flat vector illustration built from simple geometric shapes on a faint dotted grid, with crisp edges and bright blocks of colour',
    palette: 'Deep cobalt blue and fresh mint with a pop of bright pink on a pale ground',
    mood: 'Logical, playful and inventive',
    banner: 'A friendly little robot rolling along a winding path of bright square tiles toward a glowing star on the right, calm plain pale grid on the left',
    bannerAlt: 'A tall rocket built from stacked geometric blocks ready to launch, with small turning gears beside it on the right, calm plain pale grid on the left',
    welcome: { subject: 'A friendly little robot with a round body and a single antenna', alt: 'A friendly robot' },
    instructor: { subject: 'A closed laptop with a plain lid beside a mug of tea', alt: 'A closed laptop and a mug' },
    cards: [
      'A row of light bulbs, some glowing and some dark',
      'A small rover robot crossing a floor of coloured tiles',
      'Colourful interlocking gears turning together',
    ],
  },
});
