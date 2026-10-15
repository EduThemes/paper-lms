import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import axe from 'axe-core';
import { makeApi, renderRoute } from './harness';
import { stubLocalStorage } from './memoryStorage';
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
vi.mock('@/components/Layout', () => ({ default: ({ children }) => <main>{children}</main> }));
vi.mock('@/hooks/useIsTeacher', () => ({ default: () => true }));
vi.mock('@/contexts/AuthContext', () => ({ useAuth: () => ({ user: { id: 1 } }) }));

const { default: PageDesignerPage } = await import('@/pages/PageDesignerPage');
vi.setConfig({ testTimeout: 60000 });

/** axe in jsdom: structure, names and ARIA. Colour contrast needs layout, so it runs in the real-browser axe job. */
async function violations(container) {
  const res = await axe.run(container, {
    rules: { 'color-contrast': { enabled: false }, region: { enabled: false } },
    resultTypes: ['violations'],
    iframes: false,
  });
  return res.violations.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(' ')).slice(0, 3).join(' | ')}`);
}

beforeEach(() => {
  stubLocalStorage();
  fake.current = makeApi();
});
afterEach(() => vi.unstubAllGlobals());

describe('accessibility (axe)', () => {
  it('gallery, customize (every tab) and finish have no axe violations', async () => {
    const user = userEvent.setup();
    const { container } = renderRoute(<PageDesignerPage />, { path: '/courses/:courseId/designer', url: '/courses/42/designer' });
    const search = await screen.findByLabelText('Search looks');
    fireEvent.change(search, { target: { value: THEMES[0].name } });
    await screen.findAllByRole('button', { name: `Use ${THEMES[0].name}` });
    expect(await violations(container)).toEqual([]);

    await user.click(screen.getAllByRole('button', { name: `Use ${THEMES[0].name}` })[0]);
    await screen.findByRole('heading', { name: 'Sections on this page' });
    expect(await violations(container)).toEqual([]);

    await user.click(screen.getByRole('button', { name: /^Edit Quick links/ }));
    expect(await violations(container)).toEqual([]);

    for (const tab of ['Look', /Check/]) {
      await user.click(screen.getByRole('tab', { name: tab }));
      expect(await violations(container)).toEqual([]);
    }

    await user.click(screen.getByRole('button', { name: /Next: Finish/ }));
    expect(await violations(container)).toEqual([]);
  });
});
