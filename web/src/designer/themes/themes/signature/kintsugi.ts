import { defineTheme, FONTS } from '../../define';

export default defineTheme({
  id: 'kintsugi',
  name: 'Kintsugi',
  description: 'Stoneware mended with seams of gold, photographed in soft window light. A quiet theme about repair, resilience and second drafts.',
  audiences: ['higher-ed', 'professional', 'specialty'],
  tags: ['kintsugi', 'ceramics', 'gold', 'still-life', 'resilience', 'counseling', 'sel'],
  defaultLayout: 'editorial',
  iconStyle: 'outline',
  headingFont: FONTS.georgia,
  corners: 'soft',
  collection: 'signature',
  premium: true,
  style: 'kintsugi still-life photography',
  colourBlind:
    'Gold seams on charcoal and pale glaze read by lightness alone; no colourway relies on red against green.',
  palettes: [
    { id: 'raku', name: 'Raku', primary: '#262422', onPrimary: '#FFFFFF', secondary: '#EADFC6', onSecondary: '#2E2614', accent: '#C9A23A', surface: '#F5F2EC', onSurface: '#1C1A18', background: '#FFFFFF', text: '#1C1A18', link: '#6E5414' },
    { id: 'celadon', name: 'Celadon', primary: '#2F4A44', onPrimary: '#FFFFFF', secondary: '#DCE8E2', onSecondary: '#22362F', accent: '#C9A23A', surface: '#F1F6F3', onSurface: '#18241F', background: '#FFFFFF', text: '#18241F', link: '#2D5A50' },
    { id: 'indigo-glaze', name: 'Indigo Glaze', primary: '#22305A', onPrimary: '#FFFFFF', secondary: '#E2E5F0', onSecondary: '#22305A', accent: '#C9A23A', surface: '#F2F3F8', onSurface: '#181D2E', background: '#FFFFFF', text: '#181D2E', link: '#2E4580' },
  ],
  icons: {
    start: 'sparkle', syllabus: 'scroll-text', modules: 'layers', assignments: 'pen-line', discussions: 'messages-square',
    grades: 'award', help: 'heart-handshake', instructor: 'coffee', calendar: 'calendar-days', resources: 'library',
  },
  art: {
    medium: 'Refined still-life photography of handmade stoneware with gold kintsugi repairs, soft diffused window light, shallow depth of field and a smooth plain backdrop',
    palette: 'Charcoal glaze, warm stone grey and molten gold with soft ivory',
    mood: 'Quiet, resilient and beautifully imperfect',
    banner: 'A large charcoal stoneware bowl mended with branching seams of gold, with a small cracked cup beside it, on the right, a smooth plain warm grey backdrop and tabletop continuing to the left',
    bannerAlt: 'A tall pale celadon vase with a bright river of gold repair running down its side, holding a single dried branch, on the right, the same soft plain backdrop and linen-covered tabletop on the left',
    welcome: { subject: 'A small tea bowl with a gold-mended crack', alt: 'A tea bowl mended with gold' },
    instructor: { subject: 'A fine brush resting beside a small dish of powdered gold', alt: 'A brush beside a dish of gold powder' },
    cards: [
      'Three stacked plates mended with gold seams',
      'A ceramic shard joined with gold beside a smooth pebble',
      'A gold-mended teapot on a linen cloth',
    ],
  },
});
