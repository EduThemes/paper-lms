import React, { useCallback, useEffect, useState } from 'react';
import { RefreshCcw, User, AlertTriangle, Check } from 'lucide-react';
import {
  Dialog,
  DialogContent,
  DialogTitle,
} from '@/components/ui/dialog';
import { api } from '../../services/api';

// PseudonymPicker is the modal a learner uses to switch their per-
// course alias (W3-B). Reachable only when policy allows learner
// switching (LearnerCanSwitch=true) — for K-5/M68 tenants the server
// gates the catalog endpoint with 403 and the parent component hides
// the launching button entirely. This component does NOT enforce the
// gate on its own; if rendered, it assumes the server allows it.
//
// Body shapes the picker can submit:
//   { pool_code, name }              → server validates name ∈ pool
//   { pool_code, regenerate: true }  → server rolls a fresh deterministic name
//   { pool_code: "first_name" }      → use legal first-name token
//
// On 409 (UNIQUE collision on the chosen name in this course pool),
// the picker offers a re-roll. The server's atomic ON CONFLICT path
// guarantees there's no TOCTOU window where a second click could land
// the same name a peer just claimed.
export default function PseudonymPicker({ courseId, open, onClose, onSaved }) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [catalog, setCatalog] = useState(null);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState(null);
  const [savedName, setSavedName] = useState(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    setSavedName(null);
    setSaveError(null);
    try {
      const data = await api.gamification.getPseudonymPools(courseId);
      setCatalog(data);
    } catch (err) {
      console.error('PseudonymPicker: failed to load pools', err);
      setError(err.message || 'Could not load name pools.');
    } finally {
      setLoading(false);
    }
  }, [courseId]);

  useEffect(() => {
    if (open) load();
  }, [open, load]);

  const handlePick = async (poolCode, name) => {
    setSaving(true);
    setSaveError(null);
    try {
      const result = await api.gamification.updateMyPseudonym(courseId, { pool_code: poolCode, name });
      setSavedName(result.name);
      if (onSaved) onSaved(result);
    } catch (err) {
      console.error('PseudonymPicker: pick failed', err);
      setSaveError(err.message || 'Could not save name.');
    } finally {
      setSaving(false);
    }
  };

  const handleRegenerate = async (poolCode) => {
    setSaving(true);
    setSaveError(null);
    try {
      const result = await api.gamification.updateMyPseudonym(courseId, { pool_code: poolCode, regenerate: true });
      setSavedName(result.name);
      if (onSaved) onSaved(result);
      // Refresh the catalog so the visible samples reflect any
      // attempt-counter advance the server applied.
      load();
    } catch (err) {
      console.error('PseudonymPicker: regenerate failed', err);
      setSaveError(err.message || 'Could not generate a new name.');
    } finally {
      setSaving(false);
    }
  };

  const handleFirstName = async () => {
    setSaving(true);
    setSaveError(null);
    try {
      const result = await api.gamification.updateMyPseudonym(courseId, { pool_code: 'first_name' });
      setSavedName('your first name');
      if (onSaved) onSaved(result);
    } catch (err) {
      console.error('PseudonymPicker: first-name failed', err);
      setSaveError(err.message || 'Could not switch to first-name mode.');
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(next) => { if (!next) onClose(); }}>
      <DialogContent
        className="max-w-2xl max-h-[85vh] overflow-y-auto rounded-lg bg-surface-0 shadow-xl border border-surface-raised p-0"
        aria-describedby={undefined}
      >
        <header className="flex items-center justify-between px-5 py-4 border-b border-surface-raised sticky top-0 bg-surface-0 z-10">
          <DialogTitle id="pseudonym-picker-title" className="text-base font-semibold text-text-primary">
            Pick your leaderboard name
          </DialogTitle>
        </header>

        <div className="px-5 py-4 space-y-4 text-sm text-text-primary">
          {loading && <p className="text-text-secondary">Loading name pools…</p>}

          {error && (
            <div className="flex items-start gap-2 rounded-lg border border-accent-warning bg-accent-warning/10 p-3 text-accent-warning">
              <AlertTriangle className="w-4 h-4 mt-0.5" aria-hidden="true" />
              <span>{error}</span>
            </div>
          )}

          {savedName && (
            <div className="flex items-start gap-2 rounded-lg border border-accent-success bg-accent-success/10 p-3 text-accent-success">
              <Check className="w-4 h-4 mt-0.5" aria-hidden="true" />
              <span>
                Your name is now <strong>{savedName}</strong>. Refresh the
                leaderboard to see it.
              </span>
            </div>
          )}

          {saveError && (
            <div className="flex items-start gap-2 rounded-lg border border-accent-error bg-accent-error/10 p-3 text-accent-error">
              <AlertTriangle className="w-4 h-4 mt-0.5" aria-hidden="true" />
              <span>{saveError}</span>
            </div>
          )}

          {catalog && (
            <>
              {catalog.current_name && (
                <p className="text-text-secondary">
                  Currently:{' '}
                  <strong className="text-text-primary">{catalog.current_name}</strong>{' '}
                  (pool: <code className="text-xs">{catalog.current_pool_code}</code>)
                </p>
              )}

              {catalog.first_name_available && (
                <section className="rounded-lg border border-surface-raised bg-surface-1 p-4">
                  <header className="flex items-center gap-2 mb-2">
                    <User className="w-4 h-4 text-text-secondary" aria-hidden="true" />
                    <h3 className="font-semibold">Use my first name</h3>
                  </header>
                  <p className="text-text-secondary mb-3">
                    Show peers the first part of your real name on the leaderboard.
                  </p>
                  <button
                    type="button"
                    disabled={saving}
                    onClick={handleFirstName}
                    className="rounded-md border border-brand-600 px-3 py-1.5 text-sm font-medium text-brand-700 hover:bg-brand-50 disabled:opacity-50"
                  >
                    Use my first name
                  </button>
                </section>
              )}

              {(catalog.pools || []).map((pool) => (
                <section key={pool.code} className="rounded-lg border border-surface-raised bg-surface-0 p-4">
                  <header className="flex items-center justify-between mb-2">
                    <div>
                      <h3 className="font-semibold">{pool.label}</h3>
                      <p className="text-text-secondary text-xs">
                        {pool.description} · {pool.candidate_count.toLocaleString()} possible names
                      </p>
                    </div>
                    <button
                      type="button"
                      disabled={saving}
                      onClick={() => handleRegenerate(pool.code)}
                      className="inline-flex items-center gap-1 rounded-md border border-surface-raised px-3 py-1.5 text-xs font-medium text-text-primary hover:bg-surface-1 disabled:opacity-50"
                    >
                      <RefreshCcw className="w-3.5 h-3.5" aria-hidden="true" />
                      Roll a new one
                    </button>
                  </header>
                  <div className="flex flex-wrap gap-2 mt-3">
                    {(pool.samples || []).map((sample) => (
                      <button
                        key={sample}
                        type="button"
                        disabled={saving}
                        onClick={() => handlePick(pool.code, sample)}
                        className="rounded-full border border-surface-raised bg-surface-1 hover:bg-brand-50 hover:border-brand-300 px-3 py-1 text-sm transition-colors disabled:opacity-50"
                      >
                        {sample}
                      </button>
                    ))}
                  </div>
                </section>
              ))}
            </>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
