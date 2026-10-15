import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { COURSE, makeApi, renderRoute } from './harness';
import { stubLocalStorage } from './memoryStorage';
import { THEMES } from '../lib/engine';
import { renderForSave } from '../lib/savePage';
import { makeDoc } from './fixtures';

const fake = vi.hoisted(() => ({ current: null }));

vi.mock('@/services/api', () => ({
  get api() {
    return fake.current.api;
  },
  request: (...a) => fake.current.request(...a),
  requestPaged: (...a) => fake.current.requestPaged(...a),
  ApiError: class ApiError extends Error {},
}));
vi.mock('@/components/Layout', () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock('@/hooks/useIsTeacher', () => ({ default: () => true }));
vi.mock('@/contexts/AuthContext', () => ({ useAuth: () => ({ user: { id: 1 } }) }));

const { default: PageDesignerPage } = await import('@/pages/PageDesignerPage');

vi.setConfig({ testTimeout: 30000 });

const theme = THEMES[0];
const open = (url = '/courses/42/designer?kind=home') => renderRoute(<PageDesignerPage />, { path: '/courses/:courseId/designer', url });

async function pickFirstTheme(user) {
  const search = await screen.findByLabelText('Search looks');
  fireEvent.change(search, { target: { value: theme.name } });
  const buttons = await screen.findAllByRole('button', { name: `Use ${theme.name}` });
  await user.click(buttons[0]);
  await screen.findByRole('heading', { name: 'Sections on this page' });
}

beforeEach(() => {
  stubLocalStorage();
  fake.current = makeApi();
});
afterEach(() => vi.unstubAllGlobals());

describe('choose → customize → finish', () => {
  it('starts from the gallery, fills the course details, and saves a new page with its design', async () => {
    const user = userEvent.setup();
    open();
    expect(await screen.findByRole('heading', { level: 1, name: /Page Designer/ })).toBeInTheDocument();
    await pickFirstTheme(user);

    // The outline lists the starter sections; the course name went into the title.
    expect(screen.getByRole('button', { name: /^Edit Course title: Biology 101/ })).toBeInTheDocument();
    // Custom CSS is off for this course: no Advanced tab.
    expect(screen.queryByRole('tab', { name: 'Advanced' })).toBeNull();

    await user.click(screen.getByRole('button', { name: /Next: Finish/ }));
    expect(screen.getByLabelText(/Page title/)).toHaveValue('Biology 101');
    await user.click(screen.getByRole('button', { name: 'Save page' }));

    await screen.findByText('“Biology 101” was saved.');
    const { api } = fake.current;
    expect(api.createPage).toHaveBeenCalledTimes(1);
    const [courseId, wp] = api.createPage.mock.calls[0];
    expect(courseId).toBe('42');
    expect(wp).toMatchObject({ title: 'Biology 101', published: false, front_page: false });
    expect(wp.design).toMatchObject({ version: 1, kind: 'home', themeId: theme.id });
    expect(wp.body).toContain(`data-pd-hash`);
    expect(wp.body).toContain('href="/courses/42/');
    expect(wp.css).toContain(':root');
    expect(api.updateCourse).not.toHaveBeenCalled();
    expect(screen.getByRole('link', { name: /Open the page/ })).toHaveAttribute('href', '/courses/42/pages/biology-101');

    // Saving again updates the same page instead of making another.
    await user.click(screen.getByRole('button', { name: 'Save changes' }));
    await screen.findByText('“Biology 101” was updated.');
    expect(api.createPage).toHaveBeenCalledTimes(1);
    expect(api.updatePage).toHaveBeenCalledWith('42', 'biology-101', expect.objectContaining({ title: 'Biology 101' }));
  });

  it('sets the page as the course home page: published front page and default_view wiki', async () => {
    const user = userEvent.setup();
    open();
    await pickFirstTheme(user);
    await user.click(screen.getByRole('button', { name: /Next: Finish/ }));
    await user.click(screen.getByRole('switch', { name: 'Make it the course home page' }));
    expect(screen.getByRole('switch', { name: /Publish now/ })).toBeChecked();
    await user.click(screen.getByRole('button', { name: 'Save page' }));
    await screen.findByText('It’s now the course home page.');
    const wp = fake.current.api.createPage.mock.calls[0][1];
    expect(wp).toMatchObject({ front_page: true, published: true });
    expect(fake.current.api.updateCourse).toHaveBeenCalledWith('42', { default_view: 'wiki' });
    // The course header takes the page's look (the course had none).
    await screen.findByText('The course header now uses this look.');
    const lookCall = fake.current.api.updateCourse.mock.calls.find(([, c]) => 'look' in c);
    expect(lookCall[1].look).toMatchObject({ theme_id: expect.any(String), colors: { primary: expect.stringMatching(/^#[0-9a-f]{6}$/) } });
  });

  it('leaves the course header alone when the teacher turns the look off', async () => {
    const user = userEvent.setup();
    open();
    await pickFirstTheme(user);
    await user.click(screen.getByRole('button', { name: /Next: Finish/ }));
    await user.click(screen.getByRole('switch', { name: 'Make it the course home page' }));
    await user.click(screen.getByRole('switch', { name: 'Use this look for the course header' }));
    await user.click(screen.getByRole('button', { name: 'Save page' }));
    await screen.findByText('It’s now the course home page.');
    expect(fake.current.api.updateCourse.mock.calls.some(([, c]) => 'look' in c)).toBe(false);
  });

  it('deletes a section with an undo toast, and Cmd/Ctrl+Z brings it back', async () => {
    const user = userEvent.setup();
    open();
    await pickFirstTheme(user);
    const outline = screen.getByRole('heading', { name: 'Sections on this page' }).closest('section');
    const before = within(outline).getAllByRole('listitem').length;
    await user.click(within(outline).getByRole('button', { name: /^Delete Course title/ }));
    expect(within(outline).getAllByRole('listitem')).toHaveLength(before - 1);
    expect(screen.getByText('“Course title: Biology 101” was deleted.')).toBeInTheDocument();
    act(() => {
      fireEvent.keyDown(document.body, { key: 'z', ctrlKey: true });
    });
    expect(within(outline).getAllByRole('listitem')).toHaveLength(before);
    act(() => {
      fireEvent.keyDown(document.body, { key: 'z', ctrlKey: true, shiftKey: true });
    });
    expect(within(outline).getAllByRole('listitem')).toHaveLength(before - 1);
  });

  it('edits a section and changes the look on the Look tab', async () => {
    const user = userEvent.setup();
    open();
    await pickFirstTheme(user);
    await user.click(screen.getByRole('button', { name: /^Edit Course title: Biology 101/ }));
    const title = screen.getByLabelText(/^Title/);
    await user.clear(title);
    await user.type(title, 'Cells and Life');
    await user.click(screen.getByRole('button', { name: 'Back to all sections' }));
    expect(screen.getByRole('button', { name: /^Edit Course title: Cells and Life/ })).toBeInTheDocument();
    await user.click(screen.getByRole('tab', { name: 'Look' }));
    await user.selectOptions(screen.getByLabelText('Layout'), 'timeline');
    await user.click(screen.getByRole('tab', { name: /Check/ }));
    expect(screen.getByRole('heading', { name: 'Accessibility check' })).toBeInTheDocument();
  });

  it('shows the Custom CSS panel when the course allows it and saves the CSS after the design’s', async () => {
    fake.current.api.getCourse.mockResolvedValue({ ...COURSE, custom_css_enabled: true });
    const user = userEvent.setup();
    open();
    await pickFirstTheme(user);
    await user.click(screen.getByRole('tab', { name: 'Advanced' }));
    fireEvent.change(screen.getByLabelText('CSS for this page'), { target: { value: ':root h2{letter-spacing:.02em}' } });
    await user.click(screen.getByRole('button', { name: /Next: Finish/ }));
    await user.click(screen.getByRole('button', { name: 'Save page' }));
    await screen.findByText('“Biology 101” was saved.');
    const wp = fake.current.api.createPage.mock.calls[0][1];
    expect(wp.css.trim().endsWith(':root h2{letter-spacing:.02em}')).toBe(true);
    expect(wp.design.customCss).toBe(':root h2{letter-spacing:.02em}');
  });

  it('starts K-2 courses with the K-2 audience filter', async () => {
    fake.current.api.getCourse.mockResolvedValue({ ...COURSE, ui_mode: 'k2' });
    open();
    expect(await screen.findByRole('radio', { name: 'K-2' })).toBeChecked();
  });
});

describe('editing a saved page', () => {
  const savedPage = (over = {}) => {
    const r = renderForSave({ ...makeDoc(), themeId: theme.id }, { courseId: '42', customCssEnabled: false });
    return { page_id: 5, url: 'biology-101', title: 'Biology 101', body: r.html, css: r.css, design: r.doc, published: true, front_page: true, ...over };
  };

  it('opens a designed page in Customize and saves back to it', async () => {
    fake.current.api.getPage.mockResolvedValue(savedPage());
    const user = userEvent.setup();
    open('/courses/42/designer?page=biology-101');
    await screen.findByRole('heading', { name: 'Sections on this page' });
    expect(screen.queryByText('This page was changed outside the designer')).toBeNull();
    await user.click(screen.getByRole('button', { name: /Next: Finish/ }));
    expect(screen.getByRole('switch', { name: 'Make it the course home page' })).toBeChecked();
    await user.click(screen.getByRole('button', { name: 'Save changes' }));
    await screen.findByText('“Biology 101” was updated.');
    expect(fake.current.api.createPage).not.toHaveBeenCalled();
    expect(fake.current.api.updatePage.mock.calls[0][1]).toBe('biology-101');
  });

  it('warns when the HTML was changed elsewhere', async () => {
    const p = savedPage();
    fake.current.api.getPage.mockResolvedValue({ ...p, body: p.body.replace('Biology 101', 'Edited by hand') });
    open('/courses/42/designer?page=biology-101');
    expect(await screen.findByText('This page was changed outside the designer')).toBeInTheDocument();
  });

  it('never opens a page that has no design', async () => {
    fake.current.api.getPage.mockResolvedValue({ page_id: 6, url: 'syllabus-notes', title: 'Notes', body: '<p>Mine</p>', design: null });
    open('/courses/42/designer?page=syllabus-notes');
    expect(await screen.findByText('This page wasn’t made in the Page Designer')).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Sections on this page' })).toBeNull();
  });
});
