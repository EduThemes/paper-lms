import { defineTheme } from '../../define';

export default defineTheme({
  id: 'womens-history-month',
  name: 'Women’s History Month',
  description: 'Art Nouveau curves and suffrage colours: a telescope, a microscope, a violin and a small aeroplane. Science, art and flight, with no likenesses.',
  audiences: ['upper-elementary', 'secondary', 'higher-ed'],
  tags: ['heritage', 'womens-history-month', 'history', 'stem', 'art-nouveau'],
  collection: 'special-days',
  observance: {
    name: 'Women’s History Month',
    traditions: [],
    months: [3, 10],
    regionalMonths: { US: [3], GB: [3], IE: [3], AU: [3], NZ: [3], CA: [10] },
    note: 'March in the US, UK and Australia; Canada marks it in October. International Women’s Day is March 8. Use real photographs for real people.',
  },
  defaultLayout: 'editorial',
  iconStyle: 'outline',
  corners: 'soft',
  style: 'Art Nouveau line illustration',
  colourBlind:
    'Violet and sage come from the suffrage palette and differ in lightness; gold carries contrast, and red versus green is never used.',
  palettes: [
    { id: 'violet', name: 'Suffrage Violet', primary: '#4B2466', onPrimary: '#FFFFFF', secondary: '#DCE8C8', onSecondary: '#2E3A1A', accent: '#D4B24C', surface: '#F7F3FA', onSurface: '#231A2A', background: '#FFFFFF', text: '#231A2A', link: '#62308A' },
    { id: 'sage', name: 'Sage & Gold', primary: '#2F4A3A', onPrimary: '#FFFFFF', secondary: '#F2E2A8', onSecondary: '#3A3008', accent: '#9C6BB8', surface: '#F4F7F2', onSurface: '#1C2620', background: '#FFFFFF', text: '#1C2620', link: '#2F6248' },
    { id: 'ivory', name: 'Ink & Ivory', primary: '#2A2A3A', onPrimary: '#FFFFFF', secondary: '#E8DDF2', onSecondary: '#3A2650', accent: '#C9A84A', surface: '#F8F6F1', onSurface: '#1E1E28', background: '#FFFFFF', text: '#1E1E28', link: '#4A3A7A' },
  ],
  icons: {
    start: 'feather', syllabus: 'scroll', modules: 'library', assignments: 'pen-tool', discussions: 'messages-square',
    grades: 'award', help: 'circle-help', instructor: 'telescope', calendar: 'calendar-days', resources: 'book-open',
  },
  art: {
    medium: 'Elegant Art Nouveau-inspired illustration with flowing linework, flat colour and decorative botanical curves',
    palette: 'Suffrage violet and sage green with soft gold on ivory',
    mood: 'Bold, graceful and inspiring',
    banner: 'A brass telescope pointed at a sky of stars beside a microscope and a stack of closed books, with curling violets growing around them on the right, calm plain ivory on the left',
    bannerAlt: 'A vintage bicycle with a basket of violets and lilies leaning beside a tall stack of closed books on the right, calm plain ivory on the left',
    welcome: { subject: 'A sprig of violets tied with a ribbon', alt: 'A bunch of violets' },
    instructor: { subject: 'A fountain pen resting on a closed notebook with a plain cover', alt: 'A fountain pen on a notebook' },
    cards: [
      'A small vintage aeroplane soaring through clouds',
      'A violin resting on a velvet chair',
      'A spiral galaxy above an observatory dome',
    ],
  },
});
