import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import DiscussionTopicPageV2 from '../DiscussionTopicPageV2';
import { api } from '../../services/api';
import { discussionsApi } from '../../services/api_discussions';

vi.mock('../../services/api', () => ({
  api: {
    getDiscussionFullViewV2: vi.fn(),
    getDiscussionUnreadCount: vi.fn(),
    markDiscussionEntryRead: vi.fn(),
    markDiscussionTopicRead: vi.fn(),
    rateDiscussionEntry: vi.fn(),
    toggleDiscussionSubscription: vi.fn(),
    getDiscussionEntryVersions: vi.fn(),
    createDiscussionEntry: vi.fn(),
    createDiscussionReply: vi.fn(),
    deleteDiscussionEntry: vi.fn(),
    updateDiscussionEntryV2: vi.fn(),
    getDiscussionCheckpoints: vi.fn(),
    createDiscussionCheckpoints: vi.fn(),
    updateDiscussionCheckpoint: vi.fn(),
    deleteDiscussionCheckpoint: vi.fn(),
    getDiscussionCheckpointProgress: vi.fn(),
  },
}));

vi.mock('../../services/api_discussions', () => ({
  discussionsApi: {
    createEntry: vi.fn(),
    createReply: vi.fn(),
    updateEntry: vi.fn(),
    uploadAttachment: vi.fn(),
    mentionableUsers: vi.fn(() => Promise.resolve([])),
  },
}));

vi.mock('../../components/rce/RichContentEditorV2', () => ({
  default: ({ value, onChange, placeholder }) => (
    <textarea aria-label={placeholder} value={value} onChange={(e) => onChange(e.target.value)} />
  ),
}));

vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => ({ user: { id: 7, name: 'Jane Doe' }, loading: false }),
}));

let mockIsTeacher = false;
vi.mock('../../hooks/useIsTeacher', () => ({
  default: () => mockIsTeacher,
}));

vi.mock('../../components/Layout', () => ({
  default: ({ children }) => <div>{children}</div>,
}));

vi.mock('../../components/DiscussionCheckpointsPanel', () => ({
  default: () => null,
}));

vi.mock('../../components/RichContentViewer', () => ({
  default: ({ content }) => <div data-testid="rich">{content}</div>,
  sanitizeHTML: (s) => s,
}));

// IntersectionObserver that reports every observed element as visible.
class VisibleObserver {
  constructor(cb) {
    this.cb = cb;
  }
  observe() {
    this.cb([{ isIntersecting: true }]);
  }
  disconnect() {}
}

function entry(overrides) {
  return {
    id: 1,
    user_id: 8,
    user_name: 'John Doe',
    message: 'hello',
    rating_sum: 0,
    rating: 0,
    read_state: true,
    version_count: 0,
    workflow_state: 'active',
    replies: [],
    ...overrides,
  };
}

function view({ topic = {}, entries = [] } = {}) {
  return {
    data: {
      topic: {
        id: 5,
        title: 'Week 1',
        user_id: 3,
        discussion_type: 'threaded',
        subscribed: false,
        can_rate: true,
        locked_for_user: false,
        ...topic,
      },
      entries,
    },
  };
}

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/courses/1/discussions/5']}>
      <Routes>
        <Route path="/courses/:courseId/discussions/:topicId" element={<DiscussionTopicPageV2 />} />
        <Route path="/courses/:courseId/discussions/88" element={<div>group topic page</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('DiscussionTopicPageV2', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsTeacher = false;
    globalThis.IntersectionObserver = VisibleObserver;
    api.getDiscussionUnreadCount.mockResolvedValue({ data: { unread_count: 0 } });
    api.markDiscussionEntryRead.mockResolvedValue({});
    api.rateDiscussionEntry.mockResolvedValue({});
  });

  test('boolean read_state drives unread highlight and mark-as-read (B156)', async () => {
    api.getDiscussionFullViewV2.mockResolvedValue(view({
      entries: [
        entry({ id: 1, read_state: false, message: 'unread post' }),
        entry({ id: 2, read_state: true, message: 'read post' }),
      ],
    }));
    renderPage();

    await waitFor(() => expect(screen.getByText('unread post')).toBeInTheDocument());
    await waitFor(() => expect(api.markDiscussionEntryRead).toHaveBeenCalledWith('1', '5', 1));
    expect(api.markDiscussionEntryRead).not.toHaveBeenCalledWith('1', '5', 2);
  });

  test('subscription state comes from the topic (B159)', async () => {
    api.getDiscussionFullViewV2.mockResolvedValue(view({ topic: { subscribed: true } }));
    renderPage();
    await waitFor(() => expect(screen.getByRole('button', { name: /unsubscribe from notifications/i })).toBeInTheDocument());
    expect(screen.getByText('Subscribed')).toBeInTheDocument();
  });

  test('a single edit shows the edited marker (B160)', async () => {
    api.getDiscussionFullViewV2.mockResolvedValue(view({
      entries: [entry({ id: 1, version_count: 2, message: 'edited once' })],
    }));
    renderPage();
    await waitFor(() => expect(screen.getByText('edited once')).toBeInTheDocument());
    expect(screen.getByRole('button', { name: /view edit history/i })).toBeInTheDocument();
  });

  test('deleted parent renders a placeholder and keeps its replies (B157)', async () => {
    api.getDiscussionFullViewV2.mockResolvedValue(view({
      entries: [
        entry({
          id: 1,
          workflow_state: 'deleted',
          message: '',
          replies: [entry({ id: 2, parent_id: 1, message: 'still here' })],
        }),
      ],
    }));
    renderPage();
    await waitFor(() => expect(screen.getByText('This post has been deleted.')).toBeInTheDocument());
    expect(screen.getByText('still here')).toBeInTheDocument();
  });

  test('like toggles the viewer\'s own rating and unlikes with 0 (B158)', async () => {
    api.getDiscussionFullViewV2.mockResolvedValue(view({
      entries: [
        entry({ id: 1, rating: 1, rating_sum: 3, message: 'liked' }),
        entry({ id: 2, rating: 0, rating_sum: 0, message: 'not liked' }),
      ],
    }));
    renderPage();
    await waitFor(() => expect(screen.getByText('liked')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: /unlike \(3 likes\)/i }));
    await waitFor(() => expect(api.rateDiscussionEntry).toHaveBeenCalledWith('1', '5', 1, 0));

    fireEvent.click(screen.getByRole('button', { name: /^like \(0 likes\)/i }));
    await waitFor(() => expect(api.rateDiscussionEntry).toHaveBeenCalledWith('1', '5', 2, 1));
  });

  test('like button hidden when the topic does not allow the viewer to rate (B158)', async () => {
    api.getDiscussionFullViewV2.mockResolvedValue(view({
      topic: { can_rate: false },
      entries: [entry({ id: 1, message: 'no likes here' })],
    }));
    renderPage();
    await waitFor(() => expect(screen.getByText('no likes here')).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: /like/i })).not.toBeInTheDocument();
  });

  test('authors can delete their own post; others cannot (B052)', async () => {
    api.getDiscussionFullViewV2.mockResolvedValue(view({
      entries: [
        entry({ id: 1, user_id: 7, message: 'mine' }),
        entry({ id: 2, user_id: 8, message: 'theirs' }),
      ],
    }));
    renderPage();
    await waitFor(() => expect(screen.getByText('mine')).toBeInTheDocument());
    expect(screen.getAllByRole('button', { name: /delete this post/i })).toHaveLength(1);
  });

  test('locked topic hides posting for students', async () => {
    api.getDiscussionFullViewV2.mockResolvedValue(view({
      topic: { locked_for_user: true },
      entries: [entry({ id: 1, user_id: 7, message: 'mine' })],
    }));
    renderPage();
    await waitFor(() => expect(screen.getByText('This discussion is locked.')).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: /new reply/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /reply to this post/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /delete this post/i })).not.toBeInTheDocument();
  });

  test('posts go through the rich composer with mentions picked (G086, G088)', async () => {
    api.getDiscussionFullViewV2.mockResolvedValue(view({ entries: [] }));
    discussionsApi.createEntry.mockResolvedValue({ id: 9 });
    renderPage();
    await waitFor(() => expect(screen.getByText(/No replies yet/)).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: /write a reply/i }));
    fireEvent.change(screen.getByLabelText('Share your thoughts...'), { target: { value: '<p>Great point</p>' } });
    fireEvent.click(screen.getByRole('button', { name: 'Post Reply' }));
    await waitFor(() => expect(discussionsApi.createEntry).toHaveBeenCalledWith('1', '5', { message: '<p>Great point</p>' }));
  });

  test('a post\'s attachment is linked (G086)', async () => {
    api.getDiscussionFullViewV2.mockResolvedValue(view({
      entries: [entry({
        id: 1,
        message: 'see file',
        attachment: { id: 3, display_name: 'notes.pdf', size: 2048, url: '/api/v1/courses/1/discussion_topics/5/entries/1/attachment' },
      })],
    }));
    renderPage();
    const link = await screen.findByRole('link', { name: /notes\.pdf/ });
    expect(link).toHaveAttribute('href', '/api/v1/courses/1/discussion_topics/5/entries/1/attachment');
  });

  test('post-first topics explain why replies are hidden (G083)', async () => {
    api.getDiscussionFullViewV2.mockResolvedValue(view({ topic: { initial_post_required: true }, entries: [] }));
    renderPage();
    expect(await screen.findByText(/Post your reply first to see what others wrote/)).toBeInTheDocument();
  });

  test('graded and partially anonymous topics say so (G084, G087)', async () => {
    api.getDiscussionFullViewV2.mockResolvedValue(view({
      topic: { assignment: { id: 2, points_possible: 10 }, anonymous_state: 'partial_anonymity' },
      entries: [entry({ id: 1, message: 'first' })],
    }));
    renderPage();
    expect(await screen.findByText(/Graded discussion · 10 points/)).toBeInTheDocument();
    expect(screen.getByText('You can choose to post anonymously.')).toBeInTheDocument();
    expect(screen.getByLabelText('Post anonymously')).toBeInTheDocument();
  });

  test('a student opening a group discussion lands in their group (G085)', async () => {
    api.getDiscussionFullViewV2.mockResolvedValue(view({
      topic: { is_group_root: true, group_category_id: 4, group_topic_id: 88, can_post: false },
    }));
    renderPage();
    expect(await screen.findByText('group topic page')).toBeInTheDocument();
  });

  test('staff see the group topics of a group discussion (G085)', async () => {
    mockIsTeacher = true;
    api.getDiscussionFullViewV2.mockResolvedValue(view({
      topic: {
        is_group_root: true,
        group_category_id: 4,
        can_post: false,
        group_topics: [
          { id: 88, group_id: 1, group_name: 'Group One', entry_count: 2, unread_count: 1 },
          { id: 89, group_id: 2, group_name: 'Group Two', entry_count: 0, unread_count: 0 },
        ],
      },
    }));
    renderPage();
    expect(await screen.findByRole('link', { name: /Group One/ })).toHaveAttribute('href', '/courses/1/discussions/88');
    expect(screen.getByText('2 posts · 1 unread')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /new reply/i })).not.toBeInTheDocument();
  });
});
