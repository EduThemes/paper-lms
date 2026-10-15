import { defineTheme } from '../../define';

export default defineTheme({
  id: 'platform-hop',
  name: 'Platform Hop',
  description: 'A painted side-scrolling world of floating grassy ledges, spring pads and gold stars, with a little frog mid-hop. Bouncy and bright for K-5.',
  audiences: ['early-elementary', 'upper-elementary'],
  tags: ['platformer', 'side-scroller', 'gouache', 'nature', 'picture-first'],
  defaultLayout: 'playful',
  iconStyle: 'solid-circle',
  corners: 'round',
  collection: 'arcade',
  style: 'painterly side-scroller backdrop',
  colourBlind:
    'Grass green and coral appear together only in the scenery; bands are deep green, blue or brick with light gold, sky or peach, so lightness carries everything.',
  palettes: [
    { id: 'grass-hill', name: 'Grass Hill', primary: '#1F4D2A', onPrimary: '#FFFFFF', secondary: '#FFD866', onSecondary: '#3A2A00', accent: '#FF7A59', surface: '#F0F8EA', onSurface: '#172617', background: '#FFFFFF', text: '#172617', link: '#24603A' },
    { id: 'sky-hop', name: 'Sky Hop', primary: '#154A7A', onPrimary: '#FFFFFF', secondary: '#BFE6FF', onSecondary: '#0E2E4A', accent: '#FFB703', surface: '#EDF6FD', onSurface: '#12233A', background: '#FFFFFF', text: '#12233A', link: '#155A94' },
    { id: 'coral-cove', name: 'Coral Cove', primary: '#7A2E2A', onPrimary: '#FFFFFF', secondary: '#FFD2C2', onSecondary: '#4A1A14', accent: '#3FB7C4', surface: '#FFF2EE', onSurface: '#2A1614', background: '#FFFFFF', text: '#2A1614', link: '#9A3A30' },
  ],
  icons: {
    start: 'footprints', syllabus: 'route', modules: 'layers', assignments: 'pencil', discussions: 'message-circle',
    grades: 'star', help: 'circle-help', instructor: 'lamp', calendar: 'calendar-days', resources: 'backpack',
  },
  art: {
    medium: 'Painterly side-scrolling game backdrop in soft gouache with flat parallax layers, rounded shapes and gentle brush texture',
    palette: 'Bright grass green, sunny gold and sky blue with coral on soft cream',
    mood: 'Bouncy, sunny and inviting',
    banner: 'Floating grassy ledges stepping upward like stairs, a coiled spring pad and a trail of gold stars, with a small round frog mid-hop between ledges, on the right, layered soft hills and open sky on the left',
    bannerAlt: 'A rope bridge between two floating rock islands with a little waterfall and puffy bouncing clouds on the right, gentle parallax hills fading into open sky on the left',
    welcome: { subject: 'A small floating grassy ledge with a gold star above it', alt: 'A floating ledge with a star' },
    instructor: { subject: 'A glowing golden lantern on a floating stone ledge', alt: 'A lantern on a floating ledge' },
    cards: [
      'A small round frog leaping between lily pad ledges',
      'A cave mouth glowing with crystals on a grassy ledge',
      'A hot-air balloon lifting off a floating island',
    ],
  },
});
