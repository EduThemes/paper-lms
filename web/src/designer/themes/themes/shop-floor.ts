import { defineTheme, FONTS } from '../define';

export default defineTheme({
  id: 'shop-floor',
  name: 'Shop Floor',
  description: 'Bold screen-printed workshop art of wrenches, gears and hard hats. Built for CTE and vocational programmes.',
  audiences: ['specialty', 'secondary'],
  tags: ['cte', 'vocational', 'workshop', 'tools'],
  defaultLayout: 'sidebar',
  iconStyle: 'soft-square',
  headingFont: FONTS.verdana,
  corners: 'square',
  style: 'screen-printed workshop graphics',
  colourBlind:
    'Safety orange against steel blue is a colour-blind-safe pair; yellow highlights always carry dark text and never sit against green.',
  palettes: [
    { id: 'steel', name: 'Steel & Orange', primary: '#1F3446', onPrimary: '#FFFFFF', secondary: '#FFB44A', onSecondary: '#3A2000', accent: '#E86A1F', surface: '#F0F2F4', onSurface: '#1A1E22', background: '#FFFFFF', text: '#1A1E22', link: '#1F4A70' },
    { id: 'hazard', name: 'Hazard', primary: '#262626', onPrimary: '#FFFFFF', secondary: '#FFD43B', onSecondary: '#262626', accent: '#3A86FF', surface: '#F4F4F2', onSurface: '#1A1A1A', background: '#FFFFFF', text: '#1A1A1A', link: '#1F4FB0' },
    { id: 'copper', name: 'Copper Works', primary: '#5A2C17', onPrimary: '#FFFFFF', secondary: '#C9DCEA', onSecondary: '#1F3446', accent: '#F08A3E', surface: '#F7F1EC', onSurface: '#261A14', background: '#FFFFFF', text: '#261A14', link: '#7A3A1A' },
  ],
  icons: {
    start: 'hard-hat', syllabus: 'clipboard-list', modules: 'layers', assignments: 'wrench', discussions: 'messages-square',
    grades: 'gauge', help: 'circle-help', instructor: 'hammer', calendar: 'calendar-days', resources: 'toolbox',
  },
  art: {
    medium: 'Bold screen-printed workshop illustration in flat shapes with heavy outlines and a rough ink texture',
    palette: 'Safety orange and steel blue with graphite on concrete grey',
    mood: 'Practical, capable and proud of the craft',
    banner: 'A large wrench, a claw hammer, a gear and a hard hat hanging on a pegboard on the right, a plain concrete-grey wall on the left',
    bannerAlt: 'A sturdy workbench with a bench vice, a coiled cable and a big cog wheel on the right, a plain concrete-grey wall on the left',
    welcome: { subject: 'A safety hard hat', alt: 'A hard hat' },
    instructor: { subject: 'A metal toolbox with a wrench resting on top', alt: 'A toolbox with a wrench' },
    cards: [
      'A gear and a spanner crossed',
      'A cordless drill standing upright',
      'A stack of timber planks with a handsaw',
    ],
  },
});
