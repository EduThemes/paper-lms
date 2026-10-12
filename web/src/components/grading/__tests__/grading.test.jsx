import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import GradeInput from '../../GradeInput';
import PostGradesDialog from '../PostGradesDialog';
import FinalGradeOverrideInput from '../FinalGradeOverrideInput';
import CourseGradingPolicyPanel from '../CourseGradingPolicyPanel';
import AssignmentGroupRulesDialog from '../AssignmentGroupRulesDialog';
import { parseRules, describeRules } from '../../../utils/gradingRules';
import { gradingPolicyApi } from '../../../services/api_grading_policy';

vi.mock('../../../services/api_grading_policy', async (orig) => {
  const actual = await orig();
  return {
    ...actual,
    gradingPolicyApi: {
      getSections: vi.fn(),
      postGrades: vi.fn(),
      hideGrades: vi.fn(),
      setScoreOverride: vi.fn(),
      getPostPolicy: vi.fn(),
      setPostPolicy: vi.fn(),
      getCourseGradingSchemes: vi.fn(),
      setCourseGradingScheme: vi.fn(),
      applyLatePolicy: vi.fn(),
      updateAssignmentGroup: vi.fn(),
    },
  };
});

describe('drop rules helpers (G020)', () => {
  test('parse object, JSON and Canvas text forms', () => {
    expect(parseRules({ drop_lowest: 2, never_drop: [5] })).toEqual({ drop_lowest: 2, drop_highest: 0, never_drop: [5] });
    expect(parseRules('{"drop_highest":1}')).toEqual({ drop_lowest: 0, drop_highest: 1, never_drop: [] });
    expect(parseRules('drop_lowest:1\nnever_drop:7\n')).toEqual({ drop_lowest: 1, drop_highest: 0, never_drop: [7] });
    expect(parseRules('')).toEqual({ drop_lowest: 0, drop_highest: 0, never_drop: [] });
  });
  test('describe', () => {
    expect(describeRules({ drop_lowest: 1, drop_highest: 2 })).toBe('Drops lowest 1, highest 2');
    expect(describeRules({})).toBe('');
  });
});

describe('GradeInput grading types (G023)', () => {
  test('a letter-graded assignment sends the typed grade', async () => {
    const onSave = vi.fn().mockResolvedValue();
    render(<GradeInput value={null} pointsPossible={10} gradingType="letter_grade" studentName="Jane Doe" onSave={onSave} />);
    fireEvent.click(screen.getByRole('button', { name: /edit grade for jane doe/i }));
    const input = screen.getByRole('textbox', { name: /grade for jane doe/i });
    fireEvent.change(input, { target: { value: 'B+' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    await waitFor(() => expect(onSave).toHaveBeenCalledWith('B+'));
  });
  test('points send a number and EX is passed through', async () => {
    const onSave = vi.fn().mockResolvedValue();
    const { rerender } = render(<GradeInput value={null} pointsPossible={10} studentName="Jane Doe" onSave={onSave} />);
    fireEvent.click(screen.getByRole('button', { name: /edit grade/i }));
    let input = screen.getByRole('textbox');
    fireEvent.change(input, { target: { value: '7.5' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    await waitFor(() => expect(onSave).toHaveBeenCalledWith(7.5));
    rerender(<GradeInput value={7.5} pointsPossible={10} studentName="Jane Doe" onSave={onSave} />);
    fireEvent.click(screen.getByRole('button', { name: /edit grade/i }));
    input = screen.getByRole('textbox');
    fireEvent.change(input, { target: { value: 'EX' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    await waitFor(() => expect(onSave).toHaveBeenLastCalledWith('EX'));
  });
});

describe('PostGradesDialog (G018)', () => {
  test('posts graded-only grades for chosen sections', async () => {
    gradingPolicyApi.getSections.mockResolvedValue([{ id: 3, name: 'Period 1' }, { id: 4, name: 'Period 2' }]);
    gradingPolicyApi.postGrades.mockResolvedValue({ posted: true });
    const onDone = vi.fn();
    render(
      <PostGradesDialog open onOpenChange={() => {}} courseId={1} assignment={{ id: 9, name: 'Essay' }} mode="post" onDone={onDone} />,
    );
    fireEvent.click(screen.getByLabelText(/graded/i, { selector: 'input[type="radio"]' }));
    await screen.findByText('Specific sections');
    fireEvent.click(screen.getByLabelText('Specific sections'));
    fireEvent.click(screen.getByLabelText('Period 2'));
    fireEvent.click(screen.getByRole('button', { name: 'Post' }));
    await waitFor(() =>
      expect(gradingPolicyApi.postGrades).toHaveBeenCalledWith(1, 9, { gradedOnly: true, sectionIds: [4] }),
    );
    expect(onDone).toHaveBeenCalledWith(9, 'post', [4]);
  });
});

describe('FinalGradeOverrideInput (G029)', () => {
  test('percent, scheme grade, and clear', async () => {
    gradingPolicyApi.setScoreOverride.mockResolvedValue({});
    const student = { id: 5, name: 'Jane Doe' };
    render(<FinalGradeOverrideInput courseId={1} student={student} value={null} />);
    const input = screen.getByRole('textbox', { name: /final grade override for jane doe/i });
    fireEvent.change(input, { target: { value: '91.5%' } });
    fireEvent.blur(input);
    await waitFor(() =>
      expect(gradingPolicyApi.setScoreOverride).toHaveBeenCalledWith(1, 5, { overrideScore: 91.5, gradingPeriodId: 0 }),
    );
    fireEvent.change(input, { target: { value: 'A-' } });
    fireEvent.blur(input);
    await waitFor(() =>
      expect(gradingPolicyApi.setScoreOverride).toHaveBeenLastCalledWith(1, 5, { overrideGrade: 'A-', gradingPeriodId: 0 }),
    );
  });
});

describe('CourseGradingPolicyPanel (G018 / G039 / G022)', () => {
  test('turns on manual posting and applies the late policy', async () => {
    gradingPolicyApi.getPostPolicy.mockResolvedValue({ post_manually: false });
    gradingPolicyApi.getCourseGradingSchemes.mockResolvedValue({ grading_schemes: [], grading_standard_enabled: true });
    gradingPolicyApi.setPostPolicy.mockResolvedValue({ post_manually: true });
    gradingPolicyApi.applyLatePolicy.mockResolvedValue({ updated: 3 });
    render(<CourseGradingPolicyPanel courseId={1} />);
    const box = await screen.findByRole('checkbox', { name: /post grades manually/i });
    fireEvent.click(box);
    await waitFor(() => expect(gradingPolicyApi.setPostPolicy).toHaveBeenCalledWith(1, true));
    fireEvent.click(screen.getByRole('button', { name: /apply late policy now/i }));
    expect(await screen.findByText('Updated 3 submission(s).')).toBeInTheDocument();
  });
});

describe('AssignmentGroupRulesDialog (G020)', () => {
  test('saves weight and drop rules with never-drop assignments', async () => {
    gradingPolicyApi.updateAssignmentGroup.mockResolvedValue({ id: 2 });
    render(
      <AssignmentGroupRulesDialog
        open
        onOpenChange={() => {}}
        courseId={1}
        group={{ id: 2, name: 'Homework', group_weight: 20, rules: { drop_lowest: 1 } }}
        assignments={[{ id: 11, name: 'HW 1' }, { id: 12, name: 'HW 2' }]}
      />,
    );
    fireEvent.change(screen.getByLabelText('Drop highest'), { target: { value: '1' } });
    fireEvent.click(screen.getByLabelText('HW 2'));
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() =>
      expect(gradingPolicyApi.updateAssignmentGroup).toHaveBeenCalledWith(1, 2, {
        groupWeight: 20,
        rules: { drop_lowest: 1, drop_highest: 1, never_drop: [12] },
      }),
    );
  });
  test('refuses fractional drop counts', async () => {
    render(
      <AssignmentGroupRulesDialog open onOpenChange={() => {}} courseId={1} group={{ id: 2, name: 'Quizzes', rules: '' }} />,
    );
    fireEvent.change(screen.getByLabelText('Drop lowest'), { target: { value: '1.5' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(/whole numbers/);
  });
});
