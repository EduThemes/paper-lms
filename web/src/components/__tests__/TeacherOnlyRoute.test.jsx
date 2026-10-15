/* global vi, describe, test, expect */
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import TeacherOnlyRoute from '../TeacherOnlyRoute';

let isTeacher = null;
vi.mock('../../hooks/useIsTeacher', () => ({ default: () => isTeacher }));

function renderAt() {
  return render(
    <MemoryRouter initialEntries={['/courses/5/designer']}>
      <Routes>
        <Route path="/courses/:courseId" element={<p>Course home</p>} />
        <Route path="/courses/:courseId/designer" element={<TeacherOnlyRoute><p>Designer</p></TeacherOnlyRoute>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('TeacherOnlyRoute', () => {
  test('waits with a spinner while the role loads', () => {
    isTeacher = null;
    renderAt();
    expect(screen.getByRole('status', { name: 'Loading' })).toBeInTheDocument();
    expect(screen.queryByText('Designer')).toBeNull();
  });

  test('lets course staff in', () => {
    isTeacher = true;
    renderAt();
    expect(screen.getByText('Designer')).toBeInTheDocument();
  });

  test('sends students to the course home', () => {
    isTeacher = false;
    renderAt();
    expect(screen.getByText('Course home')).toBeInTheDocument();
    expect(screen.queryByText('Designer')).toBeNull();
  });
});
