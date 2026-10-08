import React, { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { KeyRound, AlertTriangle } from 'lucide-react';
import { api } from '../services/api';

// OAuth2 consent screen (B076). GET /api/v1/login/oauth2/auth redirects
// here instead of minting a code; nothing is issued until the signed-in
// user explicitly clicks Authorize, which POSTs (CSRF-protected) and then
// sends the browser on to the client's registered redirect_uri.
const OAuthConsentPage = () => {
  const [searchParams] = useSearchParams();
  const clientId = searchParams.get('client_id') || '';
  const redirectUri = searchParams.get('redirect_uri') || '';
  const scope = searchParams.get('scope') || '';
  const state = searchParams.get('state') || '';
  // PKCE (G136): the challenge travels from /login/oauth2/auth through this
  // screen to the authorization code; the app proves it with the verifier.
  const codeChallenge = searchParams.get('code_challenge') || '';
  const codeChallengeMethod = searchParams.get('code_challenge_method') || '';

  const [info, setInfo] = useState(null);
  const [error, setError] = useState(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    let cancelled = false;
    if (!clientId || !redirectUri) {
      setError('This authorization request is missing its client or redirect address.');
      return undefined;
    }
    api
      .getOAuthAuthorizeInfo({ client_id: clientId, redirect_uri: redirectUri, scope, state })
      .then((data) => {
        if (!cancelled) setInfo(data);
      })
      .catch(() => {
        if (!cancelled) setError('This application is not available for your account.');
      });
    return () => {
      cancelled = true;
    };
  }, [clientId, redirectUri, scope, state]);

  const decide = async (approve) => {
    setSubmitting(true);
    setError(null);
    try {
      const data = await api.submitOAuthConsent({
        client_id: clientId,
        redirect_uri: redirectUri,
        scope,
        state,
        approve,
        ...(codeChallenge ? { code_challenge: codeChallenge, code_challenge_method: codeChallengeMethod } : {}),
      });
      if (data?.redirect_url) {
        window.location.assign(data.redirect_url);
        return;
      }
      setError('The authorization server returned no redirect address.');
    } catch (err) {
      setError(err.message || 'Authorization failed.');
    }
    setSubmitting(false);
  };

  return (
    <div className="min-h-screen bg-surface-1 flex items-center justify-center px-4">
      <div className="bg-surface-0 rounded-lg shadow p-8 max-w-md w-full">
        <div className="flex items-center gap-3 mb-4">
          <KeyRound className="w-6 h-6 text-brand-600" />
          <h1 className="text-xl font-bold text-text-primary">Authorize application</h1>
        </div>

        {error && (
          <div role="alert" className="bg-accent-danger/10 border border-accent-danger/30 text-accent-danger px-4 py-3 rounded-md mb-4 flex items-center gap-2">
            <AlertTriangle className="w-4 h-4 flex-shrink-0" />
            {error}
          </div>
        )}

        {!info && !error && <p className="text-text-secondary">Loading…</p>}

        {info && (
          <>
            <p className="text-text-secondary mb-4">
              <strong className="text-text-primary">{info.client_name}</strong> is requesting access to your
              account. It will be able to act on your behalf.
            </p>
            {Array.isArray(info.scopes) && info.scopes.length > 0 && (
              <div className="mb-4">
                <p className="text-sm font-medium text-text-secondary mb-1">Requested access</p>
                <ul className="text-sm text-text-secondary list-disc pl-5 break-all">
                  {info.scopes.map((s) => (
                    <li key={s}>{s}</li>
                  ))}
                </ul>
              </div>
            )}
            <p className="text-xs text-text-tertiary mb-6 break-all">You will be returned to {info.redirect_uri}</p>
            <div className="flex gap-3 justify-end">
              <button
                type="button"
                onClick={() => decide(false)}
                disabled={submitting}
                className="px-4 py-2 text-sm text-text-secondary hover:text-text-primary disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={() => decide(true)}
                disabled={submitting}
                className="bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm disabled:opacity-50"
              >
                Authorize
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
};

export default OAuthConsentPage;
