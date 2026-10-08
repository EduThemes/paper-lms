import React, { useState, useEffect, useMemo } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, Trash2, Plus } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { api } from '../services/api';
import { calendarApi } from '../services/api_calendar';
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
  // Scheduler parity (G097): more courses, section limits, a group set
  // that signs up as groups, and the slots already on the group.
  const [teachingCourses, setTeachingCourses] = useState([]);
  const [extraCourses, setExtraCourses] = useState([]);
  const [sectionIds, setSectionIds] = useState([]);
  const [groupSets, setGroupSets] = useState([]);
  const [groupSetId, setGroupSetId] = useState('');
  const [existingSlots, setExistingSlots] = useState([]);

  useEffect(() => {
    calendarApi.listContexts()
      .then((res) => setTeachingCourses((res.data || []).filter((c) => c.type === 'course' && c.is_instructor)))
      .catch(() => setTeachingCourses([]));
    api.getGroupCategories(courseId)
      .then((res) => setGroupSets(res.data || []))
      .catch(() => setGroupSets([]));
  }, [courseId]);

  const loadSlots = async () => {
    try {
      const res = await api.listAppointmentSlots(groupId, true);
      setExistingSlots(res.data || []);
    } catch {
      setExistingSlots([]);
    }
  };

  useEffect(() => {
    if (!isEditing) return;
    const load = async () => {
      try {
        const result = await api.getAppointmentGroup(groupId);
        const g = result.data;
        const num = (code, prefix) => Number(code.slice(prefix.length));
        setExtraCourses((g.context_codes || [])
          .filter((c) => c !== `course_${g.course_id}`)
          .map((c) => num(c, 'course_')));
        setSectionIds((g.sub_context_codes || [])
          .filter((c) => c.startsWith('course_section_'))
          .map((c) => num(c, 'course_section_')));
        setGroupSetId(g.group_category_id ? String(g.group_category_id) : '');
        loadSlots();
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
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [groupId, isEditing]);

  const toggleIn = (setter, id) => setter((list) => (list.includes(id) ? list.filter((x) => x !== id) : [...list, id]));
  const selectedCourses = teachingCourses.filter(
    (c) => String(c.id) === String(courseId) || extraCourses.includes(c.id),
  );

  const deleteExistingSlot = async (slot) => {
    if (!window.confirm(t('calendarLane.scheduler.deleteSlotConfirm'))) return;
    try {
      await calendarApi.deleteSlot(groupId, slot.id);
      await loadSlots();
    } catch (err) {
      setError(err.message);
    }
  };

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
      const subCodes = groupSetId
        ? [`group_category_${groupSetId}`]
        : sectionIds
          .filter((id) => !teachingCourses.length || selectedCourses.some((c) => (c.sections || []).some((s) => s.id === id)))
          .map((id) => `course_section_${id}`);
      const payload = {
        appointment_group: {
          ...form,
          course_id: Number(courseId),
          participants_per_appointment: Number(form.participants_per_appointment) || 1,
          max_appointments_per_participant: Number(form.max_appointments_per_participant) || 1,
          new_appointments: pendingSlots.length ? pendingSlots : undefined,
          context_codes: [`course_${courseId}`, ...extraCourses.map((id) => `course_${id}`)],
          sub_context_codes: subCodes,
        },
      };
      if (!isEditing && groupSetId) payload.appointment_group.participant_type = 'Group';
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

          <fieldset className="rounded-md border border-border-default p-4 space-y-3">
            <legend className="px-1 text-sm font-semibold text-text-secondary">{t('calendarLane.scheduler.whoCanSignUp')}</legend>
            {teachingCourses.filter((c) => String(c.id) !== String(courseId)).length > 0 && (
              <div>
                <div className="mb-1 text-xs font-medium text-text-secondary">{t('calendarLane.scheduler.alsoCourses')}</div>
                <div className="flex flex-wrap gap-3">
                  {teachingCourses.filter((c) => String(c.id) !== String(courseId)).map((c) => (
                    <label key={c.id} className="flex items-center gap-1 text-sm">
                      <input type="checkbox" checked={extraCourses.includes(c.id)} onChange={() => toggleIn(setExtraCourses, c.id)} />
                      {c.name}
                    </label>
                  ))}
                </div>
              </div>
            )}
            <div>
              <label htmlFor="ag-group-set" className="mb-1 block text-xs font-medium text-text-secondary">{t('calendarLane.scheduler.groupSet')}</label>
              <select
                id="ag-group-set"
                value={groupSetId}
                disabled={isEditing}
                onChange={(e) => setGroupSetId(e.target.value)}
                className="w-full rounded-md border border-border-strong px-3 py-2 text-sm"
              >
                <option value="">{t('calendarLane.scheduler.individuals')}</option>
                {groupSets.map((gs) => <option key={gs.id} value={gs.id}>{gs.name}</option>)}
              </select>
            </div>
            {!groupSetId && selectedCourses.some((c) => (c.sections || []).length > 0) && (
              <div>
                <div className="mb-1 text-xs font-medium text-text-secondary">{t('calendarLane.scheduler.limitSections')}</div>
                {selectedCourses.map((c) => (
                  <div key={c.id} className="mb-1">
                    {selectedCourses.length > 1 && <div className="text-xs text-text-tertiary">{c.name}</div>}
                    <div className="flex flex-wrap gap-3">
                      {(c.sections || []).map((sec) => (
                        <label key={sec.id} className="flex items-center gap-1 text-sm">
                          <input type="checkbox" checked={sectionIds.includes(sec.id)} onChange={() => toggleIn(setSectionIds, sec.id)} />
                          {sec.name}
                        </label>
                      ))}
                    </div>
                  </div>
                ))}
                <p className="text-xs text-text-tertiary">{t('calendarLane.scheduler.limitSectionsHelp')}</p>
              </div>
            )}
          </fieldset>

          {isEditing && (
            <div className="rounded-md border border-border-default p-4">
              <h2 className="mb-2 text-sm font-semibold text-text-secondary">{t('calendarLane.scheduler.currentSlots')}</h2>
              {existingSlots.length === 0 ? (
                <p className="text-sm text-text-tertiary">{t('calendarLane.scheduler.noSlots')}</p>
              ) : (
                <ul className="space-y-1 max-h-60 overflow-auto text-sm">
                  {existingSlots.map((slot) => (
                    <li key={slot.id} className="flex items-center justify-between px-2 py-1">
                      <span>
                        {new Date(slot.start_at).toLocaleString()} → {new Date(slot.end_at).toLocaleTimeString()}
                        {slot.reservation_count > 0 && (
                          <span className="ms-2 text-xs text-text-tertiary">
                            {t('calendarLane.slotFill', { count: slot.reservation_count, limit: slot.effective_limit })}
                          </span>
                        )}
                      </span>
                      <button
                        type="button"
                        onClick={() => deleteExistingSlot(slot)}
                        aria-label={t('calendarLane.scheduler.deleteSlot')}
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

          {true && (
            <div className="rounded-md border border-border-default bg-surface-1 p-4">
              <h2 className="mb-3 text-sm font-semibold text-text-secondary">{t('appointmentGroupEditor.generateSlots')}</h2>
              <div className="grid gap-3 sm:grid-cols-4">
                <div>
                  <label htmlFor="ag-gen-date" className="mb-1 block text-xs font-medium text-text-secondary">{t('common.date')}</label>
                  <input
                    type="date"
                    id="ag-gen-date"
                    value={genDate}
                    onChange={(e) => setGenDate(e.target.value)}
                    className="w-full rounded-md border border-border-strong px-2 py-1.5 text-sm"
                  />
                </div>
                <div>
                  <label htmlFor="ag-gen-start" className="mb-1 block text-xs font-medium text-text-secondary">{t('appointmentGroupEditor.start')}</label>
                  <input
                    type="time"
                    id="ag-gen-start"
                    value={genStart}
                    onChange={(e) => setGenStart(e.target.value)}
                    className="w-full rounded-md border border-border-strong px-2 py-1.5 text-sm"
                  />
                </div>
                <div>
                  <label htmlFor="ag-gen-end" className="mb-1 block text-xs font-medium text-text-secondary">{t('appointmentGroupEditor.end')}</label>
                  <input
                    type="time"
                    id="ag-gen-end"
                    value={genEnd}
                    onChange={(e) => setGenEnd(e.target.value)}
                    className="w-full rounded-md border border-border-strong px-2 py-1.5 text-sm"
                  />
                </div>
                <div>
                  <label htmlFor="ag-gen-duration" className="mb-1 block text-xs font-medium text-text-secondary">{t('appointmentGroupEditor.durationMin')}</label>
                  <input
                    type="number"
                    min={1}
                    id="ag-gen-duration"
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
