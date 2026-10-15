import { defineTheme, FONTS } from '../../define';

export default defineTheme({
  id: 'mid-century',
  name: 'Mid-Century',
  description: 'Atomic-age screen print: boomerangs, starbursts and walnut furniture in mustard, teal and tomato. Optimism with clean lines.',
  audiences: ['secondary', 'higher-ed', 'professional'],
  tags: ['mid-century', 'atomic', 'retro', 'screen-print', 'design-history'],
  defaultLayout: 'split-hero',
  iconStyle: 'solid-circle',
  headingFont: FONTS.helvetica,
  corners: 'round',
  collection: 'signature',
  premium: true,
  style: 'mid-century atomic screen print',
  colourBlind:
    'Mustard, teal and walnut differ strongly in lightness; tomato and teal never encode two states, they only decorate the art.',
  palettes: [
    { id: 'walnut', name: 'Walnut & Mustard', primary: '#3E2A1E', onPrimary: '#FFFFFF', secondary: '#F2C94C', onSecondary: '#3A2A00', accent: '#2A9D8F', surface: '#FBF4E6', onSurface: '#241A12', background: '#FFFFFF', text: '#241A12', link: '#7A3E14' },
    { id: 'teal-lounge', name: 'Teal Lounge', primary: '#12403F', onPrimary: '#FFFFFF', secondary: '#F4D38A', onSecondary: '#3A2A00', accent: '#E76F51', surface: '#EEF6F4', onSurface: '#142624', background: '#FFFFFF', text: '#142624', link: '#145A57' },
    { id: 'tomato', name: 'Tomato Soup', primary: '#5C2318', onPrimary: '#FFFFFF', secondary: '#F6D6C2', onSecondary: '#4A1B10', accent: '#2A9D8F', surface: '#FBF1EA', onSurface: '#2A1A15', background: '#FFFFFF', text: '#2A1A15', link: '#8A3420' },
  ],
  icons: {
    start: 'atom', syllabus: 'file-text', modules: 'shapes', assignments: 'pencil-ruler', discussions: 'messages-square',
    grades: 'star', help: 'circle-help', instructor: 'lamp', calendar: 'calendar-days', resources: 'armchair',
  },
  art: {
    medium: 'Mid-century modern screen-print illustration with flat atomic-age shapes, boomerangs, starbursts and kidney curves, slight ink misregistration and a fine paper grain',
    palette: 'Walnut brown, mustard yellow and teal with a dash of tomato orange on warm cream',
    mood: 'Cheerful, optimistic and tidy',
    banner: 'A low walnut sideboard on slim tapered feet holding a teal ceramic vase and a potted snake plant, with mustard starbursts and boomerang shapes floating above it, on the right, a plain warm cream wall on the left',
    bannerAlt: 'A large atomic starburst and three orbiting teal and mustard ellipses drifting over a cluster of kidney-shaped shapes on the right, the same plain cream ground stretching calmly to the left',
    welcome: { subject: 'A mustard starburst inside a teal atomic orbit', alt: '' },
    instructor: { subject: 'A walnut desk lamp with a mustard cone shade', alt: 'A mid-century desk lamp' },
    cards: [
      'A teal kidney-shaped coffee table with a ceramic bowl',
      'A boomerang mobile of floating discs',
      'A tall cactus in a mustard planter on slim wooden feet',
    ],
  },
});
