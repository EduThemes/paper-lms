import { defineTheme } from '../../define';

export default defineTheme({
  id: 'black-history-month',
  name: 'Black History Month',
  description: 'Rich mixed-media collage of a patchwork quilt, a trumpet, books and a rocket lifting off. Achievement, music and craft, with no likenesses.',
  audiences: ['upper-elementary', 'secondary', 'higher-ed'],
  tags: ['heritage', 'black-history-month', 'african-american', 'history', 'collage'],
  collection: 'special-days',
  observance: {
    name: 'Black History Month',
    traditions: [],
    months: [2, 10],
    regionalMonths: { US: [2], CA: [2], GB: [10], IE: [10] },
    note: 'US and Canada: February. UK and Ireland: October. Feature real people through primary sources (Library of Congress, NMAAHC); this art has no portraits by design.',
  },
  defaultLayout: 'split-hero',
  iconStyle: 'solid-circle',
  corners: 'soft',
  style: 'mixed-media painted-paper collage',
  colourBlind:
    'Deep green and warm gold differ strongly in lightness; terracotta is a decorative accent and is never paired with green to carry meaning.',
  palettes: [
    { id: 'forest', name: 'Forest & Gold', primary: '#1E3D2B', onPrimary: '#FFFFFF', secondary: '#F2C94C', onSecondary: '#2E2200', accent: '#C4572E', surface: '#FBF5E8', onSurface: '#1F1E18', background: '#FFFFFF', text: '#1F1E18', link: '#245A3B' },
    { id: 'terracotta', name: 'Terracotta', primary: '#6B2A16', onPrimary: '#FFFFFF', secondary: '#F5D9B8', onSecondary: '#4A1F0E', accent: '#D9A23A', surface: '#FBF3EA', onSurface: '#2A1A12', background: '#FFFFFF', text: '#2A1A12', link: '#8A3A1C' },
    { id: 'indigo', name: 'Indigo Night', primary: '#23264F', onPrimary: '#FFFFFF', secondary: '#EFD08A', onSecondary: '#3A2A00', accent: '#C4572E', surface: '#F4F2EC', onSurface: '#1C1D2A', background: '#FFFFFF', text: '#1C1D2A', link: '#33388A' },
  ],
  icons: {
    start: 'sunrise', syllabus: 'scroll', modules: 'library', assignments: 'pen-tool', discussions: 'messages-square',
    grades: 'award', help: 'circle-help', instructor: 'music', calendar: 'calendar-days', resources: 'book-open',
  },
  art: {
    medium: 'Rich mixed-media collage of painted papers, fabric scraps and touches of gold leaf with visible layered edges; the collage runs off every edge with no border',
    palette: 'Deep forest green, warm gold and terracotta with rich earth brown',
    mood: 'Proud, dignified and full of possibility',
    banner: 'A large patchwork quilt of bold geometric squares draped beside a gleaming trumpet and a tall stack of closed books with plain spines on the right, calm warm painted ground on the left',
    bannerAlt: 'A model rocket lifting off into a starry night sky beside a microscope and a stack of closed books on the right, calm deep painted sky on the left',
    welcome: { subject: 'A gleaming brass trumpet', alt: 'A trumpet' },
    instructor: { subject: 'A stack of closed books with plain spines beside a lit reading lamp', alt: 'A stack of books and a lamp' },
    cards: [
      'A patchwork quilt of bold geometric squares',
      'A tall sunflower field under a wide sky',
      'An upright bass leaning beside a vintage microphone on a stand',
    ],
  },
});
