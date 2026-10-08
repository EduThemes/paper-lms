import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import BlueprintLocksPanel from '../BlueprintLocksPanel';

const requestMock = vi.fn();
vi.mock('../../../services/api', () => ({
  request: (...args) => requestMock(...args),
  uploadFile: vi.fn(),
}));

function renderPanel() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <BlueprintLocksPanel courseId="5" />
    </QueryClientProvider>,
  );
}

describe('BlueprintLocksPanel', () => {
  beforeEach(() => {
    requestMock.mockReset();
    requestMock.mockImplementation(async (path, opts = {}) => {
      if (opts.method === 'PUT') return { data: { success: true } };
      return {
        data: {
          default_restrictions: { content: false },
          items: [
            { content_type: 'Assignment', content_id: 3, title: 'Essay', locked: false, explicit: false, restrictions: {} },
            { content_type: 'WikiPage', content_id: 8, title: 'Policies', locked: true, explicit: true, restrictions: { content: true } },
          ],
        },
      };
    });
  });

  test('lists items with their lock state and locks one', async () => {
    renderPanel();
    expect(await screen.findByText('Essay')).toBeInTheDocument();
    expect(screen.getByText(/Page · Locked: content/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Lock Essay' }));
    await waitFor(() => {
      expect(requestMock.mock.calls.some(([, o]) => o && o.method === 'PUT')).toBe(true);
    });
    const [path, opts] = requestMock.mock.calls.find(([, o]) => o && o.method === 'PUT');
    expect(path).toBe('/courses/5/blueprint_templates/default/restrict_item');
    expect(JSON.parse(opts.body)).toEqual({ content_type: 'assignment', content_id: 3, restricted: true });
  });

  test('an item with its own setting can go back to the defaults', async () => {
    renderPanel();
    fireEvent.click(await screen.findByRole('button', { name: 'Use the default restrictions for Policies' }));
    await waitFor(() => {
      expect(requestMock.mock.calls.some(([, o]) => o && o.method === 'PUT')).toBe(true);
    });
    const [, opts] = requestMock.mock.calls.find(([, o]) => o && o.method === 'PUT');
    expect(JSON.parse(opts.body)).toEqual({ content_type: 'wiki_page', content_id: 8, inherit: true });
  });
});
