import React, { useMemo, useState } from 'react';
import { X, Send } from 'lucide-react';
import { MESSAGE_CRITERIA, filterStudentsForMessage } from '../../utils/courseAnalytics';
import { messageStudentsIndividually } from '../../services/api_learner_outcomes';

/**
 * "Message students who…" (Canvas course analytics): pick a criterion, review
 * the matching students, and send each one an individual message.
 *
 * Props: students (student_summaries rows), assignments (analytics
 * assignment rows), initialStudentIds (preselected, skips the criterion),
 * onClose.
 */
export default function MessageStudentsModal({ students, assignments, initialStudentIds, onClose }) {
  const [criterion, setCriterion] = useState(initialStudentIds ? 'selected' : 'not_submitted');
  const [assignmentId, setAssignmentId] = useState(assignments?.[0]?.assignment_id ?? '');
  const [value, setValue] = useState('');
  const [excluded, setExcluded] = useState({});
  const [subject, setSubject] = useState('');
  const [message, setMessage] = useState('');
  const [sending, setSending] = useState(false);
  const [result, setResult] = useState(null);

  const meta = MESSAGE_CRITERIA.find((c) => c.value === criterion);
  const matches = useMemo(() => {
    if (criterion === 'selected') {
      const ids = new Set((initialStudentIds || []).map(String));
      return (students || []).filter((s) => ids.has(String(s.id)));
    }
    return filterStudentsForMessage(students, { criterion, assignmentId, value });
  }, [criterion, assignmentId, value, students, initialStudentIds]);
  const recipients = matches.filter((s) => !excluded[s.id]);

  const send = async (e) => {
    e.preventDefault();
    if (!recipients.length || !subject.trim() || !message.trim()) return;
    setSending(true);
    const res = await messageStudentsIndividually(recipients.map((s) => s.id), subject.trim(), message.trim());
    setSending(false);
    setResult(res);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" role="dialog" aria-modal="true" aria-labelledby="msg-students-title">
      <div className="bg-surface-0 rounded-lg shadow-xl w-full max-w-2xl max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between px-5 py-3 border-b border-border-default">
          <h2 id="msg-students-title" className="text-lg font-semibold text-text-primary">Message students</h2>
          <button type="button" onClick={onClose} aria-label="Close" className="p-1 rounded hover:bg-surface-1"><X className="w-5 h-5" /></button>
        </div>
        {result ? (
          <div className="p-5 space-y-3">
            <p className="text-sm text-text-primary" role="status">
              Sent {result.sent} message{result.sent === 1 ? '' : 's'}.
              {result.failed.length > 0 && ` ${result.failed.length} could not be sent.`}
            </p>
            <button type="button" onClick={onClose} className="px-4 py-2 text-sm rounded bg-brand-600 text-on-brand hover:bg-brand-700">Done</button>
          </div>
        ) : (
          <form onSubmit={send} className="p-5 space-y-4">
            {criterion !== 'selected' && (
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                <label className="text-sm text-text-secondary sm:col-span-1">
                  Students who
                  <select value={criterion} onChange={(e) => { setCriterion(e.target.value); setExcluded({}); }} className="mt-1 w-full rounded border border-border-strong px-2 py-1.5 text-sm">
                    {MESSAGE_CRITERIA.map((c) => <option key={c.value} value={c.value}>{c.label}</option>)}
                  </select>
                </label>
                {meta?.needsAssignment && (
                  <label className="text-sm text-text-secondary">
                    Assignment
                    <select value={assignmentId} onChange={(e) => setAssignmentId(e.target.value)} className="mt-1 w-full rounded border border-border-strong px-2 py-1.5 text-sm">
                      {(assignments || []).map((a) => <option key={a.assignment_id} value={a.assignment_id}>{a.title}</option>)}
                    </select>
                  </label>
                )}
                {meta?.needsValue && (
                  <label className="text-sm text-text-secondary">
                    {criterion === 'inactive_days' ? 'Days' : 'Score'}
                    <input type="number" min="0" value={value} onChange={(e) => setValue(e.target.value)} className="mt-1 w-full rounded border border-border-strong px-2 py-1.5 text-sm" />
                  </label>
                )}
              </div>
            )}

            <fieldset>
              <legend className="text-sm font-medium text-text-primary mb-1">
                Recipients ({recipients.length} of {matches.length})
              </legend>
              {matches.length === 0 ? (
                <p className="text-sm text-text-tertiary">No students match.</p>
              ) : (
                <ul className="max-h-40 overflow-y-auto border border-border-default rounded divide-y divide-border-subtle">
                  {matches.map((s) => (
                    <li key={s.id} className="px-3 py-1.5">
                      <label className="flex items-center gap-2 text-sm">
                        <input type="checkbox" checked={!excluded[s.id]} onChange={() => setExcluded((p) => ({ ...p, [s.id]: !p[s.id] }))} />
                        {s.name || `Student ${s.id}`}
                      </label>
                    </li>
                  ))}
                </ul>
              )}
            </fieldset>

            <label className="block text-sm text-text-secondary">
              Subject
              <input value={subject} onChange={(e) => setSubject(e.target.value)} required className="mt-1 w-full rounded border border-border-strong px-2 py-1.5 text-sm" />
            </label>
            <label className="block text-sm text-text-secondary">
              Message
              <textarea value={message} onChange={(e) => setMessage(e.target.value)} required rows={5} className="mt-1 w-full rounded border border-border-strong px-2 py-1.5 text-sm" />
            </label>
            <p className="text-xs text-text-tertiary">Each student receives a separate message and can&apos;t see who else got it.</p>
            <div className="flex justify-end gap-2">
              <button type="button" onClick={onClose} className="px-4 py-2 text-sm rounded border border-border-strong hover:bg-surface-1">Cancel</button>
              <button type="submit" disabled={sending || recipients.length === 0} className="inline-flex items-center gap-2 px-4 py-2 text-sm rounded bg-brand-600 text-on-brand hover:bg-brand-700 disabled:opacity-50">
                <Send className="w-4 h-4" /> {sending ? 'Sending…' : 'Send'}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
}
