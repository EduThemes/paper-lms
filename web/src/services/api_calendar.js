// Calendar lane API calls (G094-G097, G146): the aggregated calendar,
// recurring and section-specific events, the subscribable feed and the
// scheduler additions. Kept apart from api.js so feature lanes merge
// cleanly; it shares api.js's request helper (CSRF, ApiError, 401).

import { request } from './api';

const json = (body) => JSON.stringify(body);

function qs(params) {
  const parts = [];
  Object.entries(params).forEach(([key, value]) => {
    if (value === undefined || value === null || value === '') return;
    if (Array.isArray(value)) {
      value.forEach((v) => parts.push(`${encodeURIComponent(`${key}[]`)}=${encodeURIComponent(v)}`));
    } else {
      parts.push(`${encodeURIComponent(key)}=${encodeURIComponent(value)}`);
    }
  });
  return parts.length ? `?${parts.join('&')}` : '';
}

const toISO = (d) => (d instanceof Date ? d.toISOString() : d);

export const calendarApi = {
  // ---- aggregated calendar (G094 / G146) ----

  // Every calendar the user can show: personal, courses (with sections
  // for instructors), groups and, for admins, the account.
  listContexts: () => request('/calendar/contexts'),

  // Events, assignment and quiz due dates and appointments in a range.
  // contextCodes limits the calendars; types limits the item kinds
  // (event, assignment, appointment).
  listItems: ({ start, end, contextCodes, types } = {}) =>
    request(`/calendar/items${qs({
      start_date: toISO(start),
      end_date: toISO(end),
      context_codes: contextCodes,
      types,
    })}`),

  // ---- events: recurring series and section times (G095) ----

  getEvent: (id) => request(`/calendar_events/${id}`),

  // event may carry rrule ("FREQ=WEEKLY;BYDAY=MO;COUNT=10"), time_zone
  // and child_event_data ([{context_code, start_at, end_at}]).
  createEvent: (event) =>
    request('/calendar_events', { method: 'POST', body: json({ calendar_event: event }) }),

  // which: one | all | following (series only).
  updateEvent: (id, event, which) =>
    request(`/calendar_events/${id}`, {
      method: 'PUT',
      body: json({ calendar_event: event, ...(which ? { which } : {}) }),
    }),

  deleteEvent: (id, which) =>
    request(`/calendar_events/${id}${qs({ which })}`, { method: 'DELETE' }),

  // ---- subscribable feed (G096) ----

  getFeed: () => request('/calendar/feed'),
  rotateFeed: () => request('/calendar/feed', { method: 'POST', body: '{}' }),
  revokeFeed: () => request('/calendar/feed', { method: 'DELETE' }),

  // ---- scheduler (G097) ----

  // The caller's live reservations (and their student groups') with
  // slot times and group details.
  listMyAppointments: () => request('/users/self/appointments'),

  // Appointment groups across the caller's courses: scope reservable
  // (default) or manageable.
  listMyAppointmentGroups: (scope = 'reservable') =>
    request(`/appointment_groups${qs({ scope })}`),

  reserveSlot: (groupId, slotId, comments = '') =>
    request(`/appointment_groups/${groupId}/appointments/${slotId}/reservations`, {
      method: 'POST',
      body: json(comments ? { comments } : {}),
    }),

  cancelReservation: (groupId, slotId, reservationId) =>
    request(`/appointment_groups/${groupId}/appointments/${slotId}/reservations/${reservationId}`, {
      method: 'DELETE',
    }),

  deleteSlot: (groupId, slotId) =>
    request(`/appointment_groups/${groupId}/appointments/${slotId}`, { method: 'DELETE' }),

  // registrationStatus: registered | unregistered | all.
  listParticipants: (groupId, registrationStatus = 'all') =>
    request(`/appointment_groups/${groupId}/users${qs({ registration_status: registrationStatus })}`),

  messageParticipants: (groupId, { subject, body, registrationStatus = 'all' }) =>
    request(`/appointment_groups/${groupId}/messages`, {
      method: 'POST',
      body: json({ subject, body, registration_status: registrationStatus }),
    }),
};

export default calendarApi;
