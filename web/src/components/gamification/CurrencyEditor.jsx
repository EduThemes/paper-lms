import React, { useEffect, useState } from 'react';
import { Lock, X } from 'lucide-react';
import * as DialogPrimitive from '@radix-ui/react-dialog';
import { CurrencyIcon } from './currencyIcon';

// Curated icon palette for teachers. Strings match the keys CurrencyIcon
// understands; the four system icons (zap/gem/target/shield-check) are
// included so admins renaming system rows can pick from a consistent set.
const ICON_PALETTE = [
  'zap',
  'gem',
  'target',
  'shield-check',
  'coins',
  'sparkles',
  'award',
  'star',
  'trophy',
  'heart',
  'crown',
  'flame',
  'diamond',
  'medal',
];

const DEFAULT_COLORS = [
  '#F59E0B', // amber
  '#A855F7', // purple
  '#0EA5E9', // sky
  '#10B981', // emerald
  '#EF4444', // red
  '#3B82F6', // blue
  '#F97316', // orange
  '#64748B', // slate
];

const codeRE = /^[a-z][a-z0-9_]{1,31}$/;

// CurrencyEditor is a Radix dialog form for creating or editing a single
// currency. Pass `currency=null` for create-mode, or a row for edit-mode.
// onSave receives the API-shaped body and is responsible for the actual
// fetch + error handling; this component manages local form state only.
export default function CurrencyEditor({
  open,
  onOpenChange,
  currency,
  onSave,
  saving = false,
  saveError = null,
}) {
  const isEdit = !!currency;
  const isSystem = !!currency?.system_owned;

  const [code, setCode] = useState('');
  const [displayLabel, setDisplayLabel] = useState('');
  const [displayLabelPlural, setDisplayLabelPlural] = useState('');
  const [icon, setIcon] = useState('coins');
  const [color, setColor] = useState('#A855F7');
  const [description, setDescription] = useState('');
  const [displayOrder, setDisplayOrder] = useState(0);
  const [spendable, setSpendable] = useState(false);
  const [monotonic, setMonotonic] = useState(true);
  const [visibleToStudent, setVisibleToStudent] = useState(true);
  const [visibleInTopbar, setVisibleInTopbar] = useState(true);

  // Reset form whenever the editor opens — Radix keeps the dialog mounted
  // across opens, so we hydrate on `open` + `currency` together.
  useEffect(() => {
    if (!open) return;
    if (currency) {
      setCode(currency.code || '');
      setDisplayLabel(currency.display_label || '');
      setDisplayLabelPlural(currency.display_label_plural || '');
      setIcon(currency.icon || 'coins');
      setColor(currency.color || '#A855F7');
      setDescription(currency.description || '');
      setDisplayOrder(currency.display_order ?? 0);
      setSpendable(!!currency.spendable);
      setMonotonic(currency.monotonic !== false);
      setVisibleToStudent(currency.visible_to_student !== false);
      setVisibleInTopbar(currency.visible_in_topbar !== false);
    } else {
      setCode('');
      setDisplayLabel('');
      setDisplayLabelPlural('');
      setIcon('coins');
      setColor('#A855F7');
      setDescription('');
      setDisplayOrder(0);
      setSpendable(false);
      setMonotonic(true);
      setVisibleToStudent(true);
      setVisibleInTopbar(true);
    }
  }, [open, currency]);

  const codeValid = isEdit || codeRE.test(code);
  const labelValid = displayLabel.trim().length > 0 && displayLabel.length <= 64;
  const colorValid = /^(#[0-9A-Fa-f]{6})?$/.test(color);
  const formValid = codeValid && labelValid && colorValid;

  const handleSubmit = (e) => {
    e.preventDefault();
    if (!formValid) return;
    const body = {
      display_label: displayLabel.trim(),
      display_label_plural: displayLabelPlural.trim(),
      icon,
      color,
      display_order: Number(displayOrder) || 0,
      spendable,
      monotonic,
      visible_to_student: visibleToStudent,
      visible_in_topbar: visibleInTopbar,
      description: description.trim(),
    };
    if (!isEdit) {
      body.code = code.trim();
    }
    onSave(body);
  };

  return (
    <DialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
      <DialogPrimitive.Portal>
        <DialogPrimitive.Overlay className="fixed inset-0 z-50 bg-black/40 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 motion-reduce:transition-none" />
        <DialogPrimitive.Content
          className="fixed left-1/2 top-1/2 z-50 w-full max-w-lg -translate-x-1/2 -translate-y-1/2 rounded-lg bg-surface-0 shadow-xl border border-surface-raised data-[state=open]:animate-in data-[state=closed]:animate-out motion-reduce:duration-0"
          aria-describedby={undefined}
        >
          <header className="flex items-center justify-between px-5 py-3 border-b border-surface-raised">
            <DialogPrimitive.Title className="text-base font-semibold text-text-primary">
              {isEdit ? 'Edit currency' : 'New currency'}
              {isSystem && (
                <span className="ml-2 inline-flex items-center gap-1 text-xs font-normal text-text-tertiary">
                  <Lock className="w-3 h-3" /> System
                </span>
              )}
            </DialogPrimitive.Title>
            <DialogPrimitive.Close
              className="p-1.5 rounded-md text-text-secondary hover:bg-surface-2 hover:text-text-primary"
              aria-label="Close"
            >
              <X className="w-4 h-4" />
            </DialogPrimitive.Close>
          </header>

          <form onSubmit={handleSubmit} className="px-5 py-4 space-y-4 max-h-[80vh] overflow-y-auto">
            <div className="grid grid-cols-2 gap-3">
              <label className="flex flex-col gap-1">
                <span className="text-xs font-medium text-text-secondary">Code</span>
                <input
                  type="text"
                  value={code}
                  onChange={(e) => setCode(e.target.value)}
                  disabled={isEdit}
                  placeholder="coins"
                  className="px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary disabled:opacity-60 disabled:cursor-not-allowed focus:outline-none focus:ring-2 focus:ring-brand-400/60"
                />
                {isEdit && (
                  <span className="text-[11px] text-text-tertiary">
                    {isSystem
                      ? 'System currency — code is referenced by rules and cannot change.'
                      : 'Code is immutable after creation (rules reference currencies by code).'}
                  </span>
                )}
                {!isEdit && !codeValid && code && (
                  <span className="text-[11px] text-accent-danger">
                    Lowercase, starts with a letter, 2–32 chars.
                  </span>
                )}
              </label>
              <label className="flex flex-col gap-1">
                <span className="text-xs font-medium text-text-secondary">Display label (singular)</span>
                <input
                  type="text"
                  value={displayLabel}
                  onChange={(e) => setDisplayLabel(e.target.value)}
                  maxLength={64}
                  placeholder="Coin"
                  className="px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60"
                />
                <span className="text-[11px] text-text-tertiary">
                  Shown when the amount is 1. e.g., &ldquo;You earned 1 Coin.&rdquo;
                </span>
              </label>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <label className="flex flex-col gap-1">
                <span className="text-xs font-medium text-text-secondary">Plural label</span>
                <input
                  type="text"
                  value={displayLabelPlural}
                  onChange={(e) => setDisplayLabelPlural(e.target.value)}
                  maxLength={64}
                  placeholder="Coins"
                  className="px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60"
                />
                <span className="text-[11px] text-text-tertiary">
                  Shown for amounts ≠ 1. e.g., &ldquo;You earned 4 Coins.&rdquo;
                </span>
              </label>
              <label className="flex flex-col gap-1">
                <span className="text-xs font-medium text-text-secondary">Display order</span>
                <input
                  type="number"
                  value={displayOrder}
                  onChange={(e) => setDisplayOrder(e.target.value)}
                  className="px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60"
                />
              </label>
            </div>

            <div>
              <span className="block text-xs font-medium text-text-secondary mb-1.5">Icon</span>
              <div className="flex flex-wrap gap-1.5">
                {ICON_PALETTE.map((name) => (
                  <button
                    key={name}
                    type="button"
                    onClick={() => setIcon(name)}
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
                {DEFAULT_COLORS.map((c) => (
                  <button
                    key={c}
                    type="button"
                    onClick={() => setColor(c)}
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
                  onChange={(e) => setColor(e.target.value)}
                  placeholder="#A855F7"
                  className="ml-2 w-24 px-2 py-1 rounded-md border border-surface-raised bg-surface-1 text-xs text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60"
                />
                {!colorValid && (
                  <span className="text-[11px] text-accent-danger">6-digit hex required</span>
                )}
              </div>
            </div>

            <label className="flex flex-col gap-1">
              <span className="text-xs font-medium text-text-secondary">Description</span>
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                maxLength={500}
                rows={2}
                className="px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60"
              />
            </label>

            <fieldset className="space-y-1.5 border-t border-surface-raised pt-3">
              <legend className="text-xs font-medium text-text-secondary mb-1">Behavior</legend>
              <CheckRow
                label="Spendable"
                hint="Balance can decrease (e.g., shop spends)"
                checked={spendable}
                onChange={setSpendable}
              />
              <CheckRow
                label="Monotonic"
                hint="Lifetime-only — balance never decreases"
                checked={monotonic}
                onChange={setMonotonic}
              />
              <CheckRow
                label="Visible to student"
                hint="Hide to make this an instructor-only accounting currency"
                checked={visibleToStudent}
                onChange={setVisibleToStudent}
              />
              <CheckRow
                label="Show in top bar"
                hint="Adds a pill to every learner's top bar"
                checked={visibleInTopbar}
                onChange={setVisibleInTopbar}
              />
            </fieldset>

            {saveError && (
              <div className="text-sm text-accent-danger border border-accent-danger rounded-md px-3 py-2">
                {saveError}
              </div>
            )}

            <div className="flex justify-end gap-2 pt-2">
              <DialogPrimitive.Close
                type="button"
                className="px-3 py-1.5 rounded-md text-sm text-text-secondary hover:bg-surface-2"
              >
                Cancel
              </DialogPrimitive.Close>
              <button
                type="submit"
                disabled={!formValid || saving}
                className="px-3 py-1.5 rounded-md text-sm font-medium bg-brand-600 text-white hover:bg-brand-700 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {saving ? 'Saving…' : isEdit ? 'Save changes' : 'Create currency'}
              </button>
            </div>
          </form>
        </DialogPrimitive.Content>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  );
}

function CheckRow({ label, hint, checked, onChange }) {
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
