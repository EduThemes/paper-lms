import { describe, expect, it } from 'vitest';
import { COURSE_CODE_PLACEHOLDERS, COURSE_NAME_PLACEHOLDERS, EXAMPLE_EMAIL, courseLabel, docTitle, fillCourseDetails, starterLocale } from '../lib/course';
import { THEMES, createPage } from '../lib/engine';
import { makeDoc } from './fixtures';

const theme = THEMES[0];
const titleOf = (doc) => doc.sections.find((s) => s.type === 'title');
const instructorOf = (doc) => doc.sections.find((s) => s.type === 'instructor');
const course = { name: 'Biology 101', code: 'BIO101' };
const teacher = { name: 'Ms. Example', email: 'teacher@example.test' };

describe('fillCourseDetails', () => {
  it('puts the course name into a new home page title, leaving other placeholders alone', () => {
    const doc = createPage(theme, 'home', 'en');
    expect(titleOf(doc).text).toBe('[Course name]');
    const filled = fillCourseDetails(doc, course);
    expect(titleOf(filled).text).toBe('Biology 101');
    expect(titleOf(filled).subtitle).toContain('[Term]');
    expect(filled.id).toBe(doc.id);
  });

  it('fills the course-code slot and works in Spanish and Portuguese', () => {
    expect(titleOf(fillCourseDetails(createPage(theme, 'overview', 'en'), course)).subtitle).toMatch(/^BIO101 · /);
    for (const locale of ['es', 'pt']) {
      expect(titleOf(fillCourseDetails(createPage(theme, 'home', locale), course)).text).toBe('Biology 101');
      expect(titleOf(fillCourseDetails(createPage(theme, 'overview', locale), course)).subtitle).toMatch(/^BIO101 · /);
    }
    expect(COURSE_NAME_PLACEHOLDERS).toEqual(expect.arrayContaining(['[Course name]', '[Nombre del curso]']));
    expect(COURSE_CODE_PLACEHOLDERS).toContain('[Course code]');
  });

  it('puts the course teacher on the instructor card and in "[Instructor name]" slots', () => {
    const doc = createPage(theme, 'instructor', 'en');
    const filled = fillCourseDetails(doc, course, teacher);
    expect(instructorOf(filled).name).toBe('Ms. Example');
    expect(instructorOf(filled).email).toBe('teacher@example.test');
    const home = fillCourseDetails(createPage(theme, 'home', 'en'), course, teacher);
    expect(titleOf(home).subtitle).toContain('Ms. Example');
    expect(titleOf(home).subtitle).not.toContain('[Instructor name]');
    expect(instructorOf(doc).email).toBe(EXAMPLE_EMAIL);
  });

  it('never touches text the teacher wrote, and is a no-op without details', () => {
    const doc = makeDoc();
    expect(fillCourseDetails(doc, course, teacher)).toBe(doc);
    const fresh = createPage(theme, 'home', 'en');
    expect(fillCourseDetails(fresh, {}, null)).toBe(fresh);
  });
});

describe('labels', () => {
  it('courseLabel joins title and code', () => {
    expect(courseLabel(course)).toBe('Biology 101 · BIO101');
    expect(courseLabel({ name: 'BIO101', code: 'BIO101' })).toBe('BIO101');
    expect(courseLabel({ name: '', code: 'X' })).toBeNull();
  });

  it('docTitle uses the title section, else the fallback', () => {
    expect(docTitle(makeDoc(), 'Home page')).toBe('Biology 101');
    expect(docTitle(makeDoc([{ id: 'd', type: 'divider' }]), 'Home page')).toBe('Home page');
  });

  it('starterLocale maps Paper languages to en/es/pt', () => {
    expect(starterLocale('es-MX')).toBe('es');
    expect(starterLocale('pt')).toBe('pt');
    expect(starterLocale('fr')).toBe('en');
    expect(starterLocale(undefined)).toBe('en');
  });
});
