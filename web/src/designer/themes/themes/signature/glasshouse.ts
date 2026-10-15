import { defineTheme } from '../../define';

export default defineTheme({
  id: 'glasshouse',
  name: 'Glasshouse',
  description: 'A sunlit Victorian conservatory in fine ink and watercolour: white ironwork, palms, ferns and water lilies under glass.',
  audiences: ['higher-ed', 'secondary', 'upper-elementary'],
  tags: ['botanical', 'victorian', 'conservatory', 'watercolour', 'science', 'nature'],
  defaultLayout: 'split-hero',
  iconStyle: 'outline',
  corners: 'soft',
  collection: 'signature',
  premium: true,
  style: 'Victorian botanical ink and watercolour',
  colourBlind:
    'Fern green and orchid pink are the two colours a red-green colour-blind reader could confuse, so orchid is decoration only; white ironwork and lightness carry the structure.',
  palettes: [
    { id: 'fern-house', name: 'Fern House', primary: '#23452F', onPrimary: '#FFFFFF', secondary: '#DCEBD8', onSecondary: '#1E3A28', accent: '#E39BB5', surface: '#F2F7F0', onSurface: '#182A1E', background: '#FFFFFF', text: '#182A1E', link: '#2A5A3A' },
    { id: 'wrought-iron', name: 'Wrought Iron', primary: '#263238', onPrimary: '#FFFFFF', secondary: '#DCE6EA', onSecondary: '#1F2A30', accent: '#8FBF7A', surface: '#F2F5F6', onSurface: '#161E22', background: '#FFFFFF', text: '#161E22', link: '#2F5566' },
    { id: 'orchid', name: 'Orchid', primary: '#4E2A4A', onPrimary: '#FFFFFF', secondary: '#F1DDEB', onSecondary: '#3E1E3A', accent: '#7FB07A', surface: '#FAF2F7', onSurface: '#24182A', background: '#FFFFFF', text: '#24182A', link: '#6E3366' },
  ],
  icons: {
    start: 'sprout', syllabus: 'scroll-text', modules: 'layers', assignments: 'pen-line', discussions: 'messages-square',
    grades: 'award', help: 'circle-help', instructor: 'flower-2', calendar: 'calendar-days', resources: 'library',
  },
  art: {
    medium: 'Luminous Victorian botanical illustration in fine ink line and layered transparent watercolour, white wrought-iron tracery and sunlit glass',
    palette: 'Fern green, white ironwork and soft sky blue with touches of orchid pink',
    mood: 'Airy, abundant and calm, like a sunny morning under glass',
    banner: 'A grand domed glasshouse of curved white iron ribs and glass panes filled with tall palms and ferns on the right, soft pale sky and a gravel path fading into open space on the left',
    bannerAlt: 'Looking up into a curved glass roof of white iron ribs with a tall palm and hanging ferns spilling in from the right, clear pale glass and soft sky on the left',
    welcome: { subject: 'A small fern under a glass bell cloche', alt: 'A fern under a glass cloche' },
    instructor: { subject: 'A copper watering can beside a potted orchid', alt: 'A watering can and a potted orchid' },
    cards: [
      'A glass and white-iron plant case holding ferns',
      'A giant water lily pad floating in a glasshouse pond',
      'A spiral wrought-iron staircase wound with climbing vines',
    ],
  },
});
