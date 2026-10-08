// Course navigation tab visibility (G078). A teacher can hide tabs in
// Course Settings > Navigation; learners then can't open that tab's index
// page, even by typing the URL. Like Canvas, only the index is closed:
// a single page, file or quiz linked from a module still opens. The
// server enforces the same rule on the list endpoints.

// Index route (path under /courses/:id) -> tab id, for the tabs Canvas
// gates. Groups live under the People tab, as in Canvas.
const GATED_INDEX_ROUTES = {
  announcements: 'announcements',
  assignments: 'assignments',
  modules: 'modules',
  people: 'people',
  groups: 'people',
  pages: 'pages',
  files: 'files',
  syllabus: 'syllabus',
  outcomes: 'outcomes',
  quizzes: 'quizzes',
  conferences: 'conferences',
  collaborations: 'collaborations',
};

// courseTabForPath returns the tab id whose index `pathname` is, or null.
// Only the exact index path counts (/courses/7/pages, not
// /courses/7/pages/welcome).
export function courseTabForPath(pathname, courseId) {
  if (!pathname || courseId == null) return null;
  const base = `/courses/${courseId}/`;
  if (!pathname.startsWith(base)) return null;
  const rest = pathname.slice(base.length).replace(/\/+$/, '');
  if (!rest || rest.includes('/')) return null;
  return GATED_INDEX_ROUTES[rest] ?? null;
}

// isCourseTabHidden reads the course's navigation_tabs
// ([{ id, hidden, position }]). No configuration hides nothing.
export function isCourseTabHidden(navTabs, tabId) {
  if (!Array.isArray(navTabs) || !tabId) return false;
  const tab = navTabs.find((t) => t && String(t.id) === tabId);
  return Boolean(tab && tab.hidden === true);
}
