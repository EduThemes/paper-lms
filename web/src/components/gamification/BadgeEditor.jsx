import React from 'react';
import ScopedEntityEditor from '../scoped/ScopedEntityEditor';
import { BadgeIcon } from './BadgeIcon';

// Same curated icon set as the currency editor — the W2-A
// CurrencyIcon resolver maps these to lucide components (or emoji
// fallback). Keeping the palettes in sync means a tenant's visual
// language stays consistent between currencies and badges.
const ICON_PALETTE = [
  'trophy', 'medal', 'award', 'star', 'crown',
  'flame', 'heart', 'shield-check', 'gem', 'diamond',
  'zap', 'target', 'sparkles', 'coins',
];

const DEFAULT_COLORS = [
  '#F59E0B', '#A855F7', '#0EA5E9', '#10B981',
  '#EF4444', '#3B82F6', '#F97316', '#64748B',
];

// BadgeEditor — thin config wrapper around <ScopedEntityEditor>.
// Same shape as CurrencyEditor by design; admins shouldn't have to
// relearn the form between currencies and badges.
export default function BadgeEditor({
  open,
  onOpenChange,
  badge,
  onSave,
  saving = false,
  saveError = null,
}) {
  return (
    <ScopedEntityEditor
      entityName="badge"
      open={open}
      onOpenChange={onOpenChange}
      initialEntity={badge}
      onSave={onSave}
      saving={saving}
      saveError={saveError}
      iconPalette={ICON_PALETTE}
      colorPalette={DEFAULT_COLORS}
      formFields={{
        codePlaceholder: 'first_quiz',
        labelKey: 'name',
        labelText: 'Name',
        labelPlaceholder: 'First Quiz',
        labelMaxLength: 80,
        codeImmutableHint: 'Code is immutable after creation (rules reference badges by code).',
        codeSystemHint: 'Code is immutable after creation (rules reference badges by code).',
        initialState: (entity) => ({
          code: entity?.code || '',
          name: entity?.name || '',
          description: entity?.description || '',
          icon: entity?.icon || 'trophy',
          image_url: entity?.image_url || '',
          color: entity?.color || '#A855F7',
          internal_only: entity?.internal_only !== false,
          audience_level: entity?.audience_level || '',
        }),
      }}
      renderPreview={(s) => (
        <div className="flex items-center gap-4 p-3 rounded-md border border-surface-raised bg-surface-1">
          <BadgeIcon badge={{ icon: s.icon, image_url: s.image_url, color: s.color }} size="lg" />
          <div>
            <div className="text-sm font-medium text-text-primary">{s.name || 'Badge name'}</div>
            <div className="text-xs text-text-tertiary">
              {s.description || 'A short description shown on /profile/badges.'}
            </div>
          </div>
        </div>
      )}
      renderExtraFields={(s, setState) => {
        const setField = (k, v) => setState((prev) => ({ ...prev, [k]: v }));
        return (
          <>
            <label className="flex flex-col gap-1">
              <span className="text-xs font-medium text-text-secondary">Description</span>
              <textarea
                value={s.description}
                onChange={(e) => setField('description', e.target.value)}
                maxLength={500}
                rows={2}
                placeholder="What does it take to earn this badge?"
                className="px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60"
              />
            </label>

            <label className="flex flex-col gap-1">
              <span className="text-xs font-medium text-text-secondary">Image URL (optional)</span>
              <input
                type="text"
                value={s.image_url}
                onChange={(e) => setField('image_url', e.target.value)}
                placeholder="https://example.org/badge.png"
                className="px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60"
              />
              <span className="text-[11px] text-text-tertiary">
                Overrides the icon if set. Custom upload arrives in a later sprint — for now, paste a hosted URL.
              </span>
            </label>

            <label className="flex items-start gap-2 cursor-pointer border-t border-surface-raised pt-3">
              <input
                type="checkbox"
                checked={s.internal_only}
                onChange={(e) => setField('internal_only', e.target.checked)}
                className="mt-1 accent-brand-600"
              />
              <span className="flex-1">
                <span className="block text-sm text-text-primary">Internal-only badge</span>
                <span className="block text-xs text-text-tertiary">
                  Stays inside Paper LMS. Open Badges (OB 3.0) export to 3rd-party wallets requires admin
                  +&nbsp;parent consent and won't ship until Wave 5 — leave this on for now.
                </span>
              </span>
            </label>
          </>
        );
      }}
      buildBody={(s, isEdit) => {
        const body = {
          name: s.name.trim(),
          description: s.description.trim(),
          icon: s.icon,
          image_url: s.image_url.trim(),
          color: s.color,
          internal_only: s.internal_only,
          audience_level: s.audience_level.trim(),
        };
        if (!isEdit) body.code = s.code.trim();
        return body;
      }}
    />
  );
}
