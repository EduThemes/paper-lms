/**
 * Course details applied to new pages: the course name and code go into the starter placeholders, and the
 * course's teacher into the instructor card. Values are untrusted plain text; they only go into plain-text
 * fields, which the engine escapes on render. Everything the teacher typed is left alone.
 */
import { starterStrings } from './engine';

const LOCALES = ['en', 'es', 'pt'];

/** The starter locale for a Paper UI language: en, es or pt (else en). */
export function starterLocale(language) {
  const base = String(language ?? 'en').split('-')[0].toLowerCase();
  return LOCALES.includes(base) ? base : 'en';
}

const unique = (xs) => [...new Set(xs.filter(Boolean))];

/** "[Course name]" in every starter locale, read from the engine so they can't drift. */
export const COURSE_NAME_PLACEHOLDERS = unique(LOCALES.map((l) => starterStrings(l)['common.courseTitle']));
/** The course-code slot in the starter subtitles ("[Course code] · [Credits] · [Term]"). */
export const COURSE_CODE_PLACEHOLDERS = ['[Course code]', '[Código]'];
/** "[Your name]" on instructor cards, every locale. */
export const INSTRUCTOR_NAME_PLACEHOLDERS = unique(LOCALES.map((l) => starterStrings(l)['common.instructorName']));
/** "[Instructor name]" in starter subtitles, every locale. */
export const INSTRUCTOR_SUBTITLE_PLACEHOLDERS = ['[Instructor name]', '[Nombre del profesor]', '[Nome do professor]'];
/** The example address the starter puts on instructor cards. */
export const EXAMPLE_EMAIL = 'instructor@example.edu';

const replaceAll = (text, tokens, value) => tokens.reduce((out, token) => out.split(token).join(value), text);

/**
 * For a NEW page: course name and code into the title placeholders; the teacher's name and email into the
 * instructor card and "[Instructor name]" slots. Pure; returns the same doc when nothing changes.
 *
 * course: { name, code }   teacher: { name, email } | null
 */
export function fillCourseDetails(doc, course = {}, teacher = null) {
  const title = String(course.name ?? '').trim() || null;
  const code = String(course.code ?? '').trim() || null;
  const tName = String(teacher?.name ?? '').trim() || null;
  const tEmail = String(teacher?.email ?? '').trim() || null;
  if (!title && !code && !tName && !tEmail) return doc;
  const fill = (text) => {
    let out = text;
    if (title) out = replaceAll(out, COURSE_NAME_PLACEHOLDERS, title);
    if (code) out = replaceAll(out, COURSE_CODE_PLACEHOLDERS, code);
    if (tName) out = replaceAll(out, INSTRUCTOR_SUBTITLE_PLACEHOLDERS, tName);
    return out;
  };
  let changed = false;
  const sections = doc.sections.map((s) => {
    if (s.type === 'title') {
      const text = fill(s.text);
      const subtitle = s.subtitle === undefined ? undefined : fill(s.subtitle);
      if (text === s.text && subtitle === s.subtitle) return s;
      changed = true;
      return { ...s, text, ...(subtitle === undefined ? {} : { subtitle }) };
    }
    if (s.type === 'instructor') {
      const patch = {};
      if (tName && INSTRUCTOR_NAME_PLACEHOLDERS.includes(s.name)) patch.name = tName;
      if (tEmail && s.email === EXAMPLE_EMAIL) patch.email = tEmail;
      if (!Object.keys(patch).length) return s;
      changed = true;
      return { ...s, ...patch };
    }
    return s;
  });
  return changed ? { ...doc, sections } : doc;
}

/** Header label: "Biology 101 · BIO101", or just the title. Null without a course name. */
export function courseLabel(course = {}) {
  const title = String(course.name ?? '').trim();
  if (!title) return null;
  const code = String(course.code ?? '').trim();
  return code && code !== title ? `${title} · ${code}` : title;
}

/** Title for the saved page: the title section's text, else the given fallback (the page kind's name). */
export function docTitle(doc, fallback = '') {
  const title = doc?.sections?.find((s) => s.type === 'title');
  const text = title ? String(title.text ?? '').trim() : '';
  return text || fallback;
}
