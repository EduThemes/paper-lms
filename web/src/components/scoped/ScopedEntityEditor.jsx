import React, { useEffect, useState } from 'react';
import { Lock } from 'lucide-react';
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogClose,
} from '@/components/ui/dialog';
import { CurrencyIcon } from '../gamification/currencyIcon';

const CODE_RE = /^[a-z][a-z0-9_]{1,31}$/;

// ScopedEntityEditor — shared create/edit dialog for tenant-admin
// gamification primitives (currencies, badges, etc.). Owns form-state
// lifecycle (reset on open, code-immutability, validation, body shape)
// and the chrome shared between all editors (Dialog wrapper, header,
// code + primary-label fields, icon palette, color palette, system-
// owned badge, save/cancel buttons).
//
// Entity-specific fields slot in via `renderExtraFields`; an optional
// preview block sits above the form via `renderPreview`.
//
// @param {string} entityName            "currency" / "badge" — drives title
//                                       copy and the "System X" subtitle
// @param {boolean} open
// @param {(o:boolean)=>void} onOpenChange
// @param {object|null} initialEntity    edit-mode payload; null = create
// @param {(body:object)=>Promise<void>} onSave
// @param {boolean} saving
// @param {string|null} saveError
// @param {object} formFields            entity-specific form-state seeds
//                                       and field metadata, see callers
// @param {string[]} iconPalette
// @param {string[]} colorPalette
// @param {(state, setState)=>React.Node} renderExtraFields
// @param {(state)=>React.Node} [renderPreview]
// @param {(state, isEdit)=>object} buildBody
//
// `formFields` is the entity-specific shape:
//   {
//     codePlaceholder:        string   e.g. "coins" / "first_quiz"
//     labelKey:               string   "display_label" / "name"
//     labelText:              string   "Display label" / "Name"
//     labelPlaceholder:       string
//     labelMaxLength:         number
//     codeImmutableHint:      string   shown on edit-mode for the code field
//     codeSystemHint:         string   shown when system_owned + edit-mode
//     initialState:           (entity|null) => object   start of form state
//     validateExtras:         (state) => boolean        extra validation
//   }
export default function ScopedEntityEditor({
  entityName,
  open,
  onOpenChange,
  initialEntity,
  onSave,
  saving = false,
  saveError = null,
  formFields,
  iconPalette,
  colorPalette,
  renderExtraFields,
  renderPreview,
  buildBody,
  dialogMaxWidthClass = 'max-w-lg',
}) {
  const isEdit = !!initialEntity;
  const isSystem = !!initialEntity?.system_owned;

  // The shared state. Entity-specific seeds come in via formFields.initialState.
  // `code` + the primary label + icon + color are shared, the rest is whatever
  // the caller put on the returned object.
  const [state, setState] = useState(() => formFields.initialState(initialEntity));

  // Reset the form whenever the editor opens — Radix keeps the dialog mounted
  // across opens, so we hydrate on `open` + `initialEntity` together.
  useEffect(() => {
    if (!open) return;
    setState(formFields.initialState(initialEntity));
    // formFields.initialState is intentionally not a dep: callers pass it
    // as a stable arrow inline. Re-running on the actual data is what we want.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, initialEntity]);

  // Helpers — fields that exist on every shared editor:
  const code = state.code || '';
  const label = state[formFields.labelKey] || '';
  const icon = state.icon || iconPalette[0];
  const color = state.color || '#A855F7';

  const setField = (k, v) => setState((s) => ({ ...s, [k]: v }));

  const codeValid = isEdit || CODE_RE.test(code);
  const labelValid = label.trim().length > 0 && label.length <= formFields.labelMaxLength;
  const colorValid = /^(#[0-9A-Fa-f]{6})?$/.test(color);
  const extrasValid = formFields.validateExtras ? formFields.validateExtras(state) : true;
  const formValid = codeValid && labelValid && colorValid && extrasValid;

  const handleSubmit = (e) => {
    e.preventDefault();
    if (!formValid) return;
    onSave(buildBody(state, isEdit));
  };

  const title = isEdit ? `Edit ${entityName}` : `New ${entityName}`;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className={`${dialogMaxWidthClass} rounded-lg bg-surface-0 border border-surface-raised p-0`}
        aria-describedby={undefined}
      >
        <header className="flex items-center justify-between px-5 py-3 border-b border-surface-raised">
          <DialogTitle className="text-base font-semibold text-text-primary">
            {title}
            {isSystem && (
              <span className="ml-2 inline-flex items-center gap-1 text-xs font-normal text-text-tertiary">
                <Lock className="w-3 h-3" /> System
              </span>
            )}
          </DialogTitle>
        </header>

        <form onSubmit={handleSubmit} className="px-5 py-4 space-y-4 max-h-[80vh] overflow-y-auto">
          {renderPreview && renderPreview(state)}

          <div className="grid grid-cols-2 gap-3">
            <label className="flex flex-col gap-1">
              <span className="text-xs font-medium text-text-secondary">Code</span>
              <input
                type="text"
                value={code}
                onChange={(e) => setField('code', e.target.value)}
                disabled={isEdit}
                placeholder={formFields.codePlaceholder}
                className="px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary disabled:opacity-60 disabled:cursor-not-allowed focus:outline-none focus:ring-2 focus:ring-brand-400/60"
              />
              {isEdit && (
                <span className="text-[11px] text-text-tertiary">
                  {isSystem ? formFields.codeSystemHint : formFields.codeImmutableHint}
                </span>
              )}
              {!isEdit && !codeValid && code && (
                <span className="text-[11px] text-accent-danger">
                  Lowercase, starts with a letter, 2–32 chars.
                </span>
              )}
            </label>
            <label className="flex flex-col gap-1">
              <span className="text-xs font-medium text-text-secondary">{formFields.labelText}</span>
              <input
                type="text"
                value={label}
                onChange={(e) => setField(formFields.labelKey, e.target.value)}
                maxLength={formFields.labelMaxLength}
                placeholder={formFields.labelPlaceholder}
                className="px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60"
              />
              {formFields.labelHint && (
                <span className="text-[11px] text-text-tertiary">{formFields.labelHint}</span>
              )}
            </label>
          </div>

          {renderExtraFields && renderExtraFields(state, setState)}

          <div>
            <span className="block text-xs font-medium text-text-secondary mb-1.5">Icon</span>
            <div className="flex flex-wrap gap-1.5">
              {iconPalette.map((name) => (
                <button
                  key={name}
                  type="button"
                  onClick={() => setField('icon', name)}
                  aria-pressed={icon === name}
                  title={name}
                  className={
                    'p-1.5 rounded-md border transition-colors ' +
                    (icon === name
                      ? 'border-brand-400 bg-surface-2'
                      : 'border-surface-raised bg-surface-1 hover:bg-surface-2')
                  }
                >
                  <CurrencyIcon icon={name} color={color} className="w-4 h-4" />
                </button>
              ))}
            </div>
          </div>

          <div>
            <span className="block text-xs font-medium text-text-secondary mb-1.5">Color</span>
            <div className="flex flex-wrap gap-1.5 items-center">
              {colorPalette.map((c) => (
                <button
                  key={c}
                  type="button"
                  onClick={() => setField('color', c)}
                  aria-label={`color ${c}`}
                  aria-pressed={color === c}
                  className={
                    'w-7 h-7 rounded-md border-2 transition-transform ' +
                    (color === c ? 'border-text-primary scale-110' : 'border-surface-raised')
                  }
                  style={{ backgroundColor: c }}
                />
              ))}
              <input
                type="text"
                value={color}
                onChange={(e) => setField('color', e.target.value)}
                placeholder="#A855F7"
                className="ml-2 w-24 px-2 py-1 rounded-md border border-surface-raised bg-surface-1 text-xs text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60"
              />
              {!colorValid && (
                <span className="text-[11px] text-accent-danger">6-digit hex required</span>
              )}
            </div>
          </div>

          {saveError && (
            <div className="text-sm text-accent-danger border border-accent-danger rounded-md px-3 py-2">
              {saveError}
            </div>
          )}

          <div className="flex justify-end gap-2 pt-2">
            <DialogClose asChild>
              <button
                type="button"
                className="px-3 py-1.5 rounded-md text-sm text-text-secondary hover:bg-surface-2"
              >
                Cancel
              </button>
            </DialogClose>
            <button
              type="submit"
              disabled={!formValid || saving}
              className="px-3 py-1.5 rounded-md text-sm font-medium bg-brand-600 text-white hover:bg-brand-700 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {saving ? 'Saving…' : isEdit ? 'Save changes' : `Create ${entityName}`}
            </button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

// Shared CheckRow used by both CurrencyEditor and BadgeEditor extra-field
// blocks. Mirrors the original behavior — bool checkbox + label + hint.
export function CheckRow({ label, hint, checked, onChange }) {
  return (
    <label className="flex items-start gap-2 cursor-pointer">
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        className="mt-1 accent-brand-600"
      />
      <span className="flex-1">
        <span className="block text-sm text-text-primary">{label}</span>
        <span className="block text-xs text-text-tertiary">{hint}</span>
      </span>
    </label>
  );
}
