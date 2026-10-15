/* global vi, describe, test, expect, beforeEach */
// Readers (students, observers, anonymous public viewers) get a designed
// page as { has_design: true, body, css, scoped_css } and never `design`
// (only course staff do). Designed pages must still render as designed for
// them, and nothing teacher-only may key off `design`.
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import PageDetailPage from '../PageDetailPage';
import PublicPageView from '../PublicPageView';
import { api } from '../../services/api';
import { hasPageStyling, isDesignedPage } from '../../utils/pageDesign';

vi.mock('../../services/api', () => ({
  api: {
    getPage: vi.fn(),
    getPublicPage: vi.fn(),
    updatePage: vi.fn(),
    deletePage: vi.fn(),
  },
}));

let isTeacher = false;
vi.mock('../../hooks/useIsTeacher', () => ({ default: () => isTeacher }));
vi.mock('../../contexts/CourseUIContext', () => ({ useCourseUI: () => ({ course: { id: 3, custom_css_enabled: true } }) }));
vi.mock('../../hooks/useUnsavedChanges', () => ({ default: () => {} }));
vi.mock('../../hooks/useCrossCourseCheck', () => ({
  default: () => ({ issues: [], checkAndSave: (_body, save) => save(), dismiss: vi.fn(), confirm: vi.fn() }),
}));
vi.mock('../../components/Layout', () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock('../../components/CourseNav', () => ({ default: () => <nav /> }));
vi.mock('../../components/CrossCourseWarningDialog', () => ({ default: () => null }));
vi.mock('../../components/rce/rceDrafts', () => ({ clearRceDraft: vi.fn() }));
vi.mock('../../components/rce/RichContentEditorV2', () => ({
  default: ({ value }) => <div data-testid="tiptap-editor">{value}</div>,
}));

// The reader payload: no `design` key at all. Made-up content.
const readerPage = {
  page_id: 21,
  url: 'start-here',
  title: 'Start here',
  body: '<div id="pd-doc-21" class="paper-design" data-pd-doc="doc-21" data-pd-v="1"><h2>Welcome, explorers</h2></div>',
  css: ':root{--x:#123456}',
  scoped_css: '#paper-page-21{--x:#123456}',
  has_design: true,
  published: true,
  editing_roles: 'teachers',
};

function renderAt(path, url, element) {
  return render(
    <MemoryRouter initialEntries={[url]}>
      <Routes>
        <Route path={path} element={element} />
      </Routes>
    </MemoryRouter>,
  );
}

function expectDesigned(container) {
  const root = container.querySelector('#paper-page-21');
  expect(root).not.toBeNull();
  expect(root).toHaveAttribute('data-designed-page');
  expect(root.parentElement).toHaveClass('paper-page-frame');
  expect(root.firstElementChild).toHaveClass('paper-design');
  // Rendered raw: no prose wrapper around the design.
  expect(container.querySelector('.prose')).toBeNull();
  expect(container.querySelector('style').textContent).toBe('#paper-page-21{--x:#123456}');
}

describe('pageDesign helpers with the reader payload', () => {
  test('has_design alone marks a designed page', () => {
    expect(isDesignedPage(readerPage)).toBe(true);
    expect(isDesignedPage({ has_design: false, design: null })).toBe(false);
    expect(isDesignedPage({ design: { version: 1, id: 'd' } })).toBe(true);
    expect(isDesignedPage(null)).toBe(false);
    expect(hasPageStyling({ has_design: true, scoped_css: '' })).toBe(true);
    expect(hasPageStyling({ has_design: false, scoped_css: '' })).toBe(false);
  });
});

describe('PageDetailPage for readers', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    isTeacher = false;
  });

  test('a student sees the design rendered as designed, with no edit actions', async () => {
    api.getPage.mockResolvedValue(readerPage);
    const { container } = renderAt('/courses/:courseId/pages/:slug', '/courses/3/pages/start-here', <PageDetailPage />);
    expect(await screen.findByRole('heading', { name: 'Welcome, explorers' })).toBeInTheDocument();
    expectDesigned(container);
    expect(screen.queryByRole('link', { name: /Edit design/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /Edit/ })).toBeNull();
  });

  test('a student editor (can_edit) still cannot open a designed page in the rich text editor', async () => {
    api.getPage.mockResolvedValue({ ...readerPage, can_edit: true, editing_roles: 'teachers,students' });
    renderAt('/courses/:courseId/pages/:slug', '/courses/3/pages/start-here', <PageDetailPage />);
    expect(await screen.findByRole('heading', { name: 'Welcome, explorers' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /History/ })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Edit/ })).toBeNull();
    expect(screen.queryByTestId('tiptap-editor')).toBeNull();
  });

  test('teachers get Edit design from useIsTeacher, whichever field marks the page', async () => {
    isTeacher = true;
    api.getPage.mockResolvedValue(readerPage);
    renderAt('/courses/:courseId/pages/:slug', '/courses/3/pages/start-here', <PageDetailPage />);
    expect(await screen.findByRole('link', { name: /Edit design/i })).toHaveAttribute('href', '/courses/3/designer?page=start-here');
  });
});

describe('PublicPageView for anonymous viewers', () => {
  beforeEach(() => vi.clearAllMocks());

  test('renders a designed public page as designed without `design`', async () => {
    api.getPublicPage.mockResolvedValue(readerPage);
    const { container } = renderAt('/public/courses/:courseId/pages/:slug', '/public/courses/3/pages/start-here', <PublicPageView />);
    expect(await screen.findByRole('heading', { name: 'Welcome, explorers' })).toBeInTheDocument();
    expectDesigned(container);
    expect(screen.queryByRole('link', { name: /Edit design/i })).toBeNull();
  });

  test('website mode too', async () => {
    api.getPublicPage.mockResolvedValue({ ...readerPage, website_mode: true });
    const { container } = renderAt('/public/courses/:courseId/pages/:slug', '/public/courses/3/pages/start-here', <PublicPageView />);
    expect(await screen.findByRole('heading', { name: 'Welcome, explorers' })).toBeInTheDocument();
    expectDesigned(container);
  });
});
