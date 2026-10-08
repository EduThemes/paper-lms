import React from 'react';

// Small shared building blocks for the account-administration pages.

export function Section({ title, description, actions, children }) {
  return (
    <section className="rounded-lg border border-border-default bg-surface-0 p-5 mb-6">
      <div className="flex items-start justify-between gap-4 mb-4">
        <div>
          <h2 className="text-lg font-semibold text-text-primary">{title}</h2>
          {description && <p className="text-sm text-text-secondary mt-0.5">{description}</p>}
        </div>
        {actions && <div className="flex items-center gap-2 shrink-0">{actions}</div>}
      </div>
      {children}
    </section>
  );
}

export function Field({ label, id, hint, children }) {
  return (
    <div>
      <label htmlFor={id} className="block text-sm font-medium text-text-secondary mb-1">{label}</label>
      {children}
      {hint && <p className="text-xs text-text-tertiary mt-1">{hint}</p>}
    </div>
  );
}

export const inputClass =
  'w-full rounded-md border border-border-strong bg-surface-0 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500';

export function Button({ variant = 'primary', className = '', ...props }) {
  const styles = {
    primary: 'bg-brand-600 text-white hover:bg-brand-700',
    secondary: 'border border-border-strong bg-surface-0 text-text-primary hover:bg-surface-1',
    danger: 'border border-accent-danger/40 text-accent-danger hover:bg-accent-danger/5',
  };
  return (
    <button
      type="button"
      className={`inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium disabled:opacity-50 ${styles[variant]} ${className}`}
      {...props}
    />
  );
}

export function ErrorNote({ error }) {
  if (!error) return null;
  return (
    <div role="alert" className="mb-4 rounded-md border border-accent-danger/30 bg-accent-danger/5 p-3 text-sm text-accent-danger">
      {error.message || String(error)}
    </div>
  );
}

export function StatusNote({ message }) {
  if (!message) return null;
  return <p role="status" className="text-xs text-accent-success">{message}</p>;
}

export function Badge({ tone = 'neutral', children }) {
  const tones = {
    neutral: 'bg-surface-1 text-text-secondary border-border-default',
    good: 'bg-accent-success/10 text-accent-success border-accent-success/30',
    warn: 'bg-accent-warning/10 text-accent-warning border-accent-warning/30',
    bad: 'bg-accent-danger/10 text-accent-danger border-accent-danger/30',
    brand: 'bg-brand-50 text-brand-700 border-brand-200',
  };
  return (
    <span className={`inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium ${tones[tone]}`}>
      {children}
    </span>
  );
}

export function formatDateTime(value) {
  if (!value) return '—';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return '—';
  return d.toLocaleString();
}
