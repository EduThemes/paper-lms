import React, { useCallback, useEffect, useState } from 'react';
import { Mail } from 'lucide-react';
import { enrollmentsAdminApi } from '../../services/api_admin_enrollments';
import { ROLE_LABELS } from '../../utils/enrollmentLifecycle';

/**
 * Pending course invitations for the signed-in user, with Accept /
 * Decline (G122). Renders nothing when there are none or the lookup
 * fails. Plain state (no react-query) so it can sit on pages that are
 * rendered without a QueryClientProvider in tests.
 */
export default function CourseInvitationsBanner({ onAccepted }) {
  const [invitations, setInvitations] = useState([]);
  const [busy, setBusy] = useState(null);
  const [error, setError] = useState(null);

  const load = useCallback(async () => {
    try {
      setInvitations(await enrollmentsAdminApi.listInvitations());
    } catch {
      setInvitations([]);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const respond = async (inv, accept) => {
    setBusy(inv.id);
    setError(null);
    try {
      if (accept) {
        await enrollmentsAdminApi.acceptInvitation(inv.course_id, inv.id);
        onAccepted?.(inv);
      } else {
        await enrollmentsAdminApi.rejectInvitation(inv.course_id, inv.id);
      }
      setInvitations((list) => list.filter((i) => i.id !== inv.id));
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(null);
    }
  };

  if (invitations.length === 0) return null;

  return (
    <section aria-label="Course invitations" className="mb-6 rounded-lg border border-brand-200 bg-brand-50 p-4">
      <h2 className="flex items-center gap-2 text-sm font-semibold text-text-primary mb-2">
        <Mail className="w-4 h-4" />
        Course invitations
      </h2>
      <ul className="space-y-2">
        {invitations.map((inv) => (
          <li key={inv.id} className="flex flex-wrap items-center gap-3 text-sm">
            <span className="flex-1 min-w-[12rem] text-text-primary">
              You have been invited to join <strong>{inv.course_name || `course ${inv.course_id}`}</strong>
              {' '}as a {(ROLE_LABELS[inv.type] || 'Student').toLowerCase()}.
            </span>
            <button
              type="button"
              disabled={busy === inv.id}
              onClick={() => respond(inv, true)}
              className="px-3 py-1.5 rounded-md bg-brand-600 text-on-brand text-xs font-medium hover:bg-brand-700 disabled:opacity-50"
            >
              Accept
            </button>
            <button
              type="button"
              disabled={busy === inv.id}
              onClick={() => respond(inv, false)}
              className="px-3 py-1.5 rounded-md border border-border-strong text-xs font-medium hover:bg-surface-1 disabled:opacity-50"
            >
              Decline
            </button>
          </li>
        ))}
      </ul>
      {error && <p role="alert" className="text-sm text-accent-danger mt-2">{error}</p>}
    </section>
  );
}
