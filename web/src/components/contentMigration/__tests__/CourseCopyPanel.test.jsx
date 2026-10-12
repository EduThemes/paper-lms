import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import CourseCopyPanel from '../CourseCopyPanel';

const requestMock = vi.fn();
vi.mock('../../../services/api', () => ({
  request: (...args) => requestMock(...args),
  uploadFile: vi.fn(),
}));

function renderPanel() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <CourseCopyPanel courseId="40" />
    </QueryClientProvider>,
  );
}

describe('CourseCopyPanel', () => {
  beforeEach(() => {
    requestMock.mockReset();
    requestMock.mockImplementation(async (path, opts = {}) => {
      if (path.includes('/copy_sources')) {
        return { data: [{ id: 12, name: 'Biology F25', course_code: 'BIO-F25', label: 'Biology F25 (BIO-F25)' }] };
      }
      if (path.includes('/selectable')) {
        return {
          data: [
            { type: 'course_settings', count: 1 },
            { type: 'pages', count: 2, items: [{ id: 1, title: 'Welcome' }, { id: 2, title: 'Policies' }] },
            { type: 'assignments', count: 1, items: [{ id: 9, title: 'Lab report' }] },
          ],
        };
      }
      if (opts.method === 'POST') {
        return { data: { id: 77, workflow_state: 'queued' } };
      }
      return { data: [] };
    });
  });

  test('copies selected content from a chosen course with shifted dates', async () => {
    renderPanel();
    await screen.findByRole('option', { name: 'Biology F25 (BIO-F25)' });
    fireEvent.change(screen.getByLabelText('Copy from'), { target: { value: '12' } });
    fireEvent.click(screen.getByLabelText('Select specific content'));

    const pages = await screen.findByLabelText(/^Pages/);
    fireEvent.click(pages);

    fireEvent.click(screen.getByLabelText('Adjust events and due dates'));
    fireEvent.change(screen.getByLabelText('New course start'), { target: { value: '2026-08-31' } });

    fireEvent.click(screen.getByRole('button', { name: /Copy course content/ }));

    await waitFor(() => {
      const post = requestMock.mock.calls.find(([, o]) => o && o.method === 'POST');
      expect(post).toBeTruthy();
    });
    const [path, opts] = requestMock.mock.calls.find(([, o]) => o && o.method === 'POST');
    expect(path).toBe('/courses/40/content_migrations');
    const body = JSON.parse(opts.body);
    expect(body.migration_type).toBe('course_copy');
    expect(body.settings).toEqual({ source_course_id: 12, include_only_types: ['pages'] });
    expect(body.date_shift_options.shift_dates).toBe(true);
    expect(body.date_shift_options.new_start_date).toBe('2026-08-31');
    expect(await screen.findByRole('status')).toHaveTextContent(/Course copy started/);
  });

  test('cannot submit before choosing a source', async () => {
    renderPanel();
    await screen.findByRole('option', { name: 'Biology F25 (BIO-F25)' });
    expect(screen.getByRole('button', { name: /Copy course content/ })).toBeDisabled();
  });
});
