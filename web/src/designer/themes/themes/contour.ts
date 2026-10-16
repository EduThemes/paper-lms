import { defineTheme, FONTS } from '../define';

export default defineTheme({
  id: 'contour',
  name: 'Contour',
  description: 'Minimal topographic contour lines gathering into peaks, valleys and lakes. Spacious and analytical.',
  audiences: ['higher-ed', 'professional'],
  tags: ['topographic', 'minimal', 'line-art', 'analytical'],
  defaultLayout: 'timeline',
  iconStyle: 'outline',
  headingFont: FONTS.lato,
  corners: 'soft',
  style: 'topographic contour lines',
  colourBlind:
    'Glacier teal and warm sand differ in lightness; any data meaning would be carried by labels, never by colour alone.',
  palettes: [
    { id: 'glacier', name: 'Glacier', primary: '#1F3B57', onPrimary: '#FFFFFF', secondary: '#CDE7E3', onSecondary: '#12403B', accent: '#E4A35C', surface: '#F1F5F7', onSurface: '#18232E', background: '#FFFFFF', text: '#18232E', link: '#1F4C73' },
    { id: 'desert', name: 'Desert', primary: '#5A3825', onPrimary: '#FFFFFF', secondary: '#F3DEC4', onSecondary: '#4A2A14', accent: '#4F9DA6', surface: '#F8F3EE', onSurface: '#271C15', background: '#FFFFFF', text: '#271C15', link: '#7A4422' },
    { id: 'night', name: 'Night Survey', primary: '#1E1F3B', onPrimary: '#FFFFFF', secondary: '#D9D6F5', onSecondary: '#2A2560', accent: '#5CC8C0', surface: '#F2F2F8', onSurface: '#18192B', background: '#FFFFFF', text: '#18192B', link: '#3B3A8A' },
  ],
  icons: {
    start: 'mountain', syllabus: 'file-text', modules: 'layers', assignments: 'square-pen', discussions: 'messages-square',
    grades: 'chart-line', help: 'circle-help', instructor: 'user-round', calendar: 'calendar-days', resources: 'map',
  },
  art: {
    medium: 'Minimal topographic contour-line art with smooth flowing isolines and soft tonal fills between the lines',
    palette: 'Deep slate blue and glacier teal with warm sand on pale grey',
    mood: 'Spacious, analytical and serene',
    banner: 'Flowing contour lines that tighten into a large mountain summit on the right, sparse widely spaced lines fading out to plain pale grey on the left',
    bannerAlt: 'Contour lines shaping a broad river valley and a round lake on the right, sparse widely spaced lines fading out to plain pale grey on the left',
    welcome: { subject: 'A small island drawn as concentric contour rings', alt: '' },
    instructor: { subject: 'A hiking compass resting on a few contour lines', alt: 'A hiking compass' },
    cards: [
      'Concentric contour rings of a single hilltop',
      'Contour lines following a winding valley',
      'Contour ripples around a still lake',
    ],
  },
});
