// Page management helpers (G074): editing roles, to-do date and the
// update payload a page editor sends.

import { dateTimeLocalToISO, isoToDateTimeLocal } from './datetimeLocal';

// Canvas's three "Users allowed to edit this page" choices.
export const EDITING_ROLE_OPTIONS = [
  { value: 'teachers', label: 'Only teachers' },
  { value: 'teachers,students', label: 'Teachers and students' },
  { value: 'teachers,students,public', label: 'Anyone' },
];

// normalizeEditingRoles maps any stored value onto one of the options.
export function normalizeEditingRoles(roles) {
  const set = new Set(String(roles || 'teachers').split(',').map((r) => r.trim()).filter(Boolean));
  if (set.has('public')) return 'teachers,students,public';
  if (set.has('students') || set.has('members')) return 'teachers,students';
  return 'teachers';
}

export function editingRolesLabel(roles) {
  const value = normalizeEditingRoles(roles);
  return EDITING_ROLE_OPTIONS.find((o) => o.value === value)?.label || 'Only teachers';
}

// The front page can be neither deleted nor unpublished (Canvas rule,
// enforced by the server too).
export const canDeletePage = (page) => Boolean(page) && !page.front_page;
export const canUnpublishPage = (page) => Boolean(page) && !page.front_page;

// editFormFromPage seeds the edit form from a page payload.
export function editFormFromPage(page) {
  return {
    title: page?.title || '',
    body: page?.body || '',
    editingRoles: normalizeEditingRoles(page?.editing_roles),
    todoDate: isoToDateTimeLocal(page?.todo_date),
  };
}

// buildPageUpdate returns the wiki_page fields to PUT. A student with
// edit rights may only send the title and body; staff also send the
// editing roles and to-do date (an emptied date clears it).
export function buildPageUpdate(form, { isTeacher }) {
  const update = { title: form.title, body: form.body };
  if (isTeacher) {
    update.editing_roles = normalizeEditingRoles(form.editingRoles);
    update.todo_date = form.todoDate ? dateTimeLocalToISO(form.todoDate) || null : null;
  }
  return update;
}
