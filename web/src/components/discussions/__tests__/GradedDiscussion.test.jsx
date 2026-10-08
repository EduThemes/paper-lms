import React from 'react';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import DiscussionSubmissionPosts from '../DiscussionSubmissionPosts';
import GradedDiscussionLink, { isDiscussionAssignment } from '../GradedDiscussionLink';
import { discussionsApi } from '../../../services/api_discussions';

vi.mock('../../../services/api_discussions', () => ({
  discussionsApi: {
    studentEntries: vi.fn(),
    assignmentTopic: vi.fn(),
  },
}));

vi.mock('../../RichContentViewer', () => ({
  default: ({ content }) => <div>{content}</div>,
}));

function wrap(ui) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('graded discussions (G084)', () => {
  beforeEach(() => vi.clearAllMocks());

  test('isDiscussionAssignment reads string and array submission types', () => {
    expect(isDiscussionAssignment({ submission_types: 'discussion_topic' })).toBe(true);
    expect(isDiscussionAssignment({ submission_types: ['online_upload', 'discussion_topic'] })).toBe(true);
    expect(isDiscussionAssignment({ submission_types: 'online_text_entry' })).toBe(false);
    expect(isDiscussionAssignment(null)).toBe(false);
  });

  test('SpeedGrader lists the student\'s posts with group and attachment', async () => {
    discussionsApi.studentEntries.mockResolvedValue({
      topic: { id: 4, title: 'Debate' },
      entries: [
        { id: 1, message: 'opening argument', created_at: '2026-01-01T00:00:00Z', group_name: 'Group One' },
        { id: 2, parent_id: 1, message: 'rebuttal', created_at: '2026-01-02T00:00:00Z',
          attachment: { display_name: 'sources.pdf', size: 1024, url: '/api/v1/x' } },
      ],
    });
    wrap(<DiscussionSubmissionPosts courseId="1" assignmentId="9" userId={31} />);
    expect(await screen.findByText('opening argument')).toBeInTheDocument();
    expect(screen.getByText('rebuttal')).toBeInTheDocument();
    expect(screen.getByText('Group One')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /sources\.pdf/ })).toHaveAttribute('href', '/api/v1/x');
    expect(screen.getByRole('link', { name: 'Debate' })).toHaveAttribute('href', '/courses/1/discussions/4');
    expect(discussionsApi.studentEntries).toHaveBeenCalledWith('1', '9', 31);
  });

  test('assignment page links students to the discussion', async () => {
    discussionsApi.assignmentTopic.mockResolvedValue({ id: 4, title: 'Debate' });
    wrap(<GradedDiscussionLink courseId="1" assignment={{ id: 9, submission_types: 'discussion_topic' }} />);
    expect(await screen.findByRole('link', { name: 'Go to the discussion' })).toHaveAttribute('href', '/courses/1/discussions/4');
  });

  test('no banner for other assignments', () => {
    wrap(<GradedDiscussionLink courseId="1" assignment={{ id: 9, submission_types: 'online_upload' }} />);
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
    expect(discussionsApi.assignmentTopic).not.toHaveBeenCalled();
  });
});
