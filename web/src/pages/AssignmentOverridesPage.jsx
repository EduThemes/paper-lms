import React, { useState, useEffect, useMemo } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Plus, Trash2, Users, Calendar, X, Pencil } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { api } from '../services/api';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';

const EMPTY = { title: '', due_at: '', unlock_at: '', lock_at: '', course_section_id: '', student_ids: [] };

// toLocalInput renders an ISO date for a datetime-local input.
const toLocalInput = (iso) => {
  if (!iso) return '';
  const d = new Date(iso);
  const pad = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
};

// StudentPicker chooses course students by name (G019: no raw ids).
const StudentPicker = ({ students, value, onChange }) => {
  const [query, setQuery] = useState('');
  const chosen = students.filter((s) => value.includes(s.id));
  const matches = query.trim()
    ? students.filter((s) => !value.includes(s.id) && s.name.toLowerCase().includes(query.trim().toLowerCase())).slice(0, 8)
    : [];
  return (
    <div>
      <div className="flex flex-wrap gap-1 mb-1">
        {chosen.map((s) => (
          <span key={s.id} className="inline-flex items-center gap-1 rounded-full bg-surface-2 px-2 py-0.5 text-xs">
            {s.name}
            <button type="button" aria-label={`Remove ${s.name}`} onClick={() => onChange(value.filter((id) => id !== s.id))}>
              <X className="w-3 h-3" />
            </button>
          </span>
        ))}
      </div>
      <input
        type="text"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="Search students by name"
        aria-label="Search students"
        className="w-full border border-border-strong rounded px-3 py-2 text-sm"
      />
      {matches.length > 0 && (
        <ul className="mt-1 border border-border-default rounded bg-surface-0 shadow-sm max-h-48 overflow-y-auto" role="listbox">
          {matches.map((s) => (
            <li key={s.id}>
              <button
                type="button"
                role="option"
                aria-selected="false"
                onClick={() => { onChange([...value, s.id]); setQuery(''); }}
                className="w-full text-left px-3 py-1.5 text-sm hover:bg-surface-1"
              >
                {s.name}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
};

// AssignmentOverridesPage manages "Assign to": per-section and per-student
// dates, and whether the assignment is only for the people listed here.
const AssignmentOverridesPage = () => {
  const { t } = useTranslation();
  const { courseId, assignmentId } = useParams();
  const [assignment, setAssignment] = useState(null);
  const [overrides, setOverrides] = useState([]);
  const [sections, setSections] = useState([]);
  const [students, setStudents] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [form, setForm] = useState(null); // { id?, ...EMPTY }

  const fetchData = async () => {
    try {
      const [assignmentData, overridesData, sectionsResult, enrollmentResult] = await Promise.all([
        api.getAssignment(courseId, assignmentId),
        api.getAssignmentOverrides(courseId, assignmentId),
        api.getSections(courseId, 1, 100),
        api.getAllEnrollments(courseId).catch(() => ({ data: [] })),
      ]);
      setAssignment(assignmentData);
      setOverrides(Array.isArray(overridesData) ? overridesData : []);
      setSections(sectionsResult.data || []);
      const seen = new Set();
      const list = [];
      for (const e of enrollmentResult.data || []) {
        const isStudent = e.type === 'StudentEnrollment' || e.role === 'StudentEnrollment';
        const uid = e.user_id || e.user?.id;
        if (!isStudent || !uid || seen.has(uid)) continue;
        seen.add(uid);
        list.push({ id: uid, name: e.user?.name || e.user?.display_name || `User ${uid}` });
      }
      list.sort((a, b) => a.name.localeCompare(b.name));
      setStudents(list);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { fetchData(); }, [courseId, assignmentId]);

  const studentName = useMemo(() => {
    const m = new Map(students.map((s) => [s.id, s.name]));
    return (id) => m.get(id) || `User ${id}`;
  }, [students]);
  const sectionName = (id) => sections.find((s) => s.id === id)?.name || t('assignmentOverrides.sectionNumber', { id });

  const handleSave = async (e) => {
    e.preventDefault();
    setError(null);
    if (!form.course_section_id && form.student_ids.length === 0) {
      setError('Choose a section or at least one student.');
      return;
    }
    const override = {
      title: form.title || (form.course_section_id ? sectionName(Number(form.course_section_id)) : form.student_ids.map(studentName).join(', ')),
      due_at: form.due_at ? new Date(form.due_at).toISOString() : null,
      unlock_at: form.unlock_at ? new Date(form.unlock_at).toISOString() : null,
      lock_at: form.lock_at ? new Date(form.lock_at).toISOString() : null,
      course_section_id: form.course_section_id ? parseInt(form.course_section_id, 10) : null,
      student_ids: form.course_section_id ? [] : form.student_ids,
    };
    try {
      if (form.id) await api.updateAssignmentOverride(courseId, assignmentId, form.id, override);
      else await api.createAssignmentOverride(courseId, assignmentId, override);
      setForm(null);
      fetchData();
    } catch (err) {
      setError(err.message);
    }
  };

  const startEdit = (o) =>
    setForm({
      id: o.id,
      title: o.title || '',
      due_at: toLocalInput(o.due_at),
      unlock_at: toLocalInput(o.unlock_at),
      lock_at: toLocalInput(o.lock_at),
      course_section_id: o.course_section_id ? String(o.course_section_id) : '',
      student_ids: Array.isArray(o.student_ids) ? o.student_ids : [],
    });

  const handleDelete = async (overrideId) => {
    if (!window.confirm(t('assignmentOverrides.deleteConfirm'))) return;
    try {
      await api.deleteAssignmentOverride(courseId, assignmentId, overrideId);
      fetchData();
    } catch (err) {
      setError(err.message);
    }
  };

  const toggleOnlyVisible = async (checked) => {
    try {
      await api.updateAssignment(courseId, assignmentId, { only_visible_to_overrides: checked });
      setAssignment((prev) => ({ ...prev, only_visible_to_overrides: checked }));
    } catch (err) {
      setError(err.message);
    }
  };

  const formatDate = (dateStr) => {
    if (!dateStr) return t('assignmentOverrides.noDate');
    return new Date(dateStr).toLocaleString();
  };

  if (loading) {
    return <Layout><div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  {t('assignmentOverrides.loading')}
</div></Layout>;
  }

  return (
    <Layout>
      <CourseNav />
      <div className="mb-6">
        <Link to={`/courses/${courseId}/assignments/${assignmentId}`} className="text-brand-600 hover:underline text-sm">{t('assignmentOverrides.backToAssignment')}</Link>
        <div className="flex items-center justify-between mt-2">
          <div>
            <h2 className="text-2xl font-bold">{t('assignmentOverrides.headerTitle', { name: assignment?.name || '' })}</h2>
            <p className="text-text-tertiary text-sm">
              {t('assignmentOverrides.defaultDue', { date: assignment?.due_at ? formatDate(assignment.due_at) : t('assignmentOverrides.noDueDate') })}
            </p>
          </div>
          <button
            onClick={() => setForm(form ? null : { ...EMPTY })}
            className="flex items-center space-x-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm"
          >
            <Plus className="w-4 h-4" />
            <span>{t('assignmentOverrides.newOverride')}</span>
          </button>
        </div>
      </div>

      <div className="bg-surface-0 rounded-lg shadow p-4 mb-4">
        <label className="flex items-start gap-2 text-sm">
          <input
            type="checkbox"
            checked={!!assignment?.only_visible_to_overrides}
            onChange={(e) => toggleOnlyVisible(e.target.checked)}
            className="mt-0.5"
          />
          <span>
            Assign only to the sections and students below
            <span className="block text-xs text-text-tertiary">
              When off, everyone else gets the assignment with its default dates.
            </span>
          </span>
        </label>
      </div>

      {error && <div role="alert" className="bg-accent-danger/10 text-accent-danger p-3 rounded mb-4">{error}</div>}

      {form && (
        <div className="bg-surface-0 rounded-lg shadow p-6 mb-6">
          <h3 className="font-semibold mb-4">{form.id ? 'Edit override' : t('assignmentOverrides.createOverride')}</h3>
          <form onSubmit={handleSave} className="space-y-4">
            <input
              type="text"
              placeholder={t('assignmentOverrides.titlePlaceholder')}
              aria-label="Override title"
              value={form.title}
              onChange={e => setForm({ ...form, title: e.target.value })}
              className="w-full border border-border-strong rounded px-3 py-2 text-sm"
            />

            <div className="grid grid-cols-3 gap-3">
              {[
                ['due_at', t('assignmentOverrides.dueDate')],
                ['unlock_at', t('assignmentOverrides.unlockDate')],
                ['lock_at', t('assignmentOverrides.lockDate')],
              ].map(([key, label]) => (
                <div key={key}>
                  <label htmlFor={`override-${key}`} className="block text-xs text-text-tertiary mb-1">{label}</label>
                  <input
                    id={`override-${key}`}
                    type="datetime-local"
                    value={form[key]}
                    onChange={e => setForm({ ...form, [key]: e.target.value })}
                    className="w-full border border-border-strong rounded px-3 py-2 text-sm"
                  />
                </div>
              ))}
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <label htmlFor="override-section" className="block text-xs text-text-tertiary mb-1">{t('assignmentOverrides.sectionLabel')}</label>
                <select
                  id="override-section"
                  value={form.course_section_id}
                  onChange={e => setForm({ ...form, course_section_id: e.target.value })}
                  className="w-full border border-border-strong rounded px-3 py-2 text-sm"
                >
                  <option value="">{t('assignmentOverrides.noSection')}</option>
                  {sections.map(s => (
                    <option key={s.id} value={s.id}>{s.name}</option>
                  ))}
                </select>
              </div>
              {!form.course_section_id && (
                <div>
                  <span className="block text-xs text-text-tertiary mb-1">Students</span>
                  <StudentPicker students={students} value={form.student_ids} onChange={(ids) => setForm({ ...form, student_ids: ids })} />
                </div>
              )}
            </div>

            <div className="flex space-x-3">
              <button type="submit" className="bg-brand-600 text-white px-4 py-2 rounded hover:bg-brand-700 text-sm">
                {form.id ? 'Save' : t('common.create')}
              </button>
              <button type="button" onClick={() => setForm(null)} className="text-text-tertiary text-sm">{t('common.cancel')}</button>
            </div>
          </form>
        </div>
      )}

      <div className="space-y-3">
        {overrides.length === 0 ? (
          <div className="bg-surface-0 rounded-lg shadow p-8 text-center text-text-tertiary">{t('assignmentOverrides.empty')}</div>
        ) : (
          overrides.map(override => (
            <div key={override.id} className="bg-surface-0 rounded-lg shadow p-4">
              <div className="flex items-center justify-between">
                <div className="flex items-center space-x-3">
                  <Users className="w-5 h-5 text-orange-500" />
                  <div>
                    <p className="font-medium">{override.title || t('assignmentOverrides.fallbackOverrideName', { id: override.id })}</p>
                    <div className="flex flex-wrap items-center gap-x-4 text-xs text-text-tertiary mt-1">
                      {override.due_at && (
                        <span className="flex items-center space-x-1">
                          <Calendar className="w-3 h-3" />
                          <span>{t('assignmentOverrides.dueLabel', { date: formatDate(override.due_at) })}</span>
                        </span>
                      )}
                      {override.course_section_id && <span>{sectionName(override.course_section_id)}</span>}
                      {Array.isArray(override.student_ids) && override.student_ids.length > 0 && (
                        <span>{override.student_ids.map(studentName).join(', ')}</span>
                      )}
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-3">
                  <button aria-label={`Edit ${override.title || 'override'}`} onClick={() => startEdit(override)} className="text-text-disabled hover:text-brand-600">
                    <Pencil className="w-4 h-4" />
                  </button>
                  <button aria-label={`Delete ${override.title || 'override'}`} onClick={() => handleDelete(override.id)} className="text-text-disabled hover:text-accent-danger">
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              </div>
            </div>
          ))
        )}
      </div>
    </Layout>
  );
};

export default AssignmentOverridesPage;
