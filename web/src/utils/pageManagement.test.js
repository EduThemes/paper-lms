import { describe, test, expect } from 'vitest';
import {
  normalizeEditingRoles, editingRolesLabel, canDeletePage, canUnpublishPage,
  editFormFromPage, buildPageUpdate,
} from './pageManagement';

describe('page management (G074)', () => {
  test('editing roles normalize onto the three choices', () => {
    expect(normalizeEditingRoles(undefined)).toBe('teachers');
    expect(normalizeEditingRoles('teachers')).toBe('teachers');
    expect(normalizeEditingRoles('teachers, students')).toBe('teachers,students');
    expect(normalizeEditingRoles('members')).toBe('teachers,students');
    expect(normalizeEditingRoles('teachers,students,public')).toBe('teachers,students,public');
    expect(editingRolesLabel('public')).toBe('Anyone');
  });

  test('the front page cannot be deleted or unpublished', () => {
    expect(canDeletePage({ front_page: true })).toBe(false);
    expect(canUnpublishPage({ front_page: true })).toBe(false);
    expect(canDeletePage({ front_page: false })).toBe(true);
    expect(canDeletePage(null)).toBe(false);
  });

  test('students send only the title and body', () => {
    const form = editFormFromPage({ title: 'Notes', body: '<p>x</p>', editing_roles: 'teachers,students', todo_date: null });
    expect(buildPageUpdate(form, { isTeacher: false })).toEqual({ title: 'Notes', body: '<p>x</p>' });
  });

  test('teachers send roles and the to-do date, and can clear it', () => {
    const form = editFormFromPage({ title: 'Notes', body: '', editing_roles: 'teachers', todo_date: '2026-11-02T15:00:00Z' });
    const update = buildPageUpdate(form, { isTeacher: true });
    expect(update.editing_roles).toBe('teachers');
    expect(update.todo_date).toBe('2026-11-02T15:00:00.000Z');
    expect(buildPageUpdate({ ...form, todoDate: '' }, { isTeacher: true }).todo_date).toBeNull();
  });
});
