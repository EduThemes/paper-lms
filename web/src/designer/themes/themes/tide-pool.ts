import { defineTheme } from '../define';

export default defineTheme({
  id: 'tide-pool',
  name: 'Tide Pool',
  description: 'Soft gouache ocean layers: sea turtles, starfish and lighthouses in translucent blues and coral.',
  audiences: ['upper-elementary', 'early-elementary'],
  tags: ['ocean', 'gouache', 'layers', 'calm'],
  defaultLayout: 'classic-band',
  iconStyle: 'solid-circle',
  corners: 'round',
  style: 'layered ocean gouache',
  colourBlind:
    'Aqua and coral differ strongly in lightness, and the deep-blue band holds white text whatever a reader’s colour vision.',
  palettes: [
    { id: 'lagoon', name: 'Lagoon', primary: '#0B3C5D', onPrimary: '#FFFFFF', secondary: '#9FE3E0', onSecondary: '#08343A', accent: '#FF7F6B', surface: '#EAF6F8', onSurface: '#13212B', background: '#FFFFFF', text: '#13212B', link: '#0B4A70' },
    { id: 'reef', name: 'Coral Reef', primary: '#8A3324', onPrimary: '#FFFFFF', secondary: '#FFE0C2', onSecondary: '#4A2010', accent: '#3FB8AF', surface: '#FFF4EE', onSurface: '#2A1E1A', background: '#FFFFFF', text: '#2A1E1A', link: '#8A3324' },
    { id: 'deep', name: 'Deep Sea', primary: '#10204A', onPrimary: '#FFFFFF', secondary: '#7FD1E6', onSecondary: '#082A36', accent: '#F6C445', surface: '#EEF2FA', onSurface: '#141A2B', background: '#FFFFFF', text: '#141A2B', link: '#1D3A7A' },
  ],
  icons: {
    start: 'anchor', syllabus: 'scroll-text', modules: 'waves', assignments: 'pencil', discussions: 'messages-square',
    grades: 'shell', help: 'life-buoy', instructor: 'sailboat', calendar: 'calendar-days', resources: 'fish',
  },
  art: {
    medium: 'Soft gouache illustration built from stacked translucent layers, smooth brushed gradients and gentle depth',
    palette: 'Deep ocean blue and aqua with coral pink and sandy beige',
    mood: 'Calm, curious and sunlit, like peering into clear shallow water',
    banner: 'Layered ocean waves curling toward the right where a large sea turtle, coral branches and a few fish gather, a calm open water surface and pale sky on the left',
    bannerAlt: 'A rock pool with a big starfish, sea anemones and a small crab on the right, calm shallow sandy water on the left',
    welcome: { subject: 'A swimming sea turtle', alt: 'A sea turtle swimming' },
    instructor: { subject: 'A striped lighthouse on a small rock', alt: 'A lighthouse on a rock' },
    cards: [
      'A starfish on rippled sand',
      'A jellyfish drifting in blue water',
      'A spiral seashell resting on wet sand',
    ],
  },
});
