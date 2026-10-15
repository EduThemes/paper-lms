import React, { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import RichContentEditorV2 from '../rce/RichContentEditorV2';
import { clearRceDraft } from '../rce/rceDrafts';
import useCrossCourseCheck from '../../hooks/useCrossCourseCheck';
import CrossCourseWarningDialog from '../CrossCourseWarningDialog';
import { discussionsApi } from '../../services/api_discussions';
import {
  emptyTopicForm,
  topicToForm,
  formToPayload,
  topicOptionLocks,
  validateTopicForm,
} from '../../utils/discussionTopicForm';

const inputClass =
  'w-full border border-border-strong rounded-md px-3 py-2 text-sm bg-surface-0 focus:outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-50';

function Field({ label, htmlFor, error, hint, children }) {
  return (
    <div>
      <label htmlFor={htmlFor} className="block text-sm font-medium text-text-secondary mb-1">{label}</label>
      {children}
      {hint && !error && <p className="mt-1 text-xs text-text-tertiary">{hint}</p>}
      {error && <p className="mt-1 text-xs text-accent-danger" role="alert">{error}</p>}
    </div>
  );
}

function Check({ id, label, checked, onChange, disabled, hint }) {
  return (
    <div className="flex items-start gap-2">
      <input
        id={id}
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={(e) => onChange(e.target.checked)}
        className="mt-0.5 rounded border-border-strong"
      />
      <label htmlFor={id} className={`text-sm ${disabled ? 'text-text-disabled' : 'text-text-secondary'}`}>
        {label}
        {hint && <span className="block text-xs text-text-tertiary">{hint}</span>}
      </label>
    </div>
  );
}

/**
 * Create / edit form for a discussion topic with the Canvas options:
 * availability (available from, lock date, closed for comments, post
 * before seeing replies), grading, group set, anonymity and sections.
 *
 * Props: courseId, topicId (edit) or none (create), onSaved(topic),
 * onCancel().
 */
export default function DiscussionTopicForm({ courseId, topicId, onSaved, onCancel }) {
  const editing = !!topicId;
  const topicQuery = useQuery({
    queryKey: ['discussions', String(courseId), 'topic', String(topicId)],
    queryFn: () => discussionsApi.getTopic(courseId, topicId),
    enabled: editing,
  });
  const sectionsQuery = useQuery({
    queryKey: ['discussions', String(courseId), 'sections'],
    queryFn: () => discussionsApi.sections(courseId),
  });
  const groupSetsQuery = useQuery({
    queryKey: ['discussions', String(courseId), 'groupSets'],
    queryFn: () => discussionsApi.groupCategories(courseId),
  });
  const assignmentGroupsQuery = useQuery({
    queryKey: ['discussions', String(courseId), 'assignmentGroups'],
    queryFn: () => discussionsApi.assignmentGroups(courseId),
  });

  const [form, setForm] = useState(emptyTopicForm());
  const [loadedFor, setLoadedFor] = useState(null);
  const [errors, setErrors] = useState({});
  const [saveError, setSaveError] = useState(null);
  const [saving, setSaving] = useState(false);
  const { issues, checkAndSave, dismiss, confirm } = useCrossCourseCheck(courseId);

  useEffect(() => {
    if (editing && topicQuery.data && loadedFor !== topicId) {
      setForm(topicToForm(topicQuery.data));
      setLoadedFor(topicId);
    }
  }, [editing, topicQuery.data, topicId, loadedFor]);

  const set = (patch) => setForm((prev) => ({ ...prev, ...patch }));
  const locks = topicOptionLocks(form);
  const draftKey = `discussion-${courseId}-${topicId || 'new'}-message`;
  const sections = Array.isArray(sectionsQuery.data) ? sectionsQuery.data : [];
  const groupSets = Array.isArray(groupSetsQuery.data) ? groupSetsQuery.data : [];
  const assignmentGroups = Array.isArray(assignmentGroupsQuery.data) ? assignmentGroupsQuery.data : [];
  const original = editing ? topicQuery.data : undefined;
  // The group set and anonymity are fixed once people have posted (the
  // server refuses the change); keep them read-only in the form too.
  const hasPosts = !!original?.has_posts;

  const doSave = async () => {
    setSaving(true);
    setSaveError(null);
    try {
      const payload = formToPayload(form, original);
      const saved = editing
        ? await discussionsApi.updateTopic(courseId, topicId, payload)
        : await discussionsApi.createTopic(courseId, payload);
      clearRceDraft(draftKey);
      onSaved?.(saved);
    } catch (err) {
      setSaveError(err.message || 'Could not save the discussion');
    } finally {
      setSaving(false);
    }
  };

  const handleSubmit = (e) => {
    e.preventDefault();
    const found = validateTopicForm(form);
    setErrors(found);
    if (Object.keys(found).length > 0) return;
    checkAndSave(form.message, doSave);
  };

  if (editing && topicQuery.isLoading) {
    return <div className="bg-surface-0 rounded-lg shadow p-6 mb-6 text-sm text-text-tertiary">Loading…</div>;
  }

  return (
    <div className="bg-surface-0 rounded-lg shadow p-6 mb-6">
      <h3 className="font-semibold mb-4">{editing ? 'Edit Discussion' : 'New Discussion'}</h3>
      <form onSubmit={handleSubmit} className="space-y-5" noValidate>
        <Field label="Title" htmlFor="dt-title" error={errors.title}>
          <input
            id="dt-title"
            type="text"
            value={form.title}
            onChange={(e) => set({ title: e.target.value })}
            className={inputClass}
            required
          />
        </Field>

        <div>
          <span className="block text-sm font-medium text-text-secondary mb-1">Message</span>
          <RichContentEditorV2
            value={form.message}
            onChange={(html) => setForm((prev) => ({ ...prev, message: html }))}
            placeholder="Discussion topic content..."
            courseId={courseId}
            autoSaveKey={draftKey}
          />
        </div>

        <fieldset className="grid gap-4 sm:grid-cols-2">
          <legend className="text-sm font-semibold text-text-primary mb-2">Availability</legend>
          <Field label="Available from" htmlFor="dt-delayed" hint="Leave empty to post now.">
            <input
              id="dt-delayed"
              type="datetime-local"
              value={form.delayed_post_at}
              onChange={(e) => set({ delayed_post_at: e.target.value })}
              className={inputClass}
            />
          </Field>
          <Field label="Lock after" htmlFor="dt-lock" error={errors.lock_at} hint="Students can't post after this date.">
            <input
              id="dt-lock"
              type="datetime-local"
              value={form.lock_at}
              onChange={(e) => set({ lock_at: e.target.value })}
              className={inputClass}
            />
          </Field>
        </fieldset>

        <fieldset className="space-y-2">
          <legend className="text-sm font-semibold text-text-primary mb-2">Options</legend>
          <div className="flex flex-wrap gap-4 items-end">
            <Field label="Discussion type" htmlFor="dt-type">
              <select
                id="dt-type"
                value={form.discussion_type}
                onChange={(e) => set({ discussion_type: e.target.value })}
                className={inputClass}
              >
                <option value="threaded">Threaded replies</option>
                <option value="side_comment">Side comments</option>
              </select>
            </Field>
          </div>
          <Check id="dt-published" label="Published" checked={form.published} onChange={(v) => set({ published: v })} />
          <Check id="dt-pinned" label="Pinned to the top of the list" checked={form.pinned} onChange={(v) => set({ pinned: v })} />
          <Check
            id="dt-locked"
            label="Closed for comments"
            hint="Students can read but not post."
            checked={form.locked}
            onChange={(v) => set({ locked: v })}
          />
          <Check
            id="dt-initial"
            label="Participants must post before seeing replies"
            checked={form.require_initial_post}
            onChange={(v) => set({ require_initial_post: v })}
          />
          <Check id="dt-rating" label="Allow liking" checked={form.allow_rating} onChange={(v) => set({ allow_rating: v })} />
          {form.allow_rating && (
            <div className="ml-6">
              <Check
                id="dt-graders-rate"
                label="Only graders can like"
                checked={form.only_graders_can_rate}
                onChange={(v) => set({ only_graders_can_rate: v })}
              />
            </div>
          )}
        </fieldset>

        <fieldset className="space-y-3">
          <legend className="text-sm font-semibold text-text-primary mb-2">Grading</legend>
          <Check
            id="dt-graded"
            label="Graded"
            hint={locks.graded ? 'Anonymous and section-specific discussions can\'t be graded.' : 'Posting submits the assignment; grade posts in SpeedGrader.'}
            checked={form.graded}
            disabled={locks.graded && !form.graded}
            onChange={(v) => set({ graded: v })}
          />
          {errors.graded && <p className="text-xs text-accent-danger" role="alert">{errors.graded}</p>}
          {form.graded && (
            <div className="grid gap-4 sm:grid-cols-3 ml-6">
              <Field label="Points" htmlFor="dt-points" error={errors.points_possible}>
                <input
                  id="dt-points"
                  type="number"
                  min="0"
                  step="any"
                  value={form.points_possible}
                  onChange={(e) => set({ points_possible: e.target.value })}
                  className={inputClass}
                />
              </Field>
              <Field label="Due" htmlFor="dt-due">
                <input
                  id="dt-due"
                  type="datetime-local"
                  value={form.due_at}
                  onChange={(e) => set({ due_at: e.target.value })}
                  className={inputClass}
                />
              </Field>
              <Field label="Assignment group" htmlFor="dt-agroup">
                <select
                  id="dt-agroup"
                  value={form.assignment_group_id}
                  onChange={(e) => set({ assignment_group_id: e.target.value })}
                  className={inputClass}
                >
                  <option value="">Default</option>
                  {assignmentGroups.map((g) => (
                    <option key={g.id} value={String(g.id)}>{g.name}</option>
                  ))}
                </select>
              </Field>
            </div>
          )}
        </fieldset>

        <fieldset className="space-y-3">
          <legend className="text-sm font-semibold text-text-primary mb-2">Audience</legend>
          <Field
            label="Group discussion"
            htmlFor="dt-groupset"
            error={errors.group_category_id}
            hint={hasPosts ? 'The group set can\'t change after people have posted.' : 'Each group gets its own copy of the topic.'}
          >
            <select
              id="dt-groupset"
              value={form.group_category_id}
              disabled={(locks.group && !form.group_category_id) || hasPosts}
              onChange={(e) => set({ group_category_id: e.target.value })}
              className={inputClass}
            >
              <option value="">Not a group discussion</option>
              {groupSets.map((g) => (
                <option key={g.id} value={String(g.id)}>{g.name}</option>
              ))}
            </select>
          </Field>

          <Field
            label="Anonymity"
            htmlFor="dt-anon"
            hint={hasPosts ? 'Anonymity can\'t change after people have posted.' : 'Anonymous discussions can\'t be graded, grouped or limited to sections.'}
          >
            <select
              id="dt-anon"
              value={form.anonymous_state}
              disabled={(locks.anonymous && !form.anonymous_state) || hasPosts}
              onChange={(e) => set({ anonymous_state: e.target.value })}
              className={inputClass}
            >
              <option value="">Names shown</option>
              <option value="partial_anonymity">Partial: each person chooses</option>
              <option value="full_anonymity">Full: everyone is anonymous</option>
            </select>
          </Field>

          <Check
            id="dt-sections"
            label="Only some sections"
            hint={locks.sections ? 'Graded, group and anonymous discussions are for the whole course.' : undefined}
            checked={form.is_section_specific}
            disabled={locks.sections && !form.is_section_specific}
            onChange={(v) => set({ is_section_specific: v })}
          />
          {form.is_section_specific && (
            <div className="ml-6 space-y-1" role="group" aria-label="Sections">
              {sections.length === 0 && <p className="text-xs text-text-tertiary">This course has no sections.</p>}
              {sections.map((s) => (
                <Check
                  key={s.id}
                  id={`dt-section-${s.id}`}
                  label={s.name}
                  checked={form.section_ids.includes(Number(s.id))}
                  onChange={(v) =>
                    set({
                      section_ids: v
                        ? [...form.section_ids, Number(s.id)]
                        : form.section_ids.filter((id) => id !== Number(s.id)),
                    })
                  }
                />
              ))}
              {errors.section_ids && <p className="text-xs text-accent-danger" role="alert">{errors.section_ids}</p>}
            </div>
          )}
        </fieldset>

        {saveError && <p className="text-sm text-accent-danger" role="alert">{saveError}</p>}

        <div className="flex justify-end gap-3">
          <button type="button" onClick={onCancel} className="px-4 py-2 text-sm text-text-secondary hover:text-text-primary">
            Cancel
          </button>
          <button
            type="submit"
            disabled={saving}
            className="bg-brand-600 text-on-brand px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50"
          >
            {saving ? 'Saving...' : editing ? 'Save' : 'Create Discussion'}
          </button>
        </div>
      </form>
      <CrossCourseWarningDialog issues={issues} onGoBack={dismiss} onSaveAnyway={confirm} />
    </div>
  );
}
