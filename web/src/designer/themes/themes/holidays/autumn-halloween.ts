import { defineTheme } from '../../define';

export default defineTheme({
  id: 'autumn-halloween',
  name: 'Autumn & Halloween',
  description: 'Friendly claymation pumpkin patches, apples, owls and leaves. Autumn first, with gently glowing star-cut pumpkin lanterns only in the alternate banner.',
  audiences: ['early-elementary', 'upper-elementary', 'secondary'],
  tags: ['holiday', 'cultural', 'autumn', 'halloween', 'seasonal', 'pumpkins'],
  collection: 'holidays',
  observance: {
    name: 'Halloween & Autumn',
    traditions: ['North American', 'Irish and British'],
    months: [10],
    note: 'Halloween is October 31. The main banner shows autumn only (pumpkins, leaves, apples), so schools that don’t mark Halloween can still use it; carved lanterns appear only in banner-alt and card-2.',
  },
  defaultLayout: 'playful',
  iconStyle: 'solid-circle',
  corners: 'round',
  style: 'friendly claymation render',
  colourBlind:
    'Plum and pumpkin orange differ in hue family and strongly in lightness; moss green is decoration only and never paired with red or orange to mean something.',
  palettes: [
    { id: 'pumpkin', name: 'Pumpkin & Plum', primary: '#3D1F4A', onPrimary: '#FFFFFF', secondary: '#FFB25B', onSecondary: '#2E1500', accent: '#7A9A3B', surface: '#FFF4E6', onSurface: '#241A22', background: '#FFFFFF', text: '#241A22', link: '#5A2A70' },
    { id: 'harvest-moon', name: 'Harvest Moon', primary: '#1E2A44', onPrimary: '#FFFFFF', secondary: '#FFD36B', onSecondary: '#2E2000', accent: '#F07F2E', surface: '#F2F4F8', onSurface: '#161C28', background: '#FFFFFF', text: '#161C28', link: '#2A4478' },
    { id: 'maple', name: 'Maple Leaf', primary: '#6B2A12', onPrimary: '#FFFFFF', secondary: '#F7D8B0', onSecondary: '#3A2008', accent: '#C9A227', surface: '#FCF2EA', onSurface: '#2A1910', background: '#FFFFFF', text: '#2A1910', link: '#8A3414' },
  ],
  icons: {
    start: 'leaf', syllabus: 'scroll', modules: 'blocks', assignments: 'pencil', discussions: 'message-circle',
    grades: 'star', help: 'circle-help', instructor: 'squirrel', calendar: 'calendar-days', resources: 'tree-deciduous',
  },
  art: {
    medium: 'Soft 3D claymation-style render with rounded handmade shapes, gentle fingerprint texture and warm studio light',
    palette: 'Pumpkin orange and deep plum with golden yellow, moss green and warm cream',
    mood: 'Friendly, cosy and playful',
    banner: 'A cheerful pumpkin patch of round orange pumpkins with curly vines, autumn leaves and a little basket of red apples on the right, a calm warm cream background with a few drifting leaves on the left',
    bannerAlt: 'Three glowing carved pumpkin lanterns with friendly cut-out star and crescent moon shapes (no carved eyes or mouths) beside a sleepy owl on a branch under a big round moon on the right, a calm deep plum dusk sky on the left',
    welcome: { subject: 'A round orange pumpkin with a curly green vine and one leaf', alt: 'A pumpkin with a curly vine' },
    instructor: { subject: 'A cosy mug of apple cider beside a little pile of autumn leaves and acorns', alt: 'A mug of cider with leaves and acorns' },
    cards: [
      'A friendly round owl on a branch among autumn leaves',
      'One carved pumpkin lantern with a cut-out star shape, glowing softly, no carved eyes or mouth',
      'A squirrel beside an acorn and a red toadstool on fallen leaves',
    ],
  },
});
