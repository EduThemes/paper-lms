import { defineTheme, FONTS } from '../define';

export default defineTheme({
  id: 'terrazzo',
  name: 'Terrazzo',
  description: 'Speckled terrazzo, arched niches and ceramic still lifes in warm clay and ink blue. A calm studio feel for adult learners.',
  audiences: ['professional', 'higher-ed'],
  tags: ['terrazzo', 'still-life', 'warm', 'architectural'],
  defaultLayout: 'sidebar',
  iconStyle: 'soft-square',
  headingFont: FONTS.trebuchet,
  corners: 'round',
  style: 'terrazzo still life',
  colourBlind:
    'Clay and rose are decoration only; text sits on ink blue or stone white, so meaning survives red-green colour blindness.',
  palettes: [
    { id: 'clay', name: 'Clay & Ink', primary: '#2A3550', onPrimary: '#FFFFFF', secondary: '#F0D8C8', onSecondary: '#5A2F1E', accent: '#C9714F', surface: '#F7F2EE', onSurface: '#22232A', background: '#FFFFFF', text: '#22232A', link: '#2E4A7A' },
    { id: 'olive', name: 'Olive Grove', primary: '#3C4128', onPrimary: '#FFFFFF', secondary: '#E8E2C8', onSecondary: '#3C4128', accent: '#C98E4A', surface: '#F6F5EE', onSurface: '#22241A', background: '#FFFFFF', text: '#22241A', link: '#3F5A2A' },
    { id: 'rose', name: 'Rose Stone', primary: '#5C2A3A', onPrimary: '#FFFFFF', secondary: '#F6DDE0', onSecondary: '#5C2A3A', accent: '#8FA3B8', surface: '#FBF3F3', onSurface: '#2A1C21', background: '#FFFFFF', text: '#2A1C21', link: '#7A2E48' },
  ],
  icons: {
    start: 'door-open', syllabus: 'file-text', modules: 'layout-grid', assignments: 'clipboard-check', discussions: 'messages-square',
    grades: 'chart-column', help: 'circle-help', instructor: 'coffee', calendar: 'calendar-days', resources: 'folder-open',
  },
  art: {
    medium: 'Modern architectural still-life illustration with speckled terrazzo stone textures, rounded arches and soft natural shadows',
    palette: 'Warm clay and dusty rose with deep ink blue on stone white',
    mood: 'Calm, grounded and gently elegant',
    banner: 'A large terrazzo plinth, a tall arched niche and a ceramic vase with a single palm leaf on the right, a plain stone-white wall on the left',
    bannerAlt: 'Large stepped arches and a round terrazzo side table holding a ceramic cup on the right, a plain stone-white wall on the left',
    welcome: { subject: 'A ceramic vase holding a single palm leaf', alt: '' },
    instructor: { subject: 'A ceramic coffee cup on a round terrazzo coaster', alt: 'A coffee cup on a terrazzo coaster' },
    cards: [
      'A terrazzo arch standing on a clay-coloured floor',
      'Stacked stone spheres balanced on a terrazzo cylinder',
      'A small potted olive tree inside an arched niche',
    ],
  },
});
