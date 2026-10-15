import React, { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Loader2 } from 'lucide-react';
import { useAccountSettings, useSetAccountSetting } from '../../services/adminAccountsQueries';
import { Section, Button, ErrorNote } from './AdminUI';

// The Settings Engine key (internal/service/settings/catalog.go). It is
// "off anywhere above wins": the instance or a parent account turning it
// off locks it off here (GET /accounts/:account_id/settings → locked_by).
export const PAGE_CSS_KEY = 'pages.custom_css_enabled';

// Pages section of /admin/settings: whether teachers may write their own
// page CSS in this account. Designed pages keep their generated CSS either
// way; the server enforces the setting on every page write path.
export default function PageCssSettings() {
  const { t } = useTranslation();
  const query = useAccountSettings();
  const [status, setStatus] = useState('');
  const timer = useRef(null);
  const save = useSetAccountSetting({
    onSuccess: () => {
      setStatus(t('adminSettingsPage.pages.saved'));
      clearTimeout(timer.current);
      timer.current = setTimeout(() => setStatus(''), 2500);
    },
  });
  useEffect(() => () => clearTimeout(timer.current), []);

  const setting = Array.isArray(query.data) ? query.data.find((s) => s.key === PAGE_CSS_KEY) : null;

  const write = (value) => {
    setStatus('');
    save.mutate({ key: PAGE_CSS_KEY, value });
  };

  let body;
  if (query.isLoading) {
    body = (
      <Loader2
        className="h-5 w-5 animate-spin motion-reduce:animate-none text-text-tertiary"
        aria-label={t('adminSettingsPage.pages.loading')}
        role="img"
      />
    );
  } else if (query.isError || !setting) {
    body = (
      <div role="alert" className="flex items-center gap-3 text-sm text-accent-danger">
        <span>{t('adminSettingsPage.pages.loadError')}</span>
        <Button variant="secondary" onClick={() => query.refetch()}>
          {t('adminSettingsPage.pages.retry')}
        </Button>
      </div>
    );
  } else {
    const on = setting.effective === true;
    const locked = Boolean(setting.locked);
    const explicit = setting.value !== null && setting.value !== undefined;
    let lockText = '';
    if (locked) {
      const by = setting.locked_by || {};
      if (by.scope === 'account' && by.account_name) {
        lockText = t('adminSettingsPage.pages.lockedByAccount', { name: by.account_name });
      } else if (by.scope === 'account') {
        lockText = t('adminSettingsPage.pages.lockedByParent');
      } else {
        lockText = t('adminSettingsPage.pages.lockedByInstance');
      }
    }
    const stateWord = on ? t('adminSettingsPage.pages.on') : t('adminSettingsPage.pages.off');
    const describedBy = ['page-css-help', locked ? 'page-css-lock' : null].filter(Boolean).join(' ');

    body = (
      <div className="space-y-3">
        <div className="flex items-start gap-3">
          <button
            type="button"
            role="switch"
            id="page-css-switch"
            aria-checked={on}
            aria-labelledby="page-css-label"
            aria-describedby={describedBy}
            disabled={locked || save.isPending}
            onClick={() => write(!on)}
            className={`relative mt-0.5 inline-flex h-6 w-11 shrink-0 items-center rounded-full border-2 border-transparent transition-colors motion-reduce:transition-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60 ${
              on ? 'bg-brand-600' : 'bg-text-tertiary'
            }`}
          >
            <span
              aria-hidden="true"
              className={`inline-block h-5 w-5 rounded-full bg-white shadow ring-1 ring-black/10 transition-transform motion-reduce:transition-none ${
                on ? 'translate-x-5' : 'translate-x-0'
              }`}
            />
          </button>
          <div className="min-w-0">
            <span id="page-css-label" className="block text-sm font-medium text-text-primary">
              {t('adminSettingsPage.pages.cssLabel')}
            </span>
            <span className="block text-xs text-text-secondary" aria-hidden="true">
              {stateWord}
            </span>
          </div>
        </div>

        <p id="page-css-help" className="text-sm text-text-secondary">
          {t('adminSettingsPage.pages.cssHelp')}
        </p>

        {locked && (
          <p id="page-css-lock" className="text-sm font-medium text-text-primary">
            {lockText}
          </p>
        )}

        {!locked && (
          <p className="text-xs text-text-tertiary">
            {explicit
              ? t('adminSettingsPage.pages.explicit')
              : t('adminSettingsPage.pages.inherited', { state: stateWord })}
          </p>
        )}

        {explicit && (
          <Button variant="secondary" disabled={save.isPending} onClick={() => write(null)}>
            {t('adminSettingsPage.pages.useDefault')}
          </Button>
        )}

        <ErrorNote error={save.error} />
      </div>
    );
  }

  return (
    <Section title={t('adminSettingsPage.pages.title')} description={t('adminSettingsPage.pages.description')}>
      {body}
      <p role="status" aria-live="polite" className="mt-2 min-h-[1rem] text-xs text-text-secondary">
        {save.isPending ? t('adminSettingsPage.pages.saving') : status}
      </p>
    </Section>
  );
}
