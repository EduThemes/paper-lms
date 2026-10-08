import React from 'react';
import { Link } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { CalendarCheck, MapPin, Users } from 'lucide-react';
import Page from '../components/Page';
import Layout from '../components/Layout';
import { calendarApi } from '../services/api_calendar';

const keys = {
  mine: ['appointments', 'mine'],
  reservable: ['appointments', 'groups', 'reservable'],
  manageable: ['appointments', 'groups', 'manageable'],
};

const fmtDate = (iso) => new Date(iso).toLocaleDateString(undefined, { weekday: 'long', month: 'long', day: 'numeric' });
const fmtTime = (iso) => new Date(iso).toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' });

// MyAppointmentsPage (G097): a participant's booked appointments across
// every course, with cancel, plus the scheduler groups they can still
// sign up for and, for instructors, the groups they manage.
const MyAppointmentsPage = () => {
  const { t } = useTranslation();
  const queryClient = useQueryClient();

  const mine = useQuery({
    queryKey: keys.mine,
    queryFn: async () => (await calendarApi.listMyAppointments()).data || [],
  });
  const reservable = useQuery({
    queryKey: keys.reservable,
    queryFn: async () => (await calendarApi.listMyAppointmentGroups('reservable')).data || [],
  });
  const manageable = useQuery({
    queryKey: keys.manageable,
    queryFn: async () => (await calendarApi.listMyAppointmentGroups('manageable')).data || [],
  });

  const cancel = useMutation({
    mutationFn: (r) => calendarApi.cancelReservation(r.group_id, r.slot_id, r.id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['appointments'] });
    },
  });

  const onCancel = (r) => {
    if (window.confirm(t('calendarLane.cancelReservationConfirm'))) cancel.mutate(r);
  };

  return (
    <Page query={mine} title={t('calendarLane.myAppointments')}>
      {(appointments) => {
        const now = Date.now();
        const upcoming = appointments.filter((a) => new Date(a.end_at).getTime() >= now);
        const past = appointments.filter((a) => new Date(a.end_at).getTime() < now);
        const bookedGroups = new Set(upcoming.map((a) => a.group_id));
        const openGroups = (reservable.data || []).filter((g) => !bookedGroups.has(g.id));
        return (
          <Layout>
          <div className="mx-auto max-w-4xl space-y-8">
            <div className="flex items-center justify-between">
              <h1 className="text-2xl font-bold text-text-primary flex items-center gap-2">
                <CalendarCheck className="w-6 h-6" /> {t('calendarLane.myAppointments')}
              </h1>
              <Link to="/calendar" className="text-sm text-brand-600 hover:underline">{t('calendarLane.backToCalendar')}</Link>
            </div>

            {cancel.isError && (
              <div className="rounded-md border border-accent-danger/30 bg-accent-danger/10 p-3 text-sm text-accent-danger" role="alert">
                {cancel.error.message}
              </div>
            )}

            <section aria-labelledby="appts-upcoming">
              <h2 id="appts-upcoming" className="text-lg font-semibold text-text-primary mb-3">{t('calendarLane.appointments.upcoming')}</h2>
              {upcoming.length === 0 ? (
                <p className="rounded-lg border border-dashed border-border-strong bg-surface-1 px-6 py-8 text-center text-text-tertiary">
                  {t('calendarLane.appointments.none')}
                </p>
              ) : (
                <ul className="space-y-3">
                  {upcoming.map((a) => (
                    <li key={a.id} className="rounded-lg border border-border-default bg-surface-0 p-4 flex flex-wrap items-start justify-between gap-3">
                      <div className="min-w-0">
                        <div className="font-medium text-text-primary">{a.title}</div>
                        <div className="text-sm text-text-secondary">{fmtDate(a.start_at)} · {fmtTime(a.start_at)} – {fmtTime(a.end_at)}</div>
                        <div className="mt-1 flex flex-wrap gap-3 text-xs text-text-tertiary">
                          <span>{a.course_name}</span>
                          {a.location_name && <span className="inline-flex items-center gap-1"><MapPin className="w-3 h-3" /> {a.location_name}</span>}
                          {a.participant_name && (
                            <span className="inline-flex items-center gap-1">
                              <Users className="w-3 h-3" /> {t('calendarLane.appointments.forGroup', { name: a.participant_name })}
                            </span>
                          )}
                        </div>
                        {a.comments && <p className="mt-1 text-sm text-text-tertiary">{a.comments}</p>}
                      </div>
                      <div className="flex items-center gap-3">
                        <Link to={`/courses/${a.course_id}/appointment_groups`} className="text-sm text-brand-600 hover:underline">
                          {t('calendarLane.appointments.view')}
                        </Link>
                        <button
                          type="button"
                          disabled={cancel.isPending}
                          onClick={() => onCancel(a)}
                          className="text-sm text-accent-danger hover:underline disabled:opacity-50"
                        >
                          {t('calendarLane.cancelReservation')}
                        </button>
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </section>

            {openGroups.length > 0 && (
              <section aria-labelledby="appts-open">
                <h2 id="appts-open" className="text-lg font-semibold text-text-primary mb-3">{t('calendarLane.appointments.signUp')}</h2>
                <ul className="space-y-2">
                  {openGroups.map((g) => (
                    <li key={g.id}>
                      <Link
                        to={`/courses/${g.course_id}/appointment_groups`}
                        className="block rounded-lg border border-border-default bg-surface-0 p-3 hover:bg-surface-1"
                      >
                        <span className="font-medium text-text-primary">{g.title}</span>
                        {g.participant_type === 'Group' && (
                          <span className="ms-2 text-xs text-text-tertiary">{t('calendarLane.appointments.groupSignup')}</span>
                        )}
                      </Link>
                    </li>
                  ))}
                </ul>
              </section>
            )}

            {(manageable.data || []).length > 0 && (
              <section aria-labelledby="appts-manage">
                <h2 id="appts-manage" className="text-lg font-semibold text-text-primary mb-3">{t('calendarLane.appointments.manage')}</h2>
                <ul className="space-y-2">
                  {manageable.data.map((g) => (
                    <li key={g.id}>
                      <Link
                        to={`/courses/${g.course_id}/appointment_groups`}
                        className="flex items-center justify-between rounded-lg border border-border-default bg-surface-0 p-3 hover:bg-surface-1"
                      >
                        <span className="font-medium text-text-primary">{g.title}</span>
                        <span className="text-xs text-text-tertiary">
                          {g.workflow_state === 'active' ? t('common.published') : t('common.unpublished')}
                        </span>
                      </Link>
                    </li>
                  ))}
                </ul>
              </section>
            )}

            {past.length > 0 && (
              <section aria-labelledby="appts-past">
                <h2 id="appts-past" className="text-lg font-semibold text-text-primary mb-3">{t('calendarLane.appointments.past')}</h2>
                <ul className="space-y-1 text-sm text-text-tertiary">
                  {past.map((a) => (
                    <li key={a.id}>{a.title} · {fmtDate(a.start_at)} {fmtTime(a.start_at)}</li>
                  ))}
                </ul>
              </section>
            )}
          </div>
          </Layout>
        );
      }}
    </Page>
  );
};

export default MyAppointmentsPage;
