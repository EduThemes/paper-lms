import React, { useState, useEffect, useCallback } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Plus, Calendar, MapPin, Users, Trash2, Pencil } from 'lucide-react';
import { api } from '../services/api';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import SlotPicker from '../components/appointments/SlotPicker';

const Spinner = () => (
  <svg className="h-6 w-6 animate-spin text-brand-500" viewBox="0 0 24 24" fill="none">
    <circle cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="3" className="opacity-25" />
    <path d="M22 12a10 10 0 0 1-10 10" stroke="currentColor" strokeWidth="3" className="opacity-75" />
  </svg>
);

const AppointmentGroupsPage = () => {
  const { courseId } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const [groups, setGroups] = useState([]);
  const [expandedId, setExpandedId] = useState(null);
  const [slotsByGroup, setSlotsByGroup] = useState({});
  const [reservedSlotIds, setReservedSlotIds] = useState(new Set());
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);
  const [filter, setFilter] = useState('available');

  const fetchGroups = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await api.listAppointmentGroups(courseId);
      setGroups(result.data || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [courseId]);

  const fetchMyReservations = useCallback(async () => {
    try {
      const result = await api.listMyAppointmentReservations();
      const ids = new Set((result.data || []).map((r) => r.slot_id));
      setReservedSlotIds(ids);
    } catch {
      /* non-fatal */
    }
  }, []);

  useEffect(() => {
    fetchGroups();
    fetchMyReservations();
  }, [fetchGroups, fetchMyReservations]);

  const loadSlots = async (groupId) => {
    try {
      const includeFull = filter === 'all' || isTeacher;
      const result = await api.listAppointmentSlots(groupId, includeFull);
      setSlotsByGroup((prev) => ({ ...prev, [groupId]: result.data || [] }));
    } catch (err) {
      setError(err.message);
    }
  };

  const toggle = (groupId) => {
    if (expandedId === groupId) {
      setExpandedId(null);
    } else {
      setExpandedId(groupId);
      if (!slotsByGroup[groupId]) loadSlots(groupId);
    }
  };

  const reserve = async (groupId, slot) => {
    setBusy(true);
    try {
      await api.reserveAppointmentSlot(groupId, slot.id);
      await loadSlots(groupId);
      await fetchMyReservations();
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  };

  const cancel = async (groupId, slot) => {
    setBusy(true);
    try {
      // We need to find the reservation id; reload slot info first.
      const reservations = await api.listMyAppointmentReservations();
      const mine = (reservations.data || []).find((r) => r.slot_id === slot.id);
      if (mine) {
        await api.cancelAppointmentReservation(groupId, slot.id, mine.id);
      }
      await loadSlots(groupId);
      await fetchMyReservations();
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  };

  const handleDelete = async (group) => {
    if (!confirm(`Delete "${group.title}"? Existing reservations will be canceled.`)) return;
    try {
      await api.deleteAppointmentGroup(group.id);
      setGroups((g) => g.filter((x) => x.id !== group.id));
    } catch (err) {
      setError(err.message);
    }
  };

  return (
    <Layout>
      <CourseNav courseId={courseId} />
      <div className="mx-auto max-w-5xl p-6">
        <div className="mb-6 flex items-center justify-between">
          <h1 className="text-2xl font-semibold text-text-primary">Appointment Groups</h1>
          {isTeacher && (
            <Link
              to={`/courses/${courseId}/appointment_groups/new`}
              className="inline-flex items-center gap-1 rounded-md bg-brand-600 px-3 py-2 text-sm font-medium text-white hover:bg-brand-700"
            >
              <Plus className="h-4 w-4" /> New Group
            </Link>
          )}
        </div>

        {!isTeacher && (
          <div className="mb-4 inline-flex rounded-md border bg-surface-0 text-sm">
            <button
              type="button"
              onClick={() => setFilter('available')}
              className={`px-3 py-1.5 ${filter === 'available' ? 'bg-brand-50 text-brand-700' : 'text-text-secondary'}`}
            >
              Available
            </button>
            <button
              type="button"
              onClick={() => setFilter('all')}
              className={`px-3 py-1.5 ${filter === 'all' ? 'bg-brand-50 text-brand-700' : 'text-text-secondary'}`}
            >
              All
            </button>
          </div>
        )}

        {loading && (
          <div className="flex items-center gap-2 text-text-tertiary">
            <Spinner /> Loading appointment groups…
          </div>
        )}

        {error && (
          <div className="mb-4 rounded-md border border-accent-danger/30 bg-accent-danger/10 p-3 text-sm text-accent-danger">
            {error}{' '}
            <button onClick={fetchGroups} className="ml-2 underline">
              Try Again
            </button>
          </div>
        )}

        {!loading && !groups.length && (
          <div className="rounded-lg border border-dashed border-border-strong bg-surface-1 px-6 py-10 text-center text-text-tertiary">
            <Calendar className="mx-auto mb-2 h-6 w-6 opacity-50" />
            No appointment groups have been created yet.
          </div>
        )}

        <div className="space-y-3">
          {groups.map((group) => (
            <div key={group.id} className="rounded-lg border border-border-default bg-surface-0">
              <div
                className="flex cursor-pointer items-center justify-between p-4 hover:bg-surface-1"
                onClick={() => toggle(group.id)}
              >
                <div>
                  <div className="text-base font-medium text-text-primary">{group.title}</div>
                  {group.description && (
                    <div className="mt-0.5 line-clamp-1 text-sm text-text-tertiary">{group.description}</div>
                  )}
                  <div className="mt-1 flex flex-wrap items-center gap-3 text-xs text-text-tertiary">
                    {group.location_name && (
                      <span className="inline-flex items-center gap-1">
                        <MapPin className="h-3 w-3" /> {group.location_name}
                      </span>
                    )}
                    <span className="inline-flex items-center gap-1">
                      <Users className="h-3 w-3" /> {group.participants_per_appointment} per slot
                    </span>
                  </div>
                </div>
                {isTeacher && (
                  <div className="flex items-center gap-2" onClick={(e) => e.stopPropagation()}>
                    <Link
                      to={`/courses/${courseId}/appointment_groups/${group.id}/edit`}
                      className="rounded-md p-2 text-text-tertiary hover:bg-surface-2"
                      title="Edit"
                    >
                      <Pencil className="h-4 w-4" />
                    </Link>
                    <button
                      type="button"
                      onClick={() => handleDelete(group)}
                      className="rounded-md p-2 text-text-tertiary hover:bg-accent-danger/10 hover:text-accent-danger"
                      title="Delete"
                    >
                      <Trash2 className="h-4 w-4" />
                    </button>
                  </div>
                )}
              </div>

              {expandedId === group.id && (
                <div className="border-t border-border-subtle p-4">
                  <SlotPicker
                    slots={slotsByGroup[group.id] || []}
                    reservedIds={reservedSlotIds}
                    onReserve={(slot) => reserve(group.id, slot)}
                    onCancel={(slot) => cancel(group.id, slot)}
                    locationName={group.location_name}
                    busy={busy}
                  />
                </div>
              )}
            </div>
          ))}
        </div>
      </div>
    </Layout>
  );
};

export default AppointmentGroupsPage;
