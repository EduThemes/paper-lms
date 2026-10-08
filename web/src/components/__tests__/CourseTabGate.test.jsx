import React from 'react';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import CourseTabGate from '../CourseTabGate';
import { useCourseUI } from '../../contexts/CourseUIContext';

vi.mock('../../contexts/CourseUIContext', () => ({ useCourseUI: vi.fn() }));
vi.mock('../Layout', () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock('../CourseNav', () => ({ default: () => <nav aria-label="Course navigation" /> }));

const hiddenPages = {
  id: 7,
  navigation_tabs: [
    { id: 'pages', hidden: true, position: 0 },
    { id: 'files', hidden: false, position: 1 },
  ],
};

function renderAt(path, ctx) {
  useCourseUI.mockReturnValue({ course: hiddenPages, loading: false, isStaff: false, ...ctx });
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/courses/:courseId/*" element={<CourseTabGate><p>page content</p></CourseTabGate>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('CourseTabGate (G078)', () => {
  test('a learner gets the disabled notice on a hidden tab index', () => {
    renderAt('/courses/7/pages');
    expect(screen.getByText(/That page has been disabled for this course/i)).toBeInTheDocument();
    expect(screen.queryByText('page content')).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: /course home/i })).toHaveAttribute('href', '/courses/7');
  });

  test('a single page under a hidden tab still opens', () => {
    renderAt('/courses/7/pages/welcome');
    expect(screen.getByText('page content')).toBeInTheDocument();
  });

  test('a visible tab is not gated', () => {
    renderAt('/courses/7/files');
    expect(screen.getByText('page content')).toBeInTheDocument();
  });

  test('staff see hidden tabs', () => {
    renderAt('/courses/7/pages', { isStaff: true });
    expect(screen.getByText('page content')).toBeInTheDocument();
  });

  test('nothing renders for a gated index while the course loads', () => {
    renderAt('/courses/7/pages', { loading: true, course: null });
    expect(screen.queryByText('page content')).not.toBeInTheDocument();
    expect(screen.queryByText(/disabled/i)).not.toBeInTheDocument();
  });
});
