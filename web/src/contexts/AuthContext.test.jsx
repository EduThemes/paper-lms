import { describe, test, expect, beforeEach, afterEach, vi } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';

vi.mock('../services/api', () => ({
  api: {
    getSelf: vi.fn(),
    login: vi.fn(),
    register: vi.fn(),
    logout: vi.fn(),
  },
}));

import { api } from '../services/api';
import { AuthProvider, useAuth } from '../contexts/AuthContext';

const wrapper = ({ children }) => <AuthProvider>{children}</AuthProvider>;

describe('AuthContext', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    // Default: getSelf rejects (not authenticated)
    api.getSelf.mockRejectedValue(new Error('Not authenticated'));
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  test('initial loading state is true', () => {
    api.getSelf.mockReturnValue(new Promise(() => {})); // never resolves

    const { result } = renderHook(() => useAuth(), { wrapper });

    expect(result.current.loading).toBe(true);
    expect(result.current.user).toBeNull();
  });

  test('auto login from cookie sets user on successful getSelf', async () => {
    api.getSelf.mockResolvedValue({ id: 1, name: 'Test User', email: 'test@example.com' });

    const { result } = renderHook(() => useAuth(), { wrapper });

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.user).toEqual({
      id: 1,
      name: 'Test User',
      email: 'test@example.com',
    });
  });

  test('auto login failure sets user to null and loading to false', async () => {
    api.getSelf.mockRejectedValue(new Error('Unauthorized'));

    const { result } = renderHook(() => useAuth(), { wrapper });

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.user).toBeNull();
  });

  test('login success sets user from returned data', async () => {
    // getSelf rejects initially (default in beforeEach)
    const { result } = renderHook(() => useAuth(), { wrapper });
    await waitFor(() => expect(result.current.loading).toBe(false));

    api.login.mockResolvedValue({
      token: 'abc123',
      user: { id: 2, name: 'Logged In User' },
    });

    await act(async () => {
      await result.current.login('user@test.com', 'password');
    });

    expect(result.current.user).toEqual({ id: 2, name: 'Logged In User' });
    expect(api.login).toHaveBeenCalledWith('user@test.com', 'password');
  });

  test('login error propagates the error', async () => {
    const { result } = renderHook(() => useAuth(), { wrapper });
    await waitFor(() => expect(result.current.loading).toBe(false));

    api.login.mockRejectedValue(new Error('Invalid credentials'));

    await expect(
      act(async () => {
        await result.current.login('bad@test.com', 'wrong');
      })
    ).rejects.toThrow('Invalid credentials');

    expect(result.current.user).toBeNull();
  });

  test('register success sets user from returned data', async () => {
    const { result } = renderHook(() => useAuth(), { wrapper });
    await waitFor(() => expect(result.current.loading).toBe(false));

    api.register.mockResolvedValue({
      token: 'def456',
      user: { id: 3, name: 'New User' },
    });

    await act(async () => {
      await result.current.register('New User', 'new@test.com', 'securepass');
    });

    expect(result.current.user).toEqual({ id: 3, name: 'New User' });
    expect(api.register).toHaveBeenCalledWith('New User', 'new@test.com', 'securepass');
  });

  test('logout clears user and localStorage', async () => {
    api.getSelf.mockResolvedValue({ id: 1, name: 'Test User' });
    const { result } = renderHook(() => useAuth(), { wrapper });
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.user).not.toBeNull();

    localStorage.setItem('token', 'old-token');
    localStorage.setItem('user', JSON.stringify({ id: 1 }));

    api.logout.mockResolvedValue({});

    await act(async () => {
      await result.current.logout();
    });

    expect(result.current.user).toBeNull();
    expect(localStorage.getItem('token')).toBeNull();
    expect(localStorage.getItem('user')).toBeNull();
    expect(api.logout).toHaveBeenCalled();
  });

  test('useAuth outside AuthProvider throws error', () => {
    // Suppress console.error for the expected error
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

    expect(() => {
      renderHook(() => useAuth());
    }).toThrow('useAuth must be used within an AuthProvider');

    consoleSpy.mockRestore();
  });
});
