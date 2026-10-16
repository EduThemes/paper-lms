import { defineTheme } from '../../define';

export default defineTheme({
  id: 'st-patricks-day',
  name: 'St Patrick’s Day',
  description: 'Loose watercolour Irish landscapes: green hills, dry-stone walls, a small harp and dewy shamrocks. No leprechauns, no stereotypes.',
  audiences: ['early-elementary', 'upper-elementary', 'secondary'],
  tags: ['cultural', 'st-patricks-day', 'irish', 'spring', 'watercolour'],
  collection: 'special-days',
  observance: {
    name: 'St Patrick’s Day',
    traditions: ['Irish', 'Irish diaspora'],
    months: [3],
    note: 'March 17. Ireland’s national day and a Christian feast day; most schools mark it as a celebration of Irish culture.',
  },
  defaultLayout: 'classic-band',
  iconStyle: 'outline',
  corners: 'soft',
  style: 'loose watercolour and ink landscape',
  colourBlind:
    'Greens carry the theme, so structure relies on deep green against pale panels and on soft gold; there is no red in the palette to confuse with green.',
  palettes: [
    { id: 'emerald', name: 'Emerald Hills', primary: '#144D32', onPrimary: '#FFFFFF', secondary: '#F2DE8A', onSecondary: '#3A2E00', accent: '#8FBF6A', surface: '#F0F7F1', onSurface: '#15231B', background: '#FFFFFF', text: '#15231B', link: '#17613F' },
    { id: 'moss', name: 'Moss & Mist', primary: '#2E4330', onPrimary: '#FFFFFF', secondary: '#DCE6D2', onSecondary: '#2A3A20', accent: '#B5C99A', surface: '#F4F7F1', onSurface: '#1C241C', background: '#FFFFFF', text: '#1C241C', link: '#3A5E3A' },
    { id: 'harbour', name: 'Harbour Grey', primary: '#243B4A', onPrimary: '#FFFFFF', secondary: '#D3E8DA', onSecondary: '#1E3A2A', accent: '#D9B44A', surface: '#F1F5F6', onSurface: '#1A2228', background: '#FFFFFF', text: '#1A2228', link: '#2A5266' },
  ],
  icons: {
    start: 'clover', syllabus: 'scroll', modules: 'layers', assignments: 'pencil', discussions: 'message-circle',
    grades: 'star', help: 'circle-help', instructor: 'music', calendar: 'calendar-days', resources: 'library',
  },
  art: {
    medium: 'Loose watercolour and fine ink landscape illustration with soft washes, wet-in-wet skies and delicate line detail',
    palette: 'Fresh emerald and moss greens with soft gold and misty grey-blue',
    mood: 'Fresh, lilting and welcoming',
    banner: 'Rolling green hills with old dry-stone walls, a field of clover and a soft rainbow arching over a small white cottage on the right, misty pale sky on the left',
    bannerAlt: 'A large cluster of three-leaf shamrocks with dewdrops in the foreground and soft rolling hills beyond on the right, misty pale sky on the left',
    welcome: { subject: 'A single three-leaf shamrock with a dewdrop', alt: 'A shamrock' },
    instructor: { subject: 'A small wooden harp beside a pot of shamrocks', alt: 'A harp and a pot of shamrocks' },
    cards: [
      'A rocky green coastline with rolling waves',
      'A little stone bridge over a stream between green fields',
      'A grey stone cottage with a red door beside a winding lane',
    ],
  },
});
