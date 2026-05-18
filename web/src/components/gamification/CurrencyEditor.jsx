import React from 'react';
import ScopedEntityEditor, { CheckRow } from '../scoped/ScopedEntityEditor';

// Curated icon palette for teachers. Strings match the keys CurrencyIcon
// understands; the four system icons (zap/gem/target/shield-check) are
// included so admins renaming system rows can pick from a consistent set.
const ICON_PALETTE = [
  'zap', 'gem', 'target', 'shield-check', 'coins', 'sparkles', 'award',
  'star', 'trophy', 'heart', 'crown', 'flame', 'diamond', 'medal',
];

const DEFAULT_COLORS = [
  '#F59E0B', '#A855F7', '#0EA5E9', '#10B981',
  '#EF4444', '#3B82F6', '#F97316', '#64748B',
];

// CurrencyEditor — thin config wrapper around <ScopedEntityEditor>.
// Pass `currency=null` for create-mode, or a row for edit-mode.
// `onSave` receives the API-shaped body; this component delegates
// state management entirely to the shared editor.
export default function CurrencyEditor({
  open,
  onOpenChange,
  currency,
  onSave,
  saving = false,
  saveError = null,
}) {
  return (
    <ScopedEntityEditor
      entityName="currency"
      open={open}
      onOpenChange={onOpenChange}
      initialEntity={currency}
      onSave={onSave}
      saving={saving}
      saveError={saveError}
      iconPalette={ICON_PALETTE}
      colorPalette={DEFAULT_COLORS}
      formFields={{
        codePlaceholder: 'coins',
        labelKey: 'display_label',
        labelText: 'Display label (singular)',
        labelPlaceholder: 'Coin',
        labelMaxLength: 64,
        labelHint: 'Shown when the amount is 1. e.g., "You earned 1 Coin."',
        codeImmutableHint: 'Code is immutable after creation (rules reference currencies by code).',
        codeSystemHint: 'System currency — code is referenced by rules and cannot change.',
        initialState: (entity) => ({
          code: entity?.code || '',
          display_label: entity?.display_label || '',
          display_label_plural: entity?.display_label_plural || '',
          icon: entity?.icon || 'coins',
          color: entity?.color || '#A855F7',
          description: entity?.description || '',
          display_order: entity?.display_order ?? 0,
          spendable: !!entity?.spendable,
          monotonic: entity?.monotonic !== false,
          visible_to_student: entity?.visible_to_student !== false,
          visible_in_topbar: entity?.visible_in_topbar !== false,
        }),
      }}
      renderExtraFields={(s, setState) => {
        const setField = (k, v) => setState((prev) => ({ ...prev, [k]: v }));
        return (
          <>
            <div className="grid grid-cols-2 gap-3">
              <label className="flex flex-col gap-1">
                <span className="text-xs font-medium text-text-secondary">Plural label</span>
                <input
                  type="text"
                  value={s.display_label_plural}
                  onChange={(e) => setField('display_label_plural', e.target.value)}
                  maxLength={64}
                  placeholder="Coins"
                  className="px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60"
                />
                <span className="text-[11px] text-text-tertiary">
                  Shown for amounts ≠ 1. e.g., "You earned 4 Coins."
                </span>
              </label>
              <label className="flex flex-col gap-1">
                <span className="text-xs font-medium text-text-secondary">Display order</span>
                <input
                  type="number"
                  value={s.display_order}
                  onChange={(e) => setField('display_order', e.target.value)}
                  className="px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60"
                />
              </label>
            </div>

            <label className="flex flex-col gap-1">
              <span className="text-xs font-medium text-text-secondary">Description</span>
              <textarea
                value={s.description}
                onChange={(e) => setField('description', e.target.value)}
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
                checked={s.spendable}
                onChange={(v) => setField('spendable', v)}
              />
              <CheckRow
                label="Monotonic"
                hint="Lifetime-only — balance never decreases"
                checked={s.monotonic}
                onChange={(v) => setField('monotonic', v)}
              />
              <CheckRow
                label="Visible to student"
                hint="Hide to make this an instructor-only accounting currency"
                checked={s.visible_to_student}
                onChange={(v) => setField('visible_to_student', v)}
              />
              <CheckRow
                label="Show in top bar"
                hint="Adds a pill to every learner's top bar"
                checked={s.visible_in_topbar}
                onChange={(v) => setField('visible_in_topbar', v)}
              />
            </fieldset>
          </>
        );
      }}
      buildBody={(s, isEdit) => {
        const body = {
          display_label: s.display_label.trim(),
          display_label_plural: s.display_label_plural.trim(),
          icon: s.icon,
          color: s.color,
          display_order: Number(s.display_order) || 0,
          spendable: s.spendable,
          monotonic: s.monotonic,
          visible_to_student: s.visible_to_student,
          visible_in_topbar: s.visible_in_topbar,
          description: s.description.trim(),
        };
        if (!isEdit) body.code = s.code.trim();
        return body;
      }}
    />
  );
}
