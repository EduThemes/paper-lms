import { defineTheme } from '../define';

export default defineTheme({
  id: 'circuit-garden',
  name: 'Circuit Garden',
  description: 'Circuit-board traces that grow like vines, with glowing nodes and leaf shapes. For digital literacy and computing.',
  audiences: ['specialty', 'upper-elementary', 'secondary'],
  tags: ['digital-literacy', 'computing', 'technology', 'growth'],
  defaultLayout: 'card-grid',
  iconStyle: 'duotone',
  corners: 'soft',
  style: 'circuit-trace botanicals',
  colourBlind:
    'Mint and yellow never carry meaning on their own; any online/offline style state also needs an icon.',
  palettes: [
    { id: 'navy', name: 'Night Circuit', primary: '#13234A', onPrimary: '#FFFFFF', secondary: '#B8F2DC', onSecondary: '#0C3A2A', accent: '#FFD23F', surface: '#EEF2FA', onSurface: '#141B2E', background: '#FFFFFF', text: '#141B2E', link: '#24459A' },
    { id: 'grape', name: 'Grape Chip', primary: '#3A1F5D', onPrimary: '#FFFFFF', secondary: '#FFE08A', onSecondary: '#3A2A00', accent: '#4FD1C5', surface: '#F4F0FA', onSurface: '#1F1A2A', background: '#FFFFFF', text: '#1F1A2A', link: '#4E2A80' },
    { id: 'daylight', name: 'Daylight', primary: '#0E4F5C', onPrimary: '#FFFFFF', secondary: '#FFD9B8', onSecondary: '#4A2200', accent: '#7C5CFF', surface: '#EFF7F8', onSurface: '#14252A', background: '#FFFFFF', text: '#14252A', link: '#0E5566' },
  ],
  icons: {
    start: 'power', syllabus: 'clipboard-list', modules: 'cpu', assignments: 'code', discussions: 'messages-square',
    grades: 'badge-check', help: 'circle-help', instructor: 'bot', calendar: 'calendar-days', resources: 'shield-check',
  },
  art: {
    medium: 'Flat vector illustration of circuit-board traces growing like plants, with small glowing round nodes and rounded leaf shapes',
    palette: 'Deep navy and electric mint with sunny yellow nodes',
    mood: 'Curious, hopeful and safe, technology that grows',
    banner: 'Large circuit traces branching into vines and leaves around a glowing light-bulb-shaped flower on the right, plain smooth navy space on the left',
    bannerAlt: 'A large tree whose roots and branches are circuit traces, with a padlock and a shield sprouting among its leaves on the right, plain smooth navy space on the left',
    welcome: { subject: 'A seedling made of circuit traces with two glowing leaf nodes', alt: 'A seedling made of circuit traces' },
    instructor: { subject: 'A computer mouse whose cable curls into a circuit-trace vine', alt: 'A computer mouse with a circuit-vine cable' },
    cards: [
      'A padlock surrounded by circuit-trace leaves',
      'A cloud shape with circuit-trace roots',
      'A light bulb filled with circuit vines',
    ],
  },
});
