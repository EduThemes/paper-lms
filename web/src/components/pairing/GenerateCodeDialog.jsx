import React, { useEffect, useState } from 'react';
import { X, Copy, Check, RefreshCw, ShieldAlert } from 'lucide-react';
import { api } from '../../services/api';

/**
 * GenerateCodeDialog
 *
 * Student-facing modal for generating a parent/observer pairing code. The
 * generated code is displayed in a large, readable font with a copy button.
 *
 * Props:
 *   - open: boolean — whether the modal is visible.
 *   - onClose: () => void — fires when the user dismisses the modal.
 *   - onGenerated?: (pairingCode) => void — optional callback after success.
 */
const GenerateCodeDialog = ({ open, onClose, onGenerated }) => {
  const [pairingCode, setPairingCode] = useState(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [copied, setCopied] = useState(false);

  const generate = async () => {
    setLoading(true);
    setError(null);
    setCopied(false);
    try {
      const result = await api.generatePairingCode();
      setPairingCode(result);
      if (onGenerated) onGenerated(result);
    } catch (err) {
      setError(err.message || 'Could not generate pairing code.');
    } finally {
      setLoading(false);
    }
  };

  // Auto-generate the first code when opened.
  useEffect(() => {
    if (open && !pairingCode && !loading) {
      generate();
    }
    if (!open) {
      setPairingCode(null);
      setError(null);
      setCopied(false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const handleCopy = async () => {
    if (!pairingCode?.code) return;
    try {
      await navigator.clipboard.writeText(pairingCode.code);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (e) {
      setError('Could not copy to clipboard.');
    }
  };

  const formatExpiry = (iso) => {
    if (!iso) return '';
    try {
      const d = new Date(iso);
      return d.toLocaleString();
    } catch (e) {
      return iso;
    }
  };

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="pairing-code-title"
    >
      <div className="relative w-full max-w-md rounded-xl bg-surface-0 shadow-xl">
        <div className="flex items-start justify-between border-b border-border-default px-6 py-4">
          <h2 id="pairing-code-title" className="text-lg font-semibold text-text-primary">
            Generate a parent pairing code
          </h2>
          <button
            type="button"
            onClick={onClose}
            className="rounded-md p-1 text-text-disabled hover:bg-surface-2 hover:text-text-secondary"
            aria-label="Close"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="px-6 py-6">
          {loading && !pairingCode && (
            <div className="flex items-center justify-center py-8">
              <svg
                className="h-8 w-8 animate-spin text-brand-600"
                xmlns="http://www.w3.org/2000/svg"
                fill="none"
                viewBox="0 0 24 24"
              >
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                <path
                  className="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"
                />
              </svg>
            </div>
          )}

          {error && (
            <div className="mb-4 rounded-md border border-accent-danger/30 bg-accent-danger/10 px-4 py-3 text-sm text-accent-danger">
              <p className="mb-2">{error}</p>
              <button
                type="button"
                onClick={generate}
                className="inline-flex items-center gap-1 text-sm font-medium text-accent-danger underline hover:text-red-900"
              >
                <RefreshCw className="h-3 w-3" /> Try Again
              </button>
            </div>
          )}

          {pairingCode && (
            <div>
              <p className="mb-3 text-sm text-text-secondary">
                Share this code with your parent. They will use it to link their account to yours.
              </p>
              <div className="my-4 flex items-center justify-between gap-3 rounded-lg border-2 border-blue-200 bg-brand-50 px-4 py-5">
                <span className="select-all font-mono text-3xl font-bold tracking-widest text-blue-900">
                  {pairingCode.code}
                </span>
                <button
                  type="button"
                  onClick={handleCopy}
                  className="inline-flex items-center gap-1 rounded-md bg-brand-600 px-3 py-2 text-sm font-medium text-white hover:bg-brand-700"
                >
                  {copied ? (
                    <>
                      <Check className="h-4 w-4" /> Copied
                    </>
                  ) : (
                    <>
                      <Copy className="h-4 w-4" /> Copy
                    </>
                  )}
                </button>
              </div>
              <p className="text-xs text-text-tertiary">
                Expires: <span className="font-medium">{formatExpiry(pairingCode.expires_at)}</span>
              </p>
              <div className="mt-4 flex items-start gap-2 rounded-md border border-accent-warning/30 bg-accent-warning/10 px-3 py-2 text-xs text-amber-800">
                <ShieldAlert className="mt-0.5 h-4 w-4 flex-shrink-0" />
                <p>
                  Share this code with your parent only. Anyone with this code can link their
                  account to yours and view your courses, grades, and assignments.
                </p>
              </div>
              <div className="mt-5 flex justify-end gap-2">
                <button
                  type="button"
                  onClick={generate}
                  disabled={loading}
                  className="rounded-md border border-border-strong bg-surface-0 px-4 py-2 text-sm font-medium text-text-secondary hover:bg-surface-1 disabled:opacity-50"
                >
                  {loading ? 'Generating…' : 'Generate another'}
                </button>
                <button
                  type="button"
                  onClick={onClose}
                  className="rounded-md bg-brand-600 px-4 py-2 text-sm font-medium text-white hover:bg-brand-700"
                >
                  Done
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default GenerateCodeDialog;
