import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Rss, Copy, RefreshCw, X } from 'lucide-react';
import { calendarApi } from '../../services/api_calendar';

// CalendarFeedPanel manages the subscribable ICS feed (G096): a private
// URL Google Calendar, Outlook or Apple Calendar can poll without a
// session. Resetting the link revokes the old one.
const CalendarFeedPanel = ({ onClose, onDownload }) => {
  const { t } = useTranslation();
  const [feed, setFeed] = useState(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    let live = true;
    calendarApi.getFeed()
      .then((res) => { if (live) setFeed(res.data); })
      .catch((err) => { if (live) setError(err.message); })
      .finally(() => { if (live) setLoading(false); });
    return () => { live = false; };
  }, []);

  const run = async (fn) => {
    setBusy(true);
    setError(null);
    setCopied(false);
    try {
      const res = await fn();
      setFeed(res.data);
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  };

  const create = () => run(() => calendarApi.rotateFeed());
  const reset = () => {
    if (!window.confirm(t('calendarLane.feed.resetConfirm'))) return;
    run(() => calendarApi.rotateFeed());
  };
  const revoke = () => {
    if (!window.confirm(t('calendarLane.feed.revokeConfirm'))) return;
    run(() => calendarApi.revokeFeed());
  };

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(feed.feed_url);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  };

  return (
    <section className="bg-surface-0 rounded-lg shadow border border-border-default p-4 mb-4" aria-labelledby="calendar-feed-heading">
      <div className="flex items-start justify-between gap-2">
        <h3 id="calendar-feed-heading" className="font-semibold text-text-primary flex items-center gap-2">
          <Rss className="w-4 h-4" /> {t('calendarLane.feed.title')}
        </h3>
        <button type="button" onClick={onClose} className="p-1 text-text-tertiary hover:text-text-secondary" aria-label={t('common.close')}>
          <X className="w-4 h-4" />
        </button>
      </div>
      <p className="text-sm text-text-tertiary mt-1">{t('calendarLane.feed.help')}</p>

      {error && <div className="mt-3 text-sm text-accent-danger" role="alert">{error}</div>}

      {loading ? (
        <div className="mt-3 text-sm text-text-tertiary">{t('common.loading')}</div>
      ) : feed?.exists && feed?.url_available ? (
        <div className="mt-3 space-y-2">
          <label htmlFor="calendar-feed-url" className="block text-xs font-medium text-text-secondary">
            {t('calendarLane.feed.urlLabel')}
          </label>
          <div className="flex gap-2">
            <input
              id="calendar-feed-url"
              readOnly
              value={feed.feed_url}
              onFocus={(e) => e.target.select()}
              className="flex-1 min-w-0 border border-border-strong rounded-md px-3 py-2 text-sm font-mono bg-surface-1"
            />
            <button
              type="button"
              onClick={copy}
              className="inline-flex items-center gap-1 border border-border-strong rounded-md px-3 py-2 text-sm hover:bg-surface-1"
            >
              <Copy className="w-4 h-4" /> {copied ? t('calendarLane.feed.copied') : t('calendarLane.feed.copy')}
            </button>
          </div>
          <p className="text-xs text-text-tertiary">{t('calendarLane.feed.private')}</p>
          <div className="flex flex-wrap gap-3 text-sm">
            <a href={feed.webcal_url} className="text-brand-600 hover:underline">{t('calendarLane.feed.openInApp')}</a>
            <button type="button" onClick={reset} disabled={busy} className="inline-flex items-center gap-1 text-text-secondary hover:text-text-primary disabled:opacity-50">
              <RefreshCw className="w-3.5 h-3.5" /> {t('calendarLane.feed.reset')}
            </button>
            <button type="button" onClick={revoke} disabled={busy} className="text-accent-danger hover:underline disabled:opacity-50">
              {t('calendarLane.feed.revoke')}
            </button>
          </div>
        </div>
      ) : feed?.exists ? (
        <div className="mt-3 space-y-2">
          <p className="text-sm text-text-secondary">{t('calendarLane.feed.hidden')}</p>
          <div className="flex gap-3">
            <button type="button" onClick={create} disabled={busy} className="bg-brand-600 text-on-brand px-3 py-1.5 rounded-md text-sm hover:bg-brand-700 disabled:opacity-50">
              {t('calendarLane.feed.reset')}
            </button>
            <button type="button" onClick={revoke} disabled={busy} className="text-accent-danger text-sm hover:underline disabled:opacity-50">
              {t('calendarLane.feed.revoke')}
            </button>
          </div>
        </div>
      ) : (
        <div className="mt-3">
          <button type="button" onClick={create} disabled={busy} className="bg-brand-600 text-on-brand px-3 py-1.5 rounded-md text-sm hover:bg-brand-700 disabled:opacity-50">
            {t('calendarLane.feed.create')}
          </button>
        </div>
      )}

      {onDownload && (
        <div className="mt-3 pt-3 border-t border-border-subtle">
          <button type="button" onClick={onDownload} className="text-sm text-brand-600 hover:underline">
            {t('calendarLane.feed.download')}
          </button>
        </div>
      )}
    </section>
  );
};

export default CalendarFeedPanel;
