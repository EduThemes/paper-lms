import { defineTheme } from '../../define';

export default defineTheme({
  id: 'spirit-week',
  name: 'Spirit Week',
  description: 'Loud retro pep graphics: megaphones, plain pennants, pom-poms and mismatched socks. Built for dress-up days and pep rallies.',
  audiences: ['secondary', 'upper-elementary'],
  tags: ['school-event', 'spirit-week', 'pep-rally', 'homecoming', 'retro'],
  collection: 'special-days',
  observance: {
    name: 'Spirit Week',
    traditions: [],
    months: [9, 10, 2],
    note: 'Often homecoming week in autumn, or a winter spirit week. Swap the colourway for your school colours.',
  },
  defaultLayout: 'card-grid',
  iconStyle: 'solid-circle',
  corners: 'round',
  style: 'bold retro screen-print pep graphics',
  colourBlind:
    'Purple and gold are a blue-yellow pair that stays distinct for red-green colour blindness; coral is decorative only.',
  palettes: [
    { id: 'purple-gold', name: 'Purple & Gold', primary: '#3D1A6E', onPrimary: '#FFFFFF', secondary: '#FFC72C', onSecondary: '#2A1A00', accent: '#FF6B6B', surface: '#F6F0FF', onSurface: '#221833', background: '#FFFFFF', text: '#221833', link: '#5A2BA0' },
    { id: 'coral', name: 'Coral Rally', primary: '#8A2A1E', onPrimary: '#FFFFFF', secondary: '#FFD3C4', onSecondary: '#5A1A10', accent: '#2EC4B6', surface: '#FFF3EE', onSurface: '#2B1A16', background: '#FFFFFF', text: '#2B1A16', link: '#9A3320' },
    { id: 'teal', name: 'Teal Thunder', primary: '#0E4D5C', onPrimary: '#FFFFFF', secondary: '#B6F0E8', onSecondary: '#0B3A44', accent: '#FFD23F', surface: '#EEFAF8', onSurface: '#12262A', background: '#FFFFFF', text: '#12262A', link: '#0F5A6B' },
  ],
  icons: {
    start: 'megaphone', syllabus: 'list-checks', modules: 'layers', assignments: 'shirt', discussions: 'messages-square',
    grades: 'trophy', help: 'circle-help', instructor: 'flag-triangle-right', calendar: 'calendar-days', resources: 'sparkles',
  },
  art: {
    medium: 'Bold retro flat illustration with thick rounded outlines, halftone dot shading and a slightly worn screen-print texture that runs off every edge with no border',
    palette: 'Punchy electric purple and bright gold with hot coral on a warm cream ground',
    mood: 'Loud, proud and bursting with team pride',
    banner: 'A big megaphone bursting with plain triangular pennants with no markings, curly streamers and bright pom-poms on the right, a calm plain cream area on the left',
    bannerAlt: 'A playful heap of wildly mismatched striped socks, oversized sunglasses and a sparkly party hat on the right, calm open cream on the left',
    welcome: { subject: 'A bright megaphone with curly streamers', alt: 'A megaphone with streamers' },
    instructor: { subject: 'A plain gold trophy cup with ribbons tied to its handles', alt: 'A trophy cup with ribbons' },
    cards: [
      'A pile of fluffy colourful pom-poms',
      'An empty pair of chunky high-top canvas sneakers with plain sides (no logos, swooshes, stripes or badges) and mismatched laces, nobody wearing them',
      'A burst of streamers and confetti',
    ],
  },
});
