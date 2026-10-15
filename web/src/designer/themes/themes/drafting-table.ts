import { defineTheme, FONTS } from '../define';

export default defineTheme({
  id: 'drafting-table',
  name: 'Drafting Table',
  description: 'Cyanotype blueprint linework of gears, bridges and drafting tools. For engineering, architecture and the trades.',
  audiences: ['higher-ed', 'specialty'],
  tags: ['blueprint', 'engineering', 'technical', 'cte'],
  defaultLayout: 'sidebar',
  iconStyle: 'outline',
  headingFont: FONTS.verdana,
  corners: 'square',
  style: 'blueprint technical drawing',
  colourBlind:
    'Prussian blue and safety orange are the only hues, and they stay apart for every common colour-vision type.',
  palettes: [
    { id: 'prussian', name: 'Prussian Blue', primary: '#12355B', onPrimary: '#FFFFFF', secondary: '#D6E4F0', onSecondary: '#12355B', accent: '#F28C28', surface: '#EEF3F8', onSurface: '#14202E', background: '#FFFFFF', text: '#14202E', link: '#12457A' },
    { id: 'graphite', name: 'Graphite', primary: '#2B2F36', onPrimary: '#FFFFFF', secondary: '#FFE2C2', onSecondary: '#4A2600', accent: '#F28C28', surface: '#F2F3F5', onSurface: '#1A1D22', background: '#FFFFFF', text: '#1A1D22', link: '#1F4E8C' },
    { id: 'copper', name: 'Copper', primary: '#5A2E1A', onPrimary: '#FFFFFF', secondary: '#E2EEF6', onSecondary: '#12355B', accent: '#2F86C9', surface: '#F8F2EC', onSurface: '#261A14', background: '#FFFFFF', text: '#261A14', link: '#7A3A16' },
  ],
  icons: {
    start: 'drafting-compass', syllabus: 'file-text', modules: 'layers', assignments: 'pencil-ruler', discussions: 'messages-square',
    grades: 'chart-column', help: 'circle-help', instructor: 'hard-hat', calendar: 'calendar-days', resources: 'ruler',
  },
  art: {
    medium: 'Precise technical line drawing in the style of a cyanotype blueprint, crisp even line weights with faint construction lines and no dimension marks',
    palette: 'Deep Prussian blue and crisp white with a touch of safety orange',
    mood: 'Exact, capable and quietly impressive',
    banner: 'White line work on a deep Prussian blue ground that runs off every edge, with no sheet border, margin line or corner stamp: a large exploded view of interlocking gears, a bridge truss and a drafting compass on the right, a plain empty blueprint field with a faint grid on the left',
    bannerAlt: 'White line work on a deep Prussian blue ground that runs off every edge, with no sheet border, margin line or corner stamp: a large side elevation of an arched bridge with a crane arm above it on the right, a plain empty blueprint field with a faint grid on the left',
    welcome: { subject: 'Prussian blue line work on plain white: a drafting compass standing open', alt: 'A drafting compass' },
    instructor: { subject: 'Prussian blue line work on plain white: an articulated drafting lamp', alt: 'An articulated drafting lamp' },
    cards: [
      'White line work on a deep Prussian blue ground that runs off every edge, with no sheet border, margin line or corner stamp: a single large gear with a smaller gear meshing into it',
      'White line work on a deep Prussian blue ground that runs off every edge, with no sheet border, margin line or corner stamp: a stone arched bridge',
      'White line work on a deep Prussian blue ground that runs off every edge, with no sheet border, margin line or corner stamp: the timber frame of a small house',
    ],
  },
});
