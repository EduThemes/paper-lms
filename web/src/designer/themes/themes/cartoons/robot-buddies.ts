import { defineTheme } from '../../define';

export default defineTheme({
  id: 'robot-buddies',
  name: 'Robot Buddies',
  description: 'Glossy wind-up tin robots with rivets and round antennae, watering plants and walking robot dogs. Retro toy-shop charm.',
  audiences: ['upper-elementary', 'early-elementary'],
  tags: ['robots', 'tin-toy', 'retro', 'stem', 'friendly'],
  defaultLayout: 'card-grid',
  iconStyle: 'soft-square',
  corners: 'soft',
  collection: 'cartoons',
  style: 'retro tin-toy painting',
  colourBlind:
    'Navy and pale robin\'s-egg blue carry the structure through lightness; cherry red is a decorative enamel colour and never the only cue.',
  palettes: [
    { id: 'tin-toy', name: 'Tin Toy', primary: '#1F3F66', onPrimary: '#FFFFFF', secondary: '#A8DDE8', onSecondary: '#0D3340', accent: '#E23D3D', surface: '#F1F7F9', onSurface: '#15202B', background: '#FFFFFF', text: '#15202B', link: '#1F4E80' },
    { id: 'cherry', name: 'Cherry Enamel', primary: '#8A1C24', onPrimary: '#FFFFFF', secondary: '#FFE08A', onSecondary: '#3A2A00', accent: '#6EC6D9', surface: '#FFF3EF', onSurface: '#261416', background: '#FFFFFF', text: '#261416', link: '#8A1C24' },
    { id: 'chrome', name: 'Chrome', primary: '#2E3440', onPrimary: '#FFFFFF', secondary: '#C9F0A8', onSecondary: '#20380F', accent: '#FFB347', surface: '#F2F4F7', onSurface: '#1A1E26', background: '#FFFFFF', text: '#1A1E26', link: '#2E4A7A' },
  ],
  icons: {
    start: 'power', syllabus: 'list-checks', modules: 'cog', assignments: 'wrench', discussions: 'message-circle',
    grades: 'star', help: 'life-buoy', instructor: 'bot', calendar: 'calendar-days', resources: 'battery-charging',
  },
  art: {
    medium:
      'Retro tin-toy illustration of wind-up robots with glossy enamel paint, plain unprinted metal panels, rivets and soft chrome reflections against a gently painted backdrop',
    palette: "Robin's-egg blue and cherry red with polished silver on warm cream",
    mood: 'Cheerful, clanky and helpful, best friends made of tin',
    banner:
      'A tall friendly tin robot with a round antenna and a small wind-up robot with a key in its back standing together beside a pile of brass gears on the right, plain calm cream backdrop across the left',
    bannerAlt:
      'A round tin robot riding a little wind-up rocket over soft painted clouds on the right, plain calm cream sky across the left',
    welcome: { subject: 'A small round tin robot with a glowing antenna', alt: 'A small tin robot' },
    instructor: { subject: 'A little red toolbox with a wrench, a screwdriver and spare brass gears', alt: 'A toolbox with gears and a wrench' },
    cards: [
      'A tin robot watering a small potted plant',
      'A wind-up robot dog with a springy tail',
      'A tin robot balancing a stack of shiny gears',
    ],
  },
});
