import { defineTheme } from '../../define';

export default defineTheme({
  id: 'fairground',
  name: 'Fairground',
  description: 'Candy-striped carousels, a twinkling wheel and bunches of balloons at dusk. All the fun of the fair, with painted wooden horses instead of performing animals.',
  audiences: ['early-elementary', 'upper-elementary'],
  tags: ['fairground', 'carousel', 'celebration', 'bright', 'candy-stripe'],
  defaultLayout: 'playful',
  iconStyle: 'solid-circle',
  corners: 'round',
  collection: 'cartoons',
  style: 'vintage fairground cartoon',
  colourBlind:
    'Raspberry and navy primaries sit on pale yellow or pink, a lightness contrast; the cherry-and-cream stripes are pattern only.',
  palettes: [
    { id: 'candy-stripe', name: 'Candy Stripe', primary: '#8A1F4A', onPrimary: '#FFFFFF', secondary: '#FFD25E', onSecondary: '#3A2A00', accent: '#4FB3E8', surface: '#FFF1F5', onSurface: '#2A1620', background: '#FFFFFF', text: '#2A1620', link: '#8A1F4A' },
    { id: 'bluebird', name: 'Bluebird', primary: '#1F4580', onPrimary: '#FFFFFF', secondary: '#FFC2D6', onSecondary: '#5A0F38', accent: '#FFD25E', surface: '#EEF4FC', onSurface: '#15203A', background: '#FFFFFF', text: '#15203A', link: '#1F4580' },
    { id: 'twilight', name: 'Twilight Rides', primary: '#2A1F5C', onPrimary: '#FFFFFF', secondary: '#FFE08A', onSecondary: '#3A2A00', accent: '#FF6FA8', surface: '#F3F0FC', onSurface: '#1B1733', background: '#FFFFFF', text: '#1B1733', link: '#3D2D8A' },
  ],
  icons: {
    start: 'ticket', syllabus: 'list', modules: 'ferris-wheel', assignments: 'pencil', discussions: 'message-circle',
    grades: 'star', help: 'circle-help', instructor: 'party-popper', calendar: 'calendar', resources: 'gift',
  },
  art: {
    medium:
      'Bright vintage fairground cartoon illustration with candy-stripe patterns, glossy painted wood, twinkling bulb lights and a soft dusk glow',
    palette: 'Candy pink, sky blue and sunshine yellow with cherry red and cream stripes',
    mood: 'Joyful, twinkly and festive, the best evening of the summer',
    banner:
      'A big carousel with painted wooden horses under a striped canopy, a twinkling wheel ride behind it and a bunch of balloons on the right, soft open evening sky across the left',
    bannerAlt:
      'A tall striped helter-skelter slide and a swing ride with flying chairs glowing with bulb lights on the right, calm open dusk sky across the left',
    welcome: { subject: 'A spinning paper pinwheel in candy stripes', alt: 'A candy-striped pinwheel' },
    instructor: { subject: 'An empty striped straw boater hat resting on a bench beside a balloon, nobody wearing it', alt: 'A striped boater hat and a balloon' },
    cards: [
      'A painted wooden carousel horse on a twisted brass pole',
      'A big wheel ride glowing with bulb lights at dusk',
      'A bunch of shiny balloons tied to a striped post, the fairground scene filling the picture right to all four edges with no white paper edge, vignette or border',
    ],
  },
});
