import React from 'react';
import { paramsForKind } from '../../../hooks/useGamificationVocabulary';

// TriggerPicker — kind selector + per-kind inline editor.
//
// W2-E.2 ships the same three kinds the rule index recognises today
// (OnEvent, OnSchedule, OnManualTrigger). The editor inputs are
// driven entirely off the vocabulary catalog so when a new trigger
// kind lands on the backend, the UI surfaces it automatically — no
// hard-coded "is this OnEvent?" branches here beyond the kind switch
// that picks which inputs to render.
//
// `value` is the in-progress trigger_event JSON object as the parent
// would POST it: { kind, verb?, object_type?, cron?, handle? }.
// `onChange(next)` is called whenever any field mutates; parents are
// free to debounce.

export default function TriggerPicker({ value, vocab, onChange }) {
  const trigger = value || { kind: 'OnEvent' };
  const triggers = vocab?.triggers || [];

  const handleKindChange = (kind) => {
    // Reset per-kind fields when switching — keep one trigger shape
    // canonical in the JSON, no stale leftover fields from a previous
    // kind. Server-side validator would reject extras anyway.
    onChange({ kind });
  };

  const handleFieldChange = (name, fieldValue) => {
    onChange({ ...trigger, [name]: fieldValue });
  };

  const kindParams = paramsForKind(triggers, trigger.kind);

  return (
    <section className="space-y-2">
      <div className="text-xs font-medium uppercase tracking-wide text-text-tertiary">
        When
      </div>

      <div className="flex flex-wrap gap-1.5" role="radiogroup" aria-label="Trigger kind">
        {triggers.map((t) => {
          const active = trigger.kind === t.kind;
          return (
            <button
              type="button"
              key={t.kind}
              role="radio"
              aria-checked={active}
              onClick={() => handleKindChange(t.kind)}
              className={`px-2.5 py-1 rounded-md text-xs border transition-colors ${
                active
                  ? 'bg-brand-400/10 border-brand-400 text-text-primary'
                  : 'bg-surface-1 border-surface-raised text-text-secondary hover:bg-surface-2'
              }`}
            >
              {humanizeKind(t.kind)}
            </button>
          );
        })}
      </div>

      <div className="grid grid-cols-2 gap-3">
        {kindParams.map((p) => (
          <ParamField
            key={p.name}
            spec={p}
            value={trigger[p.name]}
            onChange={(v) => handleFieldChange(p.name, v)}
          />
        ))}
      </div>
    </section>
  );
}

// ParamField — generic input router based on a vocabulary ParamSpec.
// Used here for trigger params; predicate editors compose their own
// fields directly so they can pin layout per kind, but the renderer
// is identical in spirit. Co-located so the trigger picker stays a
// single-file unit; if a predicate editor ever wants a "render this
// catalog ParamSpec for me" affordance, lift this to a shared file.
function ParamField({ spec, value, onChange }) {
  const id = `trigger-${spec.name}`;
  const label = humanizeFieldName(spec.name);

  if (spec.type === 'enum') {
    return (
      <label className="flex flex-col gap-1" htmlFor={id}>
        <span className="text-xs font-medium text-text-secondary">
          {label}{spec.required && <span className="text-accent-danger"> *</span>}
        </span>
        <select
          id={id}
          value={value || ''}
          onChange={(e) => onChange(e.target.value)}
          className="px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60"
        >
          <option value="" disabled>Select…</option>
          {(spec.enum || []).map((opt) => (
            <option key={opt} value={opt}>{opt}</option>
          ))}
        </select>
      </label>
    );
  }

  if (spec.type === 'string') {
    return (
      <label className="flex flex-col gap-1" htmlFor={id}>
        <span className="text-xs font-medium text-text-secondary">
          {label}{spec.required && <span className="text-accent-danger"> *</span>}
        </span>
        <input
          id={id}
          type="text"
          value={value || ''}
          onChange={(e) => onChange(e.target.value)}
          placeholder={spec.description || ''}
          className="px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60"
        />
      </label>
    );
  }

  // Fallback for types this skeleton renderer doesn't yet handle (no
  // current triggers use int/float/bool/ref params, but additions on
  // the backend won't silently disappear — they render as a labeled
  // text input until a typed editor lands).
  return (
    <label className="flex flex-col gap-1" htmlFor={id}>
      <span className="text-xs font-medium text-text-secondary">
        {label}{spec.required && <span className="text-accent-danger"> *</span>}
      </span>
      <input
        id={id}
        type="text"
        value={value ?? ''}
        onChange={(e) => onChange(e.target.value)}
        className="px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60"
      />
    </label>
  );
}

// humanizeKind / humanizeFieldName: tiny presentation helpers. Keep
// here rather than a global util — they're only meaningful in the
// recipe-editor surface and would otherwise tempt callers to use them
// elsewhere.
function humanizeKind(kind) {
  const map = {
    OnEvent: 'When an event happens',
    OnSchedule: 'On a schedule',
    OnManualTrigger: 'Triggered manually',
    SubmittedAssignment: 'Submitted assignment',
    SubmittedQuiz: 'Submitted quiz',
    ViewedContent: 'Viewed content',
    OutcomeMastery: 'Reached outcome mastery',
    CurrencyThreshold: 'Currency balance',
    EarnedBadge: 'Earned badge',
    ReputationThreshold: 'Reputation balance',
  };
  return map[kind] || kind;
}

function humanizeFieldName(name) {
  return name
    .split('_')
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(' ');
}
