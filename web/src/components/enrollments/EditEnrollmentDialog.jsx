import React, { useState } from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { useUpdateEnrollment } from '../../hooks/useAdminEnrollments';
import { ROLE_LABELS, assignableRoles, buildEnrollmentPatch, enrollmentForm } from '../../utils/enrollmentLifecycle';

const fieldClass = 'w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500';

/**
 * Edit one enrollment: role, section, participation dates and the
 * section limit (G105 / G106 / G107). Only changed fields are sent.
 */
export default function EditEnrollmentDialog({ courseId, enrollment, sections = [], caller, onClose }) {
  const [form, setForm] = useState(() => enrollmentForm(enrollment));
  const [error, setError] = useState(null);
  const update = useUpdateEnrollment(courseId, {
    onSuccess: () => onClose(),
    onError: (err) => setError(err.message),
  });

  const roles = assignableRoles(caller);
  if (!roles.includes(form.type)) roles.unshift(form.type);
  const set = (patch) => setForm((f) => ({ ...f, ...patch }));
  const name = enrollment?.user?.name || `User ${enrollment?.user_id}`;

  const save = (e) => {
    e.preventDefault();
    const { patch, error: formError } = buildEnrollmentPatch(enrollment, form);
    if (formError) {
      setError(formError);
      return;
    }
    if (Object.keys(patch).length === 0) {
      onClose();
      return;
    }
    setError(null);
    update.mutate({ enrollmentId: enrollment.id, patch });
  };

  return (
    <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent>
        <form onSubmit={save} className="space-y-4">
          <DialogHeader>
            <DialogTitle>Edit enrollment</DialogTitle>
            <DialogDescription>{name}</DialogDescription>
          </DialogHeader>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label htmlFor="edit-enrollment-role" className="block text-sm font-medium text-text-secondary mb-1">Role</label>
              <select id="edit-enrollment-role" className={fieldClass} value={form.type} onChange={(e) => set({ type: e.target.value })}>
                {roles.map((r) => <option key={r} value={r}>{ROLE_LABELS[r] || r}</option>)}
              </select>
            </div>
            <div>
              <label htmlFor="edit-enrollment-section" className="block text-sm font-medium text-text-secondary mb-1">Section</label>
              <select id="edit-enrollment-section" className={fieldClass} value={form.course_section_id} onChange={(e) => set({ course_section_id: e.target.value })}>
                <option value="">Default section</option>
                {sections.map((s) => <option key={s.id} value={String(s.id)}>{s.name || `Section ${s.id}`}</option>)}
              </select>
            </div>
            <div>
              <label htmlFor="edit-enrollment-start" className="block text-sm font-medium text-text-secondary mb-1">Participation starts</label>
              <input id="edit-enrollment-start" type="date" className={fieldClass} value={form.start_at} onChange={(e) => set({ start_at: e.target.value })} />
            </div>
            <div>
              <label htmlFor="edit-enrollment-end" className="block text-sm font-medium text-text-secondary mb-1">Participation ends</label>
              <input id="edit-enrollment-end" type="date" className={fieldClass} value={form.end_at} onChange={(e) => set({ end_at: e.target.value })} />
            </div>
          </div>
          <p className="text-xs text-text-tertiary">
            Enrollment dates apply only when both are set; they override section, course and term dates.
            After the end date the person keeps read-only access.
          </p>
          <label className="inline-flex items-center gap-2 text-sm text-text-secondary">
            <input
              type="checkbox"
              checked={form.limit_privileges_to_course_section}
              onChange={(e) => set({ limit_privileges_to_course_section: e.target.checked })}
            />
            Can only interact with people in their section
          </label>
          {error && <p role="alert" className="text-sm text-accent-danger">{error}</p>}
          <DialogFooter>
            <button type="button" onClick={onClose} className="px-4 py-2 rounded-md text-sm border border-border-strong">Cancel</button>
            <button type="submit" disabled={update.isPending} className="px-4 py-2 rounded-md text-sm bg-brand-600 text-white hover:bg-brand-700 disabled:opacity-50">
              {update.isPending ? 'Saving...' : 'Save'}
            </button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
