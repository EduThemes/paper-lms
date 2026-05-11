import React, { useRef, useState } from 'react';
import { Link2, CheckCircle2 } from 'lucide-react';
import { api } from '../../services/api';

/**
 * RedeemCodeForm
 *
 * Observer-facing form for redeeming a parent/observer pairing code. Renders
 * three segment inputs that auto-tab to the next on three valid characters,
 * matching the pairing-code format (e.g. "K7H-PQM-3RD").
 *
 * Props:
 *   - onSuccess?: (response) => void — called with the redeem response after a
 *                                       successful link.
 *   - onError?:   (message) => void  — called with an error message if redeem
 *                                       fails (the form also displays it inline).
 *   - autoFocus?: boolean — focus the first segment on mount (default true).
 */
const ALPHA_RE = /^[A-Z0-9]*$/;

const sanitize = (s) =>
  (s || '')
    .toUpperCase()
    .replace(/[^A-Z0-9]/g, '')
    .slice(0, 3);

const RedeemCodeForm = ({ onSuccess, onError, autoFocus = true }) => {
  const [segments, setSegments] = useState(['', '', '']);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [success, setSuccess] = useState(null);
  const refs = [useRef(null), useRef(null), useRef(null)];

  const updateSegment = (idx, raw) => {
    const cleaned = sanitize(raw);
    setSegments((prev) => {
      const next = [...prev];
      next[idx] = cleaned;
      return next;
    });
    if (cleaned.length === 3 && idx < 2) {
      refs[idx + 1].current?.focus();
    }
  };

  const handleKeyDown = (idx, e) => {
    if (e.key === 'Backspace' && !segments[idx] && idx > 0) {
      refs[idx - 1].current?.focus();
    }
  };

  const handlePaste = (idx, e) => {
    const text = e.clipboardData.getData('text');
    if (!text) return;
    e.preventDefault();
    const cleaned = (text || '')
      .toUpperCase()
      .replace(/[^A-Z0-9]/g, '');
    if (cleaned.length >= 9) {
      setSegments([cleaned.slice(0, 3), cleaned.slice(3, 6), cleaned.slice(6, 9)]);
      refs[2].current?.focus();
      return;
    }
    // Otherwise, fall back to filling from the current segment forward.
    let cursor = 0;
    const next = [...segments];
    for (let i = idx; i < 3 && cursor < cleaned.length; i++) {
      next[i] = cleaned.slice(cursor, cursor + 3);
      cursor += 3;
    }
    setSegments(next);
  };

  const fullCode = segments.join('-');
  const ready = segments.every((s) => s.length === 3) && ALPHA_RE.test(segments.join(''));

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!ready || loading) return;
    setLoading(true);
    setError(null);
    setSuccess(null);
    try {
      const res = await api.redeemPairingCode(fullCode);
      setSuccess(res);
      setSegments(['', '', '']);
      if (onSuccess) onSuccess(res);
    } catch (err) {
      const msg = err.message || 'Could not redeem pairing code.';
      setError(msg);
      if (onError) onError(msg);
    } finally {
      setLoading(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-3">
      <div>
        <label className="mb-1 block text-sm font-medium text-text-secondary">
          Pairing code
        </label>
        <div className="flex items-center gap-2">
          {segments.map((seg, i) => (
            <React.Fragment key={i}>
              <input
                ref={refs[i]}
                type="text"
                inputMode="text"
                autoCapitalize="characters"
                autoComplete="off"
                spellCheck={false}
                value={seg}
                maxLength={3}
                onChange={(e) => updateSegment(i, e.target.value)}
                onKeyDown={(e) => handleKeyDown(i, e)}
                onPaste={(e) => handlePaste(i, e)}
                autoFocus={autoFocus && i === 0}
                aria-label={`Pairing code segment ${i + 1} of 3`}
                className="w-20 rounded-md border border-border-strong bg-surface-0 px-2 py-2 text-center font-mono text-lg font-semibold uppercase tracking-widest text-text-primary focus:border-brand-500 focus:outline-none focus:ring-2 focus:ring-brand-500"
                placeholder="XXX"
              />
              {i < 2 && <span className="text-text-disabled">-</span>}
            </React.Fragment>
          ))}
        </div>
        <p className="mt-1 text-xs text-text-tertiary">
          Enter the 9-character code your student shared with you.
        </p>
      </div>

      {error && (
        <div className="rounded-md border border-accent-danger/30 bg-accent-danger/10 px-3 py-2 text-sm text-accent-danger">
          {error}
        </div>
      )}

      {success && (
        <div className="flex items-start gap-2 rounded-md border border-accent-success/30 bg-accent-success/10 px-3 py-2 text-sm text-accent-success">
          <CheckCircle2 className="mt-0.5 h-4 w-4 flex-shrink-0" />
          <span>Successfully linked. Your student will appear in your dashboard.</span>
        </div>
      )}

      <button
        type="submit"
        disabled={!ready || loading}
        className="inline-flex items-center gap-2 rounded-md bg-brand-600 px-4 py-2 text-sm font-medium text-white hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-50"
      >
        <Link2 className="h-4 w-4" />
        {loading ? 'Linking…' : 'Link account'}
      </button>
    </form>
  );
};

export default RedeemCodeForm;
