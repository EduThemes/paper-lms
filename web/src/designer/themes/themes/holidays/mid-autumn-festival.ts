import { defineTheme } from '../../define';

export default defineTheme({
  id: 'mid-autumn-festival',
  name: 'Mid-Autumn Festival',
  description: 'Gouache moonlit nights with a huge full moon, glowing star, rabbit and round lanterns, plain mooncakes and pomelos. For Zhongqiu, Tết Trung Thu, Chuseok and Tsukimi.',
  audiences: ['early-elementary', 'upper-elementary', 'secondary'],
  tags: ['holiday', 'cultural', 'chinese', 'vietnamese', 'korean', 'japanese', 'moon', 'lanterns', 'harvest'],
  collection: 'holidays',
  observance: {
    name: 'Mid-Autumn Festival',
    traditions: ['Chinese (Zhongqiu)', 'Vietnamese (Tết Trung Thu)', 'Korean (Chuseok)', 'Japanese (Tsukimi)', 'Southeast Asian communities'],
    months: [9, 10],
    note: 'Full moon of the 8th lunar month, in September or early October. Korea’s Chuseok harvest holiday falls on the same day.',
  },
  defaultLayout: 'split-hero',
  iconStyle: 'solid-circle',
  corners: 'round',
  style: 'glowing gouache night painting',
  colourBlind:
    'Midnight blue and moon yellow are a blue-yellow pair with a large lightness gap; red appears only in the lanterns as decoration and never carries meaning.',
  palettes: [
    { id: 'midnight', name: 'Midnight Moon', primary: '#16224A', onPrimary: '#FFFFFF', secondary: '#F7D57A', onSecondary: '#2B2000', accent: '#D9432E', surface: '#F1F3FA', onSurface: '#151A2C', background: '#FFFFFF', text: '#151A2C', link: '#26408F' },
    { id: 'lantern', name: 'Lantern Red', primary: '#7E1D1D', onPrimary: '#FFFFFF', secondary: '#FBD9A0', onSecondary: '#3A2200', accent: '#2C3E7A', surface: '#FFF4EA', onSurface: '#2A1515', background: '#FFFFFF', text: '#2A1515', link: '#931F1F' },
    { id: 'moonlight', name: 'Moonlight', primary: '#2D2A55', onPrimary: '#FFFFFF', secondary: '#E6E1F7', onSecondary: '#22204A', accent: '#F2B544', surface: '#F5F3FB', onSurface: '#1B1A2E', background: '#FFFFFF', text: '#1B1A2E', link: '#433E8A' },
  ],
  icons: {
    start: 'moon', syllabus: 'scroll', modules: 'layers', assignments: 'pencil', discussions: 'messages-square',
    grades: 'star', help: 'circle-help', instructor: 'rabbit', calendar: 'calendar-days', resources: 'cake',
  },
  art: {
    medium: 'Dreamy gouache night painting with soft glowing lantern light and velvety matte brush texture',
    palette: 'Deep midnight blue with glowing amber, warm red and soft moon yellow',
    mood: 'Magical, gentle and warm',
    banner: 'A huge glowing full moon above soft hills and osmanthus branches, with three glowing lanterns floating below it on the right: one round red lantern, one five-pointed star lantern and one rabbit-shaped lantern, all plain and unmarked; a calm deep blue night sky on the left',
    bannerAlt: 'A round plate of four golden mooncakes, each with a plain flower-petal pattern pressed on top and no markings, beside a small teapot, a pomelo and a few half-moon rice cakes on the right, an open calm dark blue background on the left',
    welcome: { subject: 'A five-pointed star lantern made of red cellophane on a thin bamboo frame with a short tassel, plain and unmarked', alt: 'A glowing star-shaped lantern' },
    instructor: { subject: 'A small teapot and two cups beside one golden mooncake with a plain flower pattern and no markings', alt: 'A teapot and cups beside a mooncake' },
    cards: [
      'A white rabbit gazing up at a full moon among tall silver grasses',
      'A carp-shaped paper lantern glowing softly in the dark',
      'A basket of pomelos, persimmons and chestnuts for the harvest',
    ],
  },
});
