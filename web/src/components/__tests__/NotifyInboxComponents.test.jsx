import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import AnnouncementReplies from '../AnnouncementReplies';
import NotificationPolicyGrid from '../NotificationPolicyGrid';
import { announcementRepliesApi, notificationPoliciesApi } from '../../services/api_notify_inbox';

vi.mock('../../services/api_notify_inbox', () => ({
  inboxApi: {},
  announcementRepliesApi: { list: vi.fn(), create: vi.fn(), update: vi.fn(), remove: vi.fn() },
  notificationPoliciesApi: { get: vi.fn(), update: vi.fn(), channelTypes: vi.fn() },
}));

const wrap = (ui) => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={client}><MemoryRouter>{ui}</MemoryRouter></QueryClientProvider>);
};

describe('AnnouncementReplies', () => {
  beforeEach(() => vi.clearAllMocks());

  test('renders nothing when comments are off and there are no replies', () => {
    const { container } = wrap(<AnnouncementReplies announcement={{ id: 1, allow_comments: false, reply_count: 0 }} currentUserId={5} canModerate={false} />);
    expect(container.textContent).toBe('');
  });

  test('loads threaded replies on expand and posts a reply as HTML', async () => {
    announcementRepliesApi.list.mockResolvedValue([
      { id: 10, user_id: 6, user_name: 'Sam Student', message: '<p>First!</p>', parent_id: null },
      { id: 11, user_id: 5, user_name: 'Jane Doe', message: '<p>Welcome</p>', parent_id: 10 },
    ]);
    announcementRepliesApi.create.mockResolvedValue({ id: 12 });

    wrap(<AnnouncementReplies announcement={{ id: 1, allow_comments: true, reply_count: 2 }} currentUserId={5} canModerate={false} />);
    expect(announcementRepliesApi.list).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: /2 replies/ }));
    await waitFor(() => expect(screen.getByText('First!')).toBeInTheDocument());
    expect(screen.getByText('Welcome')).toBeInTheDocument();
    // Own reply is editable/deletable; the other is not deletable by a student.
    expect(screen.getAllByRole('button', { name: 'Delete reply' })).toHaveLength(1);

    fireEvent.change(screen.getByLabelText('Write a reply'), { target: { value: 'Me <too>' } });
    fireEvent.click(screen.getAllByRole('button', { name: 'Reply' }).pop());
    await waitFor(() => expect(announcementRepliesApi.create).toHaveBeenCalledWith(1, '<p>Me &lt;too&gt;</p>', undefined));
  });
});

describe('NotificationPolicyGrid', () => {
  beforeEach(() => vi.clearAllMocks());

  const grid = {
    categories: [{ key: 'grading', name: 'Grading', description: 'Your work is graded.' }],
    channels: [
      { id: 0, channel_type: 'email', address: 'jane@example.test', primary: true, confirmed: true, deliverable: true },
      { id: 3, channel_type: 'webhook', address: 'https://hooks.example.test/x', confirmed: true, deliverable: true },
    ],
    policies: [
      { channel_id: 0, category: 'grading', frequency: 'daily', explicit: false },
      { channel_id: 3, category: 'grading', frequency: 'never', explicit: true },
    ],
    frequencies: ['immediately', 'daily', 'weekly', 'never'],
    channel_types: [{ type: 'sms', available: false, reason: 'No SMS provider is configured for this site.' }],
  };

  test('shows inherited and explicit cells and saves one cell on change', async () => {
    notificationPoliciesApi.get.mockResolvedValue(grid);
    notificationPoliciesApi.update.mockResolvedValue(grid);
    wrap(<NotificationPolicyGrid />);

    const emailCell = await screen.findByLabelText('Grading on Email (account): jane@example.test');
    expect(emailCell).toHaveValue('default');
    expect(screen.getByLabelText('Grading on Webhook: https://hooks.example.test/x')).toHaveValue('never');
    expect(screen.getByText(/No SMS provider/)).toBeInTheDocument();

    fireEvent.change(emailCell, { target: { value: 'immediately' } });
    await waitFor(() => expect(notificationPoliciesApi.update).toHaveBeenCalledWith([{ channel_id: 0, category: 'grading', frequency: 'immediately' }]));
  });
});
