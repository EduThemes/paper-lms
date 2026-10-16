import { defineTheme, FONTS } from '../../define';

export default defineTheme({
  id: 'gilded-atlas',
  name: 'Gilded Atlas',
  description: 'Antique copperplate engraving with burnished gold leaf: compass roses, galleons and hatched seas on aged parchment. An explorer’s library in a single page.',
  audiences: ['higher-ed', 'professional', 'secondary'],
  tags: ['engraving', 'gold-leaf', 'cartography', 'nautical', 'antique', 'humanities'],
  defaultLayout: 'editorial',
  iconStyle: 'outline',
  headingFont: FONTS.georgia,
  corners: 'square',
  collection: 'signature',
  premium: true,
  style: 'copperplate engraving with gold leaf',
  colourBlind:
    'The engraving is ink on parchment, so shapes read by line and lightness; gold is decorative only, and verdigris and oxblood never sit side by side to mean different things.',
  palettes: [
    { id: 'navy-gold', name: 'Navy & Gold', primary: '#1B2A4A', onPrimary: '#FFFFFF', secondary: '#E8D49A', onSecondary: '#2A2210', accent: '#B8892B', surface: '#F6F1E4', onSurface: '#1C1F2A', background: '#FFFFFF', text: '#1C1F2A', link: '#2B4A7E' },
    { id: 'oxblood', name: 'Oxblood & Gilt', primary: '#5A1E1E', onPrimary: '#FFFFFF', secondary: '#EBD9B0', onSecondary: '#3A1A10', accent: '#B8892B', surface: '#F8F1E8', onSurface: '#2A1A16', background: '#FFFFFF', text: '#2A1A16', link: '#7A2A22' },
    { id: 'verdigris', name: 'Verdigris', primary: '#1F4A47', onPrimary: '#FFFFFF', secondary: '#CFE3DC', onSecondary: '#143330', accent: '#C29A3A', surface: '#EFF5F1', onSurface: '#172624', background: '#FFFFFF', text: '#172624', link: '#1F5A55' },
  ],
  icons: {
    start: 'compass', syllabus: 'scroll-text', modules: 'map', assignments: 'feather', discussions: 'messages-square',
    grades: 'award', help: 'life-buoy', instructor: 'sailboat', calendar: 'calendar-days', resources: 'library',
  },
  art: {
    medium: 'Antique copperplate engraving with fine hatched line work in dark ink on aged parchment, burnished gold-leaf highlights, the engraved image runs off every edge with no border, plate edge or margin',
    palette: 'Deep navy ink and burnished gold on warm parchment with a hint of verdigris',
    mood: 'Scholarly, adventurous and finely crafted',
    banner: 'A large gold-leaf compass rose with fine radiating rhumb lines, a galleon under full sail and curling hatched waves on the right, plain aged parchment with a faint hatched sea on the left',
    bannerAlt: 'A pair of brass navigational dividers resting across an engraved coastline of coves and small islands seen from above, gold-leaf rhumb lines fanning out from it, on the right, plain aged parchment fading into calm sea on the left',
    welcome: { subject: 'A gold-leaf compass rose', alt: 'A gilded compass rose' },
    instructor: { subject: 'A brass ship’s sextant with a smooth, unmarked arc (no scale, ticks or numerals) resting on a rolled sheet of blank parchment', alt: 'A brass sextant on rolled parchment' },
    cards: [
      'An engraved galleon under full sail on hatched waves',
      'A gilded spouting whale among curling engraved waves',
      'An engraved island with a tiny lighthouse ringed by gold-leaf rhumb lines',
    ],
  },
});
