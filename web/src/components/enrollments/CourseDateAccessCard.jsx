import React, { useEffect, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { enrollmentsAdminApi } from '../../services/api_admin_enrollments';

const KEYS = ['restrict_enrollments_to_course_dates', 'restrict_student_future_view', 'restrict_student_past_view'];

const pick = (course) => Object.fromEntries(KEYS.map((k) => [k, Boolean(course?.[k])]));

/**
 * Course settings card for date-based access (G106). Saved on its own
 * (PUT /courses/:id) so it doesn't touch the rest of the settings form.
 */
export default function CourseDateAccessCard({ courseId, course, onSaved }) {
  // The card keeps its own saved baseline so saving it never pushes a new
  // course object into the surrounding settings form (which would reset
  // that form's unsaved edits).
  const [baseline, setBaseline] = useState(() => pick(course));
  const [form, setForm] = useState(() => pick(course));
  const [message, setMessage] = useState(null);
  const courseKey = KEYS.map((k) => String(Boolean(course?.[k]))).join(',');
  useEffect(() => {
    setBaseline(pick(course));
    setForm(pick(course));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [course?.id, courseKey]);

  const save = useMutation({
    mutationFn: () => enrollmentsAdminApi.updateCourseDateAccess(courseId, form),
    onSuccess: (updated) => {
      setBaseline(pick(updated));
      setMessage('Participation settings saved.');
      onSaved?.(updated);
    },
    onError: (err) => setMessage(err.message),
  });

  const dirty = KEYS.some((k) => form[k] !== baseline[k]);
  const toggle = (k) => (e) => setForm((f) => ({ ...f, [k]: e.target.checked }));

  return (
    <section aria-labelledby="date-access-heading" className="rounded-lg border border-border-default bg-surface-0 p-4 space-y-3">
      <h3 id="date-access-heading" className="text-sm font-semibold text-text-primary">Participation dates</h3>
      <label className="flex items-start gap-2 text-sm text-text-secondary">
        <input type="checkbox" className="mt-1" checked={form.restrict_enrollments_to_course_dates} onChange={toggle('restrict_enrollments_to_course_dates')} />
        <span>
          People can only participate between the course start and end dates
          <span className="block text-xs text-text-tertiary">Otherwise the term&apos;s dates apply. After the end date the course becomes read-only.</span>
        </span>
      </label>
      <label className="flex items-start gap-2 text-sm text-text-secondary">
        <input type="checkbox" className="mt-1" checked={form.restrict_student_future_view} onChange={toggle('restrict_student_future_view')} />
        <span>Students can&apos;t view the course before it starts</span>
      </label>
      <label className="flex items-start gap-2 text-sm text-text-secondary">
        <input type="checkbox" className="mt-1" checked={form.restrict_student_past_view} onChange={toggle('restrict_student_past_view')} />
        <span>Students can&apos;t view the course after it ends</span>
      </label>
      <div className="flex items-center gap-3">
        <button
          type="button"
          disabled={!dirty || save.isPending}
          onClick={() => { setMessage(null); save.mutate(); }}
          className="px-3 py-1.5 rounded-md bg-brand-600 text-white text-sm font-medium hover:bg-brand-700 disabled:opacity-50"
        >
          {save.isPending ? 'Saving...' : 'Save participation settings'}
        </button>
        {message && <span role="status" className="text-sm text-text-secondary">{message}</span>}
      </div>
    </section>
  );
}
