import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { makeApi, renderRoute } from './harness';
import { stubLocalStorage } from './memoryStorage';
import { PAPER_MODULES } from './fixtures';
import { THEMES } from '../lib/engine';

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

const { default: CourseMakeoverPage } = await import('@/pages/CourseMakeoverPage');
vi.setConfig({ testTimeout: 30000 });

const theme = THEMES[0];
const open = (url = '/courses/42/designer/makeover') => renderRoute(<CourseMakeoverPage />, { path: '/courses/:courseId/designer/makeover', url });

async function pickTheme(user) {
  fireEvent.change(await screen.findByLabelText('Search looks'), { target: { value: theme.name } });
  await user.click((await screen.findAllByRole('button', { name: `Use ${theme.name}` }))[0]);
}

beforeEach(() => {
  stubLocalStorage();
  fake.current = makeApi({ getModules: vi.fn(async () => ({ data: PAPER_MODULES, pagination: {} })) });
});
afterEach(() => vi.unstubAllGlobals());

describe('whole-course makeover', () => {
  it('creates the pages in order, links them, sets Home as the front page and applies the look', async () => {
    const user = userEvent.setup();
    open();
    await pickTheme(user);
    await screen.findByRole('heading', { name: 'Choose the pages to make' });
    // Unpublished modules are offered but not picked by default.
    expect(screen.getByRole('checkbox', { name: 'Draft unit (unpublished)' })).not.toBeChecked();
    await user.click(screen.getByRole('button', { name: 'Create 6 pages' }));
    await screen.findByRole('heading', { name: 'Your course has a new look' }, { timeout: 10000 });

    const { api } = fake.current;
    const made = api.createPage.mock.calls.map((c) => c[1]);
    expect(made).toHaveLength(6);
    expect(made.map((w) => w.design.kind)).toEqual(['start-here', 'instructor', 'module', 'module', 'overview', 'home']);
    const home = made.at(-1);
    expect(home.front_page).toBe(true);
    expect(home.body).toMatch(/href="\/courses\/42\/pages\/[a-z0-9-]+"/);
    expect(api.createModuleItem.mock.calls.map((c) => c[1])).toEqual(['5000', '5001']);
    expect(api.createModuleItem.mock.calls[0][2]).toMatchObject({ type: 'Page', position: 1 });
    expect(api.updateCourse).toHaveBeenCalledWith('42', { default_view: 'wiki' });
    const lookCall = api.updateCourse.mock.calls.find((c) => c[1].look);
    expect(lookCall[1].look).toMatchObject({ version: 1, theme_id: theme.id });
    expect(api.updatePage).not.toHaveBeenCalled();
  });

  it('shows a failed page and retries it', async () => {
    let fail = true;
    const base = fake.current.api.createPage;
    fake.current.api.createPage = vi.fn(async (id, wp) => {
      if (fail && wp.design.kind === 'instructor') throw new Error('Server busy');
      return base(id, wp);
    });
    const user = userEvent.setup();
    open();
    await pickTheme(user);
    await screen.findByRole('heading', { name: 'Choose the pages to make' });
    await user.click(screen.getByRole('button', { name: 'Create 6 pages' }));
    expect(await screen.findByText('Server busy')).toBeInTheDocument();
    expect(screen.getByText(/We stopped because a page couldn’t be created/)).toBeInTheDocument();
    fail = false;
    await user.click(screen.getByRole('button', { name: /^Retry / }));
    await screen.findByRole('heading', { name: 'Your course has a new look' }, { timeout: 10000 });
    expect(fake.current.api.createPage).toHaveBeenCalledTimes(7);
  });

  it('?look=1 applies just the course look', async () => {
    const user = userEvent.setup();
    open('/courses/42/designer/makeover?look=1');
    expect(await screen.findByRole('heading', { level: 1, name: 'Change the course look' })).toBeInTheDocument();
    await pickTheme(user);
    await user.click(await screen.findByRole('button', { name: 'Use this look for the course' }));
    await screen.findByText('The course header uses the new look.');
    const { api } = fake.current;
    expect(api.createPage).not.toHaveBeenCalled();
    expect(api.updateCourse).toHaveBeenCalledWith('42', { look: expect.objectContaining({ version: 1, theme_id: theme.id }) });
    await user.click(screen.getByRole('button', { name: 'Make course pages in this look too' }));
    expect(await screen.findByRole('heading', { name: 'Choose the pages to make' })).toBeInTheDocument();
  });
});
