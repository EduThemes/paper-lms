import { defineTheme } from '../../define';

export default defineTheme({
  id: 'diwali',
  name: 'Diwali',
  description: 'Glowing night still-lifes of clay diyas, marigold garlands, petal rangoli and star lanterns. The festival of lights in warm marigold and indigo.',
  audiences: ['upper-elementary', 'early-elementary', 'secondary'],
  tags: ['holiday', 'religious', 'hindu', 'sikh', 'jain', 'festival-of-lights'],
  collection: 'holidays',
  observance: {
    name: 'Diwali',
    traditions: ['Hindu', 'Sikh (Bandi Chhor Divas)', 'Jain', 'some Buddhist communities'],
    months: [10, 11],
    note: 'Follows the Hindu lunisolar calendar: the main night is the new moon of Kartika, in October or November. Sikhs mark Bandi Chhor Divas and Jains mark Mahavira’s nirvana at the same time.',
  },
  defaultLayout: 'classic-band',
  iconStyle: 'solid-circle',
  corners: 'round',
  style: 'glowing night still-life render',
  colourBlind:
    'Deep indigo and marigold carry the structure and differ strongly in lightness and hue family; magenta and saffron are decoration only, never the only cue.',
  palettes: [
    { id: 'marigold', name: 'Marigold Night', primary: '#2E1A5C', onPrimary: '#FFFFFF', secondary: '#F9B233', onSecondary: '#2A1700', accent: '#D6336C', surface: '#FFF4E0', onSurface: '#2A1F14', background: '#FFFFFF', text: '#2A1F14', link: '#5B2A86' },
    { id: 'magenta', name: 'Rangoli Pink', primary: '#7A1E4E', onPrimary: '#FFFFFF', secondary: '#FFC7DD', onSecondary: '#4A0F2E', accent: '#F59E0B', surface: '#FFF0F6', onSurface: '#2B1622', background: '#FFFFFF', text: '#2B1622', link: '#8E1F5A' },
    { id: 'saffron', name: 'Saffron Lamp', primary: '#6E2A06', onPrimary: '#FFFFFF', secondary: '#FFD98A', onSecondary: '#3A2200', accent: '#2E1A5C', surface: '#FFF7E6', onSurface: '#2A1A0E', background: '#FFFFFF', text: '#2A1A0E', link: '#8A3506' },
  ],
  icons: {
    start: 'sparkles', syllabus: 'scroll-text', modules: 'layers', assignments: 'pencil', discussions: 'messages-square',
    grades: 'star', help: 'circle-help', instructor: 'lamp', calendar: 'calendar-days', resources: 'flower-2',
  },
  art: {
    medium: 'Warm night still-life render in a photographic style, glowing lamplight, shallow depth of field and a plain softly lit background with no out-of-focus lights',
    palette: 'Deep indigo night with marigold orange, saffron gold and magenta pink',
    mood: 'Glowing, joyful and welcoming',
    banner: 'Exactly seven small clay diyas standing in one single straight row, evenly spaced, all seven fully inside the picture and each with its own single lit flame, with no other lamps, candles or lights anywhere in the scene, in front of a floor rangoli of marigold petals and coloured powder in a round flower pattern on the right, a soft dark indigo dusk wall on the left',
    bannerAlt: 'Marigold garlands and three glowing star-shaped paper lanterns with plain, unmarked paper panels hanging above lit clay diyas on a stone step on the right, a calm deep indigo evening sky with soft golden bokeh on the left',
    welcome: { subject: 'A single lit clay diya, a shallow clay bowl with one flame at its lip, on a small bed of marigold petals', alt: 'A lit clay oil lamp on marigold petals' },
    instructor: { subject: 'A brass tray holding one lit clay diya, marigold flowers and a small bowl of sweets', alt: 'A brass tray with a lamp, marigolds and sweets' },
    cards: [
      'A round rangoli of coloured powder in a lotus pattern with one lit clay diya at its centre',
      'A plate of colourful Indian sweets beside a lit clay diya',
      'Strings of tiny warm lights and marigold garlands across a doorway at dusk',
    ],
  },
});
