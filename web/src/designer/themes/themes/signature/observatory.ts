import { defineTheme } from '../../define';

export default defineTheme({
  id: 'observatory',
  name: 'Observatory',
  description: 'An old observatory painted in candlelit oils: brass telescopes, orreries and celestial globes under an open dome of stars.',
  audiences: ['secondary', 'higher-ed', 'upper-elementary'],
  tags: ['astronomy', 'oil-painting', 'brass', 'science', 'night', 'wonder'],
  defaultLayout: 'split-hero',
  iconStyle: 'duotone',
  corners: 'soft',
  collection: 'signature',
  premium: true,
  style: 'old-master oil still life',
  colourBlind:
    'Brass gold on midnight blue works by lightness and by the blue-yellow axis, which red-green colour blindness keeps intact.',
  palettes: [
    { id: 'midnight', name: 'Midnight Dome', primary: '#141B3A', onPrimary: '#FFFFFF', secondary: '#EBD9A6', onSecondary: '#2A2210', accent: '#D4A84B', surface: '#F1F2F8', onSurface: '#121628', background: '#FFFFFF', text: '#121628', link: '#2C3A80' },
    { id: 'brass-lamp', name: 'Brass Lamp', primary: '#3E2C14', onPrimary: '#FFFFFF', secondary: '#F1E2BF', onSecondary: '#3A2A10', accent: '#4B6CB7', surface: '#FAF5EA', onSurface: '#231A0E', background: '#FFFFFF', text: '#231A0E', link: '#6E4A12' },
    { id: 'twilight', name: 'Twilight Violet', primary: '#2D1F4A', onPrimary: '#FFFFFF', secondary: '#E6DDF3', onSecondary: '#2D1F4A', accent: '#D4A84B', surface: '#F5F2FA', onSurface: '#1C1530', background: '#FFFFFF', text: '#1C1530', link: '#4E3590' },
  ],
  icons: {
    start: 'telescope', syllabus: 'scroll-text', modules: 'orbit', assignments: 'pen-line', discussions: 'messages-square',
    grades: 'star', help: 'circle-help', instructor: 'moon-star', calendar: 'calendar-days', resources: 'library',
  },
  art: {
    medium: 'Rich oil painting in the manner of an old-master still life, warm candlelit chiaroscuro, glowing polished brass and deep night blue, visible fine brushwork',
    palette: 'Deep midnight blue, polished brass and warm candle gold with ivory starlight',
    mood: 'Curious, hushed and full of wonder',
    banner: 'A great brass refracting telescope on a carved wooden mount angled up toward an open dome slit full of stars on the right, the smooth dark blue curve of the dome wall in soft shadow on the left',
    bannerAlt: 'A brass orrery with glowing planets on curved arms beside a celestial globe dotted with gold constellation stars, lit by one candle, on the right, deep calm blue darkness on the left',
    welcome: { subject: 'A celestial globe on a brass stand dotted with gold stars', alt: 'A celestial globe on a brass stand' },
    instructor: { subject: 'A brass spyglass resting beside a lit candle', alt: 'A brass spyglass and a candle' },
    cards: [
      'A brass orrery with a glowing golden sun',
      'A crescent moon seen through a round dome opening',
      'A small brass telescope on a tripod by an open window full of stars',
    ],
  },
});
