import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';

// Layout pulls in AuthContext / router / sidebar / etc.; we don't care
// about any of that for testing the Page contract — short-circuit it.
vi.mock('../Layout', () => ({
  default: ({ children }) => <div data-testid="layout">{children}</div>,
}));

import Page from '../Page';

function buildQuery(overrides = {}) {
  return {
    isLoading: false,
    isError: false,
    error: null,
    data: undefined,
    refetch: vi.fn(),
    ...overrides,
  };
}

describe('<Page>', () => {
  it('renders the skeleton when loading', () => {
    const query = buildQuery({ isLoading: true });
    const renderChildren = vi.fn();

    const { container } = render(
      <Page query={query} title="Courses">
        {renderChildren}
      </Page>,
    );

    // Header still visible during skeleton so the page outline doesn't pop.
    expect(screen.getByText('Courses')).toBeInTheDocument();
    // Skeleton blocks render with the `animate-pulse` class.
    expect(container.querySelector('.animate-pulse')).toBeInTheDocument();
    // children render-prop must NOT be called while loading — data is
    // still undefined.
    expect(renderChildren).not.toHaveBeenCalled();
  });

  it('renders the error message and a Try again button on error', () => {
    const refetch = vi.fn();
    const query = buildQuery({
      isError: true,
      error: new Error('Boom: server fell over'),
      refetch,
    });
    const renderChildren = vi.fn();

    render(
      <Page query={query} title="Courses">
        {renderChildren}
      </Page>,
    );

    expect(screen.getByRole('alert')).toBeInTheDocument();
    expect(screen.getByText('Boom: server fell over')).toBeInTheDocument();

    const retry = screen.getByRole('button', { name: /try again/i });
    fireEvent.click(retry);
    expect(refetch).toHaveBeenCalledTimes(1);

    expect(renderChildren).not.toHaveBeenCalled();
  });

  it('falls back to a generic message when error has no message', () => {
    const query = buildQuery({ isError: true, error: {} });
    render(<Page query={query}>{() => null}</Page>);
    expect(screen.getByText('Something went wrong.')).toBeInTheDocument();
  });

  it('calls children(data) on success and renders the result', () => {
    const data = { id: 7, name: 'Algebra I' };
    const query = buildQuery({ data });
    const renderChildren = vi.fn((c) => <div data-testid="ok">{c.name}</div>);

    render(<Page query={query}>{renderChildren}</Page>);

    expect(renderChildren).toHaveBeenCalledTimes(1);
    expect(renderChildren).toHaveBeenCalledWith(data);
    expect(screen.getByTestId('ok')).toHaveTextContent('Algebra I');
  });

  it('renders the empty state when empty(data) returns true', () => {
    const query = buildQuery({ data: [] });
    const renderChildren = vi.fn();

    render(
      <Page
        query={query}
        title="People"
        empty={(arr) => arr.length === 0}
        emptyMessage="No users yet."
      >
        {renderChildren}
      </Page>,
    );

    expect(screen.getByText('People')).toBeInTheDocument();
    expect(screen.getByText('No users yet.')).toBeInTheDocument();
    expect(renderChildren).not.toHaveBeenCalled();
  });

  it('uses a custom loadingFallback when provided', () => {
    const query = buildQuery({ isLoading: true });
    render(
      <Page
        query={query}
        loadingFallback={<div data-testid="custom-skeleton">…</div>}
      >
        {() => null}
      </Page>,
    );
    expect(screen.getByTestId('custom-skeleton')).toBeInTheDocument();
  });
});
