import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import SubmissionForm, { attemptsRemaining, fileExtension } from '../SubmissionForm';

// Text entry is the rich content editor (G010); a plain stand-in keeps
// these tests about attempts and files.
vi.mock('../rce/RichContentEditorV2', () => ({
  default: ({ value, onChange }) => (
    <textarea placeholder="Enter your submission" value={value} onChange={(e) => onChange(e.target.value)} />
  ),
}));

vi.mock('../grading/AnnotatableDocument', () => ({
  default: ({ onSubmit }) => (
    <button type="button" onClick={() => onSubmit({ submission_type: 'student_annotation' })}>Submit annotations</button>
  ),
}));

describe('SubmissionForm', () => {
  it('counts attempts like the server', () => {
    expect(attemptsRemaining({ allowed_attempts: null }, null)).toBeNull();
    expect(attemptsRemaining({ allowed_attempts: 2 }, null)).toBe(2);
    expect(attemptsRemaining({ allowed_attempts: 2 }, { attempt: 2, submitted_at: 'x', workflow_state: 'submitted' })).toBe(0);
    expect(attemptsRemaining({ allowed_attempts: 2 }, { attempt: 2, submitted_at: 'x', workflow_state: 'submitted', extra_attempts: 1 })).toBe(1);
    expect(fileExtension('Report.Final.PDF')).toBe('pdf');
  });

  it('blocks submitting once the attempts are used up', () => {
    render(
      <SubmissionForm
        courseId="1"
        assignmentId="2"
        onSubmit={vi.fn()}
        submissionTypes={['online_text_entry']}
        assignment={{ allowed_attempts: 1 }}
        existingSubmission={{ attempt: 1, submitted_at: '2026-10-01', workflow_state: 'submitted' }}
      />,
    );
    expect(screen.getByText(/used all of your attempts/)).toBeInTheDocument();
    fireEvent.change(screen.getByPlaceholderText(/Enter your submission/), { target: { value: 'again' } });
    expect(screen.getByText('Submit Assignment').closest('button')).toBeDisabled();
  });

  it('submits several files and rejects a disallowed extension', async () => {
    const onSubmit = vi.fn().mockResolvedValue({});
    render(
      <SubmissionForm
        courseId="1"
        assignmentId="2"
        onSubmit={onSubmit}
        submissionTypes={['online_upload']}
        assignment={{ allowed_extensions: ['pdf'] }}
      />,
    );
    expect(screen.getByText('Allowed file types: pdf')).toBeInTheDocument();
    const a = new File(['a'], 'one.pdf', { type: 'application/pdf' });
    const b = new File(['b'], 'two.txt', { type: 'text/plain' });
    fireEvent.change(screen.getByTestId('submission-file-input'), { target: { files: [a, b] } });
    fireEvent.click(screen.getByText('Submit Assignment'));
    expect(await screen.findByText(/two.txt: allowed file types are pdf/)).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
    fireEvent.click(screen.getByLabelText('Remove two.txt'));
    fireEvent.click(screen.getByText('Submit Assignment'));
    await waitFor(() => expect(onSubmit).toHaveBeenCalledWith({ submission_type: 'online_upload', files: [a] }));
  });

  it('offers media recording and student annotation tabs', async () => {
    const onSubmit = vi.fn().mockResolvedValue({});
    render(
      <SubmissionForm courseId="1" assignmentId="2" onSubmit={onSubmit} submissionTypes={['media_recording', 'student_annotation']} />,
    );
    expect(screen.getByText('Record audio or video, or upload a media file.')).toBeInTheDocument();
    const clip = new File(['v'], 'clip.webm', { type: 'video/webm' });
    fireEvent.change(screen.getByTestId('media-file-input'), { target: { files: [clip] } });
    fireEvent.click(screen.getByText('Submit Assignment'));
    await waitFor(() => expect(onSubmit).toHaveBeenCalledWith({ submission_type: 'media_recording', files: [clip] }));

    fireEvent.click(screen.getByText('Student Annotation'));
    fireEvent.click(screen.getByText('Submit annotations'));
    await waitFor(() => expect(onSubmit).toHaveBeenCalledWith({ submission_type: 'student_annotation' }));
  });
});
