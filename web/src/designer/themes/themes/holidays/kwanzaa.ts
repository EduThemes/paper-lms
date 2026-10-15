import { defineTheme } from '../../define';

export default defineTheme({
  id: 'kwanzaa',
  name: 'Kwanzaa',
  description: 'Bold cut-paper collage of the kinara, the mkeka mat, ears of corn, the harvest basket and the unity cup in red, black, green and gold.',
  audiences: ['upper-elementary', 'secondary', 'early-elementary'],
  tags: ['holiday', 'cultural', 'african-american', 'pan-african', 'heritage', 'winter'],
  collection: 'holidays',
  observance: {
    name: 'Kwanzaa',
    traditions: ['African American', 'Pan-African diaspora'],
    months: [12, 1],
    note: 'December 26 to January 1 every year. A cultural celebration created in 1966 and kept alongside any faith; each day honours one of the seven principles (Nguzo Saba).',
  },
  defaultLayout: 'classic-band',
  iconStyle: 'soft-square',
  corners: 'soft',
  style: 'bold cut-paper collage',
  colourBlind:
    'The kinara’s red and green candles are told apart by position (three left, black centre, three right), not colour alone; in the UI red and green never sit side by side to mean different things, and every band pairs a deep colour with pale gold.',
  palettes: [
    { id: 'umoja', name: 'Umoja Black & Gold', primary: '#1F1A17', onPrimary: '#FFFFFF', secondary: '#F2C14E', onSecondary: '#2E2000', accent: '#B3262E', surface: '#FBF3E6', onSurface: '#1F1A17', background: '#FFFFFF', text: '#1F1A17', link: '#8E1F28' },
    { id: 'red', name: 'Harvest Red', primary: '#7A1F24', onPrimary: '#FFFFFF', secondary: '#F3D9A4', onSecondary: '#3A2A05', accent: '#2E6B3A', surface: '#FBF2EE', onSurface: '#2A1517', background: '#FFFFFF', text: '#2A1517', link: '#8E2430' },
    { id: 'green', name: 'Forest Green', primary: '#1F4A2E', onPrimary: '#FFFFFF', secondary: '#F2D58A', onSecondary: '#2B2000', accent: '#B3262E', surface: '#F1F6F0', onSurface: '#162019', background: '#FFFFFF', text: '#162019', link: '#1D6036' },
  ],
  icons: {
    start: 'sunrise', syllabus: 'scroll', modules: 'layers', assignments: 'pencil', discussions: 'users',
    grades: 'star', help: 'circle-help', instructor: 'flame', calendar: 'calendar-days', resources: 'wheat',
  },
  art: {
    medium: 'Bold cut-paper collage with textured handmade papers and crisp layered edges',
    palette: 'Deep red, rich black and forest green with warm gold and kraft brown',
    mood: 'Proud, warm and united',
    banner: 'A wooden kinara holding exactly seven lit candles in one row, three red on the left, one black in the centre and three green on the right, on a woven straw mat with ears of corn and a plain carved wooden cup on the right; a calm warm kraft background on the left',
    bannerAlt: 'A woven straw mat with a basket of fruit and vegetables, several ears of dried corn and a plain carved wooden unity cup on the right, a plain open warm gold background on the left',
    welcome: { subject: 'A plain carved wooden unity cup on a woven straw mat', alt: 'A wooden unity cup on a woven mat' },
    instructor: { subject: 'A wooden kinara with exactly seven unlit candles: three red on the left, one black in the centre, three green on the right', alt: 'A kinara with seven candles' },
    cards: [
      'Three ears of dried corn tied together on a woven straw mat',
      'A basket of fruit and vegetables from the harvest',
      'A plain woven straw mkeka mat with simple natural stripes, a carved wooden unity cup resting on it',
    ],
  },
});
