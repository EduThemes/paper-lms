import { defineTheme } from '../../define';

export default defineTheme({
  id: 'new-year',
  name: 'New Year',
  description: 'Bright flat vector celebration with gold accents: fireworks over a city skyline, confetti, streamers and party hats. For January 1 or the first day of a new semester.',
  audiences: ['secondary', 'upper-elementary', 'early-elementary', 'higher-ed', 'professional', 'specialty'],
  tags: ['school-event', 'new-year', 'new-semester', 'secular', 'celebration', 'fireworks', 'winter'],
  collection: 'special-days',
  observance: {
    name: "New Year's Day",
    traditions: [],
    months: [12, 1],
    note: 'January 1 on the Gregorian calendar, and a natural fresh start for a new semester. Other new years have their own themes: Lunar New Year, Rosh Hashanah and Nowruz.',
  },
  defaultLayout: 'classic-band',
  iconStyle: 'solid-circle',
  corners: 'soft',
  style: 'flat vector with metallic gold',
  colourBlind:
    'Navy, plum and teal bands sit against pale gold, mint or peach with strong lightness contrast; magenta and coral only decorate the art.',
  palettes: [
    { id: 'midnight', name: 'Midnight Gold', primary: '#151B3B', onPrimary: '#FFFFFF', secondary: '#F4D78A', onSecondary: '#3A2A00', accent: '#E35D9C', surface: '#F1F2F8', onSurface: '#151829', background: '#FFFFFF', text: '#151829', link: '#2E3F8F' },
    { id: 'confetti', name: 'Confetti', primary: '#4A1B5C', onPrimary: '#FFFFFF', secondary: '#D9F1F0', onSecondary: '#103A38', accent: '#F2B53A', surface: '#F8F1FA', onSurface: '#221628', background: '#FFFFFF', text: '#221628', link: '#6A2A82' },
    { id: 'fresh-start', name: 'Fresh Start', primary: '#0F4A4A', onPrimary: '#FFFFFF', secondary: '#FBE3C8', onSecondary: '#4A2A08', accent: '#F27F5A', surface: '#EEF7F6', onSurface: '#132624', background: '#FFFFFF', text: '#132624', link: '#145E5E' },
  ],
  icons: {
    start: 'party-popper', syllabus: 'scroll', modules: 'layers', assignments: 'pencil', discussions: 'messages-square',
    grades: 'star', help: 'circle-help', instructor: 'sparkles', calendar: 'calendar-days', resources: 'rocket',
  },
  art: {
    medium: 'Vibrant flat vector illustration with crisp geometric shapes, shimmering metallic gold accents and a fine grain',
    palette: 'Midnight navy and shimmering gold with bright magenta, teal and coral',
    mood: 'Joyful, sparkling and full of fresh starts',
    banner: 'Big bursts of gold, magenta and teal fireworks blooming over a simple city skyline of plain rectangular towers with lit windows and no recognisable landmarks, confetti drifting down, on the right; a calm, deep navy night sky on the left',
    bannerAlt: 'Shiny striped party hats, curling paper streamers and a shower of confetti tumbling across the right side; a calm, plain deep teal background on the left',
    welcome: { subject: 'A shiny striped party hat with a gold pom-pom', alt: 'A party hat' },
    instructor: { subject: 'A fresh open notebook with blank pages and a new pencil beside a little pile of confetti', alt: 'A new notebook and pencil with confetti' },
    cards: [
      'A single gold firework bursting in a dark sky',
      'Curling paper streamers and party horns tumbling through the air with confetti',
      'A bright sunrise over snowy hills on a clear winter morning',
    ],
  },
});
