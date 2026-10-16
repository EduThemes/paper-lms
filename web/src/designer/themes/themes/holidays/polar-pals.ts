import { defineTheme } from '../../define';

export default defineTheme({
  id: 'polar-pals',
  name: 'Polar Pals',
  description: 'Soft picture-book watercolour of Arctic animals in winter: a polar bear and cub, an arctic fox, a snowy owl, a seal pup, a hare and belugas. Seasonal, not tied to any holiday.',
  audiences: ['early-elementary'],
  tags: ['seasonal', 'winter', 'secular', 'inclusive', 'arctic', 'animals', 'science'],
  collection: 'holidays',
  observance: {
    name: 'Winter Celebrations',
    traditions: [],
    months: [12, 1, 2],
    note: 'A winter animal theme tied to no holiday, for December to February. Every animal here lives in the Arctic; penguins live in the southern hemisphere, so they are left out on purpose.',
  },
  defaultLayout: 'playful',
  iconStyle: 'solid-circle',
  corners: 'round',
  style: 'picture-book watercolour and coloured pencil',
  colourBlind:
    'An ice-blue, navy and slate palette with soft pink or gold accents; every band differs from its pale partner by lightness, and there is no red-green pairing.',
  palettes: [
    { id: 'sea-ice', name: 'Sea Ice', primary: '#173F5F', onPrimary: '#FFFFFF', secondary: '#D7ECF7', onSecondary: '#0F2C40', accent: '#F4A6B8', surface: '#EEF6FB', onSurface: '#14222E', background: '#FFFFFF', text: '#14222E', link: '#1D5788' },
    { id: 'aurora', name: 'Aurora', primary: '#1E3A40', onPrimary: '#FFFFFF', secondary: '#D2F1E6', onSecondary: '#12332A', accent: '#A88BE0', surface: '#EFF8F6', onSurface: '#152628', background: '#FFFFFF', text: '#152628', link: '#1B6168' },
    { id: 'snowy-owl', name: 'Snowy Owl', primary: '#3A3550', onPrimary: '#FFFFFF', secondary: '#ECE6F5', onSecondary: '#2E2840', accent: '#F2B544', surface: '#F5F3F9', onSurface: '#1E1B2A', background: '#FFFFFF', text: '#1E1B2A', link: '#4A3F7A' },
  ],
  icons: {
    start: 'snowflake', syllabus: 'scroll', modules: 'layers', assignments: 'pencil', discussions: 'message-circle',
    grades: 'star', help: 'circle-help', instructor: 'paw-print', calendar: 'calendar-days', resources: 'mountain-snow',
  },
  art: {
    medium: 'Cosy picture-book illustration in soft watercolour and coloured pencil, round friendly animal shapes, gentle snow sparkle and cool icy light',
    palette: 'Icy sky blue and deep Arctic navy with snow white, soft aurora pink and pale mint',
    mood: 'Gentle, curious and cosy',
    banner: 'A polar bear and her cub walking across Arctic sea ice, with a snowy owl perched on an ice ridge nearby, under faint pink and mint northern lights on the right; a calm, open pale sky over flat snow on the left. Arctic animals only, no penguins',
    bannerAlt: 'A white arctic fox curled up asleep in the snow beside a white arctic hare among snowy tundra mounds on the right; a calm, open snowfield under a soft blue sky on the left. Arctic animals only, no penguins',
    welcome: { subject: 'A fluffy white snowy owl with round yellow eyes perched on a snowy branch', alt: 'A snowy owl' },
    instructor: { subject: 'A round, fluffy polar bear cub sitting in soft snow', alt: 'A polar bear cub' },
    cards: [
      'A fluffy white seal pup resting on an ice floe beside calm blue water',
      'An arctic hare in its white winter coat hopping across fresh snow, the snowy scene and sky painted right to all four edges of the picture with no white paper margin or vignette',
      'A small pod of white beluga whales swimming in clear Arctic water beneath floating sea ice',
    ],
  },
});
