import { defineTheme } from '../../define';

export default defineTheme({
  id: 'hispanic-heritage-month',
  name: 'Hispanic Heritage Month',
  description: 'Sunlit gouache of mountains, coasts, monarch butterflies, a classical guitar and a coffee plant. Many nations and landscapes, no sombreros or piñatas.',
  audiences: ['upper-elementary', 'secondary', 'higher-ed'],
  tags: ['heritage', 'hispanic-heritage-month', 'latino', 'latine', 'gouache'],
  collection: 'special-days',
  observance: {
    name: 'Hispanic Heritage Month',
    traditions: [],
    months: [9, 10],
    note: 'US: September 15 to October 15, starting on several Central American independence days. Some districts call it Latine or Latinx Heritage Month. It covers 20+ nations; avoid one-country clichés.',
  },
  defaultLayout: 'split-hero',
  iconStyle: 'solid-circle',
  corners: 'round',
  style: 'bold sunlit gouache',
  colourBlind:
    'Teal and marigold are a blue-yellow pair that stays distinct for red-green colour blindness; magenta is decorative only.',
  palettes: [
    { id: 'teal', name: 'Teal & Marigold', primary: '#0F4A55', onPrimary: '#FFFFFF', secondary: '#FFC94D', onSecondary: '#3A2600', accent: '#D6336C', surface: '#FFF7E6', onSurface: '#13252A', background: '#FFFFFF', text: '#13252A', link: '#0F5966' },
    { id: 'magenta', name: 'Magenta Bloom', primary: '#6E1748', onPrimary: '#FFFFFF', secondary: '#FFD3E6', onSecondary: '#520F35', accent: '#F2A541', surface: '#FFF2F8', onSurface: '#2A1520', background: '#FFFFFF', text: '#2A1520', link: '#8A1D5A' },
    { id: 'terracotta', name: 'Terracotta Sun', primary: '#6B2E14', onPrimary: '#FFFFFF', secondary: '#F7D7B5', onSecondary: '#4A1F0C', accent: '#2A9D8F', surface: '#FCF3EA', onSurface: '#2A1A12', background: '#FFFFFF', text: '#2A1A12', link: '#8A3E1A' },
  ],
  icons: {
    start: 'sun', syllabus: 'scroll', modules: 'layers', assignments: 'pen-tool', discussions: 'messages-square',
    grades: 'award', help: 'circle-help', instructor: 'guitar', calendar: 'calendar-days', resources: 'library',
  },
  art: {
    medium: 'Bold sunlit gouache painting with layered flat colour, confident brushwork and warm light',
    palette: 'Deep teal and marigold with bright magenta and warm terracotta',
    mood: 'Vibrant, warm and proud',
    banner: 'A sweeping landscape of high green mountains above a sunlit coast, a large flowering tree full of orange blossoms and monarch butterflies on the right, calm open sky on the left',
    bannerAlt: 'A sunlit hillside street of brightly painted houses in coral, turquoise and yellow, with magenta bougainvillea spilling over wrought-iron balconies, on the right, calm open sky on the left',
    welcome: { subject: 'A hummingbird sipping from a red hibiscus flower', alt: 'A hummingbird at a flower' },
    instructor: { subject: 'A classical guitar leaning on a stack of closed books', alt: 'A guitar and a stack of books' },
    cards: [
      'Terraced green fields high in the mountains under drifting clouds',
      'A rainforest waterfall with a bright macaw on a branch',
      'Turquoise sea and palms along a sunny coast',
    ],
  },
});
