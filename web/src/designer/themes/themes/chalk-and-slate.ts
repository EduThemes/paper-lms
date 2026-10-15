import { defineTheme, FONTS } from '../define';

export default defineTheme({
  id: 'chalk-and-slate',
  name: 'Chalk & Slate',
  description: 'Dusty chalk-pastel drawings on charcoal paper: geometric solids, paper planes and desk lamps. Classroom-classic, grown up.',
  audiences: ['secondary'],
  tags: ['chalk', 'hand-drawn', 'dark-art', 'classic'],
  defaultLayout: 'classic-band',
  iconStyle: 'outline',
  headingFont: FONTS.trebuchet,
  corners: 'square',
  style: 'chalk pastel on charcoal paper',
  colourBlind:
    'Dusty yellow and pale teal differ in lightness as well as hue; there are no red/green pairs.',
  palettes: [
    { id: 'slate', name: 'Slate', primary: '#2E3A3F', onPrimary: '#FFFFFF', secondary: '#F2E2A0', onSecondary: '#3A3000', accent: '#7CC6C0', surface: '#EEF1F0', onSurface: '#1E2426', background: '#FFFFFF', text: '#1E2426', link: '#22595A' },
    { id: 'plum', name: 'Plum Slate', primary: '#3D2B45', onPrimary: '#FFFFFF', secondary: '#F6C9B4', onSecondary: '#4A1E0E', accent: '#E9D06B', surface: '#F4EFF4', onSurface: '#231C26', background: '#FFFFFF', text: '#231C26', link: '#5A3366' },
    { id: 'blue', name: 'Blue Slate', primary: '#22344F', onPrimary: '#FFFFFF', secondary: '#CFE6F5', onSecondary: '#0E2A40', accent: '#F5A35C', surface: '#EFF3F7', onSurface: '#1A2230', background: '#FFFFFF', text: '#1A2230', link: '#22447A' },
  ],
  icons: {
    start: 'pencil', syllabus: 'list-checks', modules: 'layers', assignments: 'square-pen', discussions: 'messages-square',
    grades: 'chart-column', help: 'circle-help', instructor: 'ruler', calendar: 'calendar-days', resources: 'shapes',
  },
  art: {
    medium: 'Freehand chalk pastel illustration with dusty, textured strokes and smudged soft shading in pale chalk colours',
    palette: 'Soft chalk white, dusty yellow and pale teal against charcoal grey',
    mood: 'Thoughtful and a little nostalgic, confident rather than childish',
    banner: 'On a dark charcoal-grey paper ground, large chalk drawings of a cube, a cone and a sphere beside a sprig of leaves on the right, empty dark charcoal ground on the left',
    bannerAlt: 'On a dark charcoal-grey paper ground, a large chalk light bulb, a looping paper-airplane trail and a small potted plant on the right, empty dark charcoal ground on the left',
    welcome: { subject: 'A chalk paper airplane drawn on a small round patch of charcoal-grey paper', alt: 'A paper airplane drawn in chalk' },
    instructor: { subject: 'A chalk desk lamp drawn on a small round patch of charcoal-grey paper', alt: 'A desk lamp drawn in chalk' },
    cards: [
      'On a dark charcoal-grey paper ground, a glowing chalk light bulb',
      'On a dark charcoal-grey paper ground, chalk geometric solids casting soft shadows',
      'On a dark charcoal-grey paper ground, a chalk potted plant with broad leaves',
    ],
  },
});
