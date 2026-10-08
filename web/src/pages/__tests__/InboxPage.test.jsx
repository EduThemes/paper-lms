import React from 'react';
import { render, screen, waitFor, fireEvent, act } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import InboxPage, { loadLatestMessages, pageFromLink } from '../InboxPage';
import { api } from '../../services/api';

vi.mock('../../services/api', () => ({
  api: {
    getConversations: vi.fn(),
    getConversationMessages: vi.fn(),
    markConversationAsRead: vi.fn(),
    searchRecipients: vi.fn(),
    searchUsers: vi.fn(),
    createConversation: vi.fn(),
    createConversationMessage: vi.fn(),
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

describe('InboxPage', () => {
  beforeEach(() => vi.clearAllMocks());

  test('pageFromLink reads the page number', () => {
    expect(pageFromLink('http://h/api?page=3&per_page=50')).toBe(3);
    expect(pageFromLink(undefined)).toBeNull();
  });

  // B154: a 60-message thread opens on its newest messages (page 2), not
  // the oldest 50.
  test('loadLatestMessages fetches the last page of a long thread', async () => {
    const page1 = Array.from({ length: 50 }, (_, i) => ({ id: i + 1 }));
    const page2 = Array.from({ length: 10 }, (_, i) => ({ id: i + 51 }));
    api.getConversationMessages.mockImplementation(async (_id, page) => (
      page === 1 ? { data: page1, pagination: link(2) } : { data: page2, pagination: link(2) }
    ));

    const { items, oldestPage } = await loadLatestMessages(9);
    expect(items.map((m) => m.id)).toContain(60);
    expect(items[items.length - 1].id).toBe(60);
    // The short last page is topped up with the page before it.
    expect(items.length).toBe(60);
    expect(oldestPage).toBe(1);
  });

  test('loadLatestMessages keeps "load earlier" available for very long threads', async () => {
    api.getConversationMessages.mockImplementation(async (_id, page) => ({
      data: Array.from({ length: page === 5 ? 50 : 50 }, (_, i) => ({ id: (page - 1) * 50 + i + 1 })),
      pagination: link(5),
    }));
    const { items, oldestPage } = await loadLatestMessages(9);
    expect(items[items.length - 1].id).toBe(250);
    expect(oldestPage).toBe(5);
  });

  test('conversation list offers "load more" when there is a next page', async () => {
    api.getConversations.mockResolvedValueOnce({
      data: [{ id: 1, subject: 'First', participants: [] }],
      pagination: link(2, 2),
    }).mockResolvedValueOnce({
      data: [{ id: 2, subject: 'Second', participants: [] }],
      pagination: link(2),
    });

    render(<MemoryRouter><InboxPage /></MemoryRouter>);
    await waitFor(() => expect(screen.getByText('First')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: /Load more conversations/i }));
    await waitFor(() => expect(screen.getByText('Second')).toBeInTheDocument());
    expect(api.getConversations).toHaveBeenLastCalledWith(2, 20);
  });

  // B059: the recipient picker uses the scoped search, not admin-only /users.
  test('recipient search uses searchRecipients', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      api.getConversations.mockResolvedValue({ data: [], pagination: {} });
      api.searchRecipients.mockResolvedValue({ data: [{ id: 40, name: 'Pat Teacher' }] });

      render(<MemoryRouter><InboxPage /></MemoryRouter>);
      await waitFor(() => expect(api.getConversations).toHaveBeenCalled());
      const compose = await screen.findAllByRole('button');
      fireEvent.click(compose[0]);
      const input = await screen.findByPlaceholderText(/Search by name or email/i);
      fireEvent.change(input, { target: { value: 'pat' } });
      await act(async () => { vi.advanceTimersByTime(400); });

      await waitFor(() => expect(api.searchRecipients).toHaveBeenCalledWith('pat', 8));
      expect(api.searchUsers).not.toHaveBeenCalled();
      await waitFor(() => expect(screen.getByText('Pat Teacher')).toBeInTheDocument());
    } finally {
      vi.useRealTimers();
    }
  });
});
