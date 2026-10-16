import { defineTheme } from '../../define';

export default defineTheme({
  id: 'vaisakhi',
  name: 'Vaisakhi',
  description: 'Wood-block textile prints of golden wheat, a garlanded dhol drum, marigolds and the shared langar meal in saffron and navy. Harvest and the founding of the Khalsa.',
  audiences: ['upper-elementary', 'secondary', 'early-elementary'],
  tags: ['holiday', 'religious', 'sikh', 'punjabi', 'hindu', 'harvest', 'spring'],
  collection: 'holidays',
  observance: {
    name: 'Vaisakhi',
    traditions: ['Sikh', 'Punjabi (harvest festival)', 'Hindu (solar new year)'],
    months: [4],
    note: 'Fixed on the solar calendar: April 13 or 14. Sikhs mark the founding of the Khalsa in 1699; in Punjab it is also the spring harvest festival. Also spelled Baisakhi.',
  },
  defaultLayout: 'card-grid',
  iconStyle: 'soft-square',
  corners: 'soft',
  style: 'wood-block textile print',
  colourBlind:
    'Saffron and navy are a blue-orange pair with a large lightness gap, so they stay distinct for red-green colour blindness.',
  palettes: [
    { id: 'kesri', name: 'Kesri & Navy', primary: '#1E2F5C', onPrimary: '#FFFFFF', secondary: '#FFB547', onSecondary: '#2E1A00', accent: '#E8C14A', surface: '#FFF5E6', onSurface: '#1D1F2B', background: '#FFFFFF', text: '#1D1F2B', link: '#26408A' },
    { id: 'wheat', name: 'Golden Wheat', primary: '#5A3A0E', onPrimary: '#FFFFFF', secondary: '#F3DC95', onSecondary: '#332600', accent: '#F28C28', surface: '#FBF6E6', onSurface: '#271D0E', background: '#FFFFFF', text: '#271D0E', link: '#7A4A0C' },
    { id: 'navy', name: 'Deep Navy', primary: '#15234A', onPrimary: '#FFFFFF', secondary: '#CFD9F2', onSecondary: '#15234A', accent: '#F2A03D', surface: '#F0F3FA', onSurface: '#141A2E', background: '#FFFFFF', text: '#141A2E', link: '#23408F' },
  ],
  icons: {
    start: 'sunrise', syllabus: 'scroll', modules: 'layers', assignments: 'pencil', discussions: 'users',
    grades: 'star', help: 'circle-help', instructor: 'drum', calendar: 'calendar-days', resources: 'wheat',
  },
  art: {
    medium: 'Wood-block textile print on cotton with slightly offset inks, crisp carved motifs and visible woven texture',
    palette: 'Saffron orange and deep navy blue with golden wheat yellow on natural cotton',
    mood: 'Joyful, generous and full of harvest energy',
    banner: 'Tall golden wheat sheaves and a dhol drum decorated with marigold garlands on the right, a calm open field of pale wheat under a wide sky on the left',
    bannerAlt: 'Big polished steel cooking pots and a stack of round flatbreads beside marigolds on a long plain mat, ready for a shared community meal, on the right; an open calm cotton-coloured background on the left',
    welcome: { subject: 'A bundle of golden wheat tied with a saffron ribbon', alt: 'A bundle of golden wheat' },
    instructor: { subject: 'A dhol drum with saffron cords resting on a woven mat', alt: 'A dhol drum' },
    cards: [
      'A field of ripe wheat under a bright spring sun, the print filling the whole canvas edge to edge with no printed border, frame or plain margin around the motif',
      'A garland of marigolds over a block-printed floral pattern',
      'Round flatbreads and a steel bowl of lentils on a plain cloth',
    ],
  },
});
