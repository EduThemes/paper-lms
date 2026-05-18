import React, { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate, Link, Navigate } from 'react-router-dom';
import { Plus, BookOpen, Pencil, Trash2, Save, Link2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { api } from '../services/api';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import QuizzesSubNav from '../components/quiz/QuizzesSubNav';
import RichContentEditorV2 from '../components/rce/RichContentEditorV2';

/**
 * Stimulus passages — reusable reading/diagram content that one or more
 * questions reference. List + edit view in one page.
 */
const StimulusEditorPage = () => {
  const { t } = useTranslation();
  const { courseId, stimulusId } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const navigate = useNavigate();
  const [stimuli, setStimuli] = useState([]);
  const [linkedQuestions, setLinkedQuestions] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [message, setMessage] = useState('');
  const [messageIsError, setMessageIsError] = useState(false);
  const [form, setForm] = useState({ title: '', content: '' });
  const [saving, setSaving] = useState(false);

  const isEditing = Boolean(stimulusId);
  const isNew = stimulusId === 'new';

  const fetchList = useCallback(async () => {
    try {
      const list = await api.listStimuli(courseId);
      setStimuli(list || []);
    } catch (err) {
      setError(err.message);
    }
  }, [courseId]);

  useEffect(() => {
    setLoading(true);
    const load = async () => {
      try {
        await fetchList();
        if (isEditing && !isNew) {
          const s = await api.getStimulus(courseId, stimulusId);
          setForm({ title: s.title || '', content: s.content || '' });
          try {
            const qs = await api.getStimulusQuestions(stimulusId);
            setLinkedQuestions(qs || []);
          } catch {
            setLinkedQuestions([]);
          }
        } else if (isNew) {
          setForm({ title: '', content: '' });
          setLinkedQuestions([]);
        }
      } catch (err) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };
    load();
  }, [courseId, stimulusId, isEditing, isNew, fetchList]);

  const handleCreate = () => {
    navigate(`/courses/${courseId}/stimuli/new`);
  };

  const reportOk = (msg) => { setMessage(msg); setMessageIsError(false); };
  const reportErr = (msg) => { setMessage(msg); setMessageIsError(true); };

  const handleSave = async () => {
    if (!form.title.trim()) {
      reportErr(t('stimulusEditor.titleRequired'));
      return;
    }
    setSaving(true);
    try {
      if (isNew) {
        const created = await api.createStimulus(courseId, form);
        reportOk(t('stimulusEditor.stimulusCreated'));
        if (created?.id) {
          navigate(`/courses/${courseId}/stimuli/${created.id}`);
        }
      } else {
        await api.updateStimulus(courseId, stimulusId, form);
        reportOk(t('stimulusEditor.saved'));
        await fetchList();
      }
    } catch (err) {
      reportErr(t('itemBankManager.errorPrefix') + err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (s) => {
    if (!window.confirm(t('stimulusEditor.deleteConfirm', { title: s.title }))) return;
    try {
      await api.deleteStimulus(courseId, s.id);
      setStimuli(prev => prev.filter(x => x.id !== s.id));
      reportOk(t('stimulusEditor.deleted'));
      if (String(stimulusId) === String(s.id)) {
        navigate(`/courses/${courseId}/stimuli`);
      }
    } catch (err) {
      reportErr(t('itemBankManager.errorPrefix') + err.message);
    }
  };

  if (isTeacher === false) return <Navigate to={`/courses/${courseId}/quizzes`} replace />;

  return (
    <Layout>
      <CourseNav />
      <QuizzesSubNav />

      <header className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-text-primary flex items-center gap-2">
            <BookOpen className="w-6 h-6 text-brand-600" />
            {t('stimulusEditor.title')}
          </h1>
          <p className="text-sm text-text-tertiary mt-1">
            {t('stimulusEditor.description')}
          </p>
        </div>
        {!isEditing && (
          <button onClick={handleCreate} className="inline-flex items-center gap-1 px-3 py-1.5 bg-brand-600 text-white rounded hover:bg-brand-700 text-sm font-medium">
            <Plus className="w-4 h-4" /> {t('stimulusEditor.newStimulus')}
          </button>
        )}
      </header>

      {message && (
        <div className={`mb-4 px-4 py-2 rounded text-sm ${messageIsError ? 'bg-accent-danger/10 text-accent-danger' : 'bg-accent-success/10 text-accent-success'}`}>
          {message}
        </div>
      )}
      {error && (
        <div className="mb-4 px-4 py-2 rounded text-sm bg-accent-danger/10 text-accent-danger">{error}</div>
      )}

      {loading ? (
        <div className="p-6 text-center text-text-tertiary text-sm">{t('common.loading')}</div>
      ) : isEditing ? (
        <section className="bg-surface-0 rounded-lg shadow border border-border-default p-6">
          <Link to={`/courses/${courseId}/stimuli`} className="text-brand-600 hover:underline text-sm">
            {t('stimulusEditor.backToAll')}
          </Link>
          <div className="mt-4 space-y-4">
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">{t('common.title')}</label>
              <input
                type="text"
                value={form.title}
                onChange={(e) => setForm(f => ({ ...f, title: e.target.value }))}
                className="w-full border border-border-strong rounded px-3 py-2 text-sm bg-surface-0 text-text-primary"
                placeholder={t('stimulusEditor.titlePlaceholder')}
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">{t('stimulusEditor.passageContent')}</label>
              <RichContentEditorV2
                value={form.content}
                onChange={(html) => setForm(f => ({ ...f, content: html }))}
                placeholder={t('stimulusEditor.contentPlaceholder')}
                minHeight="240px"
                courseId={courseId}
                autoSaveKey={`stimulus-${courseId}-${stimulusId || 'new'}-content`}
              />
            </div>
            <div className="flex items-center gap-2">
              <button onClick={handleSave} disabled={saving}
                className="inline-flex items-center gap-2 px-4 py-2 bg-brand-600 text-white rounded hover:bg-brand-700 text-sm font-medium disabled:opacity-50">
                <Save className="w-4 h-4" /> {saving
                  ? t('common.saving')
                  : isNew
                  ? t('common.create')
                  : t('common.save')}
              </button>
            </div>

            {!isNew && (
              <section className="mt-6 border-t border-border-default pt-4">
                <h2 className="font-semibold text-sm text-text-primary flex items-center gap-1 mb-2">
                  <Link2 className="w-4 h-4" /> {t('stimulusEditor.linkedQuestions', { count: linkedQuestions.length })}
                </h2>
                {linkedQuestions.length === 0 ? (
                  <p className="text-xs text-text-tertiary italic">
                    {t('stimulusEditor.noLinkedQuestions')}
                  </p>
                ) : (
                  <ul className="text-sm space-y-1">
                    {linkedQuestions.map(q => (
                      <li key={q.id} className="text-text-secondary">
                        <span className="text-text-tertiary text-xs mr-2">#{q.id}</span>
                        <span dangerouslySetInnerHTML={{ __html: String(q.question_text || '').slice(0, 200) }} />
                      </li>
                    ))}
                  </ul>
                )}
              </section>
            )}
          </div>
        </section>
      ) : (
        <section className="bg-surface-0 rounded-lg shadow border border-border-default overflow-hidden">
          {stimuli.length === 0 ? (
            <div className="p-6 text-center text-text-tertiary text-sm">
              {t('stimulusEditor.emptyList')}
            </div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-surface-1 text-text-tertiary text-xs uppercase tracking-wide">
                <tr>
                  <th className="text-left px-4 py-2 font-medium">{t('common.title')}</th>
                  <th className="text-right px-4 py-2 font-medium">{t('stimulusEditor.questionsHeader')}</th>
                  <th className="text-right px-4 py-2 font-medium">{t('stimulusEditor.updatedHeader')}</th>
                  <th className="px-2 py-2 w-24"></th>
                </tr>
              </thead>
              <tbody>
                {stimuli.map(s => (
                  <tr key={s.id} className="border-t border-border-default hover:bg-surface-1">
                    <td className="px-4 py-2">
                      <Link to={`/courses/${courseId}/stimuli/${s.id}`}
                            className="font-medium text-text-primary hover:text-brand-600">
                        {s.title}
                      </Link>
                    </td>
                    <td className="px-4 py-2 text-right text-text-secondary">{s.question_count ?? '—'}</td>
                    <td className="px-4 py-2 text-right text-text-tertiary text-xs">
                      {s.updated_at ? new Date(s.updated_at).toLocaleDateString() : '—'}
                    </td>
                    <td className="px-2 py-2 text-right">
                      <Link to={`/courses/${courseId}/stimuli/${s.id}`}
                            className="inline-block p-1 text-text-disabled hover:text-brand-600" aria-label={t('stimulusEditor.editStimulusAria')}>
                        <Pencil className="w-3.5 h-3.5" />
                      </Link>
                      <button onClick={() => handleDelete(s)}
                              className="inline-block p-1 text-text-disabled hover:text-accent-danger" aria-label={t('stimulusEditor.deleteStimulusAria')}>
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>
      )}
    </Layout>
  );
};

export default StimulusEditorPage;
