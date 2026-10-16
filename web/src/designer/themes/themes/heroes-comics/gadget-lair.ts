import { defineTheme } from '../../define';

export default defineTheme({
  id: 'gadget-lair',
  name: 'Gadget Lair',
  description: 'A friendly, wildly over-the-top cartoon laboratory: crackling coils, bubbling flasks and a robot arm. Made for science classes that like a bit of drama.',
  audiences: ['upper-elementary', 'secondary'],
  tags: ['science', 'laboratory', 'cartoon', 'inventions', 'stem', 'energetic'],
  defaultLayout: 'card-grid',
  iconStyle: 'duotone',
  corners: 'soft',
  collection: 'heroes-comics',
  style: 'glossy cartoon lab painting',
  colourBlind:
    'Deep violet against lime is a large lightness step that holds for every type of colour blindness; magenta and cyan are glow effects only.',
  palettes: [
    { id: 'voltage', name: 'Voltage', primary: '#3B1C6E', onPrimary: '#FFFFFF', secondary: '#C6F36B', onSecondary: '#233300', accent: '#FF4FB8', surface: '#F5F0FF', onSurface: '#1D1530', background: '#FFFFFF', text: '#1D1530', link: '#5A2DA8' },
    { id: 'bunsen', name: 'Bunsen Blue', primary: '#0B3C5D', onPrimary: '#FFFFFF', secondary: '#9FF0E6', onSecondary: '#063A33', accent: '#FF8C42', surface: '#EDF8F8', onSurface: '#0F1E26', background: '#FFFFFF', text: '#0F1E26', link: '#0B4F7A' },
    { id: 'beaker', name: 'Beaker Orange', primary: '#4A2511', onPrimary: '#FFFFFF', secondary: '#FFC285', onSecondary: '#4A1F00', accent: '#31C6D4', surface: '#FFF4EA', onSurface: '#24170F', background: '#FFFFFF', text: '#24170F', link: '#8A3B0F' },
  ],
  icons: {
    start: 'power', syllabus: 'clipboard-list', modules: 'flask-conical', assignments: 'test-tubes', discussions: 'messages-square',
    grades: 'trophy', help: 'life-buoy', instructor: 'atom', calendar: 'calendar-days', resources: 'wrench',
  },
  art: {
    medium:
      'Glossy cartoon animation background painting with thick ink outlines, saturated flat colour, glowing highlights and crackling energy lines',
    palette: 'Deep violet and electric lime with hot magenta and glowing cyan sparks',
    mood: 'Gleefully dramatic and friendly, all sparks and bubbles and no danger',
    banner:
      'An over-the-top friendly laboratory with a giant coil crackling with harmless sparks, bubbling glass flasks linked by curly tubes and a chunky robot arm on the right, the same soft violet lab shadows fading into a calm quiet area on the left',
    bannerAlt:
      'A domed hilltop laboratory with a giant telescope and a lightning rod catching a friendly bolt under a stormy violet sky on the right, calm open stormy sky on the left',
    welcome: { subject: 'A glowing light bulb fizzing with little lightning sparks', alt: 'A glowing light bulb with sparks' },
    instructor: { subject: 'A white lab coat hanging on a hook beside a pair of safety goggles, nobody wearing them', alt: 'A lab coat and safety goggles on a hook' },
    cards: [
      'A bubbling round-bottom flask with curly glass tubing',
      'A tall coil throwing a friendly arc of violet light',
      'A chunky robot arm holding up a glowing test tube',
    ],
  },
});
