import React from 'react';
import { describe, test, expect, beforeEach, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import {
  safeReturnPath,
  rememberReturnTo,
  consumeReturnTo,
  markSessionExpired,
  consumeSessionExpired,
} from './returnTo';
import ProtectedRoute from '../components/ProtectedRoute';

let mockAuth = { user: null, loading: false };
vi.mock('../contexts/AuthContext', () => ({ useAuth: () => mockAuth }));

beforeEach(() => {
  sessionStorage.clear();
  mockAuth = { user: null, loading: false };
});

describe('safeReturnPath (G160)', () => {
  test('keeps in-app paths with query and hash', () => {
    expect(safeReturnPath('/courses/3/assignments/9?tab=2#c')).toBe('/courses/3/assignments/9?tab=2#c');
  });
  test('rejects open redirects and auth pages', () => {
    for (const bad of ['//evil.test/x', 'https://evil.test', '/\\evil.test', 'javascript:alert(1)', '/login', '/mfa/verify', '/', '', null]) {
      expect(safeReturnPath(bad)).toBeNull();
    }
  });
});

describe('return-to storage', () => {
  test('consume returns the path once', () => {
    rememberReturnTo({ pathname: '/inbox', search: '?c=1', hash: '' });
    expect(consumeReturnTo()).toBe('/inbox?c=1');
    expect(consumeReturnTo()).toBeNull();
  });
  test('session-expired flag is one-shot', () => {
    markSessionExpired();
    expect(consumeSessionExpired()).toBe(true);
    expect(consumeSessionExpired()).toBe(false);
  });
});

const tree = (initial) => (
  <MemoryRouter initialEntries={[initial]}>
    <Routes>
      <Route path="/login" element={<div>Login page</div>} />
      <Route path="/" element={<ProtectedRoute><div>Dashboard</div></ProtectedRoute>} />
      <Route path="/courses/:id" element={<ProtectedRoute><div>Course page</div></ProtectedRoute>} />
    </Routes>
  </MemoryRouter>
);

describe('ProtectedRoute deep links', () => {
  test('a signed-out deep link is remembered and sent to login', () => {
    render(tree('/courses/7?x=1'));
    expect(screen.getByText('Login page')).toBeInTheDocument();
    expect(sessionStorage.getItem('paperlms_return_to')).toBe('/courses/7?x=1');
  });

  test('after sign-in the dashboard forwards to the remembered page', () => {
    rememberReturnTo('/courses/7');
    mockAuth = { user: { id: 1 }, loading: false };
    render(tree('/'));
    expect(screen.getByText('Course page')).toBeInTheDocument();
    expect(sessionStorage.getItem('paperlms_return_to')).toBeNull();
  });

  test('without a remembered page the dashboard renders', () => {
    mockAuth = { user: { id: 1 }, loading: false };
    render(tree('/'));
    expect(screen.getByText('Dashboard')).toBeInTheDocument();
  });
});
