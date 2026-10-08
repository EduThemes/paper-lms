import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import CommentThread from '../CommentThread';
import gradingApi from '../../../services/api_grading_ui';

vi.mock('../../../services/api_grading_ui', () => ({
  default: {
    listComments: vi.fn(),
    createComment: vi.fn(),
    uploadCommentFile: vi.fn(),
    updateComment: vi.fn(),
    deleteComment: vi.fn(),
  },
}));

describe('CommentThread', () => {
  beforeEach(() => vi.clearAllMocks());

  it('lists comments with their files', async () => {
    gradingApi.listComments.mockResolvedValue([
      { id: 1, author_id: 5, author_name: 'Pat Teacher', comment: 'See the notes', attachments: [{ id: 9, display_name: 'notes.txt', url: '/f/9' }] },
    ]);
    render(<CommentThread courseId="1" assignmentId="2" submissionKey={{ userId: 3 }} currentUserId={3} />);
    expect(await screen.findByText('See the notes')).toBeInTheDocument();
    expect(screen.getByText('notes.txt')).toBeInTheDocument();
    expect(gradingApi.listComments).toHaveBeenCalledWith('1', '2', { userId: 3 });
  });

  it('uploads files first, then posts a group comment by anonymous id', async () => {
    gradingApi.uploadCommentFile.mockResolvedValue({ id: 44 });
    gradingApi.createComment.mockResolvedValue({ id: 2, author_id: 5, comment: 'Nice work', attachments: [] });
    render(
      <CommentThread
        courseId="1"
        assignmentId="2"
        submissionKey={{ anonymousId: 'ab12c' }}
        isTeacher
        isGroupAssignment
        currentUserId={5}
        initialComments={[]}
      />,
    );
    const file = new File(['x'], 'rubric.pdf', { type: 'application/pdf' });
    fireEvent.change(screen.getByTestId('comment-file-input'), { target: { files: [file] } });
    fireEvent.change(screen.getByLabelText('Comment text'), { target: { value: 'Nice work' } });
    fireEvent.click(screen.getByLabelText('Send comment'));
    await waitFor(() => expect(gradingApi.createComment).toHaveBeenCalled());
    expect(gradingApi.uploadCommentFile).toHaveBeenCalledWith('1', '2', { anonymousId: 'ab12c' }, file);
    expect(gradingApi.createComment).toHaveBeenCalledWith('1', '2', { anonymousId: 'ab12c' }, {
      text_comment: 'Nice work',
      file_ids: [44],
      group_comment: true,
    });
    expect(await screen.findByText('Nice work')).toBeInTheDocument();
    // No edit/delete controls without a user id to address the comment.
    expect(screen.queryByText('Delete')).not.toBeInTheDocument();
  });

  it('publishes a draft', async () => {
    gradingApi.updateComment.mockResolvedValue({ id: 7, draft: false, comment: 'draft text', author_id: 5 });
    render(
      <CommentThread
        courseId="1"
        assignmentId="2"
        submissionKey={{ userId: 3 }}
        isTeacher
        currentUserId={5}
        initialComments={[{ id: 7, author_id: 5, comment: 'draft text', draft: true, attachments: [] }]}
      />,
    );
    fireEvent.click(screen.getByText('Publish'));
    await waitFor(() => expect(gradingApi.updateComment).toHaveBeenCalledWith('1', '2', 3, 7, { draft: false }));
  });
});
