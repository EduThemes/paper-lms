import { defineTheme } from '../../define';

export default defineTheme({
  id: 'christmas',
  name: 'Christmas',
  description: 'Cosy needle-felted wool: a decorated evergreen, poinsettias, wreaths and a snowy village, plus a quiet star-over-the-stable banner for faith schools.',
  audiences: ['early-elementary', 'upper-elementary', 'secondary'],
  tags: ['holiday', 'religious', 'christian', 'christmas', 'winter'],
  collection: 'holidays',
  observance: {
    name: 'Christmas',
    traditions: ['Christian'],
    months: [12, 1],
    note: 'December 25 for most churches; Orthodox churches that keep the Julian calendar celebrate on January 7. Banner-alt (a star over an empty stable) suits faith schools; the rest is general Christmas.',
  },
  defaultLayout: 'playful',
  iconStyle: 'solid-circle',
  corners: 'round',
  style: 'needle-felted wool craft',
  colourBlind:
    'Red and green are never the only distinction: the evergreen band pairs with pale gold (a light-dark contrast), cranberry is a decorative accent, and the Starlight colourway drops red and green entirely.',
  palettes: [
    { id: 'evergreen', name: 'Evergreen', primary: '#1E4D3A', onPrimary: '#FFFFFF', secondary: '#F2D58A', onSecondary: '#2A2000', accent: '#B3263A', surface: '#F3F7F2', onSurface: '#17221C', background: '#FFFFFF', text: '#17221C', link: '#1F6045' },
    { id: 'cranberry', name: 'Cranberry', primary: '#7A1F2B', onPrimary: '#FFFFFF', secondary: '#F6E3B4', onSecondary: '#3A2A05', accent: '#2F6B4F', surface: '#FBF2EF', onSurface: '#2A1517', background: '#FFFFFF', text: '#2A1517', link: '#8E2433' },
    { id: 'starlight', name: 'Starlight', primary: '#1B2A4A', onPrimary: '#FFFFFF', secondary: '#F2D58A', onSecondary: '#2B2000', accent: '#9EC5E8', surface: '#F1F4FA', onSurface: '#151B2B', background: '#FFFFFF', text: '#151B2B', link: '#2A4A8A' },
  ],
  icons: {
    start: 'star', syllabus: 'scroll', modules: 'blocks', assignments: 'pencil', discussions: 'message-circle',
    grades: 'award', help: 'circle-help', instructor: 'tree-pine', calendar: 'calendar-days', resources: 'gift',
  },
  art: {
    medium: 'Needle-felted wool and stitched felt craft with soft fuzzy fibres, visible stitching and a cosy tabletop glow',
    palette: 'Deep evergreen and cranberry red with warm gold and cream wool',
    mood: 'Cosy, joyful and warm',
    banner: 'A felt evergreen tree decorated with round ornaments and a gold star on top, with a few presents wrapped in plain paper with no tags beneath it, on the right; a soft cream wool background with gentle felt snowflakes on the left',
    bannerAlt: 'A single bright gold star shining over a small wooden stable holding one empty wooden manger filled only with loose golden straw, with no cloth, blanket or bundle in it and no figures, on a quiet hill at night on the right, a calm deep blue felt sky on the left',
    welcome: { subject: 'A felt poinsettia with red bracts and a gold centre', alt: 'A felt poinsettia' },
    instructor: { subject: 'A felt mug of cocoa beside a gingerbread biscuit and a sprig of holly', alt: 'A mug of cocoa with a gingerbread biscuit' },
    cards: [
      'A felt wreath of evergreen and holly berries with a red bow',
      'Three lit candles among pine branches',
      'A snowy felt village of small cottages with glowing windows at night',
    ],
  },
});
