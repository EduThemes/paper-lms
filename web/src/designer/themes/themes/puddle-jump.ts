import { defineTheme } from '../define';

export default defineTheme({
  id: 'puddle-jump',
  name: 'Puddle Jump',
  description: 'Loose watercolour and wax-crayon resist: rain boots, frogs and umbrellas splashing through a rainy day.',
  audiences: ['early-elementary'],
  tags: ['watercolour', 'weather', 'seasons', 'gentle'],
  defaultLayout: 'classic-band',
  iconStyle: 'duotone',
  corners: 'round',
  style: 'watercolour and wax-crayon resist',
  colourBlind:
    'Yellow and blue carry the contrast, a pairing that survives red-green colour blindness; leafy green appears only in the art.',
  palettes: [
    { id: 'rainy', name: 'Rainy Day', primary: '#1B4F8C', onPrimary: '#FFFFFF', secondary: '#FFF0A8', onSecondary: '#3D3300', accent: '#6BBF59', surface: '#EEF5FC', onSurface: '#1F2933', background: '#FFFFFF', text: '#1F2933', link: '#1B4F8C' },
    { id: 'sunshower', name: 'Sunshower', primary: '#8A3B12', onPrimary: '#FFFFFF', secondary: '#CFE8F7', onSecondary: '#0F3554', accent: '#F5B83D', surface: '#FFF4E8', onSurface: '#2A2420', background: '#FFFFFF', text: '#2A2420', link: '#8A3B12' },
    { id: 'storm', name: 'Storm Cloud', primary: '#283655', onPrimary: '#FFFFFF', secondary: '#A9D3C9', onSecondary: '#10302A', accent: '#F2C14E', surface: '#F1F3F8', onSurface: '#1A1F2B', background: '#FFFFFF', text: '#1A1F2B', link: '#2B4C7E' },
  ],
  icons: {
    start: 'umbrella', syllabus: 'notebook', modules: 'layers', assignments: 'pencil-line', discussions: 'message-circle-heart',
    grades: 'star', help: 'life-buoy', instructor: 'school', calendar: 'calendar-days', resources: 'backpack',
  },
  art: {
    medium: 'Loose watercolour and wax-crayon resist illustration on textured cold-press paper with soft bleeding edges',
    palette: 'Rain-washed blue and lemon yellow with leafy green and soft grey on white paper',
    mood: 'Splashy, joyful and calm at the same time, like a picture book about a rainy day',
    banner: 'Big rain puddles with an empty pair of yellow rain boots standing on their own at the water’s edge (nobody wearing them, no legs), a toy duck and a floating leaf boat on the right, a soft pale wash of open sky and wet ground on the left',
    bannerAlt: 'A frog sitting on a lily pad under a large yellow umbrella with falling raindrops on the right, a quiet pale watery wash on the left',
    welcome: { subject: 'A yellow umbrella with a few raindrops', alt: 'A yellow umbrella in the rain' },
    instructor: { subject: 'A pair of yellow rain boots beside a small watering can', alt: 'Rain boots beside a watering can' },
    cards: [
      'A folded paper boat floating on a puddle',
      'A snail sheltering under a broad leaf in the rain',
      'A soft rainbow arching over shining puddles',
    ],
  },
});
