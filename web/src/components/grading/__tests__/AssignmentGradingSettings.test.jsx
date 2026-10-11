import { render, screen, fireEvent } from '@testing-library/react';
import AssignmentGradingSettings, {
  gradingSettingsFromAssignment,
  gradingSettingsPayload,
  validateGradingSettings,
} from '../AssignmentGradingSettings';

describe('AssignmentGradingSettings', () => {
  it('round-trips an assignment into a payload', () => {
    const form = gradingSettingsFromAssignment({
      allowed_attempts: 3,
      allowed_extensions: ['pdf', 'docx'],
      moderated_grading: true,
      grader_count: 3,
      final_grader_id: 10,
      omit_from_final_grade: true,
      unlock_at: '2026-09-01T12:00:00Z',
    });
    const payload = gradingSettingsPayload(form, ['online_upload']);
    expect(payload.allowed_attempts).toBe(3);
    expect(payload.allowed_extensions).toEqual(['pdf', 'docx']);
    expect(payload.moderated_grading).toBe(true);
    expect(payload.grader_count).toBe(3);
    expect(payload.final_grader_id).toBe(10);
    expect(payload.omit_from_final_grade).toBe(true);
    expect(new Date(payload.unlock_at).toISOString()).toBe('2026-09-01T12:00:00.000Z');
    expect(payload.lock_at).toBeNull();
  });

  it('sends unlimited attempts as -1 and strips dots from extensions', () => {
    const payload = gradingSettingsPayload({ limit_attempts: false, allowed_extensions: '.PDF, .txt' }, ['online_upload']);
    expect(payload.allowed_attempts).toBe(-1);
    expect(payload.allowed_extensions).toEqual(['PDF', 'txt']);
  });

  it('validates the window and the annotation document', () => {
    expect(validateGradingSettings({ unlock_at: '2026-09-02T00:00', lock_at: '2026-09-01T00:00' }, [])).toMatch(/before/);
    expect(validateGradingSettings({}, ['student_annotation'])).toMatch(/document/);
    expect(validateGradingSettings({ annotatable_attachment_id: '4' }, ['student_annotation'])).toBeNull();
  });

  it('shows the document picker for annotation assignments and edits fields', () => {
    const onChange = vi.fn();
    render(
      <AssignmentGradingSettings
        value={gradingSettingsFromAssignment({})}
        onChange={onChange}
        submissionTypes={['student_annotation']}
        courseFiles={[{ id: 4, display_name: 'worksheet.pdf' }]}
      />,
    );
    fireEvent.change(screen.getByLabelText('Document to annotate'), { target: { value: '4' } });
    expect(onChange).toHaveBeenCalledWith({ annotatable_attachment_id: '4' });
    fireEvent.click(screen.getByLabelText(/Several graders/));
    expect(onChange).toHaveBeenCalledWith({ moderated_grading: true });
    expect(screen.getByLabelText('Available from')).toBeInTheDocument();
  });
});
