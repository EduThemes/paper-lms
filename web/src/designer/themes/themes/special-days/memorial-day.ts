import { defineTheme } from '../../define';

export default defineTheme({
  id: 'memorial-day',
  name: 'Memorial Day',
  description: 'Luminous oil-painted dawn: a meadow of red wildflowers, old oaks along a quiet lane, a wreath on a stone wall and a still lake. Quiet and dignified, with no figures, weapons or graves.',
  audiences: ['upper-elementary', 'secondary', 'higher-ed'],
  tags: ['civic', 'remembrance', 'memorial-day', 'solemn', 'oil-painting'],
  collection: 'special-days',
  observance: {
    name: 'Memorial Day',
    traditions: [],
    months: [5],
    note: 'The last Monday in May. A US federal holiday honouring those who died while serving in the US armed forces. Veterans Day (November 11) honours everyone who served: use the Remembrance & Veterans Day theme for that.',
  },
  defaultLayout: 'editorial',
  iconStyle: 'outline',
  corners: 'soft',
  style: 'luminous oil painting',
  colourBlind:
    'Navy, deep poppy and slate bands sit against warm cream or stone with strong lightness contrast; poppy red is otherwise decorative and never set against green to carry meaning.',
  palettes: [
    { id: 'navy', name: 'Navy & Cream', primary: '#1C2A4A', onPrimary: '#FFFFFF', secondary: '#EFE4CF', onSecondary: '#2E2414', accent: '#B3261E', surface: '#F7F3EC', onSurface: '#1A1F2B', background: '#FFFFFF', text: '#1A1F2B', link: '#2B4377' },
    { id: 'poppy', name: 'Poppy', primary: '#6A1A1E', onPrimary: '#FFFFFF', secondary: '#F1E2CC', onSecondary: '#3A2414', accent: '#2B4377', surface: '#FBF4EE', onSurface: '#2A1716', background: '#FFFFFF', text: '#2A1716', link: '#8A2228' },
    { id: 'dawn-stone', name: 'Dawn Stone', primary: '#33383F', onPrimary: '#FFFFFF', secondary: '#E5E2DC', onSecondary: '#2A2A2A', accent: '#B3261E', surface: '#F4F3F0', onSurface: '#1E2125', background: '#FFFFFF', text: '#1E2125', link: '#3E4E66' },
  ],
  icons: {
    start: 'flower', syllabus: 'scroll', modules: 'layers', assignments: 'pen-tool', discussions: 'messages-square',
    grades: 'medal', help: 'circle-help', instructor: 'tree-deciduous', calendar: 'calendar-days', resources: 'library',
  },
  art: {
    // Same medium sentence as juneteenth.ts, so the reused poppy-meadow painting (card-1) matches the set.
    medium: 'Luminous oil painting with soft impasto brushwork, warm glazes and gentle light',
    palette: 'Deep navy blue and poppy red with warm cream and soft gold light',
    mood: 'Quiet, dignified and reflective',
    banner: 'A row of great old oak trees along a quiet country lane at dawn, mist lying over green fields and soft gold light through the leaves, on the right; calm soft sky on the left',
    bannerAlt: 'A calm lake at dawn with mist rising from still water, a line of dark trees on the far shore and the first gold light on the horizon, on the right; calm pale sky on the left',
    welcome: { subject: 'A small bunch of red poppies and white daisies tied with a plain navy ribbon', alt: 'Red poppies and white daisies' },
    instructor: { subject: 'A brass lantern with a lit candle resting on a weathered wooden bench', alt: 'A lit lantern on a wooden bench' },
    cards: [
      // Reused painting: tools/images/.cache/keep/juneteenth-poppy-field (keep this subject word for word).
      'A meadow of red wildflowers under a wide sky',
      'A wreath of red and white flowers resting on a low weathered stone wall in morning light',
      'A quiet harbour at dawn with still water and a distant white lighthouse',
    ],
  },
});
