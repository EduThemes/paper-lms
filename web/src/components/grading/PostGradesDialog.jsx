import React, { useEffect, useState } from 'react';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '../ui/dialog';
import { gradingPolicyApi } from '../../services/api_grading_policy';

// PostGradesDialog posts or hides an assignment's grades (G018): post to
// everyone or only graded students, optionally for some sections only.
// mode is 'post' or 'hide'.
const PostGradesDialog = ({ open, onOpenChange, courseId, assignment, mode = 'post', onDone }) => {
  const [gradedOnly, setGradedOnly] = useState(false);
  const [sections, setSections] = useState([]);
  const [bySection, setBySection] = useState(false);
  const [selected, setSelected] = useState([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    if (!open) return;
    setGradedOnly(false);
    setBySection(false);
    setSelected([]);
    setError(null);
    gradingPolicyApi
      .getSections(courseId)
      .then((data) => setSections(Array.isArray(data) ? data : data?.sections || []))
      .catch(() => setSections([]));
  }, [open, courseId]);

  const toggle = (id) =>
    setSelected((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));

  const submit = async () => {
    if (!assignment) return;
    if (bySection && selected.length === 0) {
      setError('Choose at least one section.');
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const sectionIds = bySection ? selected : [];
      if (mode === 'post') {
        await gradingPolicyApi.postGrades(courseId, assignment.id, { gradedOnly, sectionIds });
      } else {
        await gradingPolicyApi.hideGrades(courseId, assignment.id, { sectionIds });
      }
      onDone?.(assignment.id, mode, sectionIds);
      onOpenChange(false);
    } catch (e) {
      setError(e.message || 'Could not update grade visibility.');
    } finally {
      setBusy(false);
    }
  };

  const title = mode === 'post' ? 'Post grades' : 'Hide grades';
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>
            {mode === 'post'
              ? 'Students will see their grades and comments for'
              : 'Students will no longer see their grades for'}{' '}
            {assignment ? `"${assignment.name}"` : 'this assignment'}.
            {mode === 'hide' &&
              ' The assignment switches to manual posting: grades and comments added later stay hidden until you post them.'}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 text-sm">
          {mode === 'post' && (
            <fieldset className="space-y-2">
              <legend className="font-medium text-text-secondary">Post to</legend>
              <label className="flex items-start gap-2">
                <input type="radio" name="post-to" checked={!gradedOnly} onChange={() => setGradedOnly(false)} />
                <span>
                  Everyone
                  <span className="block text-xs text-text-tertiary">Grades entered later show to students right away.</span>
                </span>
              </label>
              <label className="flex items-start gap-2">
                <input type="radio" name="post-to" checked={gradedOnly} onChange={() => setGradedOnly(true)} />
                <span>
                  Graded
                  <span className="block text-xs text-text-tertiary">Only students with a grade or an excusal.</span>
                </span>
              </label>
            </fieldset>
          )}
          {sections.length > 0 && (
            <div className="space-y-2">
              <label className="flex items-center gap-2 font-medium text-text-secondary">
                <input type="checkbox" checked={bySection} onChange={(e) => setBySection(e.target.checked)} />
                Specific sections
              </label>
              {bySection && (
                <div className="ml-6 space-y-1 max-h-40 overflow-y-auto">
                  {sections.map((s) => (
                    <label key={s.id} className="flex items-center gap-2">
                      <input type="checkbox" checked={selected.includes(s.id)} onChange={() => toggle(s.id)} />
                      {s.name}
                    </label>
                  ))}
                </div>
              )}
            </div>
          )}
          {error && (
            <p role="alert" className="text-accent-danger">
              {error}
            </p>
          )}
        </div>
        <DialogFooter>
          <button
            type="button"
            onClick={() => onOpenChange(false)}
            disabled={busy}
            className="px-4 py-2 text-sm border border-border-strong rounded-md hover:bg-surface-1"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={submit}
            disabled={busy}
            className="px-4 py-2 text-sm bg-brand-600 text-white rounded-md hover:bg-brand-700 disabled:opacity-50"
          >
            {busy ? 'Saving…' : mode === 'post' ? 'Post' : 'Hide'}
          </button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

export default PostGradesDialog;
