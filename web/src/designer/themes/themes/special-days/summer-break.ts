import { defineTheme } from '../../define';

export default defineTheme({
  id: 'summer-break',
  name: 'Last Day & Summer Break',
  description: 'Sun-faded screen prints of beach umbrellas, ice pops and a canoe at sunset. A relaxed send-off for the last weeks of the year.',
  audiences: ['early-elementary', 'upper-elementary', 'secondary'],
  tags: ['school-event', 'last-day', 'summer', 'end-of-year', 'screen-print'],
  collection: 'special-days',
  observance: {
    name: 'Last Day of School',
    traditions: [],
    months: [5, 6, 7, 12],
    regionalMonths: { US: [5, 6, 7], CA: [5, 6, 7], GB: [6, 7], IE: [5, 6, 7], AU: [12], NZ: [12] },
    note: 'Northern hemisphere: May to July. Southern hemisphere: the long break starts in December.',
  },
  defaultLayout: 'classic-band',
  iconStyle: 'soft-square',
  corners: 'round',
  style: 'sun-faded screen print',
  colourBlind:
    'Ocean teal and butter yellow carry the structure; coral is a warm accent that never stands in for a meaning against green.',
  palettes: [
    { id: 'ocean', name: 'Ocean Teal', primary: '#0D4D57', onPrimary: '#FFFFFF', secondary: '#FFE08A', onSecondary: '#3A2C00', accent: '#FF7F6E', surface: '#FFF8EC', onSurface: '#13262A', background: '#FFFFFF', text: '#13262A', link: '#0D5A66' },
    { id: 'coral', name: 'Coral Sunset', primary: '#8A2E28', onPrimary: '#FFFFFF', secondary: '#FFD6C9', onSecondary: '#5A1C16', accent: '#2EC4B6', surface: '#FFF4EF', onSurface: '#2A1A18', background: '#FFFFFF', text: '#2A1A18', link: '#9A3329' },
    { id: 'lagoon', name: 'Lagoon', primary: '#1F3F7A', onPrimary: '#FFFFFF', secondary: '#BFEAF5', onSecondary: '#12384A', accent: '#FFC75F', surface: '#EEF8FB', onSurface: '#16212E', background: '#FFFFFF', text: '#16212E', link: '#1F4C8F' },
  ],
  icons: {
    start: 'sun', syllabus: 'list-checks', modules: 'layers', assignments: 'pencil', discussions: 'message-circle',
    grades: 'star', help: 'circle-help', instructor: 'umbrella', calendar: 'calendar-days', resources: 'tree-palm',
  },
  art: {
    medium: 'Sun-faded screen-print illustration with flat overlapping inks, soft halftone and a warm vintage paper grain; the image runs off every edge with no border',
    palette: 'Ocean teal and coral with sunny butter yellow on a sandy cream ground',
    mood: 'Relaxed, sunny and free',
    banner: 'A big striped beach umbrella, a beach ball and a dripping ice pop on golden sand with gentle waves on the right, calm open sea and sky on the left',
    bannerAlt: 'A big setting sun over a quiet lake with a wooden dock and an empty canoe on the right, calm still water and sky on the left',
    welcome: { subject: 'A dripping two-colour ice pop', alt: 'An ice pop' },
    instructor: { subject: 'A pair of sunglasses resting on a folded striped beach towel', alt: 'Sunglasses on a beach towel' },
    cards: [
      'A sandcastle with a little seashell on top',
      'An empty pair of flip-flops on warm sand, nobody wearing them',
      'A diamond kite flying high over the dunes',
    ],
  },
});
