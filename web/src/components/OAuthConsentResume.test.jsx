import React from 'react';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import {
  OAuthConsentGate,
  ResumePendingOAuthConsent,
  rememberPendingConsent,
  peekPendingConsent,
} from './OAuthConsentResume';

let mockAuth = { user: null, loading: false };

vi.mock('../contexts/AuthContext', () => ({
  useAuth: () => mockAuth,
}));

const CONSENT_URL = '/login/oauth2/confirm?client_id=dk_example&response_type=code&redirect_uri=https%3A%2F%2Fapp.example.com%2Fcb&state=s1';

function renderApp(initialPath) {
  return render(
    <MemoryRouter initialEntries={[initialPath]}>
      <Routes>
        <Route path="/login" element={<div>Login Page</div>} />
        <Route
          path="/login/oauth2/confirm"
          element={<OAuthConsentGate><div>Consent Screen</div></OAuthConsentGate>}
        />
        <Route
          path="/"
          element={<ResumePendingOAuthConsent><div>Dashboard</div></ResumePendingOAuthConsent>}
        />
      </Routes>
    </MemoryRouter>,
  );
}

describe('OAuth consent across sign-in (B076)', () => {
  beforeEach(() => {
    sessionStorage.clear();
    mockAuth = { user: null, loading: false };
  });

  test('a signed-out user is sent to /login and returned to the consent screen after signing in', () => {
    const first = renderApp(CONSENT_URL);
    expect(screen.getByText('Login Page')).toBeInTheDocument();
    expect(peekPendingConsent()).toBe(CONSENT_URL);
    first.unmount();

    // Every sign-in method lands on "/".
    mockAuth = { user: { id: 10 }, loading: false };
    const second = renderApp('/');
    expect(screen.getByText('Consent Screen')).toBeInTheDocument();
    expect(peekPendingConsent()).toBeNull();
    second.unmount();

    // Only once: the next visit to the dashboard stays there.
    renderApp('/');
    expect(screen.getByText('Dashboard')).toBeInTheDocument();
  });

  test('never resumes to anything but the consent screen, and expires', () => {
    mockAuth = { user: { id: 10 }, loading: false };
    sessionStorage.setItem('paper_oauth_consent_pending', JSON.stringify({ path: '//evil.example/x', at: Date.now() }));
    const r1 = renderApp('/');
    expect(screen.getByText('Dashboard')).toBeInTheDocument();
    r1.unmount();

    sessionStorage.setItem(
      'paper_oauth_consent_pending',
      JSON.stringify({ path: CONSENT_URL, at: Date.now() - 60 * 60 * 1000 }),
    );
    renderApp('/');
    expect(screen.getByText('Dashboard')).toBeInTheDocument();
  });

  test('a signed-in user sees the consent screen directly', () => {
    mockAuth = { user: { id: 10 }, loading: false };
    rememberPendingConsent('/unrelated');
    renderApp(CONSENT_URL);
    expect(screen.getByText('Consent Screen')).toBeInTheDocument();
  });
});
