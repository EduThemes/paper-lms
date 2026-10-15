import { defineTheme, FONTS } from '../../define';

export default defineTheme({
  id: 'vintage-noel',
  name: 'Vintage Noël',
  description: '1950s holiday illustration: shiny glass baubles, atomic starbursts, a flocked tree and a station wagon bringing the tree home, in cherry, aqua and candy pink.',
  audiences: ['secondary', 'higher-ed', 'professional'],
  tags: ['holiday', 'cultural', 'christmas', 'winter', 'retro', 'mid-century', '1950s'],
  collection: 'holidays',
  premium: true,
  observance: {
    name: 'Christmas',
    traditions: ['Christian', 'secular Christmas'],
    months: [12, 1],
    note: 'December 25; Orthodox churches that keep the Julian calendar celebrate on January 7. A secular retro look with no religious imagery, good for a holiday-season or end-of-term page.',
  },
  defaultLayout: 'split-hero',
  iconStyle: 'solid-circle',
  headingFont: FONTS.georgia,
  corners: 'round',
  style: '1950s holiday gouache illustration',
  colourBlind:
    'Cherry, aqua and silver differ strongly in lightness and there is no red-green pairing; candy pink is decorative only.',
  palettes: [
    { id: 'cherry', name: 'Cherry & Aqua', primary: '#7C1A26', onPrimary: '#FFFFFF', secondary: '#CDEBE6', onSecondary: '#123A36', accent: '#E89BB0', surface: '#FBF2EE', onSurface: '#2A1519', background: '#FFFFFF', text: '#2A1519', link: '#9A1F30' },
    { id: 'aqua', name: 'Atomic Aqua', primary: '#13464A', onPrimary: '#FFFFFF', secondary: '#F8D9DF', onSecondary: '#5A1A2A', accent: '#C8373D', surface: '#EEF7F6', onSurface: '#132628', background: '#FFFFFF', text: '#132628', link: '#16595E' },
    { id: 'tinsel', name: 'Silver Tinsel', primary: '#2D3340', onPrimary: '#FFFFFF', secondary: '#E4E7EC', onSecondary: '#262B36', accent: '#D9485A', surface: '#F3F4F7', onSurface: '#1B1F28', background: '#FFFFFF', text: '#1B1F28', link: '#3A4A6E' },
  ],
  icons: {
    start: 'sparkles', syllabus: 'scroll', modules: 'shapes', assignments: 'pencil', discussions: 'messages-square',
    grades: 'star', help: 'circle-help', instructor: 'tree-pine', calendar: 'calendar-days', resources: 'gift',
  },
  art: {
    medium: 'Mid-century 1950s holiday illustration in flat gouache with a soft lithograph grain, stylised atomic starbursts, rounded retro shapes and a light sprinkle of silver glitter',
    palette: 'Cherry red and soft aqua with candy pink, silver and warm cream',
    mood: 'Nostalgic, merry and elegant',
    banner: 'A rounded 1950s station wagon with chrome trim, a plain bumper and no licence plate, parked and empty with nobody inside, carrying a fresh evergreen tree tied to its roof through a snowy suburb of low ranch houses with glowing windows, atomic starbursts in the sky, on the right; a calm, plain cream winter sky over soft snow on the left',
    bannerAlt: 'A white flocked evergreen tree hung with shiny glass baubles in cherry, aqua and pink beside a low mid-century cabinet with a teal ceramic lamp, on the right; a plain warm cream wall with a few tiny silver starbursts on the left',
    welcome: { subject: 'A shiny retro glass bauble in cherry red with an indented aqua star', alt: 'A retro glass ornament' },
    instructor: { subject: 'A mid-century ceramic mug of cocoa on a kidney-shaped side table beside a small dish of ribbon candy', alt: 'A mug of cocoa on a retro side table' },
    cards: [
      'A cluster of shiny glass baubles in cherry, aqua and pink among atomic starbursts',
      'A silver tinsel tree with pink and aqua baubles in a cosy mid-century living room',
      'A snowy street of low ranch houses strung with big old-fashioned coloured bulb lights at dusk',
    ],
  },
});
