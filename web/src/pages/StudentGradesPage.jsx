import React, { useMemo, useState } from 'react';
import { useParams, Link, useSearchParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { Calculator, RotateCcw, MessageSquare, ClipboardList, ChevronDown, ChevronRight } from 'lucide-react';
import { api } from '../services/api';
import { useGradeSummary } from '../services/api_learner';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import Page from '../components/Page';
import useDocumentTitle from '../hooks/useDocumentTitle';
import { computeTotals, letterFor, STATUS_LABELS } from '../utils/gradeSummary';
import { gradeColor } from '../utils/grading';

// Student Grades page (G147). Everything comes from the server's grade
// summary: posted scores only, drop rules, grading periods, hidden totals,
// feedback comments and rubric assessments. What-If mode recomputes the
// totals in the browser with the same rules (utils/gradeSummary).

const STATUS_STYLES = {
  graded: 'bg-accent-success/15 text-accent-success',
  submitted: 'bg-brand-100 text-brand-800',
  missing: 'bg-accent-danger/15 text-accent-danger',
  late: 'bg-accent-warning/20 text-accent-warning',
  excused: 'bg-surface-2 text-text-secondary',
  not_posted: 'bg-surface-2 text-text-secondary',
  unsubmitted: 'bg-surface-2 text-text-secondary',
};

const formatDate = (iso) =>
  iso ? new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' }) : '—';

function StatusBadge({ status, late }) {
  return (
    <span className="inline-flex flex-wrap items-center justify-center gap-1">
      <span className={`inline-block rounded-full px-2 py-0.5 text-xs ${STATUS_STYLES[status] || STATUS_STYLES.unsubmitted}`}>
        {STATUS_LABELS[status] || status}
      </span>
      {late && status !== 'late' && (
        <span className={`inline-block rounded-full px-2 py-0.5 text-xs ${STATUS_STYLES.late}`}>Late</span>
      )}
    </span>
  );
}

function parseJSON(raw, fallback) {
  if (raw && typeof raw === 'object') return raw;
  try {
    return JSON.parse(raw);
  } catch {
    return fallback;
  }
}

function RubricView({ rubric }) {
  const criteria = parseJSON(rubric.criteria, []);
  const assessment = parseJSON(rubric.assessment, {});
  return (
    <div className="mt-2 rounded-md border border-border-subtle">
      <div className="border-b border-border-subtle px-3 py-2 text-xs font-semibold text-text-secondary">
        Rubric: {rubric.title}
        {!rubric.hide_score_total && rubric.score != null && (
          <span className="ms-2 font-normal">
            {rubric.score} / {rubric.points_possible} pts
          </span>
        )}
      </div>
      <table className="min-w-full text-sm">
        <caption className="sr-only">Rubric assessment for {rubric.title}</caption>
        <thead>
          <tr className="text-left text-xs text-text-secondary">
            <th scope="col" className="px-3 py-1.5">Criterion</th>
            <th scope="col" className="px-3 py-1.5">Rating</th>
            {!rubric.hide_points && <th scope="col" className="px-3 py-1.5 text-right">Points</th>}
          </tr>
        </thead>
        <tbody>
          {(Array.isArray(criteria) ? criteria : []).map((c) => {
            const a = assessment?.[c.id] || assessment?.[String(c.id)] || {};
            const rating = (c.ratings || []).find((r) => r.id === a.rating_id) ||
              (c.ratings || []).find((r) => a.points != null && Number(r.points) === Number(a.points));
            return (
              <tr key={c.id} className="border-t border-border-subtle align-top">
                <td className="px-3 py-1.5">{c.description}</td>
                <td className="px-3 py-1.5">
                  {rating?.description || '—'}
                  {a.comments && <div className="mt-1 text-xs text-text-secondary">{a.comments}</div>}
                </td>
                {!rubric.hide_points && (
                  <td className="px-3 py-1.5 text-right tabular-nums">
                    {a.points != null ? a.points : '—'} / {c.points}
                  </td>
                )}
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function AssignmentRow({ a, courseId, whatIf, whatIfValue, onWhatIf, scale, dropped }) {
  const [open, setOpen] = useState(false);
  const pts = Number(a.points_possible) || 0;
  const hasDetails = a.comments.length > 0 || !!a.rubric;
  const score = whatIf ? (whatIfValue === '' || whatIfValue === undefined ? null : Number(whatIfValue)) : a.score;
  const pct = score != null && pts > 0 ? Math.round((score / pts) * 100) : null;
  const detailsId = `grade-details-${a.id}`;
  return (
    <>
      <tr className={dropped ? 'opacity-60' : ''}>
        <td className="px-4 py-3">
          <div className="flex items-center gap-2">
            <Link to={`/courses/${courseId}/assignments/${a.id}`} className="text-sm font-medium text-brand-600 hover:underline">
              {a.name}
            </Link>
            {dropped && (
              <span className="rounded bg-surface-2 px-1.5 py-0.5 text-xs text-text-secondary" title="This score is dropped by the group's rules and does not count">
                Dropped
              </span>
            )}
          </div>
        </td>
        <td className="whitespace-nowrap px-4 py-3 text-sm text-text-secondary">{formatDate(a.due_at)}</td>
        <td className="px-4 py-3 text-center"><StatusBadge status={a.status} late={a.late} /></td>
        <td className="whitespace-nowrap px-4 py-3 text-right">
          {whatIf ? (
            <span className="inline-flex items-center gap-1">
              <input
                type="text"
                inputMode="decimal"
                value={whatIfValue ?? ''}
                onChange={(e) => onWhatIf(a.id, e.target.value)}
                className="w-16 rounded border border-purple-300 bg-purple-50 px-2 py-1 text-right text-sm font-semibold focus:outline-none focus:ring-2 focus:ring-purple-400"
                placeholder="—"
                aria-label={`What-if score for ${a.name}`}
              />
              <span className="text-sm text-text-secondary">/{pts}</span>
            </span>
          ) : a.excused ? (
            <span className="text-xs italic text-text-secondary">Excused</span>
          ) : score != null ? (
            <span>
              <span className="text-sm font-semibold text-text-primary">{score}/{pts}</span>
              {pct !== null && (
                <span className={`ms-2 text-xs font-medium ${gradeColor(letterFor(pct, scale) || '-')}`}>{pct}%</span>
              )}
              {a.points_deducted > 0 && (
                <div className="text-xs text-accent-warning">−{a.points_deducted} late penalty</div>
              )}
            </span>
          ) : (
            <span className="text-sm text-text-disabled">–/{pts}</span>
          )}
        </td>
        <td className="px-2 py-3 text-right">
          {hasDetails && (
            <button
              type="button"
              onClick={() => setOpen((v) => !v)}
              aria-expanded={open}
              aria-controls={detailsId}
              className="inline-flex items-center gap-1 rounded px-2 py-1 text-xs text-text-secondary hover:bg-surface-1"
            >
              {open ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
              {a.comments.length > 0 && (
                <span className="inline-flex items-center gap-0.5">
                  <MessageSquare className="h-3 w-3" aria-hidden="true" />
                  {a.comments.length}
                  <span className="sr-only"> comments</span>
                </span>
              )}
              {a.rubric && (
                <span className="inline-flex items-center gap-0.5">
                  <ClipboardList className="h-3 w-3" aria-hidden="true" />
                  <span className="sr-only">rubric</span>
                </span>
              )}
              <span className="sr-only">Show feedback for {a.name}</span>
            </button>
          )}
        </td>
      </tr>
      {open && hasDetails && (
        <tr id={detailsId}>
          <td colSpan={5} className="bg-surface-1 px-6 py-3">
            {a.comments.length > 0 && (
              <ul className="space-y-2">
                {a.comments.map((c) => (
                  <li key={c.id} className="text-sm">
                    <span className="font-medium">{c.author_name || 'Instructor'}</span>
                    <span className="ms-2 text-xs text-text-secondary">{formatDate(c.created_at)}</span>
                    <p className="whitespace-pre-wrap text-text-primary">{c.comment}</p>
                  </li>
                ))}
              </ul>
            )}
            {a.rubric && <RubricView rubric={a.rubric} />}
          </td>
        </tr>
      )}
    </>
  );
}

function rulesText(rules) {
  const parts = [];
  if (rules?.drop_lowest > 0) parts.push(`lowest ${rules.drop_lowest} dropped`);
  if (rules?.drop_highest > 0) parts.push(`highest ${rules.drop_highest} dropped`);
  return parts.join(', ');
}

function QuizScores({ courseId, studentId }) {
  const query = useQuery({
    queryKey: ['learner', 'quizScores', String(courseId), String(studentId)],
    queryFn: async () => {
      const quizzes = ((await api.getQuizzes(courseId, 1, 200))?.data || []).filter((q) => q.published !== false);
      const results = await Promise.allSettled(quizzes.map((q) => api.getQuizSubmissions(courseId, q.id, 1, 50)));
      return quizzes.map((q, i) => {
        const subs = results[i].status === 'fulfilled' ? results[i].value?.data || [] : [];
        const mine = subs
          .filter((s) => String(s.user_id) === String(studentId))
          .sort((x, y) => (y.attempt || 0) - (x.attempt || 0))[0];
        return { quiz: q, sub: mine };
      });
    },
  });
  if (!query.data || query.data.length === 0) return null;
  return (
    <section className="mb-6" aria-labelledby="quiz-scores-heading">
      <div className="mb-2 flex items-center justify-between">
        <h3 id="quiz-scores-heading" className="text-lg font-semibold">Quizzes</h3>
        <span className="text-xs text-text-secondary">Not included in the course total</span>
      </div>
      <div className="overflow-x-auto rounded-lg bg-surface-0 shadow">
        <table className="min-w-full">
          <caption className="sr-only">Quiz scores</caption>
          <thead>
            <tr className="border-b bg-surface-1 text-left text-xs uppercase text-text-secondary">
              <th scope="col" className="px-4 py-3">Quiz</th>
              <th scope="col" className="px-4 py-3">Due</th>
              <th scope="col" className="px-4 py-3 text-right">Score</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border-subtle">
            {query.data.map(({ quiz, sub }) => (
              <tr key={quiz.id}>
                <td className="px-4 py-3 text-sm">
                  <Link
                    className="font-medium text-brand-600 hover:underline"
                    to={sub && sub.workflow_state === 'complete'
                      ? `/courses/${courseId}/quizzes/${quiz.id}/submissions/${sub.id}/review`
                      : `/courses/${courseId}/quizzes/${quiz.id}`}
                  >
                    {quiz.title}
                  </Link>
                </td>
                <td className="px-4 py-3 text-sm text-text-secondary">{formatDate(quiz.due_at)}</td>
                <td className="px-4 py-3 text-right text-sm">
                  {sub?.score != null ? `${sub.score}/${quiz.points_possible ?? 0}` : `–/${quiz.points_possible ?? 0}`}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

const StudentGradesPage = () => {
  const { t } = useTranslation();
  const { courseId } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const observeeId = searchParams.get('observee_id') || undefined;
  const periodParam = searchParams.get('grading_period_id') ?? undefined;
  const query = useGradeSummary(courseId, { studentId: observeeId, gradingPeriodId: periodParam });
  const [whatIf, setWhatIf] = useState(false);
  const [whatIfScores, setWhatIfScores] = useState({});
  const summary = query.data;
  useDocumentTitle(summary ? `Grades: ${summary.course_name}` : t('studentGradesPage.title'));

  const totals = useMemo(() => {
    if (!summary) return null;
    if (!whatIf) {
      return {
        current: summary.totals?.current_score ?? null,
        final: summary.totals?.final_score ?? null,
        earned: summary.totals?.earned ?? 0,
        possible: summary.totals?.possible ?? 0,
        dropped: new Set(summary.assignments.filter((a) => a.dropped).map((a) => a.id)),
      };
    }
    const scores = {};
    Object.entries(whatIfScores).forEach(([id, v]) => {
      scores[id] = v === '' ? null : Number(v);
    });
    return computeTotals(summary, scores);
  }, [summary, whatIf, whatIfScores]);

  const enterWhatIf = () => {
    const initial = {};
    summary.assignments.forEach((a) => {
      if (a.score != null) initial[a.id] = String(a.score);
    });
    setWhatIfScores(initial);
    setWhatIf(true);
  };
  const onWhatIf = (id, value) => {
    if (value !== '' && !/^\d*\.?\d*$/.test(value)) return;
    setWhatIfScores((prev) => ({ ...prev, [id]: value }));
  };
  const selectPeriod = (value) => {
    const next = new URLSearchParams(searchParams);
    next.set('grading_period_id', value);
    setSearchParams(next, { replace: true });
  };

  return (
    <Page query={query} title={t('studentGradesPage.title')}>
      {(s) => {
        const groupsById = new Map(s.assignment_groups.map((g) => [g.id, g]));
        const sections = s.assignment_groups.map((g) => ({
          group: g,
          items: s.assignments.filter((a) => a.assignment_group_id === g.id),
        }));
        const other = s.assignments.filter((a) => !groupsById.has(a.assignment_group_id));
        if (other.length) sections.push({ group: { id: 0, name: sections.length ? 'Other' : 'Assignments', rules: {} }, items: other });
        const showTotals = !s.hide_totals && (s.totals || whatIf);
        const letter = totals?.current != null ? letterFor(totals.current, s.grading_scale) : '';
        return (
          <Layout>
            <CourseNav />
            <div className="mb-6">
              <Link to={`/courses/${courseId}`} className="text-sm text-brand-600 hover:underline">
                &larr; {t('course.backToCourse')}
              </Link>
              <div className="mt-2 flex flex-wrap items-center justify-between gap-3">
                <div>
                  <h1 className="text-2xl font-bold text-text-primary">
                    {observeeId ? `Grades for ${s.student_name || 'student'}: ${s.course_name}` : `My grades: ${s.course_name}`}
                  </h1>
                  {observeeId && <p className="mt-1 text-sm text-text-secondary">Viewing as parent/observer</p>}
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  {s.grading_periods.length > 0 && (
                    <label className="flex items-center gap-2 text-sm">
                      <span>Grading period</span>
                      <select
                        value={s.grading_period_id ?? 0}
                        onChange={(e) => selectPeriod(e.target.value)}
                        className="rounded-md border border-border-default bg-surface-0 px-2 py-1.5 text-sm"
                      >
                        <option value={0}>All grading periods</option>
                        {s.grading_periods.map((p) => (
                          <option key={p.id} value={p.id}>{p.title}{p.is_closed ? ' (closed)' : ''}</option>
                        ))}
                      </select>
                    </label>
                  )}
                  {whatIf && (
                    <button type="button" onClick={() => { setWhatIf(false); setWhatIfScores({}); }}
                      className="inline-flex items-center gap-1.5 rounded-lg bg-surface-2 px-3 py-1.5 text-sm font-medium">
                      <RotateCcw className="h-4 w-4" aria-hidden="true" /> Reset
                    </button>
                  )}
                  {!s.hide_totals && (
                    <button
                      type="button"
                      onClick={whatIf ? () => setWhatIf(false) : enterWhatIf}
                      aria-pressed={whatIf}
                      className={`inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm font-medium ${whatIf ? 'bg-purple-600 text-white' : 'border border-border-default bg-surface-0'}`}
                    >
                      <Calculator className="h-4 w-4" aria-hidden="true" />
                      {whatIf ? 'Exit What-If' : 'What-If grades'}
                    </button>
                  )}
                </div>
              </div>
            </div>

            {whatIf && (
              <div className="mb-6 rounded-lg border border-purple-200 bg-purple-50 px-4 py-3 text-sm text-purple-800" role="status">
                What-If mode: these are hypothetical grades, not your actual grades.
              </div>
            )}

            {showTotals ? (
              <div className="mb-6 rounded-lg bg-surface-0 p-6 shadow">
                <div className="text-sm font-medium uppercase tracking-wide text-text-secondary">
                  {whatIf ? 'Hypothetical grade' : t('studentGradesPage.currentGrade')}
                </div>
                <div className="mt-1 flex items-baseline gap-3">
                  {letter && <span className={`text-4xl font-bold ${gradeColor(letter)}`}>{letter}</span>}
                  <span className="text-2xl text-text-secondary">{totals?.current != null ? `${totals.current}%` : '—'}</span>
                </div>
                <div className="mt-1 text-sm text-text-secondary">
                  {totals?.earned ?? 0} / {totals?.possible ?? 0} points on graded work
                  {totals?.final != null && <> · {totals.final}% counting ungraded work as zero</>}
                </div>
              </div>
            ) : s.hide_totals ? (
              <p className="mb-6 rounded-lg bg-surface-1 px-4 py-3 text-sm text-text-secondary">
                Your instructor has chosen not to show course totals.
              </p>
            ) : (
              <p className="mb-6 rounded-lg bg-surface-1 px-4 py-3 text-sm text-text-secondary">
                Select a grading period to see its total.
              </p>
            )}

            {sections.filter((sec) => sec.items.length > 0).map(({ group, items }) => (
              <section key={group.id} className="mb-6" aria-labelledby={`grp-${group.id}`}>
                <div className="mb-2 flex flex-wrap items-baseline justify-between gap-2">
                  <h2 id={`grp-${group.id}`} className="text-lg font-semibold">{group.name}</h2>
                  <span className="text-sm text-text-secondary">
                    {[s.apply_assignment_group_weights && group.group_weight ? `${group.group_weight}% of total` : '',
                      rulesText(group.rules),
                      !whatIf && group.score != null ? `${group.score}%` : '']
                      .filter(Boolean).join(' · ')}
                  </span>
                </div>
                <div className="overflow-x-auto rounded-lg bg-surface-0 shadow">
                  <table className="min-w-full">
                    <caption className="sr-only">{group.name} grades</caption>
                    <thead>
                      <tr className="border-b bg-surface-1 text-left text-xs uppercase text-text-secondary">
                        <th scope="col" className="px-4 py-3">Assignment</th>
                        <th scope="col" className="px-4 py-3">Due</th>
                        <th scope="col" className="px-4 py-3 text-center">Status</th>
                        <th scope="col" className="px-4 py-3 text-right">Score</th>
                        <th scope="col" className="px-2 py-3"><span className="sr-only">Feedback</span></th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-border-subtle">
                      {items.map((a) => (
                        <AssignmentRow
                          key={a.id}
                          a={a}
                          courseId={courseId}
                          whatIf={whatIf}
                          whatIfValue={whatIfScores[a.id]}
                          onWhatIf={onWhatIf}
                          scale={s.grading_scale}
                          dropped={totals?.dropped?.has(a.id)}
                        />
                      ))}
                    </tbody>
                  </table>
                </div>
              </section>
            ))}

            {s.assignments.length === 0 && (
              <div className="rounded-lg bg-surface-0 p-8 text-center text-text-secondary shadow">
                {t('studentGradesPage.noGrades')}
              </div>
            )}

            <QuizScores courseId={courseId} studentId={observeeId || s.student_id} />
          </Layout>
        );
      }}
    </Page>
  );
};

export default StudentGradesPage;
