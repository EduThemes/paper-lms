import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { PeerReviewForm, ReceivedPeerReviews, parseRubricCriteria } from '../PeerReviews';
import gradingApi from '../../../services/api_grading_ui';

vi.mock('../../../services/api_grading_ui', () => ({
  default: {
    submitPeerReview: vi.fn(),
    listReceivedPeerReviews: vi.fn(),
    assignPeerReview: vi.fn(),
    deletePeerReview: vi.fn(),
  },
}));

const criteria = [
  { id: 'c1', description: 'Thesis', points: 5, ratings: [{ id: 'r1', points: 5, description: 'Clear' }] },
  { id: 'c2', description: 'Evidence', points: 5 },
];

describe('peer reviews', () => {
  beforeEach(() => vi.clearAllMocks());

  it('parses rubric criteria from the assignment rubric payload', () => {
    expect(parseRubricCriteria({ rubric: { data: JSON.stringify(criteria) } })).toHaveLength(2);
    expect(parseRubricCriteria(null)).toEqual([]);
  });

  it('submits a rubric-based review', async () => {
    gradingApi.submitPeerReview.mockResolvedValue({ id: 3 });
    const onSubmitted = vi.fn();
    render(<PeerReviewForm review={{ id: 3 }} pointsPossible={10} criteria={criteria} onSubmitted={onSubmitted} />);
    fireEvent.click(screen.getByText('5 - Clear'));
    fireEvent.change(screen.getByLabelText('Points for Evidence'), { target: { value: '3' } });
    fireEvent.change(screen.getByLabelText('Comments'), { target: { value: 'Good argument' } });
    fireEvent.click(screen.getByText('Submit Review'));
    await waitFor(() => expect(onSubmitted).toHaveBeenCalled());
    expect(gradingApi.submitPeerReview).toHaveBeenCalledWith(3, {
      score: undefined,
      comments: 'Good argument',
      rubricData: { c1: { points: 5 }, c2: { points: 3 } },
    });
  });

  it('shows feedback received, anonymously when configured', async () => {
    gradingApi.listReceivedPeerReviews.mockResolvedValue([
      { id: 1, reviewer_name: 'Anonymous reviewer 1', score: 8, comments: 'Strong ending', rubric_data: { c1: { points: 4 } } },
    ]);
    render(<ReceivedPeerReviews courseId="1" assignmentId="2" pointsPossible={10} criteria={criteria} />);
    expect(await screen.findByText('Anonymous reviewer 1')).toBeInTheDocument();
    expect(screen.getByText('Strong ending')).toBeInTheDocument();
    expect(screen.getByText(/Thesis:/)).toBeInTheDocument();
  });
});
