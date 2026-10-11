import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, Navigate, useParams } from 'react-router-dom';
import { ChevronLeft, ChevronRight, User, FileCheck } from 'lucide-react';
import { api } from '../services/api';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import SubmissionComments from '../components/SubmissionComments';
import { assignmentGradeWeight, enteredScore, getLetterGrade } from '../utils/grading';

// The individual gradebook view (Canvas "Individual View"): one student
// and one assignment at a time, keyboard- and screen-reader-friendly,
// with the student's running total and the assignment's statistics.

export const assignmentStats = (scores) => {
  const nums = scores.filter((n) => typeof n === 'number' && !Number.isNaN(n)).sort((a, b) => a - b);
  if (nums.length === 0) return null;
  const sum = nums.reduce((s, n) => s + n, 0);
  const mid = Math.floor(nums.length / 2);
  const median = nums.length % 2 ? nums[mid] : (nums[mid - 1] + nums[mid]) / 2;
  return { count: nums.length, mean: +(sum / nums.length).toFixed(2), median, low: nums[0], high: nums[nums.length - 1] };
};

const IndividualGradebookPage = () => {
  const { courseId } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const [course, setCourse] = useState(null);
  const [assignments, setAssignments] = useState([]);
  const [students, setStudents] = useState([]);
  const [submissions, setSubmissions] = useState({});
  const [anonymized, setAnonymized] = useState(new Set());
  const [studentId, setStudentId] = useState('');
  const [assignmentId, setAssignmentId] = useState('');
  const [grade, setGrade] = useState('');
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState(null);
  const [error, setError] = useState(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!isTeacher) return;
    (async () => {
      try {
        const [c, a, e, gb] = await Promise.all([
          api.getCourse(courseId),
          api.getAssignments(courseId, 1, 1000),
          api.getAllEnrollments(courseId),
          api.getGradebook(courseId),
        ]);
        setCourse(c);
        setAssignments((a?.data || []).filter((x) => x.workflow_state !== 'deleted'));
        const seen = new Set();
        const list = [];
        for (const en of e?.data || []) {
          if (en.type !== 'StudentEnrollment' && en.role !== 'StudentEnrollment') continue;
          const id = en.user_id || en.user?.id;
          if (!id || seen.has(id)) continue;
          seen.add(id);
          list.push({ id, name: en.user?.name || `User ${id}`, sortable: en.user?.sortable_name || en.user?.name || '' });
        }
        list.sort((x, y) => x.sortable.localeCompare(y.sortable));
        setStudents(list);
        setSubmissions(gb?.submissions || {});
        setAnonymized(new Set((gb?.anonymized_assignment_ids || []).map(Number)));
      } catch (err) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    })();
  }, [courseId, isTeacher]);

  const cellFor = useCallback((sid, aid) => submissions?.[String(sid)]?.[String(aid)] || null, [submissions]);
  const student = students.find((s) => String(s.id) === String(studentId)) || null;
  const assignment = assignments.find((a) => String(a.id) === String(assignmentId)) || null;
  const cell = student && assignment ? cellFor(student.id, assignment.id) : null;
  const isAnonymized = assignment && anonymized.has(Number(assignment.id));

  useEffect(() => {
    const v = enteredScore(cell);
    setGrade(v == null ? '' : String(v));
    setMessage(null);
  }, [studentId, assignmentId, cell?.score]); // eslint-disable-line react-hooks/exhaustive-deps

  const total = useMemo(() => {
    if (!student) return null;
    let earned = 0;
    let possible = 0;
    for (const a of assignments) {
      const c = cellFor(student.id, a.id);
      const w = assignmentGradeWeight(a, c);
      if (w.counts && c && c.score != null) {
        earned += parseFloat(c.score) || 0;
        possible += w.possible;
      }
    }
    if (possible === 0) return { earned, possible, pct: null };
    return { earned, possible, pct: +((earned / possible) * 100).toFixed(1) };
  }, [student, assignments, cellFor]);

  const stats = useMemo(() => {
    if (!assignment || isAnonymized) return null;
    return assignmentStats(students.map((s) => {
      const c = cellFor(s.id, assignment.id);
      return c && c.score != null ? parseFloat(c.score) : null;
    }));
  }, [assignment, students, cellFor, isAnonymized]);

  const step = (list, current, set, dir) => {
    const idx = list.findIndex((x) => String(x.id) === String(current));
    const next = list[Math.min(list.length - 1, Math.max(0, idx + dir))];
    if (next) set(String(next.id));
  };

  const save = async (e) => {
    e.preventDefault();
    if (!student || !assignment) return;
    setSaving(true);
    setError(null);
    try {
      const saved = await api.gradeSubmission(courseId, assignment.id, student.id, { posted_grade: grade.trim() });
      setSubmissions((prev) => ({
        ...prev,
        [String(student.id)]: { ...(prev[String(student.id)] || {}), [String(assignment.id)]: { ...(cell || {}), ...(saved || {}) } },
      }));
      setMessage(`Grade saved for ${student.name}.`);
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const toggleExcused = async () => {
    if (!student || !assignment) return;
    try {
      const saved = await api.gradeSubmission(courseId, assignment.id, student.id, { excused: !cell?.excused });
      setSubmissions((prev) => ({
        ...prev,
        [String(student.id)]: { ...(prev[String(student.id)] || {}), [String(assignment.id)]: { ...(cell || {}), ...(saved || {}) } },
      }));
    } catch (err) {
      setError(err.message);
    }
  };

  if (isTeacher === false) return <Navigate to={`/courses/${courseId}`} replace />;

  const select = 'border border-border-strong rounded-md px-3 py-2 text-sm min-w-[14rem]';
  return (
    <Layout>
      <CourseNav />
      <div className="mb-4">
        <Link to={`/courses/${courseId}/gradebook`} className="text-brand-600 hover:underline text-sm">&larr; Back to Gradebook</Link>
        <h2 className="text-2xl font-bold text-text-primary mt-2">Individual Gradebook{course ? `: ${course.name}` : ''}</h2>
      </div>
      {error && <div className="mb-4 p-3 rounded-md text-sm bg-accent-danger/10 text-accent-danger border border-accent-danger/30" role="alert">{error}</div>}
      {loading ? (
        <p className="text-text-tertiary">Loading...</p>
      ) : (
        <div className="space-y-6">
          <section className="bg-surface-0 rounded-lg shadow p-4 flex flex-wrap items-end gap-4" aria-label="Selection">
            <div className="text-sm text-text-secondary">
              <label htmlFor="ig-student">Student</label>
              <div className="flex items-center gap-1 mt-1">
                <button type="button" aria-label="Previous student" onClick={() => step(students, studentId, setStudentId, -1)} className="p-2 rounded hover:bg-surface-2"><ChevronLeft className="w-4 h-4" /></button>
                <select id="ig-student" value={studentId} onChange={(e) => setStudentId(e.target.value)} className={select}>
                  <option value="">Select a student</option>
                  {students.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
                </select>
                <button type="button" aria-label="Next student" onClick={() => step(students, studentId, setStudentId, 1)} className="p-2 rounded hover:bg-surface-2"><ChevronRight className="w-4 h-4" /></button>
              </div>
            </div>
            <div className="text-sm text-text-secondary">
              <label htmlFor="ig-assignment">Assignment</label>
              <div className="flex items-center gap-1 mt-1">
                <button type="button" aria-label="Previous assignment" onClick={() => step(assignments, assignmentId, setAssignmentId, -1)} className="p-2 rounded hover:bg-surface-2"><ChevronLeft className="w-4 h-4" /></button>
                <select id="ig-assignment" value={assignmentId} onChange={(e) => setAssignmentId(e.target.value)} className={select}>
                  <option value="">Select an assignment</option>
                  {assignments.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}
                </select>
                <button type="button" aria-label="Next assignment" onClick={() => step(assignments, assignmentId, setAssignmentId, 1)} className="p-2 rounded hover:bg-surface-2"><ChevronRight className="w-4 h-4" /></button>
              </div>
            </div>
          </section>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <section className="bg-surface-0 rounded-lg shadow p-4" aria-label="Student information">
              <h3 className="font-semibold text-text-primary flex items-center gap-2 mb-3"><User className="w-4 h-4" /> Student information</h3>
              {student ? (
                <div className="space-y-2 text-sm">
                  <p className="font-medium">{student.name}</p>
                  {total && (
                    <p>
                      Current total: {total.pct == null ? '—' : `${total.pct}% (${total.earned} / ${total.possible})`}
                      {total.pct != null && <span className="ml-2 font-semibold">{getLetterGrade(total.pct)}</span>}
                      {course?.hide_final_grades && <span className="ml-2 text-xs text-text-tertiary">(hidden from the student)</span>}
                    </p>
                  )}
                  <table className="w-full text-xs mt-2">
                    <caption className="sr-only">Grades for {student.name}</caption>
                    <thead><tr className="text-left text-text-tertiary"><th scope="col" className="py-1">Assignment</th><th scope="col">Score</th><th scope="col">Status</th></tr></thead>
                    <tbody>
                      {assignments.map((a) => {
                        const c = cellFor(student.id, a.id);
                        const anon = anonymized.has(Number(a.id));
                        return (
                          <tr key={a.id} className="border-t border-border-subtle">
                            <td className="py-1">
                              <button type="button" className="text-brand-600 hover:underline text-left" onClick={() => setAssignmentId(String(a.id))}>{a.name}</button>
                              {a.omit_from_final_grade && <span className="ml-1 text-text-tertiary">(not counted)</span>}
                            </td>
                            <td>{anon ? 'Anonymous' : c?.excused ? 'EX' : c?.score != null ? `${c.score} / ${a.points_possible ?? 0}` : '—'}</td>
                            <td>{c?.late_policy_status && c.late_policy_status !== 'none' ? c.late_policy_status : c?.missing ? 'missing' : c?.late ? 'late' : ''}</td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              ) : (
                <p className="text-sm text-text-tertiary">Select a student to see their grades.</p>
              )}
            </section>

            <section className="bg-surface-0 rounded-lg shadow p-4" aria-label="Assignment information">
              <h3 className="font-semibold text-text-primary flex items-center gap-2 mb-3"><FileCheck className="w-4 h-4" /> Assignment information</h3>
              {assignment ? (
                <div className="space-y-2 text-sm">
                  <p>
                    <Link to={`/courses/${courseId}/assignments/${assignment.id}`} className="font-medium text-brand-600 hover:underline">{assignment.name}</Link>
                  </p>
                  <p>{assignment.points_possible ?? 0} points{assignment.due_at ? ` · due ${new Date(assignment.due_at).toLocaleString()}` : ''}</p>
                  {stats ? (
                    <dl className="grid grid-cols-5 gap-2 text-xs">
                      <div><dt className="text-text-tertiary">Graded</dt><dd>{stats.count}</dd></div>
                      <div><dt className="text-text-tertiary">Average</dt><dd>{stats.mean}</dd></div>
                      <div><dt className="text-text-tertiary">Median</dt><dd>{stats.median}</dd></div>
                      <div><dt className="text-text-tertiary">High</dt><dd>{stats.high}</dd></div>
                      <div><dt className="text-text-tertiary">Low</dt><dd>{stats.low}</dd></div>
                    </dl>
                  ) : (
                    <p className="text-xs text-text-tertiary">{isAnonymized ? 'Statistics are hidden while students are anonymized.' : 'No grades yet.'}</p>
                  )}
                  <Link to={`/courses/${courseId}/assignments/${assignment.id}/speedgrader`} className="text-xs text-brand-600 hover:underline">Open in SpeedGrader</Link>
                </div>
              ) : (
                <p className="text-sm text-text-tertiary">Select an assignment to see its details.</p>
              )}
            </section>
          </div>

          {student && assignment && (
            <section className="bg-surface-0 rounded-lg shadow p-4 space-y-4" aria-label="Grade">
              <h3 className="font-semibold text-text-primary">Grade for {student.name} — {assignment.name}</h3>
              {isAnonymized ? (
                <p className="text-sm text-text-tertiary">This assignment anonymizes students; grade it in SpeedGrader.</p>
              ) : (
                <>
                  <form onSubmit={save} className="flex flex-wrap items-center gap-3">
                    <label htmlFor="ig-grade" className="text-sm text-text-secondary">Score</label>
                    <input id="ig-grade" type="number" step="any" min="0" value={grade} onChange={(e) => setGrade(e.target.value)} disabled={!!cell?.excused}
                      className="w-28 border border-border-strong rounded-md px-3 py-2 text-sm" />
                    <span className="text-sm text-text-tertiary">/ {assignment.points_possible ?? 0}</span>
                    <button type="submit" disabled={saving || !!cell?.excused} className="px-4 py-2 bg-brand-600 text-white rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50">
                      {saving ? 'Saving...' : 'Save grade'}
                    </button>
                    <label className="flex items-center gap-2 text-sm text-text-secondary">
                      <input type="checkbox" checked={!!cell?.excused} onChange={toggleExcused} /> Excused
                    </label>
                  </form>
                  {cell?.points_deducted > 0 && <p className="text-xs text-accent-danger">Late penalty -{cell.points_deducted} (final {cell.score})</p>}
                  {message && <p className="text-sm text-accent-success" role="status">{message}</p>}
                  <SubmissionComments courseId={courseId} assignmentId={assignment.id} userId={student.id} isTeacher />
                </>
              )}
            </section>
          )}
        </div>
      )}
    </Layout>
  );
};

export default IndividualGradebookPage;
