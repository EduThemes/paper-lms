import React from 'react';
import { createRoot } from 'react-dom/client';
import { act } from 'react';
import useAccountId from '../useAccountId';

// B070: admin pages must target the signed-in user's tenant, never a
// hardcoded account 1. Rendered with react-dom directly so the test does
// not depend on @testing-library.

let mockAuth = { user: null, refreshUser: vi.fn() };
vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => mockAuth,
}));

function renderHookValue() {
  let value;
  const Probe = () => {
    value = useAccountId();
    return null;
  };
  const container = document.createElement('div');
  const root = createRoot(container);
  act(() => {
    root.render(<Probe />);
  });
  return { value, unmount: () => act(() => root.unmount()) };
}

describe('useAccountId', () => {
  beforeAll(() => {
    globalThis.IS_REACT_ACT_ENVIRONMENT = true;
  });

  it("returns the user's account_id", () => {
    mockAuth = { user: { id: 7, account_id: 5 }, refreshUser: vi.fn() };
    const { value, unmount } = renderHookValue();
    expect(value).toBe(5);
    expect(mockAuth.refreshUser).not.toHaveBeenCalled();
    unmount();
  });

  it('never falls back to account 1 and refreshes the user once when account_id is missing', () => {
    mockAuth = { user: { id: 7 }, refreshUser: vi.fn() };
    const { value, unmount } = renderHookValue();
    expect(value).toBeNull();
    expect(mockAuth.refreshUser).toHaveBeenCalledTimes(1);
    unmount();
  });

  it('returns null when signed out', () => {
    mockAuth = { user: null, refreshUser: vi.fn() };
    const { value, unmount } = renderHookValue();
    expect(value).toBeNull();
    expect(mockAuth.refreshUser).not.toHaveBeenCalled();
    unmount();
  });
});
