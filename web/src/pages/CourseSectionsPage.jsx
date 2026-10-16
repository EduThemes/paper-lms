import React, { useMemo, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Layers, Plus, Pencil, Trash2, Shuffle, Undo2 } from 'lucide-react';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import Page from '../components/Page';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { useCourseSections, useRoster, useSectionMutation, useMyCourses } from '../hooks/useAdminEnrollments';
import { crosslistTargets, fromDateInput, sectionPeopleCounts, toDateInput } from '../utils/enrollmentLifecycle';

const fieldClass = 'w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500';

const fmtDate = (iso) => (iso ? new Date(iso).toLocaleDateString() : null);

/**
 * Course sections (G109): create, rename, set participation dates,
 * delete empty sections, and cross-list a section into another course or
 * move it back (G108). Teachers and admins only (the API enforces it).
 */
export default function CourseSectionsPage() {
  const { courseId } = useParams();
  const sectionsQuery = useCourseSections(courseId);
  // Every live enrollment, to show how many people each section holds.
  const roster = useRoster(courseId, []);
  const counts = useMemo(() => sectionPeopleCounts(roster.data || []), [roster.data]);

  return (
    <Page query={sectionsQuery} title="Sections">
      {(sections) => <SectionsView courseId={courseId} sections={sections || []} counts={counts} />}
    </Page>
  );
}

function SectionsView({ courseId, sections, counts }) {
  const [editing, setEditing] = useState(null); // section or {} for new
  const [crosslisting, setCrosslisting] = useState(null);
  const [error, setError] = useState(null);
  const [notice, setNotice] = useState(null);

  const mutation = useSectionMutation(courseId, {
    onSuccess: (_d, vars) => {
      setError(null);
      setNotice({
        create: 'Section created.',
        update: 'Section saved.',
        delete: 'Section deleted.',
        crosslist: 'Section cross-listed. Its people moved with it.',
        uncrosslist: 'Section moved back to its original course.',
      }[vars.kind]);
      setEditing(null);
      setCrosslisting(null);
    },
    onError: (err) => setError(err.message),
  });

  const remove = (section) => {
    // eslint-disable-next-line no-alert
    if (!window.confirm(`Delete the section "${section.name}"?`)) return;
    mutation.mutate({ kind: 'delete', sectionId: section.id });
  };
  const uncrosslist = (section) => {
    // eslint-disable-next-line no-alert
    if (!window.confirm(`Move "${section.name}" and its people back to its original course? Grades stay in this course.`)) return;
    mutation.mutate({ kind: 'uncrosslist', sectionId: section.id });
  };

  return (
    <Layout>
      <CourseNav />
      <div className="mb-6">
        <Link to={`/courses/${courseId}/people`} className="text-brand-600 hover:underline text-sm">&larr; Back to People</Link>
        <div className="flex flex-wrap items-center justify-between gap-2 mt-2">
          <h2 className="text-2xl font-bold text-text-primary flex items-center gap-2">
            <Layers className="w-6 h-6" />
            Sections
          </h2>
          <button
            type="button"
            onClick={() => setEditing({})}
            className="inline-flex items-center gap-2 bg-brand-600 text-on-brand px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium"
          >
            <Plus className="w-4 h-4" />
            Add Section
          </button>
        </div>
        <p className="text-sm text-text-tertiary mt-1">
          People without a section are in the course&apos;s default section ({counts[0] || 0}).
        </p>
      </div>

      {error && (
        <div role="alert" className="bg-accent-danger/10 border border-accent-danger/30 text-accent-danger rounded-md p-3 mb-4 text-sm">{error}</div>
      )}
      {notice && !error && (
        <div role="status" className="bg-accent-success/10 border border-accent-success/30 text-accent-success rounded-md p-3 mb-4 text-sm">{notice}</div>
      )}

      <div className="bg-surface-0 rounded-lg shadow overflow-hidden">
        {sections.length === 0 ? (
          <div className="p-6 text-center text-text-tertiary">No sections yet.</div>
        ) : (
          <ul className="divide-y">
            {sections.map((s) => (
              <li key={s.id} className="flex flex-wrap items-center gap-3 px-4 py-3">
                <div className="flex-1 min-w-[12rem]">
                  <div className="text-sm font-medium text-text-primary">{s.name}</div>
                  <div className="text-xs text-text-tertiary">
                    {counts[s.id] || 0} {(counts[s.id] || 0) === 1 ? 'person' : 'people'}
                    {(s.start_at || s.end_at) && (
                      <> · {fmtDate(s.start_at) || '…'} – {fmtDate(s.end_at) || '…'}</>
                    )}
                    {s.restrict_enrollments_to_section_dates && <> · participation limited to these dates</>}
                    {s.nonxlist_course_id && <> · cross-listed from course {s.nonxlist_course_id}</>}
                  </div>
                </div>
                <div className="flex items-center gap-1">
                  <button type="button" aria-label={`Edit ${s.name}`} onClick={() => setEditing(s)} className="p-2 rounded-md hover:bg-surface-2"><Pencil className="w-4 h-4" /></button>
                  {s.nonxlist_course_id ? (
                    <button type="button" aria-label={`De-cross-list ${s.name}`} onClick={() => uncrosslist(s)} className="p-2 rounded-md hover:bg-surface-2"><Undo2 className="w-4 h-4" /></button>
                  ) : (
                    <button type="button" aria-label={`Cross-list ${s.name}`} onClick={() => setCrosslisting(s)} className="p-2 rounded-md hover:bg-surface-2"><Shuffle className="w-4 h-4" /></button>
                  )}
                  <button
                    type="button"
                    aria-label={`Delete ${s.name}`}
                    onClick={() => remove(s)}
                    disabled={(counts[s.id] || 0) > 0}
                    title={(counts[s.id] || 0) > 0 ? 'Move or remove its people first' : undefined}
                    className="p-2 rounded-md hover:bg-surface-2 text-accent-danger disabled:opacity-40"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>

      {editing && (
        <SectionDialog
          section={editing}
          pending={mutation.isPending}
          onClose={() => setEditing(null)}
          onSave={(payload) => (editing.id
            ? mutation.mutate({ kind: 'update', sectionId: editing.id, section: payload })
            : mutation.mutate({ kind: 'create', section: payload }))}
        />
      )}
      {crosslisting && (
        <CrosslistDialog
          section={crosslisting}
          pending={mutation.isPending}
          onClose={() => setCrosslisting(null)}
          onConfirm={(targetCourseId) => mutation.mutate({ kind: 'crosslist', sectionId: crosslisting.id, targetCourseId })}
        />
      )}
    </Layout>
  );
}

function SectionDialog({ section, pending, onClose, onSave }) {
  const [name, setName] = useState(section.name || '');
  const [start, setStart] = useState(toDateInput(section.start_at));
  const [end, setEnd] = useState(toDateInput(section.end_at));
  const [restrict, setRestrict] = useState(Boolean(section.restrict_enrollments_to_section_dates));
  const [error, setError] = useState(null);

  const submit = (e) => {
    e.preventDefault();
    if (!name.trim()) {
      setError('A section needs a name.');
      return;
    }
    if (start && end && end < start) {
      setError('The end date must be after the start date.');
      return;
    }
    onSave({
      name: name.trim(),
      start_at: fromDateInput(start),
      end_at: fromDateInput(end),
      restrict_enrollments_to_section_dates: restrict,
    });
  };

  return (
    <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent>
        <form onSubmit={submit} className="space-y-4">
          <DialogHeader>
            <DialogTitle>{section.id ? 'Edit section' : 'New section'}</DialogTitle>
          </DialogHeader>
          <div>
            <label htmlFor="section-name" className="block text-sm font-medium text-text-secondary mb-1">Name</label>
            <input id="section-name" className={fieldClass} value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label htmlFor="section-start" className="block text-sm font-medium text-text-secondary mb-1">Starts</label>
              <input id="section-start" type="date" className={fieldClass} value={start} onChange={(e) => setStart(e.target.value)} />
            </div>
            <div>
              <label htmlFor="section-end" className="block text-sm font-medium text-text-secondary mb-1">Ends</label>
              <input id="section-end" type="date" className={fieldClass} value={end} onChange={(e) => setEnd(e.target.value)} />
            </div>
          </div>
          <label className="inline-flex items-center gap-2 text-sm text-text-secondary">
            <input type="checkbox" checked={restrict} onChange={(e) => setRestrict(e.target.checked)} />
            People in this section can only participate between these dates
          </label>
          {error && <p role="alert" className="text-sm text-accent-danger">{error}</p>}
          <DialogFooter>
            <button type="button" onClick={onClose} className="px-4 py-2 rounded-md text-sm border border-border-strong">Cancel</button>
            <button type="submit" disabled={pending} className="px-4 py-2 rounded-md text-sm bg-brand-600 text-on-brand hover:bg-brand-700 disabled:opacity-50">
              {pending ? 'Saving...' : 'Save'}
            </button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function CrosslistDialog({ section, pending, onClose, onConfirm }) {
  const coursesQuery = useMyCourses();
  const targets = crosslistTargets(coursesQuery.data || [], section);
  const [target, setTarget] = useState('');

  return (
    <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent>
        <form
          onSubmit={(e) => { e.preventDefault(); if (target) onConfirm(Number(target)); }}
          className="space-y-4"
        >
          <DialogHeader>
            <DialogTitle>Cross-list {section.name}</DialogTitle>
            <DialogDescription>
              The section and everyone in it move into the course you pick. Their submissions and grades stay with
              this course&apos;s assignments. You can move the section back later.
            </DialogDescription>
          </DialogHeader>
          <div>
            <label htmlFor="crosslist-target" className="block text-sm font-medium text-text-secondary mb-1">Course</label>
            <select id="crosslist-target" className={fieldClass} value={target} onChange={(e) => setTarget(e.target.value)} disabled={coursesQuery.isLoading}>
              <option value="">{coursesQuery.isLoading ? 'Loading courses…' : 'Choose a course'}</option>
              {targets.map((c) => <option key={c.id} value={String(c.id)}>{c.name}{c.course_code ? ` (${c.course_code})` : ''}</option>)}
            </select>
            {coursesQuery.isError && <p className="text-sm text-accent-danger mt-1">Could not load your courses.</p>}
          </div>
          <DialogFooter>
            <button type="button" onClick={onClose} className="px-4 py-2 rounded-md text-sm border border-border-strong">Cancel</button>
            <button type="submit" disabled={pending || !target} className="px-4 py-2 rounded-md text-sm bg-brand-600 text-on-brand hover:bg-brand-700 disabled:opacity-50">
              {pending ? 'Moving...' : 'Cross-list'}
            </button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
