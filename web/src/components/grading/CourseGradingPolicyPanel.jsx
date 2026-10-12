import React, { useEffect, useState } from 'react';
import { gradingPolicyApi } from '../../services/api_grading_policy';

// CourseGradingPolicyPanel holds the course-level grading policy:
//   - the default post policy for the course's assignments (G018);
//   - which grading scheme turns course totals into grades, or none (G039);
//   - re-applying late / missing statuses and the missing-submission
//     deduction to past-due work now (G022).
const CourseGradingPolicyPanel = ({ courseId }) => {
  const [postManually, setPostManually] = useState(false);
  const [schemes, setSchemes] = useState([]);
  const [standardId, setStandardId] = useState('');
  const [enabled, setEnabled] = useState(true);
  const [effective, setEffective] = useState(null);
  const [message, setMessage] = useState(null);
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  const load = async () => {
    try {
      const [pp, gs] = await Promise.all([
        gradingPolicyApi.getPostPolicy(courseId),
        gradingPolicyApi.getCourseGradingSchemes(courseId),
      ]);
      setPostManually(!!pp?.post_manually);
      setSchemes(gs?.grading_schemes || []);
      setStandardId(gs?.grading_standard_id ? String(gs.grading_standard_id) : '');
      setEnabled(gs?.grading_standard_enabled !== false);
      setEffective(gs?.effective_grading_scheme || null);
    } catch (e) {
      setError(e.message);
    }
  };

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [courseId]);

  const act = async (fn, done) => {
    setBusy(true);
    setError(null);
    setMessage(null);
    try {
      const res = await fn();
      setMessage(typeof done === 'function' ? done(res) : done);
      await load();
    } catch (e) {
      setError(e.message || 'Could not save.');
    } finally {
      setBusy(false);
    }
  };

  return (
    <section aria-labelledby="grading-policy-heading" className="bg-surface-0 rounded-lg shadow p-6 space-y-5">
      <h3 id="grading-policy-heading" className="text-lg font-semibold">Grading policy</h3>

      <div>
        <h4 className="text-sm font-medium text-text-secondary mb-2">Grade posting</h4>
        <label className="flex items-start gap-2 text-sm">
          <input
            type="checkbox"
            checked={postManually}
            disabled={busy}
            onChange={(e) =>
              act(() => gradingPolicyApi.setPostPolicy(courseId, e.target.checked),
                e.target.checked ? 'Grades are now posted manually.' : 'Grades are now posted automatically.')
            }
          />
          <span>
            Post grades manually
            <span className="block text-xs text-text-tertiary">
              Grades stay hidden from students until you post them. Applies to every assignment in the course.
            </span>
          </span>
        </label>
      </div>

      <div>
        <h4 className="text-sm font-medium text-text-secondary mb-2">Grading scheme</h4>
        <label className="flex items-center gap-2 text-sm mb-2">
          <input
            type="checkbox"
            checked={enabled}
            disabled={busy}
            onChange={(e) =>
              act(() => gradingPolicyApi.setCourseGradingScheme(courseId, { enabled: e.target.checked }), 'Grading scheme updated.')
            }
          />
          Show letter grades for course totals
        </label>
        <div className="flex items-center gap-2">
          <label htmlFor="course-grading-scheme" className="text-sm text-text-secondary">Scheme</label>
          <select
            id="course-grading-scheme"
            value={standardId}
            disabled={busy || !enabled}
            onChange={(e) =>
              act(() => gradingPolicyApi.setCourseGradingScheme(courseId, { gradingStandardId: e.target.value ? Number(e.target.value) : null }),
                'Grading scheme updated.')
            }
            className="border border-border-strong rounded-md px-2 py-1.5 text-sm"
          >
            <option value="">Course default (newest course scheme or the standard scale)</option>
            {schemes.map((s) => (
              <option key={s.id} value={s.id}>
                {s.title} {s.context_type === 'Account' ? '(account)' : ''}
              </option>
            ))}
          </select>
        </div>
        {effective && enabled && (
          <p className="mt-2 text-xs text-text-tertiary">
            In use: {effective.title}
            {Array.isArray(effective.data) && effective.data.length > 0 && (
              <> — {effective.data.map(([n, v]) => `${n} ≥ ${Math.round(v * 10000) / 100}%`).join(', ')}</>
            )}
          </p>
        )}
      </div>

      <div>
        <h4 className="text-sm font-medium text-text-secondary mb-2">Late and missing work</h4>
        <p className="text-xs text-text-tertiary mb-2">
          Statuses and the missing-submission deduction are applied automatically every hour. Apply them now after
          changing the late policy or due dates.
        </p>
        <button
          type="button"
          disabled={busy}
          onClick={() => act(() => gradingPolicyApi.applyLatePolicy(courseId), (r) => `Updated ${r?.updated ?? 0} submission(s).`)}
          className="px-3 py-1.5 text-sm border border-border-strong rounded-md hover:bg-surface-1 disabled:opacity-50"
        >
          Apply late policy now
        </button>
      </div>

      {message && <p role="status" className="text-sm text-accent-success">{message}</p>}
      {error && <p role="alert" className="text-sm text-accent-danger">{error}</p>}
    </section>
  );
};

export default CourseGradingPolicyPanel;
