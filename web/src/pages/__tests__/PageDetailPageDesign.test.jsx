/* global vi, describe, test, expect, beforeEach */
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import PageDetailPage from '../PageDetailPage';
import { api } from '../../services/api';

vi.mock('../../services/api', () => ({
  api: {
    getPage: vi.fn(),
    updatePage: vi.fn(),
    deletePage: vi.fn(),
  },
}));

let isTeacher = true;
let course = { id: 3, custom_css_enabled: true };
vi.mock('../../hooks/useIsTeacher', () => ({ default: () => isTeacher }));
vi.mock('../../contexts/CourseUIContext', () => ({ useCourseUI: () => ({ course }) }));
vi.mock('../../hooks/useUnsavedChanges', () => ({ default: () => {} }));
vi.mock('../../hooks/useCrossCourseCheck', () => ({
  default: () => ({ issues: [], checkAndSave: (_body, save) => save(), dismiss: vi.fn(), confirm: vi.fn() }),
}));
vi.mock('../../components/Layout', () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock('../../components/CourseNav', () => ({ default: () => <nav /> }));
vi.mock('../../components/CrossCourseWarningDialog', () => ({ default: () => null }));
vi.mock('../../components/rce/rceDrafts', () => ({ clearRceDraft: vi.fn() }));
// The rich text editor is a marker: designed pages must never reach it.
vi.mock('../../components/rce/RichContentEditorV2', () => ({
  default: ({ value }) => <div data-testid="tiptap-editor">{value}</div>,
}));

const designedPage = {
  page_id: 21,
  url: 'start-here',
  title: 'Start here',
  body: '<a href="/courses/3/modules"><div class="card"><h3>Modules</h3></div></a><div class="spacer"></div>',
  css: ':root{--x:#123456}',
  scoped_css: '#paper-page-21{--x:#123456}',
  design: { version: 1, id: 'doc-21' },
  published: true,
  editing_roles: 'teachers',
};

const ordinaryPage = {
  page_id: 22,
  url: 'syllabus-notes',
  title: 'Syllabus notes',
  body: '<p>Read me.</p>',
  css: '',
  scoped_css: '',
  design: null,
  published: true,
  editing_roles: 'teachers',
};

function renderPage(slug) {
  return render(
    <MemoryRouter initialEntries={[`/courses/3/pages/${slug}`]}>
      <Routes>
        <Route path="/courses/:courseId/pages/:slug" element={<PageDetailPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('PageDetailPage with designed pages', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    isTeacher = true;
    course = { id: 3, custom_css_enabled: true };
  });

  test('renders the design raw and offers Edit design as the primary action', async () => {
    api.getPage.mockResolvedValue(designedPage);
    const { container } = renderPage('start-here');
    expect(await screen.findByRole('heading', { level: 1, name: 'Start here' })).toBeInTheDocument();
    expect(container.querySelector('#paper-page-21')).not.toBeNull();
    expect(container.querySelector('.prose')).toBeNull();
    expect(screen.getByRole('link', { name: /Edit design/i }))
      .toHaveAttribute('href', '/courses/3/designer?page=start-here');
    expect(screen.queryByRole('button', { name: /^Edit$/ })).toBeNull();
  });

  test('Edit as HTML asks first, never opens the rich text editor, and saves design: null', async () => {
    api.getPage.mockResolvedValue(designedPage);
    api.updatePage.mockResolvedValue({ ...designedPage, body: '<p>New</p>', design: null });
    renderPage('start-here');

    fireEvent.click(await screen.findByRole('button', { name: /Edit as HTML/i }));
    const dialog = await screen.findByRole('dialog', { name: /Edit this page as HTML/i });
    expect(dialog).toHaveTextContent(/detached from its design/i);
    expect(screen.queryByTestId('tiptap-editor')).toBeNull();

    // Cancel keeps the page as it is.
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expect(screen.queryByLabelText('HTML')).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: /Edit as HTML/i }));
    const again = await screen.findByRole('dialog', { name: /Edit this page as HTML/i });
    fireEvent.click(within(again).getByRole('button', { name: 'Edit as HTML' }));

    const html = await screen.findByLabelText('HTML');
    expect(html.value).toBe(designedPage.body);
    expect(screen.queryByTestId('tiptap-editor')).toBeNull();

    fireEvent.change(html, { target: { value: '<p>New</p>' } });
    fireEvent.click(screen.getByRole('button', { name: /Save Page/i }));
    await waitFor(() => expect(api.updatePage).toHaveBeenCalled());
    const [, , payload] = api.updatePage.mock.calls[0];
    expect(payload.design).toBeNull();
    expect(payload.body).toBe('<p>New</p>');
    // After detaching, the page is ordinary: the usual Edit button is back.
    expect(await screen.findByRole('button', { name: /^Edit$/ })).toBeInTheDocument();
  });

  test('student editors cannot edit a designed page here', async () => {
    isTeacher = false;
    api.getPage.mockResolvedValue({ ...designedPage, can_edit: true });
    renderPage('start-here');
    expect(await screen.findByRole('heading', { level: 1, name: 'Start here' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Edit/ })).toBeNull();
    expect(screen.queryByRole('link', { name: /Edit design/i })).toBeNull();
  });
});

describe('PageDetailPage page CSS panel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    isTeacher = true;
    course = { id: 3, custom_css_enabled: true };
  });

  test('teachers edit an ordinary page\'s CSS under Advanced and it is saved as css', async () => {
    api.getPage.mockResolvedValue(ordinaryPage);
    api.updatePage.mockResolvedValue({ ...ordinaryPage, css: 'p{color:red}', scoped_css: '#paper-page-22 p{color:red}' });
    renderPage('syllabus-notes');

    fireEvent.click(await screen.findByRole('button', { name: /^Edit$/ }));
    expect(screen.getByTestId('tiptap-editor')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /Advanced/ }));
    const css = screen.getByLabelText(/Page CSS/);
    expect(screen.getByText(/follows Paper.s dark mode/i)).toBeInTheDocument();
    fireEvent.change(css, { target: { value: 'p{color:red}' } });
    fireEvent.click(screen.getByRole('button', { name: /Save Page/i }));

    await waitFor(() => expect(api.updatePage).toHaveBeenCalled());
    const [, , payload] = api.updatePage.mock.calls[0];
    expect(payload.css).toBe('p{color:red}');
    expect('design' in payload).toBe(false);
  });

  test('unchanged CSS is not sent', async () => {
    api.getPage.mockResolvedValue(ordinaryPage);
    api.updatePage.mockResolvedValue(ordinaryPage);
    renderPage('syllabus-notes');
    fireEvent.click(await screen.findByRole('button', { name: /^Edit$/ }));
    fireEvent.click(screen.getByRole('button', { name: /Save Page/i }));
    await waitFor(() => expect(api.updatePage).toHaveBeenCalled());
    expect('css' in api.updatePage.mock.calls[0][2]).toBe(false);
  });

  test('hidden when the account turned hand-written CSS off', async () => {
    course = { id: 3, custom_css_enabled: false };
    api.getPage.mockResolvedValue(ordinaryPage);
    renderPage('syllabus-notes');
    fireEvent.click(await screen.findByRole('button', { name: /^Edit$/ }));
    expect(screen.queryByRole('button', { name: /Advanced/ })).toBeNull();
  });

  test('hidden for student editors', async () => {
    isTeacher = false;
    api.getPage.mockResolvedValue({ ...ordinaryPage, can_edit: true, editing_roles: 'teachers,students' });
    renderPage('syllabus-notes');
    fireEvent.click(await screen.findByRole('button', { name: /^Edit$/ }));
    expect(screen.getByTestId('tiptap-editor')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Advanced/ })).toBeNull();
  });

  test('an ordinary page with CSS is shown scoped', async () => {
    api.getPage.mockResolvedValue({ ...ordinaryPage, css: 'p{color:red}', scoped_css: '#paper-page-22 p{color:red}' });
    const { container } = renderPage('syllabus-notes');
    expect(await screen.findByText('Read me.')).toBeInTheDocument();
    expect(container.querySelector('#paper-page-22 .reading-surface')).not.toBeNull();
    expect(container.querySelector('style').textContent).toBe('#paper-page-22 p{color:red}');
  });
});
