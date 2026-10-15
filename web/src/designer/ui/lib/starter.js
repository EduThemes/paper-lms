/** Starting a page: the page kinds, and the doc a gallery card shows (and "Use this look" opens). */
import { createPage } from './engine';
import { fillCourseDetails } from './course';
import { paperizeDoc } from './sections';

export const KINDS = ['home', 'start-here', 'overview', 'module', 'instructor'];

/** A new page in a theme: starter content with the course's name, code and teacher, Paper link targets. */
export function starterDoc({ theme, kind, locale, layout, paletteId, palette, course, teacher }) {
  const doc = paperizeDoc(fillCourseDetails(createPage(theme, kind, locale, layout || undefined), course, teacher));
  return { ...doc, paletteId: paletteId || doc.paletteId, ...(palette ? { palette } : {}) };
}

/** "5 minutes ago", "on 10/2/2026", via the designer's translate function. */
export function timeAgo(iso, t) {
  const then = new Date(iso);
  if (isNaN(then.getTime())) return '';
  const mins = Math.round((Date.now() - then.getTime()) / 60000);
  if (mins < 1) return t('time.justNow');
  if (mins < 60) return t('time.minutes', { count: mins });
  const hours = Math.round(mins / 60);
  if (hours < 24) return t('time.hours', { count: hours });
  return t('time.on', { date: then.toLocaleDateString() });
}

/** Which "why it matters" text fits an accessibility issue, from its id and message. */
export function whyKey(issue) {
  const s = `${issue.id} ${issue.message}`.toLowerCase();
  if (/this course|course isn|\{course\}|course id/.test(s)) return 'checkWhy.course';
  if (/alt|image|photo|picture|describ/.test(s)) return 'checkWhy.alt';
  if (/contrast|colou?r/.test(s)) return 'checkWhy.contrast';
  if (/heading/.test(s)) return 'checkWhy.heading';
  if (/link|button text|click here/.test(s)) return 'checkWhy.link';
  if (/empty|blank|missing|no label/.test(s)) return 'checkWhy.empty';
  return 'checkWhy.default';
}

/** An image address the teacher may use: https, or a file in this course. */
export const isOwnImageSrc = (src) => /^(https:\/\/\S+|\/api\/v1\/files\/\d+\/download)$/i.test(String(src ?? '').trim());
