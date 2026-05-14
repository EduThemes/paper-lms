import React, { useEffect, useState } from 'react';
import { api } from '../../../services/api';

// Shared entity pickers for the recipe builder. Each one fetches the
// W2-B/W2-D list endpoint on mount and renders a <select>. Kept
// lightweight (native select, no Radix popover) because the recipe
// editor already mounts a Radix dialog and stacking poppovers inside
// dialogs is a known accessibility/focus-trap headache.
//
// scope semantics: pickers are scope-agnostic in W2-E.3 (they show
// every row the tenant has access to — site + course in one list).
// That mirrors how AwardCurrency.code / AwardBadge.code resolve at
// runtime: the effect walks from the firing rule's scope up to site,
// picking whichever row matches the code first. Filtering the picker
// would lie about runtime behavior.

const selectCls =
  'px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60';

// CurrencyPicker — returns a currency `code` string.
// Use for AwardCurrency.code and CurrencyThreshold.code.
export function CurrencyPicker({ value, onChange, required = false, label = 'Currency' }) {
  const [rows, setRows] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    let alive = true;
    api.gamification.listCurrencies().then(
      (data) => {
        if (!alive) return;
        const list = (data?.currencies || []).slice().sort((a, b) =>
          (a.display_label || a.code).localeCompare(b.display_label || b.code),
        );
        setRows(list);
        setLoading(false);
      },
      (err) => {
        if (!alive) return;
        setError(err);
        setLoading(false);
      },
    );
    return () => {
      alive = false;
    };
  }, []);

  return (
    <label className="flex flex-col gap-1">
      <span className="text-xs font-medium text-text-secondary">
        {label}{required && <span className="text-accent-danger"> *</span>}
      </span>
      <select
        value={value || ''}
        onChange={(e) => onChange(e.target.value)}
        className={selectCls}
        disabled={loading || !!error}
      >
        <option value="" disabled>
          {loading ? 'Loading…' : error ? 'Failed to load' : 'Select currency…'}
        </option>
        {rows.map((c) => (
          <option key={`${c.scope_type}-${c.id}`} value={c.code}>
            {c.display_label || c.code} ({c.code}){c.scope_type === 'course' ? ' • course' : ''}
          </option>
        ))}
      </select>
    </label>
  );
}

// BadgePicker — returns either a badge `code` (default) or its `id`,
// based on `returnId`. AwardBadge uses code; EarnedBadge predicate
// uses id. Same underlying list, two return shapes.
export function BadgePicker({ value, onChange, required = false, label = 'Badge', returnId = false }) {
  const [rows, setRows] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    let alive = true;
    api.gamification.listBadges().then(
      (data) => {
        if (!alive) return;
        const list = (data?.badges || []).slice().sort((a, b) =>
          (a.name || a.code).localeCompare(b.name || b.code),
        );
        setRows(list);
        setLoading(false);
      },
      (err) => {
        if (!alive) return;
        setError(err);
        setLoading(false);
      },
    );
    return () => {
      alive = false;
    };
  }, []);

  const stringValue = value == null ? '' : String(value);

  return (
    <label className="flex flex-col gap-1">
      <span className="text-xs font-medium text-text-secondary">
        {label}{required && <span className="text-accent-danger"> *</span>}
      </span>
      <select
        value={stringValue}
        onChange={(e) => {
          const raw = e.target.value;
          if (raw === '') {
            onChange(undefined);
            return;
          }
          onChange(returnId ? Number(raw) : raw);
        }}
        className={selectCls}
        disabled={loading || !!error}
      >
        <option value="" disabled>
          {loading ? 'Loading…' : error ? 'Failed to load' : 'Select badge…'}
        </option>
        {rows.map((b) => (
          <option key={b.id} value={returnId ? b.id : b.code}>
            {b.name || b.code} ({b.code}){b.scope_type === 'course' ? ' • course' : ''}
          </option>
        ))}
      </select>
    </label>
  );
}
