import React, { useState, useEffect, useMemo } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, Trash2, Plus } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { api } from '../services/api';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';

/**
 * Slot generator helper: given a date, start time, end time, and slot duration
 * (minutes), produce an array of {start_at, end_at} ISO pairs.
 */
const generateSlots = (date, startTime, endTime, durationMin) => {
  if (!date || !startTime || !endTime || !durationMin) return [];
  const start = new Date(`${date}T${startTime}`);
  const end = new Date(`${date}T${endTime}`);
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) return [];
  if (end <= start) return [];
  const out = [];
  const stepMs = durationMin * 60 * 1000;
  for (let cursor = start.getTime(); cursor + stepMs <= end.getTime() + 1; cursor += stepMs) {
    const s = new Date(cursor);
    const e = new Date(cursor + stepMs);
    out.push({ start_at: s.toISOString(), end_at: e.toISOString() });
  }
  return out;
};

const AppointmentGroupEditorPage = () => {
  const { t } = useTranslation();
  const { courseId, groupId } = useParams();
  const navigate = useNavigate();
  const isEditing = Boolean(groupId);

  const [form, setForm] = useState({
    title: '',
    description: '',
    location_name: '',
    location_address: '',
    participants_per_appointment: 1,
    max_appointments_per_participant: 1,
    workflow_state: 'active',
  });
  const [genDate, setGenDate] = useState('');
  const [genStart, setGenStart] = useState('09:00');
  const [genEnd, setGenEnd] = useState('12:00');
  const [genDuration, setGenDuration] = useState(15);
  const [pendingSlots, setPendingSlots] = useState([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState(null);
  const [loading, setLoading] = useState(isEditing);

  useEffect(() => {
    if (!isEditing) return;
    const load = async () => {
      try {
        const result = await api.getAppointmentGroup(groupId);
        const g = result.data;
        setForm({
          title: g.title || '',
          description: g.description || '',
          location_name: g.location_name || '',
          location_address: g.location_address || '',
          participants_per_appointment: g.participants_per_appointment ?? 1,
          max_appointments_per_participant: g.max_appointments_per_participant ?? 1,
          workflow_state: g.workflow_state || 'active',
        });
      } catch (err) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };
    load();
  }, [groupId, isEditing]);

  const generated = useMemo(
    () => generateSlots(genDate, genStart, genEnd, Number(genDuration) || 0),
    [genDate, genStart, genEnd, genDuration]
  );

  const addGenerated = () => {
    if (!generated.length) return;
    setPendingSlots((s) => [...s, ...generated]);
  };

  const removeSlot = (idx) => {
    setPendingSlots((s) => s.filter((_, i) => i !== idx));
  };

  const onChange = (field) => (e) => setForm((f) => ({ ...f, [field]: e.target.value }));

  const submit = async (e) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      const payload = {
        appointment_group: {
          ...form,
          course_id: Number(courseId),
          participants_per_appointment: Number(form.participants_per_appointment) || 1,
          max_appointments_per_participant: Number(form.max_appointments_per_participant) || 1,
          new_appointments: isEditing ? undefined : pendingSlots,
        },
      };
      if (isEditing) {
        await api.updateAppointmentGroup(groupId, payload);
      } else {
        await api.createAppointmentGroup(courseId, payload);
      }
      navigate(`/courses/${courseId}/appointment_groups`);
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <Layout>
        <CourseNav courseId={courseId} />
        <div className="p-6 text-text-tertiary">{t('common.loading')}</div>
      </Layout>
    );
  }

  return (
    <Layout>
      <CourseNav courseId={courseId} />
      <div className="mx-auto max-w-3xl p-6">
        <button
          type="button"
          onClick={() => navigate(`/courses/${courseId}/appointment_groups`)}
          className="mb-4 inline-flex items-center gap-1 text-sm text-text-secondary hover:text-text-primary"
        >
          <ArrowLeft className="h-4 w-4" /> {t('common.back')}
        </button>

        <h1 className="mb-6 text-2xl font-semibold text-text-primary">
          {isEditing ? t('appointmentGroupEditor.editTitle') : t('appointmentGroupEditor.newTitle')}
        </h1>

        {error && (
          <div className="mb-4 rounded-md border border-accent-danger/30 bg-accent-danger/10 p-3 text-sm text-accent-danger">{error}</div>
        )}

        <form onSubmit={submit} className="space-y-6 rounded-lg border border-border-default bg-surface-0 p-6">
          <div>
            <label className="mb-1 block text-sm font-medium text-text-secondary">{t('common.title')}</label>
            <input
              type="text"
              required
              value={form.title}
              onChange={onChange('title')}
              className="w-full rounded-md border border-border-strong px-3 py-2 focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
            />
          </div>

          <div>
            <label className="mb-1 block text-sm font-medium text-text-secondary">{t('common.description')}</label>
            <textarea
              rows={3}
              value={form.description}
              onChange={onChange('description')}
              className="w-full rounded-md border border-border-strong px-3 py-2 focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
            />
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <label className="mb-1 block text-sm font-medium text-text-secondary">{t('appointmentGroupEditor.locationName')}</label>
              <input
                type="text"
                value={form.location_name}
                onChange={onChange('location_name')}
                className="w-full rounded-md border border-border-strong px-3 py-2"
              />
            </div>
            <div>
              <label className="mb-1 block text-sm font-medium text-text-secondary">{t('appointmentGroupEditor.locationAddress')}</label>
              <input
                type="text"
                value={form.location_address}
                onChange={onChange('location_address')}
                className="w-full rounded-md border border-border-strong px-3 py-2"
              />
            </div>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <label className="mb-1 block text-sm font-medium text-text-secondary">{t('appointmentGroupEditor.participantsPerSlot')}</label>
              <input
                type="number"
                min={1}
                value={form.participants_per_appointment}
                onChange={onChange('participants_per_appointment')}
                className="w-full rounded-md border border-border-strong px-3 py-2"
              />
            </div>
            <div>
              <label className="mb-1 block text-sm font-medium text-text-secondary">{t('appointmentGroupEditor.maxPerStudent')}</label>
              <input
                type="number"
                min={1}
                value={form.max_appointments_per_participant}
                onChange={onChange('max_appointments_per_participant')}
                className="w-full rounded-md border border-border-strong px-3 py-2"
              />
            </div>
          </div>

          {!isEditing && (
            <div className="rounded-md border border-border-default bg-surface-1 p-4">
              <h2 className="mb-3 text-sm font-semibold text-text-secondary">{t('appointmentGroupEditor.generateSlots')}</h2>
              <div className="grid gap-3 sm:grid-cols-4">
                <div>
                  <label className="mb-1 block text-xs font-medium text-text-secondary">{t('common.date')}</label>
                  <input
                    type="date"
                    value={genDate}
                    onChange={(e) => setGenDate(e.target.value)}
                    className="w-full rounded-md border border-border-strong px-2 py-1.5 text-sm"
                  />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-medium text-text-secondary">{t('appointmentGroupEditor.start')}</label>
                  <input
                    type="time"
                    value={genStart}
                    onChange={(e) => setGenStart(e.target.value)}
                    className="w-full rounded-md border border-border-strong px-2 py-1.5 text-sm"
                  />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-medium text-text-secondary">{t('appointmentGroupEditor.end')}</label>
                  <input
                    type="time"
                    value={genEnd}
                    onChange={(e) => setGenEnd(e.target.value)}
                    className="w-full rounded-md border border-border-strong px-2 py-1.5 text-sm"
                  />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-medium text-text-secondary">{t('appointmentGroupEditor.durationMin')}</label>
                  <input
                    type="number"
                    min={1}
                    value={genDuration}
                    onChange={(e) => setGenDuration(e.target.value)}
                    className="w-full rounded-md border border-border-strong px-2 py-1.5 text-sm"
                  />
                </div>
              </div>
              <div className="mt-3 flex items-center justify-between">
                <span className="text-xs text-text-tertiary">
                  {t('appointmentGroupEditor.slotsPreview', { count: generated.length })}
                </span>
                <button
                  type="button"
                  onClick={addGenerated}
                  disabled={!generated.length}
                  className="inline-flex items-center gap-1 rounded-md bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700 disabled:opacity-50"
                >
                  <Plus className="h-4 w-4" /> {t('appointmentGroupEditor.addToGroup')}
                </button>
              </div>

              {pendingSlots.length > 0 && (
                <ul className="mt-4 space-y-1 max-h-60 overflow-auto rounded-md border border-border-default bg-surface-0 p-2 text-sm">
                  {pendingSlots.map((s, idx) => (
                    <li key={`${s.start_at}-${idx}`} className="flex items-center justify-between px-2 py-1">
                      <span>
                        {new Date(s.start_at).toLocaleString()} → {new Date(s.end_at).toLocaleTimeString()}
                      </span>
                      <button
                        type="button"
                        onClick={() => removeSlot(idx)}
                        className="text-text-disabled hover:text-accent-danger"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )}

          <div className="flex items-center justify-end gap-2">
            <button
              type="button"
              onClick={() => navigate(`/courses/${courseId}/appointment_groups`)}
              className="rounded-md border border-border-strong bg-surface-0 px-4 py-2 text-sm font-medium text-text-secondary hover:bg-surface-1"
            >
              {t('common.cancel')}
            </button>
            <button
              type="submit"
              disabled={saving}
              className="rounded-md bg-brand-600 px-4 py-2 text-sm font-medium text-white hover:bg-brand-700 disabled:opacity-50"
            >
              {saving
                ? t('common.saving')
                : isEditing
                ? t('appointmentGroupEditor.saveChanges')
                : t('appointmentGroupEditor.createGroup')}
            </button>
          </div>
        </form>
      </div>
    </Layout>
  );
};

export default AppointmentGroupEditorPage;
