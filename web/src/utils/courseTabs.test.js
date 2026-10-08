import { courseTabForPath, isCourseTabHidden } from './courseTabs';

describe('courseTabForPath (G078)', () => {
  test('maps a gated index route to its tab', () => {
    expect(courseTabForPath('/courses/7/pages', '7')).toBe('pages');
    expect(courseTabForPath('/courses/7/files/', '7')).toBe('files');
    expect(courseTabForPath('/courses/7/groups', '7')).toBe('groups');
    expect(courseTabForPath('/courses/7/discussions', '7')).toBe('discussions');
    expect(courseTabForPath('/courses/7/quizzes', 7)).toBe('quizzes');
  });

  test('leaves single items, the home page and ungated tabs open', () => {
    expect(courseTabForPath('/courses/7/pages/welcome', '7')).toBeNull();
    expect(courseTabForPath('/courses/7', '7')).toBeNull();
    expect(courseTabForPath('/courses/7/', '7')).toBeNull();
    expect(courseTabForPath('/courses/7/discussions/12', '7')).toBeNull();
    expect(courseTabForPath('/courses/7/grades', '7')).toBeNull();
    expect(courseTabForPath('/courses/70/pages', '7')).toBeNull();
    expect(courseTabForPath('/courses/7/pages', undefined)).toBeNull();
  });
});

describe('isCourseTabHidden (G078)', () => {
  const tabs = [
    { id: 'home', hidden: false, position: 0 },
    { id: 'pages', hidden: true, position: 1 },
    { id: 'files', position: 2 },
  ];
  test('reads the hidden flag', () => {
    expect(isCourseTabHidden(tabs, 'pages')).toBe(true);
    expect(isCourseTabHidden(tabs, 'files')).toBe(false);
    expect(isCourseTabHidden(tabs, 'quizzes')).toBe(false);
  });
  test('the groups index closes with its own tab or with People', () => {
    expect(isCourseTabHidden([{ id: 'groups', hidden: true }], 'groups')).toBe(true);
    expect(isCourseTabHidden([{ id: 'people', hidden: true }, { id: 'groups', hidden: false }], 'groups')).toBe(true);
    expect(isCourseTabHidden([{ id: 'people', hidden: false }, { id: 'groups', hidden: false }], 'groups')).toBe(false);
    expect(isCourseTabHidden([{ id: 'groups', hidden: true }], 'people')).toBe(false);
  });
  test('no configuration hides nothing', () => {
    expect(isCourseTabHidden(null, 'pages')).toBe(false);
    expect(isCourseTabHidden([], 'pages')).toBe(false);
    expect(isCourseTabHidden('[]', 'pages')).toBe(false);
  });
});
