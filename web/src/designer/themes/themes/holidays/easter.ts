import { defineTheme } from '../../define';

export default defineTheme({
  id: 'easter',
  name: 'Easter',
  description: 'Textured gouache of folk-patterned eggs, white lilies, hot cross buns and spring lambs, plus a draped cross at sunrise in the alternate banner for faith schools.',
  audiences: ['early-elementary', 'upper-elementary', 'secondary'],
  tags: ['holiday', 'religious', 'christian', 'easter', 'spring'],
  collection: 'holidays',
  observance: {
    name: 'Easter',
    traditions: ['Christian'],
    months: [3, 4, 5],
    note: 'The Sunday after the first full moon on or after the spring equinox (late March to late April). Orthodox Easter uses the Julian calendar and can be up to five weeks later. Banner-alt (a draped cross) suits faith schools.',
  },
  defaultLayout: 'playful',
  iconStyle: 'solid-circle',
  corners: 'round',
  style: 'textured gouache picture-book painting',
  colourBlind:
    'Lilac and butter yellow carry the structure and differ in lightness and hue family (blue-violet vs yellow); leaf green is decoration only.',
  palettes: [
    { id: 'lilac', name: 'Lilac Morning', primary: '#4B2E6E', onPrimary: '#FFFFFF', secondary: '#F6E7A1', onSecondary: '#332A00', accent: '#7BB661', surface: '#F6F1FB', onSurface: '#221A2C', background: '#FFFFFF', text: '#221A2C', link: '#5E3A8A' },
    { id: 'sky', name: 'Spring Sky', primary: '#1F4E79', onPrimary: '#FFFFFF', secondary: '#FBE7A1', onSecondary: '#332800', accent: '#E9A8C4', surface: '#EEF5FB', onSurface: '#15202C', background: '#FFFFFF', text: '#15202C', link: '#1E5A8C' },
    { id: 'meadow', name: 'Meadow', primary: '#2E5130', onPrimary: '#FFFFFF', secondary: '#E8DDF5', onSecondary: '#2E1F45', accent: '#F2C94C', surface: '#F2F7EF', onSurface: '#18231A', background: '#FFFFFF', text: '#18231A', link: '#2D6630' },
  ],
  icons: {
    start: 'sunrise', syllabus: 'scroll', modules: 'blocks', assignments: 'pencil', discussions: 'message-circle',
    grades: 'star', help: 'circle-help', instructor: 'egg', calendar: 'calendar-days', resources: 'flower-2',
  },
  art: {
    medium: 'Textured gouache picture-book painting with soft dry-brush edges and fresh spring light',
    palette: 'Soft lilac and butter yellow with fresh leaf green and sky blue on cream',
    mood: 'Joyful, fresh and hopeful',
    banner: 'A woven basket of eggs decorated with folk dot-and-stripe patterns among white lilies and tulips on the right, a soft open lilac morning sky over a gentle green hill on the left',
    bannerAlt: 'A simple plain wooden cross draped with a white cloth on a quiet green hill with white lilies, a warm sunrise behind it, on the right; an open soft pastel sky on the left',
    welcome: { subject: 'A white Easter lily in bloom', alt: 'A white lily' },
    instructor: { subject: 'A plate of hot cross buns beside a small vase of daffodils', alt: 'Hot cross buns and daffodils' },
    cards: [
      'Three eggs decorated with folk patterns in a nest of straw',
      'A spring meadow of daffodils and tulips under a soft sky',
      'A white lamb resting in a flowering meadow',
    ],
  },
});
