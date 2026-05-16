import React from 'react';
import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import NextToBeatBanner from '../NextToBeatBanner';

describe('NextToBeatBanner', () => {
  it('renders nothing when nextToBeat is null', () => {
    const { container } = render(<NextToBeatBanner nextToBeat={null} />);
    expect(container.firstChild).toBeNull();
  });

  it('renders the specific number when gap is small', () => {
    render(
      <NextToBeatBanner
        nextToBeat={{ name: 'Brave Otter', gap: 23, currency_label: 'XP' }}
      />,
    );
    // Renders the literal number and the peer's name.
    expect(screen.getByText('23')).toBeInTheDocument();
    expect(screen.getByText('Brave Otter')).toBeInTheDocument();
    expect(screen.getByText(/Earn/)).toBeInTheDocument();
  });

  it('softens the copy when the gap is large', () => {
    render(
      <NextToBeatBanner
        nextToBeat={{ name: 'Wandering Phoenix', gap: 6800, currency_label: 'XP' }}
      />,
    );
    // The large-gap variant does NOT include the literal number — it
    // frames the progression as effort, not distance. Per the
    // behavioral-research rationale (03-claude-behavioral.md:281–286),
    // showing a daunting number to a far-behind learner is itself
    // demotivating.
    expect(screen.queryByText('6800')).not.toBeInTheDocument();
    expect(screen.queryByText('6,800')).not.toBeInTheDocument();
    expect(screen.getByText(/Keep going/)).toBeInTheDocument();
    expect(screen.getByText('Wandering Phoenix')).toBeInTheDocument();
  });

  it('falls back to default copy when fields are missing', () => {
    render(<NextToBeatBanner nextToBeat={{ gap: 10 }} />);
    expect(screen.getByText('the next learner')).toBeInTheDocument();
    expect(screen.getByText(/points/)).toBeInTheDocument();
  });
});
