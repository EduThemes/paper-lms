import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import CourseStorageCard from '../CourseStorageCard';
import { api } from '../../../services/api';
import { pagesFilesApi } from '../../../services/api_pages_files';

vi.mock('../../../services/api', () => ({ api: { updateCourse: vi.fn() } }));
vi.mock('../../../services/api_pages_files', () => ({ pagesFilesApi: { getCourseQuota: vi.fn() } }));
let role = 'admin';
vi.mock('../../../contexts/AuthContext', () => ({ useAuth: () => ({ user: { id: 1, role } }) }));

function renderCard(props = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <CourseStorageCard courseId="3" course={{ storage_quota_mb: null }} {...props} />
    </QueryClientProvider>,
  );
}

describe('CourseStorageCard (G072)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    role = 'admin';
    pagesFilesApi.getCourseQuota.mockResolvedValue({ quota: 500 * 1024 * 1024, quota_used: 50 * 1024 * 1024 });
  });

  test('shows usage and lets an admin set the quota', async () => {
    api.updateCourse.mockResolvedValue({ id: 3, storage_quota_mb: 1000 });
    const onSaved = vi.fn();
    renderCard({ onSaved });
    expect(await screen.findByText(/50 MB of 500 MB used \(10%\)/)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Course quota (MB)'), { target: { value: '1000' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save quota' }));
    await waitFor(() => expect(api.updateCourse).toHaveBeenCalledWith('3', { storage_quota_mb: 1000 }));
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith({ id: 3, storage_quota_mb: 1000 }));
  });

  test('an empty value resets to the account default', async () => {
    api.updateCourse.mockResolvedValue({ id: 3, storage_quota_mb: null });
    renderCard({ course: { storage_quota_mb: 20 } });
    fireEvent.change(screen.getByLabelText('Course quota (MB)'), { target: { value: '' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save quota' }));
    await waitFor(() => expect(api.updateCourse).toHaveBeenCalledWith('3', { storage_quota_mb: null }));
  });

  test('teachers see usage only', async () => {
    role = 'user';
    renderCard();
    expect(await screen.findByText(/used/)).toBeInTheDocument();
    expect(screen.queryByLabelText('Course quota (MB)')).toBeNull();
  });
});
