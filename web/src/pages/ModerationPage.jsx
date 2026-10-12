import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { Scale, CheckCircle, AlertCircle } from 'lucide-react';
import Layout from '../components/Layout';
import gradingApi from '../services/api_grading_ui';

// Moderated grading (Canvas "Moderate" / assignment grade summary): the
// moderator compares every grader's provisional grade per student,
// picks one (or enters a final grade), then publishes the grades.

const scoreOf = (pg) => (pg ? (pg.score ?? pg.grade ?? '—') : '—');

const ModerationPage = () => {
  const { courseId, assignmentId } = useParams();
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(null);
  const [finalInputs, setFinalInputs] = useState({});
  const [published, setPublished] = useState(null);

  const load = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      setData(await gradingApi.getModeration(courseId, assignmentId));
    } catch (err) {
      setError(err.status === 403 ? 'Only the moderator can moderate this assignment.' : err.message);
    } finally {
      setLoading(false);
    }
  }, [courseId, assignmentId]);

  useEffect(() => { load(); }, [load]);

  const graderNames = useMemo(() => {
    const names = (data?.graders || []).map((g) => g.name);
    return Array.from(new Set(names));
  }, [data]);

  const studentKey = (s) => s.anonymous_id || s.user_id;
  const studentRef = (s) => (s.anonymous_id ? { anonymousId: s.anonymous_id } : { userId: s.user_id });

  const select = async (student, pg) => {
    setBusy(`sel-${studentKey(student)}`);
    setError(null);
    try {
      await gradingApi.selectProvisionalGrade(courseId, assignmentId, { ...studentRef(student), provisionalGradeId: pg.id });
      setData((prev) => ({
        ...prev,
        students: prev.students.map((s) => (studentKey(s) === studentKey(student) ? { ...s, selected_provisional_grade_id: pg.id } : s)),
      }));
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(null);
    }
  };

  const saveFinal = async (student) => {
    const value = finalInputs[studentKey(student)];
    if (value === undefined || value === '') return;
    setBusy(`final-${studentKey(student)}`);
    setError(null);
    try {
      const pg = await gradingApi.saveProvisionalGrade(courseId, assignmentId, { ...studentRef(student), postedGrade: String(value), final: true });
      await gradingApi.selectProvisionalGrade(courseId, assignmentId, { ...studentRef(student), provisionalGradeId: pg.id });
      setData((prev) => ({
        ...prev,
        students: prev.students.map((s) => {
          if (studentKey(s) !== studentKey(student)) return s;
          const others = (s.provisional_grades || []).filter((p) => !(p.final && p.mine));
          return { ...s, provisional_grades: [...others, { ...pg, final: true, mine: true, scorer_name: 'You' }], selected_provisional_grade_id: pg.id };
        }),
      }));
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(null);
    }
  };

  const publish = async () => {
    if (!window.confirm('Publish the selected grades? Graders can no longer change provisional grades afterwards.')) return;
    setBusy('publish');
    setError(null);
    try {
      const res = await gradingApi.publishModeratedGrades(courseId, assignmentId);
      setPublished(res || {});
      await load();
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(null);
    }
  };

  const assignment = data?.assignment;
  const isPublished = !!assignment?.grades_published;
  const unselected = (data?.students || []).filter((s) => !s.selected_provisional_grade_id && (s.provisional_grades || []).length > 0).length;

  return (
    <Layout>
      <div className="mb-4">
        <Link to={`/courses/${courseId}/assignments/${assignmentId}`} className="text-brand-600 hover:underline text-sm">
          &larr; Back to Assignment
        </Link>
        <div className="flex items-center justify-between mt-2 flex-wrap gap-2">
          <h2 className="text-2xl font-bold text-text-primary flex items-center gap-2">
            <Scale className="w-6 h-6" /> Moderate{assignment ? `: ${assignment.name}` : ''}
          </h2>
          {assignment && !isPublished && (
            <button
              type="button"
              onClick={publish}
              disabled={busy === 'publish'}
              className="inline-flex items-center gap-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50"
            >
              {busy === 'publish' ? 'Publishing...' : 'Release grades'}
            </button>
          )}
          {isPublished && (
            <span className="inline-flex items-center gap-1 text-sm text-accent-success">
              <CheckCircle className="w-4 h-4" /> Grades released
            </span>
          )}
        </div>
        {assignment && (
          <p className="text-sm text-text-tertiary mt-1">
            {assignment.points_possible ?? 0} points · up to {assignment.grader_count} graders
            {assignment.anonymize_students ? ' · students anonymized' : ''}
          </p>
        )}
      </div>

      {error && (
        <div className="mb-4 p-3 rounded-md text-sm bg-accent-danger/10 text-accent-danger border border-accent-danger/30 flex items-center gap-2" role="alert">
          <AlertCircle className="w-4 h-4" /> {error}
        </div>
      )}
      {published && (
        <div className="mb-4 p-3 rounded-md text-sm bg-accent-success/10 text-accent-success border border-accent-success/30" role="status">
          Grades released{published.published != null ? ` for ${published.published} student${published.published === 1 ? '' : 's'}` : ''}.
        </div>
      )}
      {!isPublished && unselected > 0 && (
        <p className="mb-3 text-sm text-accent-warning">{unselected} student{unselected === 1 ? ' has' : 's have'} provisional grades but no selection yet.</p>
      )}

      {loading ? (
        <div className="text-text-tertiary py-8 text-center">Loading...</div>
      ) : data ? (
        <div className="bg-surface-0 rounded-lg shadow overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead className="bg-surface-1">
              <tr>
                <th scope="col" className="text-left px-4 py-2 font-semibold text-text-secondary">Student</th>
                {graderNames.map((name) => (
                  <th key={name} scope="col" className="text-left px-4 py-2 font-semibold text-text-secondary">{name}</th>
                ))}
                <th scope="col" className="text-left px-4 py-2 font-semibold text-text-secondary">Moderator&apos;s grade</th>
                <th scope="col" className="text-left px-4 py-2 font-semibold text-text-secondary">Final</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border-subtle">
              {(data.students || []).map((s) => {
                const pgs = s.provisional_grades || [];
                const finalPg = pgs.find((p) => p.final);
                const selected = pgs.find((p) => p.id === s.selected_provisional_grade_id);
                return (
                  <tr key={studentKey(s)}>
                    <th scope="row" className="text-left px-4 py-2 font-medium text-text-primary">{s.name}</th>
                    {graderNames.map((name) => {
                      const pg = pgs.find((p) => !p.final && p.scorer_name === name);
                      return (
                        <td key={name} className="px-4 py-2">
                          {pg ? (
                            <label className="inline-flex items-center gap-2">
                              <input
                                type="radio"
                                name={`sel-${studentKey(s)}`}
                                checked={s.selected_provisional_grade_id === pg.id}
                                disabled={isPublished || busy != null}
                                onChange={() => select(s, pg)}
                                aria-label={`Select ${name}'s grade for ${s.name}`}
                              />
                              {scoreOf(pg)}
                            </label>
                          ) : (
                            <span className="text-text-disabled">—</span>
                          )}
                        </td>
                      );
                    })}
                    <td className="px-4 py-2">
                      {isPublished ? (
                        <span>{finalPg ? scoreOf(finalPg) : '—'}</span>
                      ) : (
                        <div className="flex items-center gap-2">
                          {finalPg && (
                            <input
                              type="radio"
                              name={`sel-${studentKey(s)}`}
                              checked={s.selected_provisional_grade_id === finalPg.id}
                              disabled={busy != null}
                              onChange={() => select(s, finalPg)}
                              aria-label={`Select the moderator's grade for ${s.name}`}
                            />
                          )}
                          <input
                            type="number"
                            step="any"
                            min="0"
                            aria-label={`Moderator's grade for ${s.name}`}
                            value={finalInputs[studentKey(s)] ?? (finalPg?.score ?? '')}
                            onChange={(e) => setFinalInputs((p) => ({ ...p, [studentKey(s)]: e.target.value }))}
                            className="w-20 border border-border-strong rounded px-2 py-1 text-sm"
                          />
                          <button
                            type="button"
                            onClick={() => saveFinal(s)}
                            disabled={busy != null || finalInputs[studentKey(s)] === undefined}
                            className="text-xs text-brand-600 hover:underline disabled:opacity-50"
                          >
                            Save
                          </button>
                        </div>
                      )}
                    </td>
                    <td className="px-4 py-2 font-semibold">
                      {isPublished ? (s.current_score ?? '—') : selected ? scoreOf(selected) : '—'}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      ) : null}
    </Layout>
  );
};

export default ModerationPage;
