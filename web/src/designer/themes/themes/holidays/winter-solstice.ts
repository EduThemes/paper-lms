import { defineTheme } from '../../define';

export default defineTheme({
  id: 'winter-solstice',
  name: 'Winter Solstice',
  description: 'Glowing oil pastel on black paper for the longest night: the sun returning between standing stones, lanterns in snow, a Yalda table of pomegranates and watermelon, and Dongzhi tangyuan.',
  audiences: ['upper-elementary', 'secondary', 'higher-ed'],
  tags: ['holiday', 'cultural', 'seasonal', 'winter', 'solstice', 'astronomy', 'yalda', 'dongzhi', 'inclusive'],
  collection: 'holidays',
  observance: {
    name: 'Winter Solstice',
    traditions: ['Iranian and Afghan (Yalda / Shab-e Chelleh)', 'Chinese (Dongzhi)', 'Korean (Dongji)', 'Japanese (Tōji)', 'many cultures worldwide'],
    months: [12],
    note: 'December 21 or 22 in the northern hemisphere: the longest night, after which the days grow longer. Marked as Yalda, Dongzhi, Tōji and, by modern Pagans, Yule. Southern-hemisphere schools have theirs in June.',
  },
  defaultLayout: 'editorial',
  iconStyle: 'outline',
  corners: 'soft',
  style: 'oil pastel on black paper',
  colourBlind:
    'Built on deep indigo against pale gold (a blue-yellow pair with strong lightness contrast); pomegranate red is a decorative accent and nothing depends on red versus green.',
  palettes: [
    { id: 'longest-night', name: 'Longest Night', primary: '#16193A', onPrimary: '#FFFFFF', secondary: '#F6D58E', onSecondary: '#3A2800', accent: '#C2364A', surface: '#F1F1F8', onSurface: '#151729', background: '#FFFFFF', text: '#151729', link: '#343E8A' },
    { id: 'pomegranate', name: 'Pomegranate', primary: '#5E1424', onPrimary: '#FFFFFF', secondary: '#F7DCCF', onSecondary: '#4A1220', accent: '#E4A93A', surface: '#FBF2EF', onSurface: '#2A1418', background: '#FFFFFF', text: '#2A1418', link: '#8A1E34' },
    { id: 'returning-sun', name: 'Returning Sun', primary: '#22384A', onPrimary: '#FFFFFF', secondary: '#FBE0B8', onSecondary: '#4A2A08', accent: '#F29F4B', surface: '#F3F6F8', onSurface: '#17222C', background: '#FFFFFF', text: '#17222C', link: '#245A7E' },
  ],
  icons: {
    start: 'sunrise', syllabus: 'scroll', modules: 'layers', assignments: 'pen-line', discussions: 'messages-square',
    grades: 'star', help: 'circle-help', instructor: 'flame', calendar: 'calendar-days', resources: 'moon-star',
  },
  art: {
    medium: 'Velvety oil-pastel drawing on deep black paper, glowing colours laid over the darkness with soft smudged light, fine star specks and rich waxy texture',
    palette: 'Deep midnight indigo and pomegranate red with warm candle amber and pale dawn gold',
    mood: 'Quiet, hopeful and full of warm light',
    banner: 'A ring of ancient standing stones on a snowy hilltop with the low winter sun rising gold between two of the stones and its first rays spreading across the snow, on the right; a calm, deep indigo night sky fading toward dawn on the left',
    bannerAlt: 'Glowing paper lanterns set in deep snow beneath a sky thick with stars, a few bare birch trees catching their light, on the right; a calm, dark starry sky over smooth snow on the left',
    welcome: { subject: 'A ripe pomegranate split open to show glistening red seeds', alt: 'A pomegranate split open' },
    instructor: { subject: 'A bowl of round sweet glutinous rice balls in pale pink and white floating in warm ginger soup', alt: 'A bowl of tangyuan rice balls' },
    cards: [
      'A low table set for the longest night with a sliced watermelon, split pomegranates, a bowl of nuts and dried fruit and lit candles',
      'A thin crescent moon and a sky full of stars over a snowy pine ridge',
      'Bright yellow yuzu fruits floating in steaming water in a round wooden tub',
    ],
  },
});
