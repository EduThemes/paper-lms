/* global vi, describe, test, expect, beforeEach */
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import CoursePage from '../CoursePage';
import { api } from '../../services/api';

vi.mock('../../services/api', () => ({
  api: {
    getCourse: vi.fn(),
    getModules: vi.fn(),
    getAssignments: vi.fn(),
    getQuizzes: vi.fn(),
    getCourseAnnouncements: vi.fn(),
    getPages: vi.fn(),
    getEnrollments: vi.fn(),
    getCourseFrontPage: vi.fn(),
  },
}));

let isTeacher = false;
vi.mock('../../hooks/useIsTeacher', () => ({ default: () => isTeacher }));
vi.mock('../../contexts/AuthContext', () => ({ useAuth: () => ({ user: { id: 1 } }) }));
vi.mock('../../hooks/useDocumentTitle', () => ({ default: () => {} }));
vi.mock('../../components/Layout', () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock('../../components/CourseNav', () => ({ default: () => <nav aria-label="Course" /> }));
vi.mock('../../components/CourseStaffBar', () => ({ default: () => null }));

const empty = { data: [] };

function renderCourse() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/courses/4']}>
        <Routes>
          <Route path="/courses/:courseId" element={<CoursePage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const designedFront = {
  page_id: 30,
  url: 'home',
  title: 'Welcome home',
  body: '<div class="paper-design"><h2>Hello, class</h2></div>',
  scoped_css: '',
  design: { version: 1, id: 'doc' },
};

describe('CoursePage course home and the Page Designer', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    isTeacher = false;
    for (const fn of ['getModules', 'getAssignments', 'getQuizzes', 'getCourseAnnouncements', 'getPages', 'getEnrollments']) {
      api[fn].mockResolvedValue(empty);
    }
    try { window.localStorage.clear(); } catch { /* no storage */ }
  });

  test('a designed front page spans the full width, outside the two-column grid', async () => {
    api.getCourse.mockResolvedValue({ id: 4, name: 'Biology', default_view: 'wiki' });
    api.getCourseFrontPage.mockResolvedValue(designedFront);
    const { container } = renderCourse();

    const region = await screen.findByRole('region', { name: 'Welcome home' });
    expect(screen.getByRole('heading', { name: 'Hello, class' })).toBeInTheDocument();
    // Not inside the 2/3 main column.
    expect(region.closest('.md\\:col-span-2')).toBeNull();
    expect(container.querySelector('.md\\:col-span-2 [data-testid="designed-front-page"]')).toBeNull();
    // Up next still renders, below.
    expect(screen.getAllByText('Up Next').length).toBeGreaterThan(0);
    // Requested once even though the page and the view share the query.
    expect(api.getCourseFrontPage).toHaveBeenCalledTimes(1);
  });

  test('an ordinary front page stays in the main column', async () => {
    api.getCourse.mockResolvedValue({ id: 4, name: 'Biology', default_view: 'wiki' });
    api.getCourseFrontPage.mockResolvedValue({ page_id: 31, url: 'home', title: 'Plain home', body: '<p>Plain body</p>', design: null });
    const { container } = renderCourse();
    expect(await screen.findByText('Plain body')).toBeInTheDocument();
    expect(container.querySelector('.md\\:col-span-2').textContent).toContain('Plain body');
    expect(screen.queryByTestId('designed-front-page')).toBeNull();
  });

  test('teachers without a front-page home are invited to design one, and can dismiss it', async () => {
    isTeacher = true;
    api.getCourse.mockResolvedValue({ id: 4, name: 'Biology', default_view: 'modules' });
    renderCourse();

    const title = await screen.findByRole('heading', { name: 'Design your home page' });
    expect(title).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /Design your home page/i }))
      .toHaveAttribute('href', '/courses/4/designer?kind=home');
    expect(screen.getByRole('link', { name: /Give the whole course a makeover/i }))
      .toHaveAttribute('href', '/courses/4/designer/makeover');
    expect(api.getCourseFrontPage).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: 'Dismiss' }));
    await waitFor(() => expect(screen.queryByRole('heading', { name: 'Design your home page' })).toBeNull());
  });

  test('teachers see the invitation when the front-page home has no page yet', async () => {
    isTeacher = true;
    api.getCourse.mockResolvedValue({ id: 4, name: 'Biology', default_view: 'wiki' });
    api.getCourseFrontPage.mockRejectedValue(Object.assign(new Error('not found'), { status: 404 }));
    renderCourse();
    expect(await screen.findByRole('heading', { name: 'Design your home page' })).toBeInTheDocument();
  });

  test('students never see the invitation', async () => {
    api.getCourse.mockResolvedValue({ id: 4, name: 'Biology', default_view: 'modules' });
    renderCourse();
    expect(await screen.findByRole('heading', { level: 1, name: 'Biology' })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Design your home page' })).toBeNull();
  });
});
