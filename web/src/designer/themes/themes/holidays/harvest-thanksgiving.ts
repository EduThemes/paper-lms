import { defineTheme } from '../../define';

export default defineTheme({
  id: 'harvest-thanksgiving',
  name: 'Harvest & Thanksgiving',
  description: 'Painterly oil still-lifes of pumpkins, squash, apples, corn and wheat in warm autumn light. Harvest and gratitude, without Pilgrim-and-Indian imagery.',
  audiences: ['upper-elementary', 'early-elementary', 'secondary'],
  tags: ['holiday', 'cultural', 'thanksgiving', 'harvest', 'gratitude', 'autumn'],
  collection: 'holidays',
  observance: {
    name: 'Thanksgiving & Harvest Festivals',
    traditions: ['United States', 'Canada', 'harvest festivals worldwide'],
    months: [9, 10, 11],
    regionalMonths: { US: [11], CA: [10], GB: [9, 10], IE: [9, 10], AU: [3, 4], NZ: [3, 4] },
    note: 'US Thanksgiving is the 4th Thursday of November and Canada’s the 2nd Monday of October; many schools hold harvest festivals in September or October. Feature Native nations and voices rather than “Pilgrims and Indians” crafts.',
  },
  defaultLayout: 'classic-band',
  iconStyle: 'soft-square',
  corners: 'soft',
  style: 'painterly oil still life',
  colourBlind:
    'Warm browns and oranges against pale gold rely on lightness, not hue; olive green is decoration only and never set against red to mean something.',
  palettes: [
    { id: 'pumpkin', name: 'Pumpkin Pie', primary: '#5A2410', onPrimary: '#FFFFFF', secondary: '#F7B267', onSecondary: '#2E1500', accent: '#8A9A3B', surface: '#FFF4E8', onSurface: '#2A1A10', background: '#FFFFFF', text: '#2A1A10', link: '#8A3412' },
    { id: 'russet', name: 'Russet & Wheat', primary: '#6B2416', onPrimary: '#FFFFFF', secondary: '#F3D9A4', onSecondary: '#3A2A05', accent: '#E07A2E', surface: '#FBF2EC', onSurface: '#2A1612', background: '#FFFFFF', text: '#2A1612', link: '#8A2E1A' },
    { id: 'olive', name: 'Olive Grove', primary: '#38401A', onPrimary: '#FFFFFF', secondary: '#F2D58A', onSecondary: '#2B2000', accent: '#D9622B', surface: '#F6F6EC', onSurface: '#1E2210', background: '#FFFFFF', text: '#1E2210', link: '#4A5512' },
  ],
  icons: {
    start: 'leaf', syllabus: 'scroll', modules: 'layers', assignments: 'pencil', discussions: 'messages-square',
    grades: 'star', help: 'circle-help', instructor: 'apple', calendar: 'calendar-days', resources: 'wheat',
  },
  art: {
    medium: 'Rich painterly oil still life with visible brushstrokes and warm low autumn light',
    palette: 'Pumpkin orange and deep russet with golden wheat, olive and cream',
    mood: 'Warm, grateful and abundant',
    banner: 'A harvest still life of pumpkins, squash, red apples, ears of corn and a sheaf of wheat on a rustic wooden table on the right, a calm warm cream wall with soft light on the left',
    bannerAlt: 'A golden autumn field with round hay bales and orange-leaved trees under a low harvest sun on the right, an open pale gold sky on the left',
    welcome: { subject: 'A small orange pumpkin with a curling green stem', alt: 'A small pumpkin' },
    instructor: { subject: 'A woven basket of red apples and pears', alt: 'A basket of apples and pears' },
    cards: [
      'A pie with a lattice crust beside a sprig of autumn leaves',
      'A sheaf of golden wheat tied with twine',
      'A row of colourful squash and gourds on a wooden shelf',
    ],
  },
});
