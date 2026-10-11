import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import CustomGradeStatusesPage from '../CustomGradeStatusesPage';
import gradingApi from '../../services/api_grading_ui';

vi.mock('../../services/api_grading_ui', () => ({
  default: {
    listAccountGradeStatuses: vi.fn(),
    createGradeStatus: vi.fn(),
    updateGradeStatus: vi.fn(),
    deleteGradeStatus: vi.fn(),
  },
}));
vi.mock('../../components/Layout', () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock('../../hooks/useAccountId', () => ({ default: () => 1 }));

describe('CustomGradeStatusesPage', () => {
  beforeEach(() => vi.clearAllMocks());

  it('lists, creates and deletes statuses', async () => {
    gradingApi.listAccountGradeStatuses.mockResolvedValue([
      { id: 3, name: 'Incomplete', color: '#FFAA00', applies_to_submissions: true, applies_to_final_grade: false },
    ]);
    gradingApi.createGradeStatus.mockResolvedValue({ id: 4, name: 'Resubmit', color: '#E0E0E0', applies_to_submissions: true });
    gradingApi.deleteGradeStatus.mockResolvedValue({});
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    render(<CustomGradeStatusesPage />);
    expect(await screen.findByDisplayValue('Incomplete')).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Resubmit' } });
    fireEvent.click(screen.getByText('Add status'));
    await waitFor(() => expect(gradingApi.createGradeStatus).toHaveBeenCalledWith(1, {
      name: 'Resubmit', color: '#E0E0E0', applies_to_submissions: true, applies_to_final_grade: false,
    }));
    expect(await screen.findByDisplayValue('Resubmit')).toBeInTheDocument();
    fireEvent.click(screen.getByLabelText('Delete Incomplete'));
    await waitFor(() => expect(gradingApi.deleteGradeStatus).toHaveBeenCalledWith(1, 3));
  });
});
