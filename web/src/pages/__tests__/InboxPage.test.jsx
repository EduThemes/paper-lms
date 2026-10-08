import React from 'react';
import { render, screen, waitFor, fireEvent, act, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import InboxPage, { loadLatestMessages, pageFromLink } from '../InboxPage';
import { inboxApi } from '../../services/api_notify_inbox';

vi.mock('../../services/api_notify_inbox', () => ({
  inboxApi: {
    listConversations: vi.fn(),
    getMessages: vi.fn(),
    markRead: vi.fn(),
    unreadCount: vi.fn(),
    contexts: vi.fn(),
    searchRecipients: vi.fn(),
    createConversation: vi.fn(),
    addMessage: vi.fn(),
    batchUpdate: vi.fn(),
    forward: vi.fn(),
    uploadAttachment: vi.fn(),
  },
}));

vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => ({ user: { id: 50, name: 'Jane Doe', role: 'user' }, loading: false }),
}));

vi.mock('../../components/Layout', () => ({
  default: ({ children }) => <div>{children}</div>,
}));

const link = (last, next) => ({
  last: `http://localhost/api/v1/x?page=${last}&per_page=50`,
  ...(next ? { next: `http://localhost/api/v1/x?page=${next}&per_page=20` } : {}),
});

const renderPage = () => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter><InboxPage /></MemoryRouter>
    </QueryClientProvider>,
  );
};

const conv = (over = {}) => ({
  id: 1,
  subject: 'First',
  workflow_state: 'read',
  starred: false,
  participants: [{ id: 50, name: 'Jane Doe' }, { id: 40, name: 'Pat Teacher' }],
  message_count: 1,
  ...over,
});

describe('InboxPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    inboxApi.unreadCount.mockResolvedValue(0);
    inboxApi.contexts.mockResolvedValue([]);
    inboxApi.markRead.mockResolvedValue({});
    inboxApi.batchUpdate.mockResolvedValue({ updated: 1 });
    inboxApi.getMessages.mockResolvedValue({ data: [], pagination: {} });
  });

  test('pageFromLink reads the page number', () => {
    expect(pageFromLink('http://h/api?page=3&per_page=50')).toBe(3);
    expect(pageFromLink(undefined)).toBeNull();
  });

  // B154: a 60-message thread opens on its newest messages (page 2), not
  // the oldest 50.
  test('loadLatestMessages fetches the last page of a long thread', async () => {
    const page1 = Array.from({ length: 50 }, (_, i) => ({ id: i + 1 }));
    const page2 = Array.from({ length: 10 }, (_, i) => ({ id: i + 51 }));
    inboxApi.getMessages.mockImplementation(async (_id, page) => (
      page === 1 ? { data: page1, pagination: link(2) } : { data: page2, pagination: link(2) }
    ));

    const { items, oldestPage } = await loadLatestMessages(9);
    expect(items[items.length - 1].id).toBe(60);
    expect(items.length).toBe(60);
    expect(oldestPage).toBe(1);
  });

  test('loadLatestMessages keeps "load earlier" available for very long threads', async () => {
    inboxApi.getMessages.mockImplementation(async (_id, page) => ({
      data: Array.from({ length: 50 }, (_, i) => ({ id: (page - 1) * 50 + i + 1 })),
      pagination: link(5),
    }));
    const { items, oldestPage } = await loadLatestMessages(9);
    expect(items[items.length - 1].id).toBe(250);
    expect(oldestPage).toBe(5);
  });

  test('conversation list offers "load more" when there is a next page', async () => {
    inboxApi.listConversations.mockResolvedValueOnce({
      data: [conv()],
      pagination: link(2, 2),
    }).mockResolvedValueOnce({
      data: [conv({ id: 2, subject: 'Second' })],
      pagination: link(2),
    });

    renderPage();
    await waitFor(() => expect(screen.getByText('First')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: /Load more conversations/i }));
    await waitFor(() => expect(screen.getByText('Second')).toBeInTheDocument());
    expect(inboxApi.listConversations).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2, perPage: 20 }));
  });

  test('switching to the Unread scope and a course filter refetches with them', async () => {
    inboxApi.listConversations.mockResolvedValue({ data: [conv({ workflow_state: 'unread' })], pagination: {} });
    inboxApi.contexts.mockResolvedValue([
      { context_type: 'Course', context_id: 5, context_code: 'course_5', name: 'Biology', permissions: { send_messages_all: false } },
    ]);

    renderPage();
    await waitFor(() => expect(screen.getByText('First')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('tab', { name: /Unread/ }));
    await waitFor(() => expect(inboxApi.listConversations).toHaveBeenLastCalledWith(expect.objectContaining({ scope: 'unread' })));

    await waitFor(() => expect(screen.getByRole('option', { name: 'Biology' })).toBeInTheDocument());
    fireEvent.change(screen.getByLabelText(/Filter by course or group/i), { target: { value: 'course_5' } });
    await waitFor(() => expect(inboxApi.listConversations).toHaveBeenLastCalledWith(expect.objectContaining({ scope: 'unread', filter: 'course_5' })));
  });

  test('starring a conversation sends the star event', async () => {
    inboxApi.listConversations.mockResolvedValue({ data: [conv()], pagination: {} });
    renderPage();
    await waitFor(() => expect(screen.getByText('First')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: 'Star First' }));
    await waitFor(() => expect(inboxApi.batchUpdate).toHaveBeenCalledWith([1], 'star'));
  });

  test('batch archive applies to the checked conversations', async () => {
    inboxApi.listConversations.mockResolvedValue({ data: [conv(), conv({ id: 2, subject: 'Second' })], pagination: {} });
    renderPage();
    await waitFor(() => expect(screen.getByText('Second')).toBeInTheDocument());
    fireEvent.click(screen.getByLabelText('Select First'));
    fireEvent.click(screen.getByLabelText('Select Second'));
    const toolbar = screen.getByRole('toolbar', { name: /Selected conversations/i });
    fireEvent.click(within(toolbar).getByRole('button', { name: /Archive/ }));
    await waitFor(() => expect(inboxApi.batchUpdate).toHaveBeenCalledWith([1, 2], 'archive'));
  });

  test('opening an unread conversation marks it read and shows attachments', async () => {
    inboxApi.listConversations.mockResolvedValue({ data: [conv({ workflow_state: 'unread' })], pagination: {} });
    inboxApi.getMessages.mockResolvedValue({
      data: [{
        id: 7, user_id: 40, user_name: 'Pat Teacher', body: 'See attached',
        attachments: [{ id: 3, display_name: 'syllabus.pdf', size: 2048, url: '/api/v1/conversations/1/attachments/3' }],
      }],
      pagination: {},
    });
    renderPage();
    await waitFor(() => expect(screen.getByText('First')).toBeInTheDocument());
    fireEvent.click(screen.getByText('First'));
    await waitFor(() => expect(inboxApi.markRead).toHaveBeenCalledWith(1));
    const link2 = await screen.findByRole('link', { name: /syllabus\.pdf/ });
    expect(link2).toHaveAttribute('href', '/api/v1/conversations/1/attachments/3');
  });

  test('"Reply" to a message restricts the reply to its author', async () => {
    inboxApi.listConversations.mockResolvedValue({
      data: [conv({ participants: [{ id: 50, name: 'Jane Doe' }, { id: 40, name: 'Pat Teacher' }, { id: 41, name: 'Sam Student' }] })],
      pagination: {},
    });
    inboxApi.getMessages.mockResolvedValue({ data: [{ id: 7, user_id: 40, user_name: 'Pat Teacher', body: 'Hello all' }], pagination: {} });
    inboxApi.addMessage.mockResolvedValue({ id: 8 });
    renderPage();
    fireEvent.click(await screen.findByText('First'));
    await screen.findByText('Hello all');
    fireEvent.click(screen.getByRole('button', { name: 'Reply' }));
    expect(screen.getByText(/Replying to Pat Teacher/)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Reply message'), { target: { value: 'Thanks' } });
    fireEvent.click(screen.getByRole('button', { name: 'Send message' }));
    await waitFor(() => expect(inboxApi.addMessage).toHaveBeenCalledWith(1, { message: 'Thanks', attachment_ids: [], recipients: [40] }));
  });

  // B059 / G093: the recipient picker uses the scoped address book.
  test('recipient search uses the address book and sends individual messages', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      inboxApi.listConversations.mockResolvedValue({ data: [], pagination: {} });
      inboxApi.searchRecipients.mockResolvedValue([{ id: 40, name: 'Pat Teacher' }, { id: 41, name: 'Sam Student' }]);
      inboxApi.createConversation.mockResolvedValue({ id: 9 });

      renderPage();
      await waitFor(() => expect(inboxApi.listConversations).toHaveBeenCalled());
      fireEvent.click(screen.getByRole('button', { name: /compose/i }));
      const input = await screen.findByLabelText('Search recipients');
      fireEvent.change(input, { target: { value: 'pat' } });
      await act(async () => { vi.advanceTimersByTime(400); });

      await waitFor(() => expect(inboxApi.searchRecipients).toHaveBeenCalledWith({ search: 'pat', context: '', perPage: 20 }));
      fireEvent.click(await screen.findByRole('option', { name: /Pat Teacher/ }));
      fireEvent.change(screen.getByLabelText('Search recipients'), { target: { value: 'sam' } });
      await act(async () => { vi.advanceTimersByTime(400); });
      fireEvent.click(await screen.findByRole('option', { name: /Sam Student/ }));

      fireEvent.click(screen.getByLabelText(/individual message to each recipient/i));
      fireEvent.change(screen.getByLabelText(/Subject/i), { target: { value: 'Hi' } });
      fireEvent.change(screen.getByLabelText(/^Message/i), { target: { value: 'Hello' } });
      fireEvent.click(screen.getByRole('button', { name: /^Send$/i }));
      await waitFor(() => expect(inboxApi.createConversation).toHaveBeenCalledWith({
        subject: 'Hi', body: 'Hello', recipients: [40, 41], group_conversation: false,
      }));
    } finally {
      vi.useRealTimers();
    }
  });
});
