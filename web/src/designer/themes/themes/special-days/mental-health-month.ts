import { defineTheme, FONTS } from '../../define';

export default defineTheme({
  id: 'mental-health-month',
  name: 'Mental Health Awareness',
  description: 'Gentle matte gouache: a sunny windowsill of plants, a parting cloud and a green awareness ribbon. Quiet and kind for counselling and advisory pages.',
  audiences: ['specialty', 'secondary', 'higher-ed'],
  tags: ['awareness', 'mental-health', 'wellbeing', 'counseling', 'sel', 'gouache'],
  collection: 'special-days',
  observance: {
    name: 'Mental Health Awareness Month',
    traditions: [],
    months: [5, 10],
    note: 'US: May. UK: Mental Health Awareness Week, in May. World Mental Health Day: October 10. Link your counsellor and a local crisis line.',
  },
  defaultLayout: 'sidebar',
  iconStyle: 'outline',
  headingFont: FONTS.trebuchet,
  corners: 'round',
  style: 'gentle matte gouache',
  colourBlind:
    'Lime green is only the awareness-ribbon accent; structure comes from deep colour on pale panels, and moods are never shown as red versus green.',
  palettes: [
    { id: 'ribbon', name: 'Green Ribbon', primary: '#24503E', onPrimary: '#FFFFFF', secondary: '#DDF2B8', onSecondary: '#2A3D12', accent: '#F6B89A', surface: '#F3F9F0', onSurface: '#1A2621', background: '#FFFFFF', text: '#1A2621', link: '#236048' },
    { id: 'mist', name: 'Misty Blue', primary: '#24466A', onPrimary: '#FFFFFF', secondary: '#DCE9F5', onSecondary: '#24466A', accent: '#F7C8A8', surface: '#F3F7FB', onSurface: '#18222E', background: '#FFFFFF', text: '#18222E', link: '#275585' },
    { id: 'peach', name: 'Soft Peach', primary: '#6A3B2C', onPrimary: '#FFFFFF', secondary: '#FDE1D3', onSecondary: '#5A2E20', accent: '#A9D8C6', surface: '#FFF6F1', onSurface: '#2A1E1A', background: '#FFFFFF', text: '#2A1E1A', link: '#84452F' },
  ],
  icons: {
    start: 'sprout', syllabus: 'clipboard-list', modules: 'layers', assignments: 'notebook-pen', discussions: 'message-circle-heart',
    grades: 'sun', help: 'life-buoy', instructor: 'heart-handshake', calendar: 'calendar-days', resources: 'hand-heart',
  },
  art: {
    medium: 'Gentle matte gouache illustration with soft pastel colour, rounded shapes and lots of breathing room',
    palette: 'Soft lime green and misty blue with warm peach and a little deep teal',
    mood: 'Gentle, hopeful and kind',
    banner: 'A sunny windowsill lined with thriving potted plants and a cup of tea, soft light streaming through a tall window on the right, a calm plain pale wall on the left',
    bannerAlt: 'A small grey cloud parting to let a gentle sun shine on a hill of new green shoots on the right, soft open sky on the left',
    welcome: { subject: 'A lime-green awareness ribbon tied in a loop', alt: 'A green awareness ribbon' },
    instructor: { subject: 'A folded knitted blanket on a chair with a cup of tea beside it', alt: 'A blanket and a cup of tea' },
    cards: [
      'A watering can watering a sprouting plant',
      'A cosy window seat with cushions on a rainy day',
      'A quiet forest path with soft light between the trees',
    ],
  },
});
