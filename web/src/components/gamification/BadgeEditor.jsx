import React, { useEffect, useState } from 'react';
import { Lock, X } from 'lucide-react';
import * as DialogPrimitive from '@radix-ui/react-dialog';
import { CurrencyIcon } from './currencyIcon';
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

const codeRE = /^[a-z][a-z0-9_]{1,31}$/;

// BadgeEditor — Radix dialog for create + edit. Same shape as
// CurrencyEditor (W2-B); fields differ but the layout, icon palette,
// color palette, and system-owned lock behavior are deliberately
// identical so admins don't have to relearn the UI.
export default function BadgeEditor({
  open,
  onOpenChange,
  badge,
  onSave,
  saving = false,
  saveError = null,
}) {
  const isEdit = !!badge;
  const isSystem = !!badge?.system_owned;

  const [code, setCode] = useState('');
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [icon, setIcon] = useState('trophy');
  const [imageURL, setImageURL] = useState('');
  const [color, setColor] = useState('#A855F7');
  const [internalOnly, setInternalOnly] = useState(true);
  const [audienceLevel, setAudienceLevel] = useState('');

  useEffect(() => {
    if (!open) return;
    if (badge) {
      setCode(badge.code || '');
      setName(badge.name || '');
      setDescription(badge.description || '');
      setIcon(badge.icon || 'trophy');
      setImageURL(badge.image_url || '');
      setColor(badge.color || '#A855F7');
      setInternalOnly(badge.internal_only !== false);
      setAudienceLevel(badge.audience_level || '');
    } else {
      setCode('');
      setName('');
      setDescription('');
      setIcon('trophy');
      setImageURL('');
      setColor('#A855F7');
      setInternalOnly(true);
      setAudienceLevel('');
    }
  }, [open, badge]);

  const codeValid = isEdit || codeRE.test(code);
  const nameValid = name.trim().length > 0 && name.length <= 80;
  const colorValid = /^(#[0-9A-Fa-f]{6})?$/.test(color);
  const formValid = codeValid && nameValid && colorValid;

  const handleSubmit = (e) => {
    e.preventDefault();
    if (!formValid) return;
    const body = {
      name: name.trim(),
      description: description.trim(),
      icon,
      image_url: imageURL.trim(),
      color,
      internal_only: internalOnly,
      audience_level: audienceLevel.trim(),
    };
    if (!isEdit) {
      body.code = code.trim();
    }
    onSave(body);
  };

  const previewBadge = { icon, image_url: imageURL, color };

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
              {isEdit ? 'Edit badge' : 'New badge'}
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
            <div className="flex items-center gap-4 p-3 rounded-md border border-surface-raised bg-surface-1">
              <BadgeIcon badge={previewBadge} size="lg" />
              <div>
                <div className="text-sm font-medium text-text-primary">{name || 'Badge name'}</div>
                <div className="text-xs text-text-tertiary">{description || 'A short description shown on /profile/badges.'}</div>
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <label className="flex flex-col gap-1">
                <span className="text-xs font-medium text-text-secondary">Code</span>
                <input
                  type="text"
                  value={code}
                  onChange={(e) => setCode(e.target.value)}
                  disabled={isEdit}
                  placeholder="first_quiz"
                  className="px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary disabled:opacity-60 disabled:cursor-not-allowed focus:outline-none focus:ring-2 focus:ring-brand-400/60"
                />
                {isEdit && (
                  <span className="text-[11px] text-text-tertiary">
                    Code is immutable after creation (rules reference badges by code).
                  </span>
                )}
                {!isEdit && !codeValid && code && (
                  <span className="text-[11px] text-accent-danger">
                    Lowercase, starts with a letter, 2–32 chars.
                  </span>
                )}
              </label>
              <label className="flex flex-col gap-1">
                <span className="text-xs font-medium text-text-secondary">Name</span>
                <input
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  maxLength={80}
                  placeholder="First Quiz"
                  className="px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60"
                />
              </label>
            </div>

            <label className="flex flex-col gap-1">
              <span className="text-xs font-medium text-text-secondary">Description</span>
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                maxLength={500}
                rows={2}
                placeholder="What does it take to earn this badge?"
                className="px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60"
              />
            </label>

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

            <label className="flex flex-col gap-1">
              <span className="text-xs font-medium text-text-secondary">Image URL (optional)</span>
              <input
                type="text"
                value={imageURL}
                onChange={(e) => setImageURL(e.target.value)}
                placeholder="https://example.org/badge.png"
                className="px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60"
              />
              <span className="text-[11px] text-text-tertiary">
                Overrides the icon if set. Custom upload arrives in a later sprint — for now, paste a hosted URL.
              </span>
            </label>

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

            <label className="flex items-start gap-2 cursor-pointer border-t border-surface-raised pt-3">
              <input
                type="checkbox"
                checked={internalOnly}
                onChange={(e) => setInternalOnly(e.target.checked)}
                className="mt-1 accent-brand-600"
              />
              <span className="flex-1">
                <span className="block text-sm text-text-primary">Internal-only badge</span>
                <span className="block text-xs text-text-tertiary">
                  Stays inside Paper LMS. Open Badges (OB 3.0) export to 3rd-party wallets requires admin
                  +&nbsp;parent consent and won&rsquo;t ship until Wave 5 — leave this on for now.
                </span>
              </span>
            </label>

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
                {saving ? 'Saving…' : isEdit ? 'Save changes' : 'Create badge'}
              </button>
            </div>
          </form>
        </DialogPrimitive.Content>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  );
}
