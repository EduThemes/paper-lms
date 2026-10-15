import { defineTheme } from '../../define';

export default defineTheme({
  id: 'marbled-paper',
  name: 'Marbled Paper',
  description: 'Ebru marbling in indigo, saffron and rose: combed swirls, peacock feathering and floated tulips that drift across the page.',
  audiences: ['higher-ed', 'professional', 'secondary'],
  tags: ['marbling', 'ebru', 'pattern', 'craft', 'arts', 'literature'],
  defaultLayout: 'classic-band',
  iconStyle: 'soft-square',
  corners: 'soft',
  collection: 'signature',
  premium: true,
  style: 'ebru marbled paper',
  colourBlind:
    'Indigo against saffron is a strong blue-yellow and light-dark contrast; rose and teal veins are pattern only and never mark a state.',
  palettes: [
    { id: 'indigo', name: 'Indigo & Saffron', primary: '#1E2A5A', onPrimary: '#FFFFFF', secondary: '#F4D9A0', onSecondary: '#3A2A00', accent: '#D9667F', surface: '#F5F3EE', onSurface: '#181E36', background: '#FFFFFF', text: '#181E36', link: '#2E3F8A' },
    { id: 'rose', name: 'Rose & Saffron', primary: '#5A1F3A', onPrimary: '#FFFFFF', secondary: '#F6DCC0', onSecondary: '#3E1A20', accent: '#E2A52E', surface: '#FBF1F3', onSurface: '#2A1520', background: '#FFFFFF', text: '#2A1520', link: '#8A2C58' },
    { id: 'lagoon', name: 'Lagoon', primary: '#0F4246', onPrimary: '#FFFFFF', secondary: '#CDE7E5', onSecondary: '#0F3436', accent: '#E0A43A', surface: '#EEF6F6', onSurface: '#132628', background: '#FFFFFF', text: '#132628', link: '#11616A' },
  ],
  icons: {
    start: 'droplet', syllabus: 'scroll-text', modules: 'layers', assignments: 'paintbrush', discussions: 'messages-square',
    grades: 'award', help: 'circle-help', instructor: 'palette', calendar: 'calendar-days', resources: 'library',
  },
  art: {
    medium: 'Traditional ebru marbled paper art with flowing combed swirls, feathered peacock patterns and fine veins of floating colour that run off every edge, with no border, margin or sheet edge',
    palette: 'Deep indigo, saffron and rose with teal veins on cream',
    mood: 'Rich, fluid and handcrafted',
    banner: 'Bold sweeping marbled swirls and a large feathered peacock pattern flowing across the right, the same marbling softening into broad calm drifts of pale cream with faint indigo veins on the left',
    bannerAlt: 'Large floated ebru tulips and carnations blooming out of combed marbled ripples on the right, wide quiet cream marbling with a few thin drifting veins on the left',
    welcome: { subject: 'A single floated ebru tulip in indigo and rose', alt: 'A marbled tulip' },
    instructor: { subject: 'A wooden marbling comb resting on a shallow tray of floating swirled colour', alt: 'A marbling comb on a tray of floating colour' },
    cards: [
      'A spiral of combed marbled colour',
      'A single floated ebru tulip on rippled marbling',
      'Stone-pattern marbling of round drifting drops',
    ],
  },
});
