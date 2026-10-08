import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import PagesPage from '../PagesPage';
import { api } from '../../services/api';

vi.mock('../../services/api', () => ({
  api: {
    getPages: vi.fn(),
    createPage: vi.fn(),
  },
}));

vi.mock('../../hooks/useIsTeacher', () => ({ default: () => false }));
vi.mock('../../hooks/useCrossCourseCheck', () => ({
  default: () => ({ issues: [], checkAndSave: vi.fn(), dismiss: vi.fn(), confirm: vi.fn() }),
}));
vi.mock('../../components/Layout', () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock('../../components/CourseNav', () => ({ default: () => <nav /> }));
vi.mock('../../components/CrossCourseWarningDialog', () => ({ default: () => null }));
vi.mock('../../components/rce/RichContentEditorV2', () => ({ default: () => null }));

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/courses/3/pages']}>
      <Routes>
        <Route path="/courses/:courseId/pages" element={<PagesPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

// B148: courses with more pages than one API page get a pager instead of
// a silently truncated list.
describe('PagesPage pagination', () => {
  beforeEach(() => vi.clearAllMocks());

  test('pages through the course pages', async () => {
    api.getPages.mockImplementation((courseId, page) => Promise.resolve(page === 1
      ? { data: [{ page_id: 1, url: 'week-1', title: 'Week 1' }], pagination: { next: 'n' } }
      : { data: [{ page_id: 2, url: 'week-30', title: 'Week 30' }], pagination: { prev: 'p' } }));

    renderPage();
    expect(await screen.findByText('Week 1')).toBeInTheDocument();
    expect(api.getPages).toHaveBeenCalledWith('3', 1, 50);

    fireEvent.click(screen.getByRole('button', { name: /Next/i }));
    expect(await screen.findByText('Week 30')).toBeInTheDocument();
    expect(api.getPages).toHaveBeenLastCalledWith('3', 2, 50);
  });

  test('no pager when everything fits on one page', async () => {
    api.getPages.mockResolvedValue({ data: [{ page_id: 1, url: 'week-1', title: 'Week 1' }], pagination: {} });
    renderPage();
    expect(await screen.findByText('Week 1')).toBeInTheDocument();
    expect(screen.queryByRole('navigation', { name: /Pages pagination/i })).toBeNull();
  });
});
