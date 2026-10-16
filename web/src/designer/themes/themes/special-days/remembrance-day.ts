import { defineTheme } from '../../define';

export default defineTheme({
  id: 'remembrance-day',
  name: 'Remembrance & Veterans Day',
  description: 'Muted watercolour poppies in a wheat field at dusk, a lone poppy by a stone wall and a candle. Quiet and reflective, with no soldiers or weapons.',
  audiences: ['upper-elementary', 'secondary', 'higher-ed'],
  tags: ['civic', 'remembrance', 'veterans-day', 'solemn', 'watercolour'],
  collection: 'special-days',
  observance: {
    name: 'Remembrance Day and Veterans Day',
    traditions: [],
    months: [11],
    note: 'November 11. Remembrance Day (UK, Canada, Australia and other Commonwealth countries) honours those who died in war; US Veterans Day honours everyone who served. For US Memorial Day in May, use the Memorial Day theme.',
  },
  defaultLayout: 'editorial',
  iconStyle: 'outline',
  corners: 'square',
  style: 'muted watercolour',
  colourBlind:
    'Poppy red appears only in the art; the palettes are built on slate and stone with strong lightness contrast, so nothing relies on red versus green.',
  palettes: [
    { id: 'stone', name: 'Poppy & Stone', primary: '#2F3B45', onPrimary: '#FFFFFF', secondary: '#E6D6CC', onSecondary: '#3A2A22', accent: '#B3261E', surface: '#F5F3F0', onSurface: '#1E2226', background: '#FFFFFF', text: '#1E2226', link: '#3A4E66' },
    { id: 'dusk', name: 'Dusk Slate', primary: '#26324A', onPrimary: '#FFFFFF', secondary: '#DCE2EA', onSecondary: '#26324A', accent: '#B3261E', surface: '#F2F4F7', onSurface: '#1A1F2A', background: '#FFFFFF', text: '#1A1F2A', link: '#2F4878' },
    { id: 'field', name: 'Field Sage', primary: '#2E3E30', onPrimary: '#FFFFFF', secondary: '#DDE5D8', onSecondary: '#2A3A2A', accent: '#B3261E', surface: '#F3F5F1', onSurface: '#1C221C', background: '#FFFFFF', text: '#1C221C', link: '#36583A' },
  ],
  icons: {
    start: 'flower', syllabus: 'scroll', modules: 'layers', assignments: 'pen-tool', discussions: 'messages-square',
    grades: 'medal', help: 'circle-help', instructor: 'flower-2', calendar: 'calendar-days', resources: 'library',
  },
  art: {
    medium: 'Muted watercolour illustration with soft washes, gentle edges and lots of quiet space',
    palette: 'Poppy red and soft sage with warm stone grey and deep slate',
    mood: 'Quiet, reflective and respectful',
    banner: 'A field of red poppies among ripening wheat stretching toward a soft horizon at dusk on the right, calm misty sky on the left',
    bannerAlt: 'A single red poppy bending in the breeze beside a weathered stone wall, with distant hills at dusk on the right, calm misty sky on the left',
    welcome: { subject: 'A single red poppy', alt: 'A red poppy' },
    instructor: { subject: 'A small wreath of red poppies and laurel leaves', alt: 'A wreath of poppies' },
    cards: [
      'A lone oak tree in a misty field at dawn',
      'A lit candle beside a sprig of rosemary',
      'A flock of birds flying over still water at dusk, the wash of sky and water reaching all four edges of the picture with no white paper margin',
    ],
  },
});
