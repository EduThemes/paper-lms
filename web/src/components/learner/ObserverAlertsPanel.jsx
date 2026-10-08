import React, { useState } from 'react';
import { Link } from 'react-router-dom';
import { BellRing, Settings2, Check, X, AlertTriangle } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from '@/components/ui/dialog';
import {
  learnerApi, useObserverAlerts, useAlertThresholds, useMissingSubmissions, useLearnerMutation,
} from '../../services/api_learner';

// Observer alerts for one linked student (G149): the alert feed (mark
// read / dismiss), the student's missing work, and the alert settings
// (Canvas observer alert thresholds).

export const ALERT_TYPES = [
  { type: 'course_grade_low', label: 'Course grade below', threshold: true, defaultValue: 70 },
  { type: 'course_grade_high', label: 'Course grade above', threshold: true, defaultValue: 90 },
  { type: 'assignment_grade_low', label: 'Assignment grade below', threshold: true, defaultValue: 70 },
  { type: 'assignment_grade_high', label: 'Assignment grade above', threshold: true, defaultValue: 90 },
  { type: 'assignment_missing', label: 'Assignment missing', threshold: false },
  { type: 'course_announcement', label: 'Course announcements', threshold: false },
];

// thresholdChanges diffs the settings form against the saved thresholds:
// returns { create: [...], update: [...], remove: [ids] }.
export function thresholdChanges(saved, form) {
  const byType = new Map((saved || []).map((t) => [t.alert_type, t]));
  const create = [];
  const update = [];
  const remove = [];
  ALERT_TYPES.forEach(({ type, threshold }) => {
    const f = form[type] || {};
    const existing = byType.get(type);
    if (!f.enabled) {
      if (existing) remove.push(existing.id);
      return;
    }
    const value = threshold ? Number(f.value) : null;
    if (!existing) create.push({ alertType: type, threshold: value });
    else if (threshold && Number(existing.threshold) !== value) update.push({ id: existing.id, threshold: value });
  });
  return { create, update, remove };
}

function SettingsDialog({ studentId, studentName, saved, onClose }) {
  const [form, setForm] = useState(() => {
    const init = {};
    ALERT_TYPES.forEach(({ type, defaultValue }) => {
      const t = (saved || []).find((x) => x.alert_type === type);
      init[type] = { enabled: !!t, value: t?.threshold ?? defaultValue ?? '' };
    });
    return init;
  });
  const [error, setError] = useState('');
  const save = useLearnerMutation(async () => {
    const { create, update, remove } = thresholdChanges(saved, form);
    for (const c of create) await learnerApi.createAlertThreshold({ studentId, ...c });
    for (const u of update) await learnerApi.updateAlertThreshold(u.id, u.threshold);
    for (const id of remove) await learnerApi.deleteAlertThreshold(id);
  }, { onSuccess: onClose, onError: (e) => setError(e.message) });
  const set = (type, patch) => setForm((f) => ({ ...f, [type]: { ...f[type], ...patch } }));
  return (
    <Dialog open onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Alerts for {studentName || 'your student'}</DialogTitle>
          <DialogDescription>Choose what you want to be alerted about. Grade alerts use percentages.</DialogDescription>
        </DialogHeader>
        <form className="space-y-3" onSubmit={(e) => { e.preventDefault(); setError(''); save.mutate(); }}>
          {error && <p role="alert" className="text-sm text-accent-danger">{error}</p>}
          {ALERT_TYPES.map(({ type, label, threshold }) => (
            <div key={type} className="flex items-center gap-3">
              <input
                id={`alert-${type}`}
                type="checkbox"
                checked={form[type].enabled}
                onChange={(e) => set(type, { enabled: e.target.checked })}
              />
              <label htmlFor={`alert-${type}`} className="flex-1 text-sm">{label}</label>
              {threshold && (
                <span className="flex items-center gap-1">
                  <label htmlFor={`alert-${type}-value`} className="sr-only">{label} percentage</label>
                  <input
                    id={`alert-${type}-value`}
                    type="number"
                    min="0"
                    max="100"
                    disabled={!form[type].enabled}
                    value={form[type].value}
                    onChange={(e) => set(type, { value: e.target.value })}
                    className="w-20 rounded-md border border-border-default bg-surface-0 px-2 py-1 text-sm disabled:opacity-50"
                  />
                  <span className="text-sm text-text-secondary">%</span>
                </span>
              )}
            </div>
          ))}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>Cancel</Button>
            <Button type="submit" disabled={save.isPending}>{save.isPending ? 'Saving…' : 'Save'}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export default function ObserverAlertsPanel({ studentId, studentName }) {
  const alerts = useObserverAlerts(studentId);
  const thresholds = useAlertThresholds(studentId);
  const missing = useMissingSubmissions(studentId);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const setState = useLearnerMutation(({ id, state }) => learnerApi.setAlertState(id, state));
  const list = alerts.data || [];
  const missingList = missing.data || [];
  return (
    <div className="mt-4 grid grid-cols-1 gap-4 lg:grid-cols-2">
      <Card>
        <CardHeader className="flex-row items-center justify-between space-y-0">
          <CardTitle className="flex items-center gap-2 text-base">
            <BellRing className="h-5 w-5 text-brand-600" aria-hidden="true" />
            Alerts
          </CardTitle>
          <Button size="sm" variant="outline" onClick={() => setSettingsOpen(true)} disabled={thresholds.isLoading}>
            <Settings2 className="me-1 h-4 w-4" aria-hidden="true" /> Alert settings
          </Button>
        </CardHeader>
        <CardContent>
          {alerts.isLoading ? (
            <p className="text-sm text-text-secondary">Loading alerts…</p>
          ) : list.length === 0 ? (
            <p className="text-sm text-text-secondary">
              {(thresholds.data || []).length === 0
                ? 'No alerts are set up. Use Alert settings to choose what you hear about.'
                : 'No new alerts.'}
            </p>
          ) : (
            <ul className="divide-y divide-border-subtle">
              {list.map((a) => (
                <li key={a.id} className="flex items-start gap-2 py-2">
                  <div className="min-w-0 flex-1">
                    <Link
                      to={a.html_url || '#'}
                      onClick={() => a.workflow_state === 'unread' && setState.mutate({ id: a.id, state: 'read' })}
                      className={`text-sm hover:underline ${a.workflow_state === 'unread' ? 'font-semibold text-text-primary' : 'text-text-secondary'}`}
                    >
                      {a.workflow_state === 'unread' && <span className="sr-only">Unread: </span>}
                      {a.title}
                    </Link>
                    <div className="text-xs text-text-tertiary">
                      {new Date(a.action_date).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' })}
                    </div>
                  </div>
                  {a.workflow_state === 'unread' && (
                    <button type="button" onClick={() => setState.mutate({ id: a.id, state: 'read' })}
                      className="rounded p-1 hover:bg-surface-1" aria-label={`Mark read: ${a.title}`}>
                      <Check className="h-4 w-4" aria-hidden="true" />
                    </button>
                  )}
                  <button type="button" onClick={() => setState.mutate({ id: a.id, state: 'dismissed' })}
                    className="rounded p-1 hover:bg-surface-1" aria-label={`Dismiss: ${a.title}`}>
                    <X className="h-4 w-4" aria-hidden="true" />
                  </button>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <AlertTriangle className="h-5 w-5 text-accent-danger" aria-hidden="true" />
            Missing work
          </CardTitle>
        </CardHeader>
        <CardContent>
          {missing.isLoading ? (
            <p className="text-sm text-text-secondary">Loading…</p>
          ) : missingList.length === 0 ? (
            <p className="text-sm text-text-secondary">Nothing missing. Nice!</p>
          ) : (
            <ul className="divide-y divide-border-subtle">
              {missingList.map((m) => (
                <li key={m.id} className="py-2 text-sm">
                  <span className="font-medium text-text-primary">{m.name}</span>
                  <div className="text-xs text-text-tertiary">
                    {m.course_name}
                    {m.due_at && ` · was due ${new Date(m.due_at).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })}`}
                  </div>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
      {settingsOpen && (
        <SettingsDialog
          studentId={studentId}
          studentName={studentName}
          saved={thresholds.data}
          onClose={() => setSettingsOpen(false)}
        />
      )}
    </div>
  );
}
