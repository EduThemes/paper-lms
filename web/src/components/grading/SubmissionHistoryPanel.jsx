import React, { useEffect, useState } from 'react';
import { History } from 'lucide-react';
import { gradingPolicyApi } from '../../services/api_grading_policy';
import SafeHTML from '../SafeHTML';

// SubmissionHistoryPanel lists every attempt of a student's submission
// (G028), newest first, with the grade each attempt received.
const SubmissionHistoryPanel = ({ courseId, assignmentId, userId, refreshKey }) => {
  const [versions, setVersions] = useState(null);
  const [open, setOpen] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    if (!courseId || !assignmentId || !userId) return;
    let cancelled = false;
    setError(null);
    gradingPolicyApi
      .getSubmissionHistory(courseId, assignmentId, userId)
      .then((data) => {
        if (!cancelled) setVersions(Array.isArray(data) ? [...data].reverse() : []);
      })
      .catch((e) => {
        if (!cancelled) {
          setVersions([]);
          setError(e.message);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [courseId, assignmentId, userId, refreshKey]);

  if (versions === null) return null;
  if (versions.length === 0) {
    return error ? <p className="text-xs text-text-tertiary">Submission history is unavailable.</p> : null;
  }

  return (
    <section aria-label="Submission history" className="rounded-md border border-border-default">
      <h3 className="flex items-center gap-2 px-3 py-2 text-sm font-semibold border-b border-border-default">
        <History className="h-4 w-4 text-text-tertiary" />
        Submission history ({versions.length} attempt{versions.length === 1 ? '' : 's'})
      </h3>
      <ul className="divide-y divide-border-subtle">
        {versions.map((v) => (
          <li key={v.id} className="px-3 py-2 text-sm">
            <button
              type="button"
              onClick={() => setOpen(open === v.id ? null : v.id)}
              aria-expanded={open === v.id}
              className="flex w-full items-center justify-between gap-2 text-left"
            >
              <span>
                Attempt {v.attempt}
                {v.current && <span className="ml-2 text-xs text-brand-600">current</span>}
                {v.late && <span className="ml-2 text-xs text-accent-danger">late</span>}
              </span>
              <span className="text-xs text-text-tertiary">
                {v.submitted_at ? new Date(v.submitted_at).toLocaleString() : ''}
                {v.grade != null && v.grade !== '' ? ` · ${v.grade}` : v.score != null ? ` · ${v.score}` : ''}
              </span>
            </button>
            {open === v.id && (
              <div className="mt-2 space-y-2 text-text-secondary">
                {v.body && <SafeHTML html={v.body} className="prose prose-sm max-w-none" />}
                {v.url && (
                  <a href={v.url} target="_blank" rel="noopener noreferrer" className="text-brand-600 hover:underline break-all">
                    {v.url}
                  </a>
                )}
                {Array.isArray(v.attachments) && v.attachments.length > 0 && (
                  <ul className="list-disc ml-5">
                    {v.attachments.map((a, i) => (
                      <li key={a.id || i}>{a.display_name || a.filename || `File ${i + 1}`}</li>
                    ))}
                  </ul>
                )}
                {!v.body && !v.url && !(v.attachments && v.attachments.length) && (
                  <p className="text-xs text-text-tertiary">No content recorded for this attempt.</p>
                )}
              </div>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
};

export default SubmissionHistoryPanel;
