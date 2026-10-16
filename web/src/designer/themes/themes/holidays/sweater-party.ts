import { defineTheme } from '../../define';

export default defineTheme({
  id: 'sweater-party',
  name: 'Sweater Party',
  description: 'Everything is knitted: chunky reindeer, snowflakes, fir trees and fluffy pom-poms in loud holiday-sweater colours. Made for sweater day and winter spirit days.',
  audiences: ['upper-elementary', 'secondary'],
  tags: ['school-event', 'holiday', 'cultural', 'christmas', 'winter', 'spirit-week', 'knit', 'sweater'],
  collection: 'holidays',
  observance: {
    name: 'Holiday Sweater Day',
    traditions: [],
    months: [12],
    note: 'Usually a Friday in mid-December and often a charity fundraiser. The reindeer, fir trees and snowflakes also suit a winter spirit day; swap the colourway for school colours.',
  },
  defaultLayout: 'card-grid',
  iconStyle: 'solid-circle',
  corners: 'round',
  style: 'chunky knitted-wool illustration',
  colourBlind:
    'Each band is a deep cranberry, teal or navy against pale cream, pink or butter yellow, so pairs differ by lightness; red and green only meet as decoration in the knit.',
  palettes: [
    { id: 'cranberry', name: 'Cranberry Knit', primary: '#8A1C2B', onPrimary: '#FFFFFF', secondary: '#F5E3C8', onSecondary: '#3E2210', accent: '#2F7D5B', surface: '#FCF3EE', onSurface: '#2A1518', background: '#FFFFFF', text: '#2A1518', link: '#9E2234' },
    { id: 'peppermint', name: 'Peppermint', primary: '#14514A', onPrimary: '#FFFFFF', secondary: '#FBE3E6', onSecondary: '#5A1424', accent: '#E85D75', surface: '#EEF7F5', onSurface: '#132A26', background: '#FFFFFF', text: '#132A26', link: '#16655C' },
    { id: 'navy', name: 'Navy & Gold', primary: '#1C2D57', onPrimary: '#FFFFFF', secondary: '#F6E7B8', onSecondary: '#3A2C00', accent: '#E04A4A', surface: '#F1F3F9', onSurface: '#151C2E', background: '#FFFFFF', text: '#151C2E', link: '#2A4590' },
  ],
  icons: {
    start: 'party-popper', syllabus: 'scroll', modules: 'layers', assignments: 'pencil', discussions: 'message-circle',
    grades: 'award', help: 'circle-help', instructor: 'shirt', calendar: 'calendar-days', resources: 'snowflake',
  },
  art: {
    medium: 'Illustration made entirely of chunky knitted wool, every shape built from visible V-shaped knit stitches like a cheerful holiday sweater, with fluffy pom-poms and bright even studio light',
    palette: 'Cranberry red and snowy cream with evergreen, navy blue and sunny gold yarn',
    mood: 'Playful, cheerful and a little silly',
    banner: 'A big knitted reindeer with tall antlers prancing among rows of knitted snowflakes and zigzag knit bands, with fluffy white pom-poms, on the right; a calm, plain cream knitted field with only a few tiny snowflakes on the left',
    bannerAlt: 'Three knitted beanies with fluffy pom-poms and a striped knitted scarf heaped together with tinsel and jingle bells, lying flat with nobody wearing them, on the right; a smooth, plain navy knitted background continuing to the left',
    welcome: { subject: 'A knitted reindeer motif with a fluffy brown pom-pom nose and jingle bells on its antlers', alt: 'A knitted reindeer' },
    instructor: { subject: 'A snowflake-pattern holiday sweater folded neatly on a chair beside a mug of cocoa, with nobody wearing it', alt: 'A folded holiday sweater and a mug of cocoa' },
    cards: [
      'A flat knitted pattern of snowflakes and little fir trees above a row of fluffy pom-poms, with no animals, snowmen, characters or faces',
      'A knitted mug of cocoa topped with white pom-pom marshmallows',
      'Knitted fir trees on a snowy knitted hill under a sky of tiny knitted stars, with no animals, reindeer, snowmen, characters or faces',
    ],
  },
});
