import { defineTheme } from '../../define';

export default defineTheme({
  id: 'santas-workshop',
  name: "Santa's Workshop",
  description: "Storybook 3D toy workshop: a bench of carved wooden toys, stockings, milk and cookies, ordinary brown reindeer and a sleigh silhouetted against the moon. Santa is a carved-wood storybook figure seen from behind, a sleigh against the moon, or boots vanishing up the chimney.",
  audiences: ['early-elementary', 'upper-elementary'],
  tags: ['holiday', 'cultural', 'christmas', 'santa', 'winter', 'toys', 'secular'],
  collection: 'holidays',
  observance: {
    name: 'Christmas',
    traditions: ['Christian', 'secular Christmas'],
    months: [12, 1],
    note: 'December 25; Orthodox churches that keep the Julian calendar celebrate on January 7. A secular Santa-and-toys look with no religious imagery. Not every family does Santa, so keep the page copy light.',
  },
  defaultLayout: 'playful',
  iconStyle: 'solid-circle',
  corners: 'round',
  style: 'storybook 3D toy workshop render',
  colourBlind:
    'Cranberry and pine green are never the only distinction: every band pairs a deep colour with pale cream or ice blue (a light-dark contrast), and green appears only as a decorative accent.',
  palettes: [
    { id: 'workshop', name: 'Workshop Red', primary: '#7A1E22', onPrimary: '#FFFFFF', secondary: '#F4DDA6', onSecondary: '#3A2400', accent: '#2E6B4A', surface: '#FBF3EA', onSurface: '#2A1714', background: '#FFFFFF', text: '#2A1714', link: '#9A2328' },
    { id: 'moonlight', name: 'Moonlit Sleigh', primary: '#183A5A', onPrimary: '#FFFFFF', secondary: '#D8E8F5', onSecondary: '#10253D', accent: '#C8373D', surface: '#F0F5FA', onSurface: '#15202C', background: '#FFFFFF', text: '#15202C', link: '#1F5385' },
    { id: 'cookies', name: 'Milk & Cookies', primary: '#5A3420', onPrimary: '#FFFFFF', secondary: '#F3E1C7', onSecondary: '#3A2210', accent: '#C8373D', surface: '#FAF4EC', onSurface: '#2A1B12', background: '#FFFFFF', text: '#2A1B12', link: '#7A4520' },
  ],
  icons: {
    start: 'star', syllabus: 'scroll', modules: 'blocks', assignments: 'pencil', discussions: 'message-circle',
    grades: 'award', help: 'circle-help', instructor: 'hammer', calendar: 'calendar-days', resources: 'gift',
  },
  art: {
    medium: 'Warm storybook 3D render of a miniature toy workshop with chunky carved wooden toys, matte painted wood, felt and knitted wool, soft rounded shapes and glowing lamplight',
    palette: 'Cranberry red and pine green with warm honey wood, cream and soft gold light',
    mood: 'Cheerful, cosy and magical',
    banner: 'Santa Claus as a round, jolly carved-wood storybook figure in a red suit with white fur trim and a red hat, seen from behind as he works at a long wooden workbench crowded with wooden toys, a rocking horse, spinning tops, a toy train with plain unmarked carriages and stacks of presents wrapped in plain paper, under a warm hanging lamp with curls of wood shavings, on the right; the same softly lit plain wooden workshop wall continuing calmly on the left',
    bannerAlt: 'A snowy village of small rooftops and chimneys at night with a huge glowing full moon on the right, the tiny dark silhouette of a sleigh pulled by a line of reindeer flying across the bright disc of the moon; a calm deep blue starry sky over the snowy rooftops on the left',
    welcome: { subject: 'A red velvet hat with white fur trim and a pom-pom resting on top of a bulging sack of wrapped presents, with nobody wearing it', alt: "Santa's hat on a sack of presents" },
    instructor: { subject: 'A glass of milk and a plate of cookies left out on a brick hearth, with a carrot for the reindeer', alt: 'Milk, cookies and a carrot left by the fireplace' },
    cards: [
      'A wooden toy train with plain unmarked carriages carrying tiny wrapped presents along a track through soft felt snow hills',
      'Two ordinary brown reindeer with dark noses and velvety antlers resting in fresh snow beside a little red barn at twilight, with no elves, people, dolls or other figures anywhere',
      'A brick fireplace with only the soles and heels of a pair of black boots with white fur cuffs disappearing up into the chimney, nothing above the fur cuffs visible, a little soot falling onto the hearth and plain knitted stockings hanging on the mantel',
    ],
  },
});
