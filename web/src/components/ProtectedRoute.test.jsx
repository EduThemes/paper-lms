import React from 'react';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import ProtectedRoute from './ProtectedRoute';

let mockAuth = { user: null, loading: true };

vi.mock('../contexts/AuthContext', () => ({
  useAuth: () => mockAuth,
}));

function renderProtectedRoute(children = <div>Protected Content</div>) {
  return render(
    <MemoryRouter initialEntries={['/protected']}>
      <ProtectedRoute>{children}</ProtectedRoute>
    </MemoryRouter>,
  );
}

describe('ProtectedRoute', () => {
  beforeEach(() => {
    mockAuth = { user: null, loading: true };
  });

  test('shows loading state while auth is loading', () => {
    mockAuth = { user: null, loading: true };
    renderProtectedRoute();

    expect(screen.getByText('Loading...')).toBeInTheDocument();
    expect(screen.queryByText('Protected Content')).not.toBeInTheDocument();
  });

  test('redirects to /login when not authenticated', () => {
    mockAuth = { user: null, loading: false };

    const { container } = render(
      <MemoryRouter initialEntries={['/protected']}>
        <ProtectedRoute>
          <div>Protected Content</div>
        </ProtectedRoute>
      </MemoryRouter>,
    );

    expect(screen.queryByText('Protected Content')).not.toBeInTheDocument();
    expect(screen.queryByText('Loading...')).not.toBeInTheDocument();
    // Navigate component renders nothing visible; the absence of children
    // and loading text confirms the redirect was triggered
    expect(container.textContent).toBe('');
  });

  test('renders children when authenticated', () => {
    mockAuth = { user: { id: 1, name: 'Test User' }, loading: false };
    renderProtectedRoute(<div>Protected Content</div>);

    expect(screen.getByText('Protected Content')).toBeInTheDocument();
    expect(screen.queryByText('Loading...')).not.toBeInTheDocument();
  });
});
