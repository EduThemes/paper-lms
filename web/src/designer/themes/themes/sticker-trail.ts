import { defineTheme } from '../define';

export default defineTheme({
  id: 'sticker-trail',
  name: 'Sticker Trail',
  description: 'Glossy die-cut adventure stickers on kraft paper: compasses, tents and binoculars for young explorers.',
  audiences: ['upper-elementary'],
  tags: ['stickers', 'adventure', 'outdoors', 'bold'],
  defaultLayout: 'card-grid',
  iconStyle: 'soft-square',
  corners: 'soft',
  style: 'die-cut vinyl stickers',
  colourBlind:
    'Forest teal against sunset orange is a blue-orange pairing that stays distinct for every common type of colour blindness.',
  palettes: [
    { id: 'trail', name: 'Trailhead', primary: '#0F4C4A', onPrimary: '#FFFFFF', secondary: '#FFB347', onSecondary: '#3B2200', accent: '#4DA8DA', surface: '#F5EFE3', onSurface: '#1F2421', background: '#FFFFFF', text: '#1F2421', link: '#0F4C4A' },
    { id: 'canyon', name: 'Canyon', primary: '#6B2E1F', onPrimary: '#FFFFFF', secondary: '#F2D398', onSecondary: '#3E2A08', accent: '#E07A3A', surface: '#FAF1E6', onSurface: '#2A211C', background: '#FFFFFF', text: '#2A211C', link: '#7A3420' },
    { id: 'arctic', name: 'Arctic', primary: '#1C3D6E', onPrimary: '#FFFFFF', secondary: '#BFE3F2', onSecondary: '#0D2B45', accent: '#FF8C42', surface: '#EFF6FA', onSurface: '#18212D', background: '#FFFFFF', text: '#18212D', link: '#1C3D6E' },
  ],
  icons: {
    start: 'compass', syllabus: 'map', modules: 'route', assignments: 'backpack', discussions: 'messages-square',
    grades: 'medal', help: 'life-buoy', instructor: 'tent', calendar: 'calendar-days', resources: 'binoculars',
  },
  art: {
    medium: 'Glossy die-cut vinyl sticker illustrations with thick white outlines, a soft drop shadow and bold flat colours',
    palette: 'Forest teal, sunset orange and golden yellow with sky blue on light kraft paper',
    mood: 'Adventurous, collectable and upbeat',
    banner: 'A cluster of large overlapping adventure stickers – a compass, binoculars, a backpack, a pine tree and a tent – on the right, plain light kraft paper on the left',
    bannerAlt: 'Three large stickers of a mountain peak, a canoe and a campfire in a loose row on the right, barely overlapping, each fully inside the picture with kraft paper showing around it, plain light kraft paper on the left',
    welcome: { subject: 'A single sticker of a hiking backpack', alt: 'A backpack sticker' },
    instructor: { subject: 'A single sticker of a steaming enamel camp mug', alt: 'A camp mug sticker' },
    cards: [
      'A compass sticker on kraft paper',
      'A pair of binoculars sticker on kraft paper',
      'A sticker of a tent beneath two pine trees on kraft paper',
    ],
  },
});
