import React, { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';

const API_URL = import.meta.env.VITE_API_URL || '/api/v1';

async function apiRequest(path, options = {}) {
  const response = await fetch(`${API_URL}${path}`, {
    ...options,
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...options.headers },
  });
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    throw new Error(body.errors?.[0]?.message || `Request failed: ${response.status}`);
  }
  return response.json();
}

/* ─── Shield Icon ─── */
const ShieldIcon = ({ className }) => (
  <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
    <path d="M9 12l2 2 4-4" />
  </svg>
);

/* ─── Checkmark Icon ─── */
const CheckCircleIcon = ({ className }) => (
  <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <circle cx="12" cy="12" r="10" />
    <path d="M9 12l2 2 4-4" />
  </svg>
);

/* ─── X Circle Icon ─── */
const XCircleIcon = ({ className }) => (
  <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <circle cx="12" cy="12" r="10" />
    <path d="M15 9l-6 6M9 9l6 6" />
  </svg>
);

/* ─── Data Collection Info ─── */
const DATA_COLLECTED = [
  { label: 'Student name and school email address', purpose: 'Account identification and login' },
  { label: 'Assignment submissions and grades', purpose: 'Academic progress tracking' },
  { label: 'Course enrollment information', purpose: 'Class roster management' },
  { label: 'Discussion posts and comments', purpose: 'Classroom collaboration' },
  { label: 'Login timestamps and activity logs', purpose: 'Security and usage analytics' },
];

/* ─── COPPA Rights ─── */
const COPPA_RIGHTS = [
  'Review the personal information collected from your child',
  'Request deletion of your child\'s personal information',
  'Refuse further collection or use of your child\'s information',
  'Withdraw consent at any time by contacting the school administrator',
];

const ParentalConsentPage = () => {
  const { token } = useParams();
  const [consent, setConsent] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [submitting, setSubmitting] = useState(false);
  const [result, setResult] = useState(null); // 'granted' | 'denied'

  useEffect(() => {
    const fetchConsent = async () => {
      try {
        const data = await apiRequest(`/consent/verify/${token}`);
        setConsent(data.data || data);
      } catch (err) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };
    if (token) fetchConsent();
  }, [token]);

  const handleGrant = async () => {
    setSubmitting(true);
    setError(null);
    try {
      await apiRequest(`/consent/grant/${token}`, { method: 'POST' });
      setResult('granted');
    } catch (err) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  };

  const handleDeny = async () => {
    setSubmitting(true);
    setError(null);
    try {
      await apiRequest(`/consent/deny/${token}`, { method: 'POST' });
      setResult('denied');
    } catch (err) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  };

  /* ── Loading ── */
  if (loading) {
    return (
      <div className="min-h-screen bg-brand-50 flex items-center justify-center px-4">
        <div className="text-center">
          <div className="h-8 w-8 border-3 border-brand-600 border-t-transparent rounded-full animate-spin mx-auto mb-4" role="status" aria-label="Loading" />
          <p className="text-text-secondary">Loading consent details...</p>
        </div>
      </div>
    );
  }

  /* ── Error / Invalid Token ── */
  if (error && !consent) {
    return (
      <div className="min-h-screen bg-brand-50 flex items-center justify-center px-4">
        <div className="bg-surface-0 rounded-2xl shadow-lg p-8 max-w-lg w-full text-center">
          <XCircleIcon className="w-16 h-16 text-accent-danger mx-auto mb-4" />
          <h1 className="text-xl font-bold text-text-primary mb-2">Invalid or Expired Link</h1>
          <p className="text-text-secondary mb-6">
            This consent link is no longer valid. It may have expired or already been used.
          </p>
          <p className="text-sm text-text-tertiary">
            If you believe this is an error, please contact your child's school administrator.
          </p>
        </div>
      </div>
    );
  }

  /* ── Consent Granted ── */
  if (result === 'granted') {
    return (
      <div className="min-h-screen bg-brand-50 flex items-center justify-center px-4">
        <div className="bg-surface-0 rounded-2xl shadow-lg p-8 max-w-lg w-full text-center">
          <CheckCircleIcon className="w-16 h-16 text-accent-success mx-auto mb-4" />
          <h1 className="text-xl font-bold text-text-primary mb-2">Consent Granted</h1>
          <p className="text-text-secondary mb-4">
            Thank you for providing consent for <strong>{consent?.student_name}</strong>.
          </p>
          <div className="bg-accent-success/10 border border-accent-success/30 rounded-lg p-4 text-left mb-6">
            <h2 className="font-semibold text-accent-success mb-2">What happens next:</h2>
            <ul className="text-sm text-accent-success space-y-1 list-disc list-inside">
              <li>Your child's account at <strong>{consent?.account_name}</strong> is now fully active</li>
              <li>They can sign in and access all course materials</li>
              <li>You can withdraw consent at any time by contacting the school</li>
            </ul>
          </div>
          <p className="text-xs text-text-disabled">You may close this window.</p>
        </div>
      </div>
    );
  }

  /* ── Consent Denied ── */
  if (result === 'denied') {
    return (
      <div className="min-h-screen bg-brand-50 flex items-center justify-center px-4">
        <div className="bg-surface-0 rounded-2xl shadow-lg p-8 max-w-lg w-full text-center">
          <XCircleIcon className="w-16 h-16 text-accent-warning mx-auto mb-4" />
          <h1 className="text-xl font-bold text-text-primary mb-2">Consent Not Granted</h1>
          <p className="text-text-secondary mb-4">
            You have chosen not to provide consent for <strong>{consent?.student_name}</strong>.
          </p>
          <div className="bg-accent-warning/10 border border-accent-warning/30 rounded-lg p-4 text-left mb-6">
            <h2 className="font-semibold text-accent-warning mb-2">What this means:</h2>
            <ul className="text-sm text-accent-warning space-y-1 list-disc list-inside">
              <li>Your child's account will be limited and may not have full access to course features</li>
              <li>No personal information beyond basic enrollment will be stored</li>
              <li>Teachers may need to provide alternative participation methods</li>
              <li>You can change your decision at any time by contacting the school</li>
            </ul>
          </div>
          <p className="text-sm text-text-tertiary">
            Questions? Contact your child's school administrator.
          </p>
        </div>
      </div>
    );
  }

  /* ── Consent Form ── */
  return (
    <div className="min-h-screen bg-brand-50 py-8 px-4">
      <div className="max-w-2xl mx-auto">
        {/* Header */}
        <div className="bg-surface-0 rounded-2xl shadow-lg overflow-hidden">
          <div className="bg-brand-600 px-6 py-8 text-center">
            <ShieldIcon className="w-12 h-12 text-white mx-auto mb-3" />
            <h1 className="text-2xl font-bold text-white">Parental Consent Request</h1>
            <p className="text-brand-100 mt-1 text-sm">Paper LMS - Student Data Privacy</p>
          </div>

          <div className="p-6 sm:p-8">
            {/* Student Info */}
            <div className="bg-brand-50 border border-brand-100 rounded-lg p-4 mb-6">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div>
                  <p className="text-xs font-medium text-brand-500 uppercase tracking-wider">Student</p>
                  <p className="text-text-primary font-semibold">{consent?.student_name || 'Student'}</p>
                </div>
                <div>
                  <p className="text-xs font-medium text-brand-500 uppercase tracking-wider">School / Account</p>
                  <p className="text-text-primary font-semibold">{consent?.account_name || 'School'}</p>
                </div>
              </div>
            </div>

            {/* What is being requested */}
            <div className="mb-6">
              <h2 className="text-lg font-semibold text-text-primary mb-2">What is being requested?</h2>
              <p className="text-text-secondary text-sm leading-relaxed">
                {consent?.account_name || 'Your child\'s school'} uses Paper LMS as its learning management system.
                Because your child is under 13 years of age, the Children's Online Privacy Protection Act (COPPA)
                requires parental consent before collecting personal information.
              </p>
            </div>

            {/* Data Collected */}
            <div className="mb-6">
              <h2 className="text-lg font-semibold text-text-primary mb-3">Information Collected</h2>
              <div className="space-y-2">
                {DATA_COLLECTED.map((item, i) => (
                  <div key={i} className="flex items-start gap-3 p-3 bg-surface-1 rounded-lg">
                    <div className="w-1.5 h-1.5 rounded-full bg-brand-500 mt-2 shrink-0" aria-hidden="true" />
                    <div>
                      <p className="text-sm font-medium text-text-primary">{item.label}</p>
                      <p className="text-xs text-text-tertiary">{item.purpose}</p>
                    </div>
                  </div>
                ))}
              </div>
            </div>

            {/* COPPA Rights */}
            <div className="mb-6">
              <h2 className="text-lg font-semibold text-text-primary mb-3">Your Rights Under COPPA</h2>
              <p className="text-text-secondary text-sm mb-3">
                As a parent or legal guardian, you have the right to:
              </p>
              <ul className="space-y-2">
                {COPPA_RIGHTS.map((right, i) => (
                  <li key={i} className="flex items-start gap-2 text-sm text-text-secondary">
                    <CheckCircleIcon className="w-4 h-4 text-brand-500 mt-0.5 shrink-0" />
                    <span>{right}</span>
                  </li>
                ))}
              </ul>
            </div>

            {/* Error */}
            {error && (
              <div className="mb-4 p-3 bg-accent-danger/10 border border-accent-danger/30 rounded-lg text-accent-danger text-sm" role="alert" aria-live="assertive">
                {error}
              </div>
            )}

            {/* Consent Buttons */}
            <div className="flex flex-col sm:flex-row gap-3 mb-6">
              <button
                type="button"
                onClick={handleGrant}
                disabled={submitting}
                className="flex-1 bg-accent-success text-white py-3 px-6 rounded-lg font-semibold hover:bg-accent-success/90 focus:outline-none focus:ring-2 focus:ring-accent-success focus:ring-offset-2 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                aria-label="Grant parental consent"
              >
                {submitting ? 'Processing...' : 'I Give Consent'}
              </button>
              <button
                type="button"
                onClick={handleDeny}
                disabled={submitting}
                className="flex-1 bg-accent-danger text-white py-3 px-6 rounded-lg font-semibold hover:bg-accent-danger/90 focus:outline-none focus:ring-2 focus:ring-accent-danger focus:ring-offset-2 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                aria-label="Deny parental consent"
              >
                {submitting ? 'Processing...' : 'I Do Not Consent'}
              </button>
            </div>

            {/* Footer Links */}
            <div className="border-t border-border-subtle pt-5">
              <div className="flex flex-col sm:flex-row items-center justify-between gap-3 text-xs text-text-disabled">
                <a
                  href="/privacy"
                  className="text-brand-500 hover:underline focus:underline focus:outline-none"
                >
                  Read Full Privacy Policy
                </a>
                <p>
                  Questions? Contact{' '}
                  <a
                    href="mailto:privacy@paperlms.com"
                    className="text-brand-500 hover:underline focus:underline focus:outline-none"
                  >
                    privacy@paperlms.com
                  </a>
                </p>
              </div>
            </div>
          </div>
        </div>

        {/* Trust footer */}
        <p className="text-center text-xs text-text-disabled mt-6">
          Paper LMS is committed to protecting student privacy in compliance with COPPA, FERPA, and applicable state laws.
        </p>
      </div>
    </div>
  );
};

export default ParentalConsentPage;
