import React from 'react';
import { CurrencyPicker, BadgePicker } from './EntityPickers';

// PredicateEditor — switch on predicate kind, render a per-kind inline
// form. Each editor reads + writes a plain JSON object that round-
// trips through `predicates.DecodePredicate` on the backend (W2-E.1
// validator). When a kind is unknown to the UI (catalog ahead of the
// frontend bundle) we render the raw JSON as a readonly fallback so
// the rule remains editable as a whole even if this leaf can't be.
//
// ref-typed fields (assignment_id, quiz_id, content_id, outcome_id,
// badge_id) are number inputs in E.2 — real entity pickers land in
// W2-E.3 alongside the effects palette's currency + badge pickers.

export default function PredicateEditor({ value, vocab, onChange }) {
  const predicate = value || {};

  switch (predicate.kind) {
    case 'SubmittedAssignment':
      return <SubmittedAssignmentFields value={predicate} onChange={onChange} />;
    case 'SubmittedQuiz':
      return <SubmittedQuizFields value={predicate} onChange={onChange} />;
    case 'ViewedContent':
      return <ViewedContentFields value={predicate} onChange={onChange} />;
    case 'OutcomeMastery':
      return <OutcomeMasteryFields value={predicate} vocab={vocab} onChange={onChange} />;
    case 'CurrencyThreshold':
      return <CurrencyThresholdFields value={predicate} onChange={onChange} />;
    case 'EarnedBadge':
      return <EarnedBadgeFields value={predicate} onChange={onChange} />;
    case 'ReputationThreshold':
      return <ReputationThresholdFields value={predicate} onChange={onChange} />;
    default:
      return <UnknownKindFallback value={predicate} />;
  }
}

// ----------------------------------------------------------------------
// Field-group components. Inline rather than per-file because each one
// is two to four inputs.
// ----------------------------------------------------------------------

function SubmittedAssignmentFields({ value, onChange }) {
  return (
    <Row>
      <UintField label="Assignment ID" name="assignment_id" required value={value} onChange={onChange} />
      <FloatField label="Min score" name="min_score" value={value} onChange={onChange} />
      <FloatField label="Max score" name="max_score" value={value} onChange={onChange} />
      <BoolField label="Require on-time" name="require_on_time" value={value} onChange={onChange} />
    </Row>
  );
}

function SubmittedQuizFields({ value, onChange }) {
  return (
    <Row>
      <UintField label="Quiz ID" name="quiz_id" required value={value} onChange={onChange} />
      <FloatField label="Min score" name="min_score" value={value} onChange={onChange} />
      <FloatField label="Max score" name="max_score" value={value} onChange={onChange} />
    </Row>
  );
}

function ViewedContentFields({ value, onChange }) {
  return (
    <Row>
      <UintField label="Content ID" name="content_id" required value={value} onChange={onChange} />
      <IntField label="Min views" name="min_views" value={value} onChange={onChange} />
      <IntField label="Min seconds viewed" name="min_seconds_viewed" value={value} onChange={onChange} />
    </Row>
  );
}

function OutcomeMasteryFields({ value, vocab, onChange }) {
  const levels = vocab?.mastery_levels || ['novice', 'familiar', 'proficient', 'mastered'];
  return (
    <Row>
      <UintField label="Outcome ID" name="outcome_id" required value={value} onChange={onChange} />
      <label className="flex flex-col gap-1">
        <span className="text-xs font-medium text-text-secondary">Min level <span className="text-accent-danger">*</span></span>
        <select
          value={value.min_level || ''}
          onChange={(e) => onChange({ ...value, min_level: e.target.value })}
          className={selectCls}
        >
          <option value="" disabled>Select…</option>
          {levels.map((lvl) => (
            <option key={lvl} value={lvl}>{lvl}</option>
          ))}
        </select>
      </label>
      <StringField label="Calc method (optional)" name="calc_method" value={value} onChange={onChange} />
    </Row>
  );
}

function CurrencyThresholdFields({ value, onChange }) {
  return (
    <Row>
      <CurrencyPicker
        value={value.code}
        onChange={(code) => onChange({ ...value, code })}
        required
        label="Currency"
      />
      <IntField label="Min amount" name="min_amount" required value={value} onChange={onChange} />
    </Row>
  );
}

function EarnedBadgeFields({ value, onChange }) {
  return (
    <Row>
      <BadgePicker
        value={value.badge_id}
        onChange={(id) => onChange({ ...value, badge_id: id })}
        required
        returnId
        label="Badge"
      />
    </Row>
  );
}

function ReputationThresholdFields({ value, onChange }) {
  return (
    <Row>
      <IntField label="Min amount" name="min_amount" required value={value} onChange={onChange} />
    </Row>
  );
}

function UnknownKindFallback({ value }) {
  return (
    <div className="rounded-md border border-accent-warning/40 bg-accent-warning/10 px-2.5 py-1.5 text-xs text-text-secondary">
      Unknown predicate kind <code className="font-mono">{value.kind || '?'}</code> — saved as-is. Open in a newer client to edit.
    </div>
  );
}

// ----------------------------------------------------------------------
// Field primitives. Shared so the seven editors above stay compact.
// ----------------------------------------------------------------------

const inputCls =
  'px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60';
const selectCls = inputCls;

function Row({ children }) {
  return <div className="grid grid-cols-2 gap-2.5">{children}</div>;
}

function UintField({ label, name, value, onChange, required = false }) {
  return (
    <label className="flex flex-col gap-1">
      <span className="text-xs font-medium text-text-secondary">
        {label}{required && <span className="text-accent-danger"> *</span>}
      </span>
      <input
        type="number"
        min="0"
        step="1"
        value={value[name] ?? ''}
        onChange={(e) => {
          const raw = e.target.value;
          onChange({ ...value, [name]: raw === '' ? undefined : Number(raw) });
        }}
        className={inputCls}
      />
    </label>
  );
}

function IntField(props) {
  // Same input shape as UintField — separate label so future negative-
  // allowed predicates (none today) don't have to rewrite UintField's
  // contract.
  return <UintField {...props} />;
}

function FloatField({ label, name, value, onChange, required = false }) {
  return (
    <label className="flex flex-col gap-1">
      <span className="text-xs font-medium text-text-secondary">
        {label}{required && <span className="text-accent-danger"> *</span>}
      </span>
      <input
        type="number"
        step="any"
        value={value[name] ?? ''}
        onChange={(e) => {
          const raw = e.target.value;
          onChange({ ...value, [name]: raw === '' ? undefined : Number(raw) });
        }}
        className={inputCls}
      />
    </label>
  );
}

function BoolField({ label, name, value, onChange }) {
  return (
    <label className="flex items-center gap-2 text-xs font-medium text-text-secondary">
      <input
        type="checkbox"
        checked={!!value[name]}
        onChange={(e) => onChange({ ...value, [name]: e.target.checked })}
        className="h-3.5 w-3.5 rounded border-surface-raised"
      />
      {label}
    </label>
  );
}

function StringField({ label, name, value, onChange, required = false }) {
  return (
    <label className="flex flex-col gap-1">
      <span className="text-xs font-medium text-text-secondary">
        {label}{required && <span className="text-accent-danger"> *</span>}
      </span>
      <input
        type="text"
        value={value[name] ?? ''}
        onChange={(e) => onChange({ ...value, [name]: e.target.value })}
        className={inputCls}
      />
    </label>
  );
}

// PREDICATE_KINDS is the ordered list the "Add condition" dropdown
// uses to populate. Exported so ConditionNode can render the menu
// without duplicating the list.
export const PREDICATE_KINDS = [
  'SubmittedAssignment',
  'SubmittedQuiz',
  'ViewedContent',
  'OutcomeMastery',
  'CurrencyThreshold',
  'EarnedBadge',
  'ReputationThreshold',
];

// Returns the empty starter object for a fresh predicate of the given
// kind. Mirrors what the backend's validator expects post-merge.
export function emptyPredicate(kind) {
  return { kind };
}
