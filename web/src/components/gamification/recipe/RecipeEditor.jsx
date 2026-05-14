import React, { useEffect, useState } from 'react';
import { X } from 'lucide-react';
import * as DialogPrimitive from '@radix-ui/react-dialog';
import { useGamificationVocabulary } from '../../../hooks/useGamificationVocabulary';
import TriggerPicker from './TriggerPicker';
import ConditionNode from './ConditionNode';
import EffectsPalette from './EffectsPalette';

// RecipeEditor — top-level recipe authoring dialog.
//
//   <RecipeEditor
//     open
//     onOpenChange={…}
//     recipe={existingRow}        // omit for create
//     onSave={async (body) => …}  // POST/PATCH the rule
//     saving={…}
//     saveError={…}
//   />
//
// Body shape matches what the W2-E.1 rule write API expects:
//
//   {
//     name, description, audience_level, enabled,
//     trigger_event:    { kind, ...kind-specific },
//     condition_set:    { kind:"ConditionSet", op, threshold?, children },
//     effects:          [ { kind, ...kind-specific }, ... ],
//     cooldown_seconds: int | null,
//     max_per_window:   { window, count } | null,
//   }
//
// For PATCH the server also accepts `clear_cooldown` /
// `clear_max_per_window` boolean flags — we emit them when the user
// has explicitly nulled a field so the patch path can distinguish
// "leave it alone" from "remove the limit." On Create those flags
// are harmless extras the server ignores.

const DEFAULT_TRIGGER = { kind: 'OnEvent' };
const DEFAULT_CONDITION = { kind: 'ConditionSet', op: 'AND', children: [] };

export default function RecipeEditor({
  open,
  onOpenChange,
  recipe = null,
  onSave,
  saving = false,
  saveError = null,
}) {
  const { vocab, loading, error: vocabError } = useGamificationVocabulary();
  const isEdit = !!recipe;

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [audienceLevel, setAudienceLevel] = useState('higher_ed');
  const [enabled, setEnabled] = useState(true);
  const [trigger, setTrigger] = useState(DEFAULT_TRIGGER);
  const [condition, setCondition] = useState(DEFAULT_CONDITION);
  const [effects, setEffects] = useState([]);
  // Optional rate-limit fields. `null` = "not set" (server-side NULL).
  // Numeric 0 would mean "no rate limit applies in practice"; the
  // distinction matters because PATCH needs `clear_cooldown` to drop
  // a previously-set value.
  const [cooldownSeconds, setCooldownSeconds] = useState(null);
  const [maxPerWindow, setMaxPerWindow] = useState(null); // {window, count} | null

  useEffect(() => {
    if (!open) return;
    if (recipe) {
      setName(recipe.name || '');
      setDescription(recipe.description || '');
      setAudienceLevel(recipe.audience_level || 'higher_ed');
      setEnabled(recipe.enabled !== false);
      setTrigger(parseJSON(recipe.trigger_event, DEFAULT_TRIGGER));
      setCondition(parseJSON(recipe.condition_set, DEFAULT_CONDITION));
      setEffects(parseJSON(recipe.effects, []));
      setCooldownSeconds(recipe.cooldown_seconds ?? null);
      setMaxPerWindow(parseJSON(recipe.max_per_window, null));
    } else {
      setName('');
      setDescription('');
      setAudienceLevel('higher_ed');
      setEnabled(true);
      setTrigger(DEFAULT_TRIGGER);
      setCondition(DEFAULT_CONDITION);
      setEffects([]);
      setCooldownSeconds(null);
      setMaxPerWindow(null);
    }
  }, [open, recipe]);

  const audiences = vocab?.audiences || ['k5', 'm68', 'h912', 'higher_ed', 'corp', 'pro'];

  const nameValid = name.trim().length > 0 && name.length <= 200;
  const formValid = nameValid && !!trigger.kind;

  const handleSubmit = (e) => {
    e.preventDefault();
    if (!formValid || !onSave) return;
    const body = {
      name: name.trim(),
      description: description.trim(),
      audience_level: audienceLevel,
      enabled,
      trigger_event: trigger,
      condition_set: condition,
      effects,
    };
    if (cooldownSeconds != null && cooldownSeconds > 0) {
      body.cooldown_seconds = cooldownSeconds;
    } else if (isEdit && recipe?.cooldown_seconds != null) {
      body.clear_cooldown = true;
    }
    if (maxPerWindow && maxPerWindow.window && maxPerWindow.count > 0) {
      body.max_per_window = maxPerWindow;
    } else if (isEdit && recipe?.max_per_window) {
      body.clear_max_per_window = true;
    }
    onSave(body);
  };

  const windowKinds = vocab?.windows || ['day', 'week', 'lifetime'];

  return (
    <DialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
      <DialogPrimitive.Portal>
        <DialogPrimitive.Overlay className="fixed inset-0 z-50 bg-black/40 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 motion-reduce:transition-none" />
        <DialogPrimitive.Content
          className="fixed left-1/2 top-1/2 z-50 w-full max-w-2xl -translate-x-1/2 -translate-y-1/2 rounded-lg bg-surface-0 shadow-xl border border-surface-raised data-[state=open]:animate-in data-[state=closed]:animate-out motion-reduce:duration-0"
          aria-describedby={undefined}
        >
          <header className="flex items-center justify-between px-5 py-3 border-b border-surface-raised">
            <DialogPrimitive.Title className="text-base font-semibold text-text-primary">
              {isEdit ? 'Edit recipe' : 'New recipe'}
            </DialogPrimitive.Title>
            <DialogPrimitive.Close
              className="p-1.5 rounded-md text-text-secondary hover:bg-surface-2 hover:text-text-primary"
              aria-label="Close"
            >
              <X className="w-4 h-4" />
            </DialogPrimitive.Close>
          </header>

          <form onSubmit={handleSubmit} className="px-5 py-4 space-y-4 max-h-[80vh] overflow-y-auto">
            {loading && (
              <div className="text-xs text-text-tertiary">Loading vocabulary…</div>
            )}
            {vocabError && (
              <div className="rounded-md border border-accent-danger/40 bg-accent-danger/10 px-2.5 py-1.5 text-xs text-accent-danger">
                Could not load the recipe vocabulary. The editor needs it
                to render trigger and condition shapes — try again, or
                ask an admin to verify the gamification service is
                reachable.
              </div>
            )}

            <div className="grid grid-cols-3 gap-3">
              <label className="col-span-2 flex flex-col gap-1">
                <span className="text-xs font-medium text-text-secondary">Name <span className="text-accent-danger">*</span></span>
                <input
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  maxLength={200}
                  placeholder="Award XP when a quiz is completed"
                  className="px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60"
                />
              </label>
              <label className="flex flex-col gap-1">
                <span className="text-xs font-medium text-text-secondary">Audience</span>
                <select
                  value={audienceLevel}
                  onChange={(e) => setAudienceLevel(e.target.value)}
                  className="px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60"
                >
                  {audiences.map((a) => (
                    <option key={a} value={a}>{a}</option>
                  ))}
                </select>
              </label>
            </div>

            <label className="flex flex-col gap-1">
              <span className="text-xs font-medium text-text-secondary">Description</span>
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                maxLength={2000}
                rows={2}
                placeholder="Optional. What does this recipe reward, and why?"
                className="px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60"
              />
            </label>

            <label className="inline-flex items-center gap-2 text-xs font-medium text-text-secondary">
              <input
                type="checkbox"
                checked={enabled}
                onChange={(e) => setEnabled(e.target.checked)}
                className="h-3.5 w-3.5 rounded border-surface-raised"
              />
              Enabled
            </label>

            <TriggerPicker value={trigger} vocab={vocab} onChange={setTrigger} />

            <section className="space-y-2">
              <div className="text-xs font-medium uppercase tracking-wide text-text-tertiary">If</div>
              <ConditionNode value={condition} vocab={vocab} onChange={setCondition} />
            </section>

            <section className="space-y-2">
              <div className="text-xs font-medium uppercase tracking-wide text-text-tertiary">Then</div>
              <EffectsPalette value={effects} onChange={setEffects} />
            </section>

            <section className="space-y-2">
              <div className="text-xs font-medium uppercase tracking-wide text-text-tertiary">Rate limit (optional)</div>
              <div className="grid grid-cols-3 gap-3">
                <label className="flex flex-col gap-1">
                  <span className="text-xs font-medium text-text-secondary">Cooldown seconds</span>
                  <input
                    type="number"
                    min="1"
                    step="1"
                    value={cooldownSeconds ?? ''}
                    placeholder="None"
                    onChange={(e) => {
                      const raw = e.target.value;
                      setCooldownSeconds(raw === '' ? null : Number(raw));
                    }}
                    className="px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60"
                  />
                </label>
                <label className="flex flex-col gap-1">
                  <span className="text-xs font-medium text-text-secondary">Max per window</span>
                  <input
                    type="number"
                    min="1"
                    step="1"
                    value={maxPerWindow?.count ?? ''}
                    placeholder="None"
                    onChange={(e) => {
                      const raw = e.target.value;
                      if (raw === '') {
                        setMaxPerWindow(null);
                      } else {
                        setMaxPerWindow({
                          window: maxPerWindow?.window || 'day',
                          count: Number(raw),
                        });
                      }
                    }}
                    className="px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60"
                  />
                </label>
                <label className="flex flex-col gap-1">
                  <span className="text-xs font-medium text-text-secondary">Window</span>
                  <select
                    value={maxPerWindow?.window || ''}
                    onChange={(e) => {
                      const w = e.target.value;
                      if (!w) {
                        setMaxPerWindow(null);
                      } else {
                        setMaxPerWindow({
                          window: w,
                          count: maxPerWindow?.count || 1,
                        });
                      }
                    }}
                    disabled={!maxPerWindow}
                    className="px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60 disabled:opacity-60"
                  >
                    <option value="">—</option>
                    {windowKinds.map((w) => (
                      <option key={w} value={w}>{w}</option>
                    ))}
                  </select>
                </label>
              </div>
            </section>

            {saveError && (
              <div className="rounded-md border border-accent-danger/40 bg-accent-danger/10 px-2.5 py-1.5 text-xs text-accent-danger">
                {saveError}
              </div>
            )}

            <footer className="flex justify-end gap-2 pt-2 border-t border-surface-raised">
              <DialogPrimitive.Close
                type="button"
                className="px-3 py-1.5 rounded-md text-sm text-text-secondary hover:bg-surface-2"
              >
                Cancel
              </DialogPrimitive.Close>
              <button
                type="submit"
                disabled={!formValid || saving}
                className="px-3 py-1.5 rounded-md text-sm bg-brand-500 text-white disabled:opacity-50 disabled:cursor-not-allowed hover:bg-brand-600"
              >
                {saving ? 'Saving…' : isEdit ? 'Save changes' : 'Create recipe'}
              </button>
            </footer>
          </form>
        </DialogPrimitive.Content>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  );
}

// parseJSON — the server emits JSONB fields as raw JSON in the
// response; this is the defensive parse so an edit-from-existing
// flow doesn't blow up if a field is unexpectedly a string blob.
function parseJSON(value, fallback) {
  if (value == null) return fallback;
  if (typeof value === 'object') return value;
  try {
    return JSON.parse(value);
  } catch {
    return fallback;
  }
}
