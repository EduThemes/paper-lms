import { defineTheme } from '../../define';

export default defineTheme({
  id: 'navidad',
  name: 'Navidad',
  description: 'Glowing paper-craft dioramas from Las Posadas to Día de Reyes: farolitos on adobe walls, a seven-point star piñata, nochebuena flowers, ponche, tamales and a rosca de reyes with three paper crowns.',
  audiences: ['upper-elementary', 'early-elementary', 'secondary'],
  tags: ['holiday', 'religious', 'cultural', 'christian', 'christmas', 'latino', 'mexican', 'navidad', 'winter'],
  collection: 'holidays',
  observance: {
    name: 'Navidad: Las Posadas to Día de Reyes',
    traditions: ['Mexican', 'Mexican American', 'Latin American', 'Spanish'],
    months: [12, 1],
    note: 'Las Posadas runs nine nights, December 16–24; Nochebuena (Christmas Eve) is the big family night, and Día de Reyes on January 6 brings the rosca de reyes. Christian in origin, and customs differ by country.',
  },
  defaultLayout: 'card-grid',
  iconStyle: 'solid-circle',
  corners: 'round',
  style: 'candlelit paper-craft diorama',
  colourBlind:
    'Deep red, indigo and plum bands sit against pale gold or pink, so every pair differs strongly in lightness; green appears only as a decorative accent.',
  palettes: [
    { id: 'nochebuena', name: 'Nochebuena', primary: '#7A1525', onPrimary: '#FFFFFF', secondary: '#FBD9A0', onSecondary: '#3A2400', accent: '#1E7A5C', surface: '#FFF4EC', onSurface: '#2A1418', background: '#FFFFFF', text: '#2A1418', link: '#981C30' },
    { id: 'posada', name: 'Noche de Posada', primary: '#1D2550', onPrimary: '#FFFFFF', secondary: '#FFD98A', onSecondary: '#3A2600', accent: '#E0457B', surface: '#F2F2FA', onSurface: '#171A2E', background: '#FFFFFF', text: '#171A2E', link: '#30408C' },
    { id: 'pinata', name: 'Piñata', primary: '#5A1E5C', onPrimary: '#FFFFFF', secondary: '#FFD6E6', onSecondary: '#4A0F2E', accent: '#1FB5A8', surface: '#FBF0F7', onSurface: '#271628', background: '#FFFFFF', text: '#271628', link: '#7A2A7E' },
  ],
  icons: {
    start: 'star', syllabus: 'scroll', modules: 'layers', assignments: 'pencil', discussions: 'messages-square',
    grades: 'award', help: 'circle-help', instructor: 'flame', calendar: 'calendar-days', resources: 'crown',
  },
  art: {
    medium: 'Handmade paper-craft diorama photographed at night, made of papier-mâché, cut tissue paper and painted paperboard glowing in warm candlelight, with soft shadows and a shallow depth of field',
    palette: 'Nochebuena red and warm candle gold with deep night indigo and bright magenta pink',
    mood: 'Warm, festive and welcoming',
    banner: 'A row of glowing paper-bag farolitos lining the top of an adobe wall and a low flat rooftop, with red nochebuena flowers in clay pots below, under a deep blue night sky on the right; calm, open night sky above smooth adobe on the left',
    bannerAlt: 'A bright round star piñata with long cones ending in fringed tissue tassels, hanging above a courtyard strung with tiny warm lights, on the right; an open, plain night sky on the left',
    welcome: { subject: 'A flat, front-facing seven-point star piñata seen straight on: a round centre with exactly seven cones radiating evenly around it like a seven-pointed star, no cones behind or in front, each cone ending in one tissue tassel, in bright pink, orange and gold tissue', alt: 'A star piñata' },
    instructor: { subject: 'A small glazed clay mug of steaming fruit ponche with a cinnamon stick, beside a clay pot holding guava, apple slices and sugar cane', alt: 'A clay mug of warm fruit ponche' },
    cards: [
      'A red nochebuena poinsettia in a painted clay pot glowing in candlelight',
      'An oval rosca de reyes ring bread topped with strips of candied fruit and sugar, with three plain gold paper crowns beside it',
      'A wooden table set for Nochebuena with a clay pot of tamales, a basket of golden buñuelos and lit candles',
    ],
  },
});
