import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import PortfolioSharingPanel from '../PortfolioSharingPanel';

const createShareLink = vi.fn();
vi.mock('../../../services/api', () => ({
  api: { getPortfolioComments: vi.fn(async () => ({ data: [] })), updatePortfolio: vi.fn() },
}));
vi.mock('../../../services/api_learner_outcomes', () => ({
  portfolioApi: {
    createShareLink: (...args) => createShareLink(...args),
    revokeShareLink: vi.fn(async () => ({})),
    deleteComment: vi.fn(),
  },
}));

function renderPanel(portfolio, onPortfolioChange = vi.fn()) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={qc}>
      <PortfolioSharingPanel portfolio={portfolio} onPortfolioChange={onPortfolioChange} />
    </QueryClientProvider>,
  );
  return onPortfolioChange;
}

describe('PortfolioSharingPanel', () => {
  beforeEach(() => createShareLink.mockReset());

  it('says an existing link is active without showing it (only its digest is stored)', () => {
    renderPanel({ id: 4, has_share_link: true });
    expect(screen.getByText(/A private link is active/)).toBeInTheDocument();
    expect(screen.queryByLabelText('Private sharing link')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Turn off' })).toBeInTheDocument();
  });

  it('shows the link right after it is created', async () => {
    createShareLink.mockResolvedValue({ share_token: 'made-up-token' });
    const onChange = renderPanel({ id: 4 });
    fireEvent.click(screen.getByRole('button', { name: /Create private link/ }));
    await waitFor(() => expect(onChange).toHaveBeenCalledWith({ share_token: 'made-up-token', has_share_link: true }));
  });

  it('offers to create a link when there is none', () => {
    renderPanel({ id: 4, has_share_link: false });
    expect(screen.getByRole('button', { name: /Create private link/ })).toBeInTheDocument();
  });
});
