import { defineTheme, FONTS } from '../../define';

export default defineTheme({
  id: 'manga-sketch',
  name: 'Manga Sketch',
  description: 'Crisp pen lines, dotted screentone and speed lines with a single soft wash: blossom hills, seaside trains and summer clouds.',
  audiences: ['secondary', 'higher-ed'],
  tags: ['manga', 'screentone', 'ink', 'landscape', 'expressive'],
  defaultLayout: 'split-hero',
  iconStyle: 'outline',
  headingFont: FONTS.trebuchet,
  corners: 'soft',
  collection: 'heroes-comics',
  style: 'manga ink and screentone',
  colourBlind:
    'Ink black against a pale pink or blue wash is a pure lightness contrast; the colour washes are atmosphere and never carry meaning on their own.',
  palettes: [
    { id: 'sakura', name: 'Sakura', primary: '#2B2B33', onPrimary: '#FFFFFF', secondary: '#FFC9D9', onSecondary: '#5A1530', accent: '#FF7FA8', surface: '#FFF3F7', onSurface: '#1C1A20', background: '#FFFFFF', text: '#1C1A20', link: '#A3234F' },
    { id: 'summer-sky', name: 'Summer Sky', primary: '#13416B', onPrimary: '#FFFFFF', secondary: '#BEE3F8', onSecondary: '#0D3350', accent: '#FFB703', surface: '#EEF7FD', onSurface: '#111C26', background: '#FFFFFF', text: '#111C26', link: '#13416B' },
    { id: 'matcha', name: 'Matcha', primary: '#2F4A2B', onPrimary: '#FFFFFF', secondary: '#DDEBC5', onSecondary: '#26391A', accent: '#E86A5B', surface: '#F3F8EC', onSurface: '#1A2117', background: '#FFFFFF', text: '#1A2117', link: '#2F5A2B' },
  ],
  icons: {
    start: 'flower-2', syllabus: 'scroll', modules: 'layers', assignments: 'pen-tool', discussions: 'messages-square',
    grades: 'sparkles', help: 'circle-help', instructor: 'brush', calendar: 'calendar-days', resources: 'notebook-pen',
  },
  art: {
    medium:
      'Black-and-white manga-style ink illustration with crisp pen lines, dotted screentone shading, a few speed lines and one soft colour wash, drawn as one single unframed scene that runs off every edge with no panel borders or gutters',
    palette: 'Ink black and cherry-blossom pink with pale sky blue on white paper',
    mood: 'Wistful, breezy and full of feeling, like the first day of summer',
    banner:
      'A hilltop with a single cherry tree in full blossom, petals streaming on the wind past a country road and a line of telegraph poles on the right, a wide calm screentone sky on the left',
    bannerAlt:
      'A small train crossing a long seaside bridge under towering summer clouds on the right, the same calm pale sky and still sea continuing across the left',
    welcome: { subject: 'A single cherry-blossom branch with petals drifting away', alt: 'A cherry-blossom branch' },
    instructor: { subject: 'A dip pen and a small bottle of ink beside a sketchbook open to blank pages', alt: 'A dip pen, ink bottle and sketchbook' },
    cards: [
      'A bicycle leaning on a seaside railing under towering summer clouds',
      'A rooftop water tank and chain-link fence under swirling clouds',
      'A cat napping on a sunny windowsill beside a potted plant',
    ],
  },
});
