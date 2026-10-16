import { defineTheme } from '../../define';

export default defineTheme({
  id: 'deco-skyline',
  name: 'Deco Skyline',
  description: 'Art Deco architecture in gold foil line on deep navy: stepped towers, sunburst crowns and searchlights. Glamour with a strict grid.',
  audiences: ['secondary', 'higher-ed', 'professional'],
  tags: ['art-deco', 'architecture', 'gold-foil', 'geometric', '1920s'],
  defaultLayout: 'classic-band',
  iconStyle: 'duotone',
  corners: 'square',
  collection: 'signature',
  premium: true,
  style: 'Art Deco gold-foil architectural illustration',
  colourBlind:
    'Gold on navy works by lightness; the Emerald Room colourway pairs green with gold only, never with red, so nothing depends on telling red from green.',
  palettes: [
    { id: 'navy-gold', name: 'Midnight Gold', primary: '#14213D', onPrimary: '#FFFFFF', secondary: '#E9D8A6', onSecondary: '#2A2110', accent: '#C9A227', surface: '#F5F2E8', onSurface: '#141A2A', background: '#FFFFFF', text: '#141A2A', link: '#23407A' },
    { id: 'emerald-room', name: 'Emerald Room', primary: '#0F3B33', onPrimary: '#FFFFFF', secondary: '#E6D9A8', onSecondary: '#2A2410', accent: '#C9A227', surface: '#EFF5F2', onSurface: '#13241F', background: '#FFFFFF', text: '#13241F', link: '#11594A' },
    { id: 'onyx-rose', name: 'Onyx & Rose Gold', primary: '#1E1B1F', onPrimary: '#FFFFFF', secondary: '#EED7CC', onSecondary: '#3A2620', accent: '#C48B6E', surface: '#F7F2EF', onSurface: '#1A1618', background: '#FFFFFF', text: '#1A1618', link: '#7A3E2E' },
  ],
  icons: {
    start: 'sparkles', syllabus: 'scroll-text', modules: 'building-2', assignments: 'pen-tool', discussions: 'messages-square',
    grades: 'award', help: 'circle-help', instructor: 'lamp-desk', calendar: 'calendar-days', resources: 'landmark',
  },
  art: {
    medium: 'Art Deco architectural illustration in crisp metallic gold foil line work on a deep navy ground that runs off every edge, stepped setbacks, sunburst fans and precise geometric ornament',
    palette: 'Deep navy and burnished gold with ivory highlights',
    mood: 'Glamorous, ordered and optimistic',
    banner: 'A soaring stepped Art Deco skyscraper with a gold sunburst crown and fanning searchlight beams, flanked by two smaller towers, on the right, plain deep navy night sky on the left',
    bannerAlt: 'A grand Art Deco arched doorway of gold sunburst fans and fluted pilasters seen straight on at the right, a smooth plain navy wall continuing to the left',
    welcome: { subject: 'A gold Art Deco sunburst fan', alt: '' },
    instructor: { subject: 'A streamlined Art Deco desk lamp with a fluted gold shade', alt: 'An Art Deco desk lamp' },
    cards: [
      'The spire of an Art Deco tower with a gold sunburst crown',
      'A geometric gold fountain with stepped basins',
      'The bow of an ocean liner drawn in streamlined gold lines',
    ],
  },
});
