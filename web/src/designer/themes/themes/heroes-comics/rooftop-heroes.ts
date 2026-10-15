import { defineTheme } from '../../define';

export default defineTheme({
  id: 'rooftop-heroes',
  name: 'Rooftop Heroes',
  description: 'Young caped heroes seen from behind on a city rooftop, watching a golden sunrise. Hopeful animated-film light for classes that are just getting started.',
  audiences: ['upper-elementary', 'secondary'],
  tags: ['superhero', 'city', 'sunrise', 'cinematic', 'hopeful'],
  defaultLayout: 'classic-band',
  iconStyle: 'duotone',
  corners: 'soft',
  collection: 'heroes-comics',
  style: 'cel-shaded animated-film painting',
  colourBlind:
    'Deep indigo against golden yellow is a blue-yellow pair with a large lightness gap; coral is a decorative glow and never the only cue.',
  palettes: [
    { id: 'dawn', name: 'Dawn Patrol', primary: '#2B2D6E', onPrimary: '#FFFFFF', secondary: '#FFC66B', onSecondary: '#3D2400', accent: '#FF7A6B', surface: '#FFF4EC', onSurface: '#1E1F3A', background: '#FFFFFF', text: '#1E1F3A', link: '#2B2D6E' },
    { id: 'skyline', name: 'Skyline', primary: '#0F4C75', onPrimary: '#FFFFFF', secondary: '#BFE3F5', onSecondary: '#0C3550', accent: '#F5A524', surface: '#EEF6FB', onSurface: '#10202E', background: '#FFFFFF', text: '#10202E', link: '#0F4C75' },
    { id: 'dusk', name: 'Dusk Watch', primary: '#5A1F4F', onPrimary: '#FFFFFF', secondary: '#FFB8A8', onSecondary: '#4A1410', accent: '#8A7CFF', surface: '#FBF0F5', onSurface: '#241622', background: '#FFFFFF', text: '#241622', link: '#6E2462' },
  ],
  icons: {
    start: 'sunrise', syllabus: 'scroll', modules: 'building-2', assignments: 'target', discussions: 'messages-square',
    grades: 'medal', help: 'shield-check', instructor: 'shield', calendar: 'calendar-days', resources: 'backpack',
  },
  art: {
    medium:
      'Bright cel-shaded animated-film concept painting with clean outlines, soft painted gradients and warm rim light from a low sun',
    palette: 'Golden sunrise yellow and soft coral with deep indigo shadows and pale sky blue',
    mood: 'Hopeful, brave and calm, the quiet moment before the day begins',
    banner:
      'Three small young heroes seen from behind on a rooftop ledge, plain capes streaming in the wind, watching a huge golden sun rise over city towers on the right, the same calm open dawn sky stretching across the left',
    bannerAlt:
      'Two caped heroes seen from behind, small in the distance, leaping between rooftops toward a glowing sunrise on the right, soft open peach sky over low rooftops on the left',
    welcome: { subject: 'A round shield with a simple star emblem glowing in warm light', alt: 'A glowing round shield with a star' },
    instructor: { subject: 'A folded plain cape and a domino eye mask resting on a wooden bench, nobody wearing them', alt: 'A folded cape and a mask on a bench' },
    cards: [
      'A rooftop water tower silhouetted against a golden sunrise',
      'A plain cape fluttering from a rooftop railing high above a waking city',
      'A round shield with a simple star emblem leaning against a rooftop wall at dawn',
    ],
  },
});
