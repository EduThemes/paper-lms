/* global vi, describe, test, expect, beforeEach, afterEach */
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import PagesPage from '../PagesPage';
import { api } from '../../services/api';

vi.mock('../../services/api', () => ({
  api: {
    getPages: vi.fn(),
    createPage: vi.fn(),
    updatePage: vi.fn(),
    deletePage: vi.fn(),
  },
}));

let isTeacher = false;
vi.mock('../../hooks/useIsTeacher', () => ({ default: () => isTeacher }));
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

// G074: teachers publish, unpublish and delete pages from the list; the
// front page can be neither unpublished nor deleted.
describe('PagesPage management', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    isTeacher = true;
  });
  afterEach(() => { isTeacher = false; });

  test('publishes a draft and deletes a page', async () => {
    api.getPages.mockResolvedValue({
      data: [
        { page_id: 1, url: 'home', title: 'Home', published: true, front_page: true },
        { page_id: 2, url: 'draft', title: 'Draft', published: false },
      ],
      pagination: {},
    });
    api.updatePage.mockResolvedValue({ page_id: 2, url: 'draft', title: 'Draft', published: true });
    api.deletePage.mockResolvedValue({ delete: true });
    vi.spyOn(window, 'confirm').mockReturnValue(true);

    renderPage();
    expect(await screen.findByText('Draft')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Unpublish Home' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Delete Home' })).toBeDisabled();

    fireEvent.click(screen.getByRole('button', { name: 'Publish Draft' }));
    expect(api.updatePage).toHaveBeenCalledWith('3', 2, { published: true });
    expect(await screen.findByRole('button', { name: 'Unpublish Draft' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Delete Draft' }));
    expect(api.deletePage).toHaveBeenCalledWith('3', 2);
    await waitFor(() => expect(screen.queryByText('Draft')).toBeNull());
  });
});

// Page Designer entry points: "Design a page" for teachers, and a badge on
// pages made with the designer.
describe('PagesPage and the Page Designer', () => {
  beforeEach(() => vi.clearAllMocks());
  afterEach(() => { isTeacher = false; });

  test('teachers get Design a page and see which pages are designed', async () => {
    isTeacher = true;
    api.getPages.mockResolvedValue({
      data: [
        { page_id: 1, url: 'home', title: 'Home', published: true, has_design: true },
        { page_id: 2, url: 'notes', title: 'Notes', published: true, has_design: false },
      ],
      pagination: {},
    });
    renderPage();
    expect(await screen.findByText('Home')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /Design a page/i })).toHaveAttribute('href', '/courses/3/designer');
    expect(screen.getAllByText('Designed')).toHaveLength(1);
    expect(screen.getByRole('link', { name: /Home/ })).toHaveTextContent('Designed');
  });

  test('students see neither', async () => {
    isTeacher = false;
    api.getPages.mockResolvedValue({ data: [{ page_id: 1, url: 'home', title: 'Home', has_design: true }], pagination: {} });
    renderPage();
    expect(await screen.findByText('Home')).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /Design a page/i })).toBeNull();
    expect(screen.queryByText('Designed')).toBeNull();
  });
});
