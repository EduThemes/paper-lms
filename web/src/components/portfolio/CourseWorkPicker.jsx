import React, { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { FileText, Paperclip, Link2, CheckCircle2 } from 'lucide-react';
import { api } from '../../services/api';
import { portfolioApi } from '../../services/api_learner_outcomes';

/**
 * Pick turned-in course work to add to an ePortfolio (G155): choose one of
 * your courses, then tick submissions. Work already in the portfolio is
 * shown but can't be picked again. `selected` is an array of submission ids.
 */
export default function CourseWorkPicker({ portfolioId, selected, onChange }) {
  const [courseId, setCourseId] = useState('');
  const courses = useQuery({
    queryKey: ['portfolio', 'courses'],
    queryFn: async () => {
      const res = await api.getCourses(1, 100);
      return Array.isArray(res?.data) ? res.data : [];
    },
  });
  const submissions = useQuery({
    queryKey: ['portfolio', String(portfolioId), 'importable', String(courseId)],
    queryFn: () => portfolioApi.importableSubmissions(portfolioId, courseId),
    enabled: !!courseId,
  });

  const toggle = (id) => {
    const set = new Set(selected || []);
    if (set.has(id)) set.delete(id);
    else set.add(id);
    onChange([...set]);
  };

  return (
    <div className="space-y-3">
      <div>
        <label htmlFor="cw-course" className="block text-sm font-semibold text-text-secondary mb-1">Course</label>
        <select id="cw-course" value={courseId} onChange={(e) => { setCourseId(e.target.value); onChange([]); }}
          className="w-full px-4 py-2.5 border border-border-default rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-brand-500">
          <option value="">{courses.isLoading ? 'Loading courses…' : 'Choose a course…'}</option>
          {(courses.data || []).map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
        </select>
      </div>
      {courseId && (
        <fieldset>
          <legend className="block text-sm font-semibold text-text-secondary mb-1">Your submitted work</legend>
          {submissions.isLoading ? (
            <p className="text-sm text-text-tertiary">Loading…</p>
          ) : submissions.isError ? (
            <p role="alert" className="text-sm text-accent-danger">{submissions.error?.message}</p>
          ) : (submissions.data || []).length === 0 ? (
            <p className="text-sm text-text-tertiary">You haven&apos;t turned in anything in this course yet.</p>
          ) : (
            <ul className="max-h-56 overflow-y-auto border border-border-default rounded-xl divide-y divide-border-subtle">
              {submissions.data.map((s) => (
                <li key={s.submission_id}>
                  <label className={`flex items-start gap-3 px-3 py-2 text-sm ${s.already_imported ? 'opacity-60' : 'cursor-pointer hover:bg-surface-1'}`}>
                    <input type="checkbox" className="mt-0.5" disabled={s.already_imported}
                      checked={(selected || []).includes(s.submission_id)} onChange={() => toggle(s.submission_id)} />
                    <span className="flex-1 min-w-0">
                      <span className="font-medium text-text-primary block truncate">{s.assignment_name}</span>
                      <span className="text-xs text-text-tertiary flex flex-wrap items-center gap-2">
                        {s.submitted_at && new Date(s.submitted_at).toLocaleDateString()}
                        {s.has_text && <span className="inline-flex items-center gap-0.5"><FileText className="w-3 h-3" aria-hidden="true" /> text</span>}
                        {s.file_count > 0 && <span className="inline-flex items-center gap-0.5"><Paperclip className="w-3 h-3" aria-hidden="true" /> {s.file_count} file{s.file_count === 1 ? '' : 's'}</span>}
                        {s.url && <span className="inline-flex items-center gap-0.5"><Link2 className="w-3 h-3" aria-hidden="true" /> link</span>}
                        {s.already_imported && <span className="inline-flex items-center gap-0.5 text-accent-success"><CheckCircle2 className="w-3 h-3" aria-hidden="true" /> already added</span>}
                      </span>
                    </span>
                  </label>
                </li>
              ))}
            </ul>
          )}
        </fieldset>
      )}
    </div>
  );
}
