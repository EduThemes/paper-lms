import { defineTheme } from '../../define';

export default defineTheme({
  id: 'moss-and-stone',
  name: 'Moss & Stone',
  description: 'A Japanese dry garden in soft overcast light: raked gravel, velvet moss, weathered boulders and one red maple. Calm made visible.',
  audiences: ['professional', 'higher-ed', 'specialty'],
  tags: ['garden', 'photography', 'calm', 'mindfulness', 'counseling', 'sel', 'wellbeing'],
  defaultLayout: 'editorial',
  iconStyle: 'outline',
  corners: 'soft',
  collection: 'signature',
  premium: true,
  style: 'fine-art garden photography',
  colourBlind:
    'Moss green and maple red can merge for red-green colour-blind readers, so maple is a small decorative note only; gravel, stone and moss differ in lightness.',
  palettes: [
    { id: 'moss', name: 'Moss', primary: '#2E4A2C', onPrimary: '#FFFFFF', secondary: '#DDE6D2', onSecondary: '#233A22', accent: '#B5473A', surface: '#F2F5EE', onSurface: '#1B2619', background: '#FFFFFF', text: '#1B2619', link: '#31582F' },
    { id: 'granite', name: 'Granite', primary: '#33393D', onPrimary: '#FFFFFF', secondary: '#E1E4E2', onSecondary: '#2A2F33', accent: '#6E8B5E', surface: '#F3F4F3', onSurface: '#1D2124', background: '#FFFFFF', text: '#1D2124', link: '#3C5560' },
    { id: 'autumn-maple', name: 'Autumn Maple', primary: '#5A2A22', onPrimary: '#FFFFFF', secondary: '#F0DDD4', onSecondary: '#47201A', accent: '#6E8B5E', surface: '#F9F2EE', onSurface: '#2A1A16', background: '#FFFFFF', text: '#2A1A16', link: '#8A3A2A' },
  ],
  icons: {
    start: 'footprints', syllabus: 'scroll-text', modules: 'layers', assignments: 'pen-line', discussions: 'messages-square',
    grades: 'award', help: 'heart-handshake', instructor: 'leaf', calendar: 'calendar-days', resources: 'library',
  },
  art: {
    medium: 'Serene fine-art garden photography with soft overcast light, shallow depth of field and a muted natural colour grade',
    palette: 'Moss green, pale raked-gravel grey and weathered stone with a touch of maple red',
    mood: 'Still, balanced and restorative',
    banner: 'Three weathered boulders on a velvety moss island rising from pale gravel raked in concentric rings on the right, smooth evenly raked gravel lines continuing calmly across the left',
    bannerAlt: 'A small maple with red leaves leaning over moss-covered stepping stones and a bamboo spout dripping into a stone basin on the right, soft evenly raked gravel stretching to the left',
    welcome: { subject: 'A smooth stone resting on a cushion of moss', alt: '' },
    instructor: { subject: 'A bamboo rake resting on freshly raked gravel', alt: 'A bamboo rake on raked gravel' },
    cards: [
      'A moss-covered stone basin with a floating maple leaf',
      'Stepping stones crossing a bed of moss',
      'A single boulder in gravel raked in rings',
    ],
  },
});
