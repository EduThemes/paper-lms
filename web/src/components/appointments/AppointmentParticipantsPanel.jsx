import React, { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Mail, Check } from 'lucide-react';
import { calendarApi } from '../../services/api_calendar';

// AppointmentParticipantsPanel (G097, instructor view): who may sign up
// for an appointment group and whether they have, and a form to message
// the registered, unregistered or all participants.
const AppointmentParticipantsPanel = ({ group }) => {
  const { t } = useTranslation();
  const [status, setStatus] = useState('all');
  const [people, setPeople] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [composing, setComposing] = useState(false);
  const [subject, setSubject] = useState(group.title || '');
  const [body, setBody] = useState('');
  const [recipients, setRecipients] = useState('unregistered');
  const [sending, setSending] = useState(false);
  const [sent, setSent] = useState(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await calendarApi.listParticipants(group.id, status);
      setPeople(res.data || []);
      setError(null);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [group.id, status]);

  useEffect(() => { load(); }, [load]);

  const send = async (e) => {
    e.preventDefault();
    setSending(true);
    setError(null);
    try {
      const res = await calendarApi.messageParticipants(group.id, { subject, body, registrationStatus: recipients });
      setSent(res.data?.sent ?? 0);
      setBody('');
      setComposing(false);
    } catch (err) {
      setError(err.message);
    } finally {
      setSending(false);
    }
  };

  const isGroups = group.participant_type === 'Group';

  return (
    <div className="mt-4 rounded-md border border-border-default bg-surface-1 p-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="text-sm font-semibold text-text-secondary">
          {isGroups ? t('calendarLane.participants.groupsTitle') : t('calendarLane.participants.title')}
        </h3>
        <div className="flex items-center gap-2">
          <label htmlFor={`ag-status-${group.id}`} className="sr-only">{t('calendarLane.participants.filter')}</label>
          <select
            id={`ag-status-${group.id}`}
            value={status}
            onChange={(e) => setStatus(e.target.value)}
            className="rounded-md border border-border-strong px-2 py-1 text-sm"
          >
            <option value="all">{t('calendarLane.participants.all')}</option>
            <option value="registered">{t('calendarLane.participants.registered')}</option>
            <option value="unregistered">{t('calendarLane.participants.unregistered')}</option>
          </select>
          <button
            type="button"
            onClick={() => { setComposing((c) => !c); setSent(null); }}
            className="inline-flex items-center gap-1 rounded-md border border-border-strong bg-surface-0 px-2 py-1 text-sm hover:bg-surface-2"
          >
            <Mail className="h-4 w-4" /> {t('calendarLane.participants.message')}
          </button>
        </div>
      </div>

      {error && <div className="mt-2 text-sm text-accent-danger" role="alert">{error}</div>}
      {sent !== null && (
        <div className="mt-2 text-sm text-accent-success" role="status">{t('calendarLane.participants.sent', { count: sent })}</div>
      )}

      {composing && (
        <form onSubmit={send} className="mt-3 space-y-2 rounded-md border border-border-default bg-surface-0 p-3">
          <div>
            <label htmlFor={`ag-recipients-${group.id}`} className="block text-xs font-medium text-text-secondary mb-1">
              {t('calendarLane.participants.to')}
            </label>
            <select
              id={`ag-recipients-${group.id}`}
              value={recipients}
              onChange={(e) => setRecipients(e.target.value)}
              className="w-full rounded-md border border-border-strong px-2 py-1.5 text-sm"
            >
              <option value="unregistered">{t('calendarLane.participants.toUnregistered')}</option>
              <option value="registered">{t('calendarLane.participants.toRegistered')}</option>
              <option value="all">{t('calendarLane.participants.toAll')}</option>
            </select>
          </div>
          <div>
            <label htmlFor={`ag-subject-${group.id}`} className="block text-xs font-medium text-text-secondary mb-1">
              {t('calendarLane.participants.subject')}
            </label>
            <input
              id={`ag-subject-${group.id}`}
              type="text"
              value={subject}
              onChange={(e) => setSubject(e.target.value)}
              className="w-full rounded-md border border-border-strong px-2 py-1.5 text-sm"
            />
          </div>
          <div>
            <label htmlFor={`ag-body-${group.id}`} className="block text-xs font-medium text-text-secondary mb-1">
              {t('calendarLane.participants.body')}
            </label>
            <textarea
              id={`ag-body-${group.id}`}
              required
              rows={4}
              value={body}
              onChange={(e) => setBody(e.target.value)}
              className="w-full rounded-md border border-border-strong px-2 py-1.5 text-sm"
            />
          </div>
          <div className="flex justify-end">
            <button type="submit" disabled={sending || !body.trim()} className="rounded-md bg-brand-600 px-3 py-1.5 text-sm font-medium text-on-brand hover:bg-brand-700 disabled:opacity-50">
              {sending ? t('common.saving') : t('calendarLane.participants.send')}
            </button>
          </div>
        </form>
      )}

      {loading ? (
        <div className="mt-2 text-sm text-text-tertiary">{t('common.loading')}</div>
      ) : people.length === 0 ? (
        <div className="mt-2 text-sm text-text-tertiary">{t('calendarLane.participants.empty')}</div>
      ) : (
        <ul className="mt-2 grid grid-cols-1 gap-1 sm:grid-cols-2 text-sm">
          {people.map((p) => (
            <li key={p.id} className="flex items-center gap-2">
              {p.registered ? (
                <Check className="h-4 w-4 text-accent-success" aria-label={t('calendarLane.participants.registered')} />
              ) : (
                <span className="inline-block h-4 w-4" aria-hidden="true" />
              )}
              <span>{p.name}</span>
              {p.member_ids && (
                <span className="text-xs text-text-tertiary">{t('calendarLane.participants.members', { count: p.member_ids.length })}</span>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
};

export default AppointmentParticipantsPanel;
