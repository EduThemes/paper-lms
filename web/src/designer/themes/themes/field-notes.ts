import { defineTheme } from '../define';

export default defineTheme({
  id: 'field-notes',
  name: 'Field Notes',
  description: 'Naturalist ink and watercolour studies of ferns, feathers and beetles. Quietly curious, for science and nature study.',
  audiences: ['upper-elementary', 'secondary'],
  tags: ['watercolour', 'nature', 'science', 'naturalist'],
  defaultLayout: 'sidebar',
  iconStyle: 'outline',
  corners: 'soft',
  style: 'naturalist watercolour field studies',
  colourBlind:
    'Moss and rust would merge for red-green colour blindness, so they never sit side by side to mean different things; links are blue-teal rather than green.',
  palettes: [
    { id: 'moss', name: 'Moss & Ochre', primary: '#2F4A3A', onPrimary: '#FFFFFF', secondary: '#E8D4A2', onSecondary: '#3E2F0E', accent: '#B5562F', surface: '#F6F1E4', onSurface: '#2A2620', background: '#FFFFFF', text: '#2A2620', link: '#275A7A' },
    { id: 'tidepool', name: 'Tidepool', primary: '#1E4D5C', onPrimary: '#FFFFFF', secondary: '#F2C6A0', onSecondary: '#4A2410', accent: '#E07A5F', surface: '#EEF4F2', onSurface: '#1B2629', background: '#FFFFFF', text: '#1B2629', link: '#1E4D5C' },
    { id: 'autumn', name: 'Autumn Walk', primary: '#6A3A1F', onPrimary: '#FFFFFF', secondary: '#F4D58D', onSecondary: '#3E2C00', accent: '#A3B18A', surface: '#FBF3E6', onSurface: '#2A2119', background: '#FFFFFF', text: '#2A2119', link: '#7A3E1E' },
  ],
  icons: {
    start: 'compass', syllabus: 'notebook-pen', modules: 'leaf', assignments: 'pencil-ruler', discussions: 'messages-square',
    grades: 'award', help: 'life-buoy', instructor: 'binoculars', calendar: 'calendar-days', resources: 'microscope',
  },
  art: {
    medium: 'Naturalist watercolour illustration with fine ink line and soft washes on warm cream paper, in the manner of a museum nature study',
    palette: 'Moss green, ochre and rust brown with faded sky blue on cream',
    mood: 'Quiet, observant and curious, inviting a closer look',
    banner: 'A large fern frond, an oak leaf, a long feather and a small beetle arranged like a specimen study on the right, open blank cream paper on the left',
    bannerAlt: 'A pond edge with tall cattails, a big dragonfly and smooth stones on the right, a soft blank cream wash on the left',
    welcome: { subject: 'A single pinecone with a sprig of fern', alt: 'A pinecone with a fern sprig' },
    instructor: { subject: "A brass magnifying glass resting on a bird's feather", alt: 'A magnifying glass on a feather' },
    cards: [
      'A spiral snail shell on moss',
      'A small bunch of wildflowers',
      'A dragonfly with delicate veined wings',
    ],
  },
});
