import React, { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Plus, Trash2, Users, Calendar } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { api } from '../services/api';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';

const AssignmentOverridesPage = () => {
  const { t } = useTranslation();
  const { courseId, assignmentId } = useParams();
  const [assignment, setAssignment] = useState(null);
  const [overrides, setOverrides] = useState([]);
  const [sections, setSections] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [showCreate, setShowCreate] = useState(false);
  const [newOverride, setNewOverride] = useState({
    title: '',
    due_at: '',
    unlock_at: '',
    lock_at: '',
    course_section_id: '',
    student_ids: '',
  });

  const fetchData = async () => {
    try {
      const [assignmentData, overridesData, sectionsResult] = await Promise.all([
        api.getAssignment(courseId, assignmentId),
        api.getAssignmentOverrides(courseId, assignmentId),
        api.getSections(courseId, 1, 100),
      ]);
      setAssignment(assignmentData);
      setOverrides(overridesData);
      setSections(sectionsResult.data || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { fetchData(); }, [courseId, assignmentId]);

  const handleCreate = async (e) => {
    e.preventDefault();
    try {
      const override = { title: newOverride.title };
      if (newOverride.due_at) override.due_at = new Date(newOverride.due_at).toISOString();
      if (newOverride.unlock_at) override.unlock_at = new Date(newOverride.unlock_at).toISOString();
      if (newOverride.lock_at) override.lock_at = new Date(newOverride.lock_at).toISOString();
      if (newOverride.course_section_id) override.course_section_id = parseInt(newOverride.course_section_id);
      if (newOverride.student_ids) {
        override.student_ids = newOverride.student_ids.split(',').map(id => parseInt(id.trim())).filter(Boolean);
      }
      await api.createAssignmentOverride(courseId, assignmentId, override);
      setNewOverride({ title: '', due_at: '', unlock_at: '', lock_at: '', course_section_id: '', student_ids: '' });
      setShowCreate(false);
      fetchData();
    } catch (err) {
      setError(err.message);
    }
  };

  const handleDelete = async (overrideId) => {
    if (!window.confirm(t('assignmentOverrides.deleteConfirm'))) return;
    try {
      await api.deleteAssignmentOverride(courseId, assignmentId, overrideId);
      fetchData();
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
            onClick={() => setShowCreate(!showCreate)}
            className="flex items-center space-x-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm"
          >
            <Plus className="w-4 h-4" />
            <span>{t('assignmentOverrides.newOverride')}</span>
          </button>
        </div>
      </div>

      {error && <div className="bg-accent-danger/10 text-accent-danger p-3 rounded mb-4">{error}</div>}

      {showCreate && (
        <div className="bg-surface-0 rounded-lg shadow p-6 mb-6">
          <h3 className="font-semibold mb-4">{t('assignmentOverrides.createOverride')}</h3>
          <form onSubmit={handleCreate} className="space-y-4">
            <input
              type="text"
              placeholder={t('assignmentOverrides.titlePlaceholder')}
              value={newOverride.title}
              onChange={e => setNewOverride({ ...newOverride, title: e.target.value })}
              className="w-full border border-border-strong rounded px-3 py-2 text-sm"
              required
            />

            <div className="grid grid-cols-3 gap-3">
              <div>
                <label className="block text-xs text-text-tertiary mb-1">{t('assignmentOverrides.dueDate')}</label>
                <input
                  type="datetime-local"
                  value={newOverride.due_at}
                  onChange={e => setNewOverride({ ...newOverride, due_at: e.target.value })}
                  className="w-full border border-border-strong rounded px-3 py-2 text-sm"
                />
              </div>
              <div>
                <label className="block text-xs text-text-tertiary mb-1">{t('assignmentOverrides.unlockDate')}</label>
                <input
                  type="datetime-local"
                  value={newOverride.unlock_at}
                  onChange={e => setNewOverride({ ...newOverride, unlock_at: e.target.value })}
                  className="w-full border border-border-strong rounded px-3 py-2 text-sm"
                />
              </div>
              <div>
                <label className="block text-xs text-text-tertiary mb-1">{t('assignmentOverrides.lockDate')}</label>
                <input
                  type="datetime-local"
                  value={newOverride.lock_at}
                  onChange={e => setNewOverride({ ...newOverride, lock_at: e.target.value })}
                  className="w-full border border-border-strong rounded px-3 py-2 text-sm"
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-xs text-text-tertiary mb-1">{t('assignmentOverrides.sectionLabel')}</label>
                <select
                  value={newOverride.course_section_id}
                  onChange={e => setNewOverride({ ...newOverride, course_section_id: e.target.value })}
                  className="w-full border border-border-strong rounded px-3 py-2 text-sm"
                >
                  <option value="">{t('assignmentOverrides.noSection')}</option>
                  {sections.map(s => (
                    <option key={s.id} value={s.id}>{s.name}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-xs text-text-tertiary mb-1">{t('assignmentOverrides.studentIdsLabel')}</label>
                <input
                  type="text"
                  placeholder={t('assignmentOverrides.studentIdsPlaceholder')}
                  value={newOverride.student_ids}
                  onChange={e => setNewOverride({ ...newOverride, student_ids: e.target.value })}
                  className="w-full border border-border-strong rounded px-3 py-2 text-sm"
                />
              </div>
            </div>

            <div className="flex space-x-3">
              <button type="submit" className="bg-brand-600 text-white px-4 py-2 rounded hover:bg-brand-700 text-sm">{t('common.create')}</button>
              <button type="button" onClick={() => setShowCreate(false)} className="text-text-tertiary text-sm">{t('common.cancel')}</button>
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
                    <div className="flex items-center space-x-4 text-xs text-text-tertiary mt-1">
                      {override.due_at && (
                        <span className="flex items-center space-x-1">
                          <Calendar className="w-3 h-3" />
                          <span>{t('assignmentOverrides.dueLabel', { date: formatDate(override.due_at) })}</span>
                        </span>
                      )}
                      {override.course_section_id && (
                        <span>{t('assignmentOverrides.sectionNumber', { id: override.course_section_id })}</span>
                      )}
                    </div>
                  </div>
                </div>
                <button onClick={() => handleDelete(override.id)} className="text-text-disabled hover:text-accent-danger">
                  <Trash2 className="w-4 h-4" />
                </button>
              </div>
            </div>
          ))
        )}
      </div>
    </Layout>
  );
};

export default AssignmentOverridesPage;
