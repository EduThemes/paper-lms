import { defineTheme, FONTS } from '../define';

export default defineTheme({
  id: 'care-line',
  name: 'Care Line',
  description: 'Clean, calm medical illustration: stethoscopes, heartbeat lines and molecules. For nursing, allied health and health science.',
  audiences: ['specialty', 'higher-ed'],
  tags: ['healthcare', 'nursing', 'clinical', 'calm'],
  defaultLayout: 'sidebar',
  iconStyle: 'solid-circle',
  headingFont: FONTS.lato,
  corners: 'round',
  style: 'clean clinical flat illustration',
  colourBlind:
    'Teal and coral differ in lightness; clinical states such as normal and abnormal must never be shown by colour alone.',
  palettes: [
    { id: 'teal', name: 'Clinical Teal', primary: '#0F4C5C', onPrimary: '#FFFFFF', secondary: '#CDEFF0', onSecondary: '#0F4C5C', accent: '#FF8A7A', surface: '#EFF7F8', onSurface: '#172A30', background: '#FFFFFF', text: '#172A30', link: '#0F5466' },
    { id: 'lilac', name: 'Lilac Ward', primary: '#3E2F6B', onPrimary: '#FFFFFF', secondary: '#E8E2FA', onSecondary: '#3E2F6B', accent: '#5CC2B4', surface: '#F5F3FC', onSurface: '#1E1A2E', background: '#FFFFFF', text: '#1E1A2E', link: '#4A3A82' },
    { id: 'navy', name: 'Navy Scrubs', primary: '#14325A', onPrimary: '#FFFFFF', secondary: '#FFE0D6', onSecondary: '#5A2414', accent: '#49B6C4', surface: '#F0F4F9', onSurface: '#16202C', background: '#FFFFFF', text: '#16202C', link: '#1A4A85' },
  ],
  icons: {
    start: 'heart-pulse', syllabus: 'clipboard-list', modules: 'layers', assignments: 'clipboard-check', discussions: 'messages-square',
    grades: 'activity', help: 'circle-help', instructor: 'stethoscope', calendar: 'calendar-days', resources: 'book-open',
  },
  art: {
    medium: 'Clean flat medical illustration with smooth rounded shapes, thin even outlines and soft gradients',
    palette: 'Deep teal and soft aqua with a warm coral accent on clean white',
    mood: 'Calm, caring and professional',
    banner: 'A large stethoscope curving around a soft heart shape with a smooth heartbeat line running beneath it on the right, quiet plain white-aqua space on the left',
    bannerAlt: 'A large microscope, a molecule model of linked spheres and a small potted plant on the right, the same clean white background continuing into a quiet empty area on the left, with no tinted panel, gradient box or seam',
    welcome: { subject: 'A soft heart shape with a smooth heartbeat line through it', alt: '' },
    instructor: { subject: 'A stethoscope laid in a loose loop', alt: 'A stethoscope' },
    cards: [
      'A DNA double helix',
      'A microscope beside a glass slide',
      'A molecule model of linked spheres',
    ],
  },
});
