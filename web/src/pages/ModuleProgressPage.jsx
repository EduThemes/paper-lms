import { useState } from 'react';
import { useParams, Link, Navigate } from 'react-router-dom';
import { CheckCircle2, Lock, Circle, CircleDot, X } from 'lucide-react';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import Page from '../components/Page';
import useIsTeacher from '../hooks/useIsTeacher';
import { useModuleProgress, useModuleProgressions } from '../services/api_modules';
import { progressStateLabel, requirementLabel } from '../utils/moduleProgress';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

const STATE_ICON = {
  completed: { Icon: CheckCircle2, className: 'text-accent-success' },
  started: { Icon: CircleDot, className: 'text-brand-600' },
  unlocked: { Icon: Circle, className: 'text-text-tertiary' },
  locked: { Icon: Lock, className: 'text-text-tertiary' },
};

function StateCell({ state }) {
  const cfg = STATE_ICON[state];
  const label = progressStateLabel(state);
  if (!cfg) return <span className="text-text-tertiary">{label}</span>;
  const { Icon, className } = cfg;
  return (
    <span className={cn('inline-flex items-center gap-1 text-xs', className)} title={label}>
      <Icon className="w-4 h-4" aria-hidden />
      <span>{label}</span>
    </span>
  );
}

// StudentDetail shows one student's per-item progress (the teacher's
// drill-down into the grid).
function StudentDetail({ courseId, student, onClose }) {
  const query = useModuleProgress(courseId, { userId: student.user_id });
  return (
    <section className="bg-surface-0 rounded-lg shadow p-4 mt-6" aria-label={`Progress for ${student.name}`}>
      <div className="flex items-center justify-between mb-3">
        <h3 className="font-semibold text-text-primary">{student.name}</h3>
        <Button variant="ghost" size="icon" onClick={onClose} aria-label="Close student progress">
          <X className="w-4 h-4" />
        </Button>
      </div>
      {query.isLoading && <p className="text-sm text-text-tertiary">Loading…</p>}
      {query.isError && <p className="text-sm text-accent-danger">{query.error?.message}</p>}
      {query.data && (
        <div className="space-y-4">
          {query.data.modules.map((m) => (
            <div key={m.module_id}>
              <div className="flex items-center gap-3">
                <span className="font-medium text-sm text-text-primary">{m.name}</span>
                <StateCell state={m.state} />
              </div>
              {m.lock_explanation && <p className="text-xs text-text-tertiary mt-1">{m.lock_explanation}</p>}
              <ul className="mt-1 ml-4 space-y-0.5">
                {m.items.filter((it) => it.requirement_type).map((it) => (
                  <li key={it.item_id} className="text-xs text-text-secondary flex items-center gap-2">
                    {it.completed
                      ? <CheckCircle2 className="w-3.5 h-3.5 text-accent-success" aria-label="Completed" />
                      : <Circle className="w-3.5 h-3.5 text-text-tertiary" aria-label="Not completed" />}
                    <span>{it.title || `Item #${it.item_id}`}: {requirementLabel(it.requirement_type, it.min_score)}</span>
                    {it.score !== undefined && it.score !== null && <span>(score {it.score})</span>}
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

export function ModuleProgressGrid({ data, onSelect }) {
  const modules = data?.modules || [];
  const students = data?.students || [];
  if (students.length === 0) {
    return <p className="text-sm text-text-tertiary">No active students in this course yet.</p>;
  }
  return (
    <div className="bg-surface-0 rounded-lg shadow overflow-x-auto">
      <table className="min-w-full text-sm">
        <thead>
          <tr className="border-b bg-surface-1">
            <th scope="col" className="text-left px-4 py-2 font-medium text-text-secondary">Student</th>
            {modules.map((m) => (
              <th key={m.id} scope="col" className="text-left px-4 py-2 font-medium text-text-secondary whitespace-nowrap">{m.name}</th>
            ))}
            <th scope="col" className="text-left px-4 py-2 font-medium text-text-secondary">Completed</th>
          </tr>
        </thead>
        <tbody className="divide-y">
          {students.map((s) => (
            <tr key={s.user_id} className="hover:bg-surface-1">
              <th scope="row" className="text-left px-4 py-2 font-normal">
                <button type="button" className="text-brand-600 hover:underline" onClick={() => onSelect?.(s)}>
                  {s.name}
                </button>
              </th>
              {modules.map((m) => (
                <td key={m.id} className="px-4 py-2"><StateCell state={s.states?.[m.id]} /></td>
              ))}
              <td className="px-4 py-2 text-text-secondary">{s.completed} / {modules.length}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ModuleProgressPage is the teacher's "Student progress" view (Canvas
// /courses/:id/modules/progressions): every student's state on every
// module, with a per-student drill-down.
export default function ModuleProgressPage() {
  const { courseId } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const query = useModuleProgressions(courseId);
  const [selected, setSelected] = useState(null);

  if (isTeacher === false) return <Navigate to={`/courses/${courseId}/modules`} replace />;

  return (
    <Page query={query} title="Student progress">
      {(data) => (
        <Layout>
          <CourseNav />
          <div className="mb-6">
            <Link to={`/courses/${courseId}/modules`} className="text-brand-600 hover:underline text-sm">
              &larr; Back to Modules
            </Link>
            <h2 className="text-2xl font-bold text-text-primary mt-2">Student progress</h2>
          </div>
          <ModuleProgressGrid data={data} onSelect={setSelected} />
          {selected && (
            <StudentDetail courseId={courseId} student={selected} onClose={() => setSelected(null)} />
          )}
        </Layout>
      )}
    </Page>
  );
}
