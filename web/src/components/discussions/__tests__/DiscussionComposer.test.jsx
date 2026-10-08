import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import DiscussionComposer from '../DiscussionComposer';
import { discussionsApi } from '../../../services/api_discussions';

vi.mock('../../../services/api_discussions', () => ({
  discussionsApi: {
    uploadAttachment: vi.fn(),
    mentionableUsers: vi.fn(),
  },
}));

vi.mock('../../rce/RichContentEditorV2', () => ({
  default: ({ value, onChange, placeholder }) => (
    <textarea aria-label={placeholder} value={value} onChange={(e) => onChange(e.target.value)} />
  ),
}));

function setup(props = {}) {
  const onSubmit = vi.fn().mockResolvedValue({});
  render(<DiscussionComposer courseId="1" topicId="5" onSubmit={onSubmit} placeholder="Reply" {...props} />);
  return onSubmit;
}

describe('DiscussionComposer', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    discussionsApi.mentionableUsers.mockResolvedValue([{ id: 31, name: 'Jane Doe' }]);
  });

  test('submit is disabled until there is content', () => {
    setup();
    expect(screen.getByRole('button', { name: 'Post' })).toBeDisabled();
    fireEvent.change(screen.getByLabelText('Reply'), { target: { value: '<p>hello</p>' } });
    expect(screen.getByRole('button', { name: 'Post' })).toBeEnabled();
  });

  test('attaches an uploaded file and sends its id', async () => {
    discussionsApi.uploadAttachment.mockResolvedValue({ id: 77, display_name: 'lab.pdf', size: 4096 });
    const onSubmit = setup();
    const file = new File(['made-up'], 'lab.pdf', { type: 'application/pdf' });
    fireEvent.change(screen.getByTestId('discussion-attach-input'), { target: { files: [file] } });
    await waitFor(() => expect(screen.getByText('lab.pdf')).toBeInTheDocument());
    expect(discussionsApi.uploadAttachment).toHaveBeenCalledWith('1', '5', file);

    // A file alone is enough to post.
    fireEvent.click(screen.getByRole('button', { name: 'Post' }));
    await waitFor(() => expect(onSubmit).toHaveBeenCalledWith({ message: '', attachment_id: 77 }));
  });

  test('mention picker inserts the name and sends the id', async () => {
    const onSubmit = setup();
    fireEvent.change(screen.getByLabelText('Reply'), { target: { value: '<p>Thanks</p>' } });
    fireEvent.click(screen.getByRole('button', { name: 'Mention someone' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Jane Doe' }));
    expect(screen.getByLabelText('Reply').value).toBe('<p>Thanks @Jane Doe&nbsp;</p>');
    fireEvent.click(screen.getByRole('button', { name: 'Post' }));
    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith({ message: '<p>Thanks @Jane Doe&nbsp;</p>', mentioned_user_ids: [31] }),
    );
  });

  test('anonymous choice only where allowed', async () => {
    const onSubmit = setup({ allowAnonymous: true });
    fireEvent.change(screen.getByLabelText('Reply'), { target: { value: '<p>quiet</p>' } });
    fireEvent.click(screen.getByLabelText('Post anonymously'));
    fireEvent.click(screen.getByRole('button', { name: 'Post' }));
    await waitFor(() => expect(onSubmit).toHaveBeenCalledWith({ message: '<p>quiet</p>', anonymous: true }));
  });

  test('keeps the text and shows the error when posting fails', async () => {
    const onSubmit = vi.fn().mockRejectedValue(new Error('This discussion is locked'));
    render(<DiscussionComposer courseId="1" topicId="5" onSubmit={onSubmit} placeholder="Reply" />);
    fireEvent.change(screen.getByLabelText('Reply'), { target: { value: '<p>late</p>' } });
    fireEvent.click(screen.getByRole('button', { name: 'Post' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('This discussion is locked');
    expect(screen.getByLabelText('Reply').value).toBe('<p>late</p>');
    expect(screen.queryByLabelText('Post anonymously')).not.toBeInTheDocument();
  });
});
