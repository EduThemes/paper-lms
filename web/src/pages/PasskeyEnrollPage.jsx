import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { ShieldCheck, KeyRound, AlertTriangle } from 'lucide-react';
import Layout from '../components/Layout';
import { b64urlToBytes, bytesToB64url } from '../lib/webauthn';
import { getCSRFToken } from '../services/api';

const API_URL = import.meta.env.VITE_API_URL || '/api/v1';

// PasskeyEnrollPage walks the user through registering a passkey.
//
// Two ceremonies, two server round-trips:
//   1. POST /users/self/passkeys/begin  → CredentialCreationOptions
//      + sets passkey_ceremony cookie (encrypted SessionData).
//   2. navigator.credentials.create() opens the device dialog
//      (Touch ID / Windows Hello / hardware key).
//   3. POST /users/self/passkeys/finish → server validates the
//      attestation, persists user_webauthn_credentials row.
//
// The browser deals in ArrayBuffers; the server speaks base64url-
// encoded JSON. The lib/webauthn helpers convert between them.
export default function PasskeyEnrollPage() {
  const navigate = useNavigate();
  const [nickname, setNickname] = useState('');
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);

  const supported = typeof window !== 'undefined' && window.PublicKeyCredential;

  const doEnroll = async (e) => {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      // 1. Begin: server returns CredentialCreationOptions with the
      // challenge + user id base64url-encoded as strings.
      const beginRes = await fetch(`${API_URL}/users/self/passkeys/begin`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': getCSRFToken(),
        },
        credentials: 'include',
        body: JSON.stringify({ nickname }),
      });
      if (!beginRes.ok) {
        const body = await beginRes.json().catch(() => ({}));
        throw new Error(body.errors?.[0]?.message || `begin failed (${beginRes.status})`);
      }
      const { options } = await beginRes.json();
      const publicKey = decodeCreationOptions(options.publicKey);

      // 2. Browser ceremony — opens the device dialog.
      const cred = await navigator.credentials.create({ publicKey });
      if (!cred) throw new Error('Authenticator did not produce a credential.');

      // 3. Finish — server verifies + persists.
      const payload = encodeAttestationResponse(cred);
      const finishRes = await fetch(`${API_URL}/users/self/passkeys/finish`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': getCSRFToken(),
        },
        credentials: 'include',
        body: JSON.stringify(payload),
      });
      if (!finishRes.ok) {
        const body = await finishRes.json().catch(() => ({}));
        throw new Error(body.errors?.[0]?.message || `finish failed (${finishRes.status})`);
      }
      setDone(true);
    } catch (err) {
      setError(err.message || 'Passkey registration failed.');
    } finally {
      setBusy(false);
    }
  };

  if (!supported) {
    return (
      <Layout>
        <div className="max-w-xl mx-auto p-6">
          <div className="rounded border border-amber-300 bg-amber-50 p-4 flex items-start gap-3">
            <AlertTriangle className="text-amber-600 mt-0.5" />
            <div>
              <h2 className="font-medium">Passkeys are not supported on this browser.</h2>
              <p className="text-sm text-gray-700 mt-1">
                Try Chrome, Safari, Edge, or Firefox on a recent OS. Passkeys
                require a device with biometric or PIN unlock.
              </p>
            </div>
          </div>
        </div>
      </Layout>
    );
  }

  if (done) {
    return (
      <Layout>
        <div className="max-w-xl mx-auto p-6 space-y-4">
          <div className="flex items-center gap-3">
            <ShieldCheck className="text-green-600" />
            <h1 className="text-2xl font-semibold">Passkey added</h1>
          </div>
          <p className="text-gray-700">
            You can now sign in to Paper LMS by tapping &ldquo;Sign in with a passkey&rdquo;
            on the login page. Your device&apos;s biometric or PIN unlocks the credential.
          </p>
          <div className="flex gap-2">
            <button
              onClick={() => navigate('/users/self/passkeys')}
              className="px-4 py-2 rounded bg-blue-600 text-white hover:bg-blue-700"
            >
              Manage passkeys
            </button>
            <button
              onClick={() => navigate('/')}
              className="px-4 py-2 rounded border border-gray-300 hover:bg-gray-50"
            >
              Done
            </button>
          </div>
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      <div className="max-w-xl mx-auto p-6 space-y-6">
        <div className="flex items-center gap-3">
          <KeyRound className="text-blue-600" />
          <h1 className="text-2xl font-semibold">Add a passkey</h1>
        </div>
        <p className="text-gray-700">
          A passkey replaces your password. Your device&apos;s biometric (Touch ID,
          Face ID, Windows Hello) or PIN is the second factor — no separate code needed.
        </p>
        <form onSubmit={doEnroll} className="space-y-4">
          <label className="block">
            <span className="text-sm font-medium text-gray-700">Name this passkey</span>
            <input
              type="text"
              value={nickname}
              onChange={(e) => setNickname(e.target.value)}
              placeholder="MacBook Touch ID"
              className="mt-1 w-full rounded border border-gray-300 px-3 py-2 focus:border-blue-500 focus:outline-none"
              maxLength={80}
            />
            <span className="text-xs text-gray-500 mt-1 block">
              Optional — helps you tell devices apart when you add more.
            </span>
          </label>
          {error && (
            <div className="rounded border border-red-300 bg-red-50 p-3 text-sm text-red-700">
              {error}
            </div>
          )}
          <button
            type="submit"
            disabled={busy}
            className="px-4 py-2 rounded bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-50"
          >
            {busy ? 'Waiting for device…' : 'Continue'}
          </button>
        </form>
      </div>
    </Layout>
  );
}

// decodeCreationOptions converts the JSON-friendly base64url strings
// the server sent into the ArrayBuffers WebAuthn expects.
function decodeCreationOptions(pk) {
  return {
    ...pk,
    challenge: b64urlToBytes(pk.challenge),
    user: { ...pk.user, id: b64urlToBytes(pk.user.id) },
    excludeCredentials: (pk.excludeCredentials || []).map((c) => ({
      ...c,
      id: b64urlToBytes(c.id),
    })),
  };
}

// encodeAttestationResponse mirrors the inverse: takes the browser's
// PublicKeyCredential (with ArrayBuffer fields) and turns them into
// the base64url-encoded JSON shape the server library understands.
function encodeAttestationResponse(cred) {
  const r = cred.response;
  return {
    id: cred.id,
    rawId: bytesToB64url(cred.rawId),
    type: cred.type,
    response: {
      attestationObject: bytesToB64url(r.attestationObject),
      clientDataJSON: bytesToB64url(r.clientDataJSON),
      transports: r.getTransports ? r.getTransports() : undefined,
    },
    clientExtensionResults: cred.getClientExtensionResults ? cred.getClientExtensionResults() : {},
  };
}
