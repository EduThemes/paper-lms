import React, { useEffect, useMemo, useState } from 'react';
import { Save, Trash2, Eye, EyeOff, Send, RefreshCw, AlertTriangle, CheckCircle2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import Layout from '../components/Layout';
import { api } from '../services/api';

/**
 * SuperAdminSettingsPage — the operator-facing UI for the Settings
 * Engine. Wave 4 deliverable.
 *
 * Renders left-nav by Group (from /superadmin/settings/groups) and a
 * per-group editor (from /superadmin/settings). Each row carries its
 * Source ("instance" / "env" / "default" / etc) so operators see
 * WHERE the live value came from without having to remember
 * resolution order.
 *
 * SECURITY CONTRACT (mirrors the server-side contract in
 * internal/api/v1/handlers/super_admin_settings.go):
 *
 *   - Secret values are ALWAYS rendered as "Set ✓" / "Unset" never as
 *     their plaintext. The server omits the `value` field when
 *     is_secret=true; we never display anything we shouldn't have.
 *   - Test-action buttons send NO credentials — the server reads the
 *     current effective settings and tests against them. The UI does
 *     not echo any returned secrets; only the diagnostic detail
 *     (no-secret-in-payload by server-side contract).
 *   - Clear is gated behind a confirm modal that names exactly what
 *     gets discarded — Wave 3 audit H2 made the server require an
 *     explicit scope, mirrored here.
 */

const GROUP_ORDER = [
  'Email',
  'File storage',
  'AI (Anthropic)',
  'Federated auth',
  'Passkeys',
  'Branding',
  'Quotas & limits',
];

const SOURCE_LABEL_KEYS = {
  user: 'superAdminSettings.sourceUser',
  account: 'superAdminSettings.sourceAccount',
  instance: 'superAdminSettings.sourceInstance',
  env: 'superAdminSettings.sourceEnv',
  default: 'superAdminSettings.sourceDefault',
  none: 'superAdminSettings.sourceNone',
};

const SOURCE_HINT_KEYS = {
  env: 'superAdminSettings.hintEnv',
  default: 'superAdminSettings.hintDefault',
  none: 'superAdminSettings.hintNone',
};

export default function SuperAdminSettingsPage() {
  const { t } = useTranslation();
  const [defs, setDefs] = useState([]);
  const [byKey, setByKey] = useState({});
  const [activeGroup, setActiveGroup] = useState('Email');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [scopeForWrite, setScopeForWrite] = useState('instance');
  const [accountIdForWrite, setAccountIdForWrite] = useState('');

  const loadAll = async () => {
    setLoading(true);
    setError(null);
    try {
      const [g, s] = await Promise.all([
        api.superAdminSettings.getGroups(),
        api.superAdminSettings.listSettings(),
      ]);
      setDefs(g.definitions || []);
      const map = {};
      (s.settings || []).forEach((row) => { map[row.key] = row; });
      setByKey(map);
    } catch (e) {
      setError(e.message || t('superAdminSettings.loadError'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { loadAll(); }, []);

  // Group definitions by Group field, preserving GROUP_ORDER for nav order.
  const grouped = useMemo(() => {
    const byGroup = {};
    defs.forEach((d) => {
      if (!byGroup[d.group]) byGroup[d.group] = [];
      byGroup[d.group].push(d);
    });
    const ordered = [];
    GROUP_ORDER.forEach((name) => {
      if (byGroup[name]) ordered.push([name, byGroup[name]]);
    });
    // Any group not in GROUP_ORDER (future additions) lands at the end.
    Object.keys(byGroup).forEach((name) => {
      if (!GROUP_ORDER.includes(name)) ordered.push([name, byGroup[name]]);
    });
    return ordered;
  }, [defs]);

  const activeDefs = useMemo(() => {
    const found = grouped.find(([name]) => name === activeGroup);
    return found ? found[1] : [];
  }, [grouped, activeGroup]);

  return (
    <Layout>
      <div className="p-6 max-w-7xl mx-auto">
        <header className="mb-6">
          <div className="flex items-center gap-2 text-amber-600 text-sm font-medium mb-1">
            <AlertTriangle size={16} />
            <span>{t('superAdminSettings.operatorMode')}</span>
          </div>
          <h1 className="text-2xl font-semibold text-text-primary">{t('superAdminSettings.title')}</h1>
          <p className="text-text-secondary text-sm mt-1">
            {t('superAdminSettings.subtitle')}
          </p>
        </header>

        {error && (
          <div role="alert" className="mb-4 p-3 rounded border border-red-400 bg-red-50 text-red-800">
            {error}
          </div>
        )}

        <div className="mb-4 p-4 rounded border border-amber-200 bg-amber-50">
          <div className="text-sm font-medium text-amber-900 mb-2">{t('superAdminSettings.writeScope')}</div>
          <div className="flex flex-wrap gap-3 items-center text-sm">
            <label className="flex items-center gap-2">
              <input
                type="radio" name="scope" value="instance"
                checked={scopeForWrite === 'instance'}
                onChange={(e) => setScopeForWrite(e.target.value)}
              />
              <span>{t('superAdminSettings.instanceAllTenants')}</span>
            </label>
            <label className="flex items-center gap-2">
              <input
                type="radio" name="scope" value="account"
                checked={scopeForWrite === 'account'}
                onChange={(e) => setScopeForWrite(e.target.value)}
              />
              <span>{t('superAdminSettings.accountIdLabel')}</span>
              <input
                type="number" min="1"
                className="border rounded px-2 py-1 w-24 text-sm"
                placeholder="42"
                value={accountIdForWrite}
                onChange={(e) => setAccountIdForWrite(e.target.value)}
                aria-label={t('superAdminSettings.accountIdAria')}
              />
            </label>
          </div>
        </div>

        <div className="grid grid-cols-12 gap-6">
          <nav aria-label={t('superAdminSettings.groupsNavAria')} className="col-span-12 md:col-span-3">
            <ul className="space-y-1">
              {grouped.map(([name, items]) => (
                <li key={name}>
                  <button
                    type="button"
                    onClick={() => setActiveGroup(name)}
                    className={`w-full text-left px-3 py-2 rounded text-sm ${
                      activeGroup === name
                        ? 'bg-surface-3 text-text-primary font-medium'
                        : 'text-text-secondary hover:bg-surface-2'
                    }`}
                    aria-current={activeGroup === name ? 'true' : undefined}
                  >
                    {name} <span className="text-xs opacity-60">({items.length})</span>
                  </button>
                </li>
              ))}
            </ul>
          </nav>

          <main className="col-span-12 md:col-span-9">
            {loading ? (
              <div className="text-text-secondary">{t('common.loading')}</div>
            ) : (
              <div className="space-y-4">
                {activeDefs.map((def) => (
                  <SettingRow
                    key={def.key}
                    def={def}
                    value={byKey[def.key]}
                    scope={scopeForWrite}
                    scopeID={scopeForWrite === 'account' ? Number(accountIdForWrite) || 0 : 0}
                    onChanged={loadAll}
                  />
                ))}
              </div>
            )}
          </main>
        </div>
      </div>
    </Layout>
  );
}

// ── SettingRow ─────────────────────────────────────────────────────

function SettingRow({ def, value, scope, scopeID, onChanged }) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState('');
  const [showSecretInput, setShowSecretInput] = useState(false);
  const [showSecretText, setShowSecretText] = useState(false);
  const [saving, setSaving] = useState(false);
  const [confirmClear, setConfirmClear] = useState(false);
  const [actionResult, setActionResult] = useState(null);
  const [localError, setLocalError] = useState(null);

  const isSecret = def.is_secret;
  const source = value?.source || 'none';
  const sourceLabel = SOURCE_LABEL_KEYS[source] ? t(SOURCE_LABEL_KEYS[source]) : source;
  const sourceHint = SOURCE_HINT_KEYS[source] ? t(SOURCE_HINT_KEYS[source]) : null;
  const hasValue = !!value?.has_value;

  const canEditAtScope = (def.scopes || []).includes(scope);

  const handleSave = async () => {
    setLocalError(null);
    if (scope === 'account' && !scopeID) {
      setLocalError(t('superAdminSettings.chooseAccountId'));
      return;
    }
    setSaving(true);
    try {
      await api.superAdminSettings.setSetting(def.key, {
        scope,
        scope_id: scope === 'instance' ? 0 : scopeID,
        value: draft,
      });
      setDraft('');
      setShowSecretInput(false);
      await onChanged();
    } catch (e) {
      setLocalError(e.message);
    } finally {
      setSaving(false);
    }
  };

  const handleClear = async () => {
    setLocalError(null);
    if (scope === 'account' && !scopeID) {
      setLocalError(t('superAdminSettings.chooseAccountId'));
      return;
    }
    setSaving(true);
    try {
      await api.superAdminSettings.clearSetting(def.key, {
        scope,
        scope_id: scope === 'instance' ? 0 : scopeID,
      });
      setConfirmClear(false);
      await onChanged();
    } catch (e) {
      setLocalError(e.message);
    } finally {
      setSaving(false);
    }
  };

  const runTestAction = async () => {
    setActionResult(null);
    let result;
    try {
      switch (def.test_action) {
        case 'email':
          result = await api.superAdminSettings.testEmail();
          break;
        case 'anthropic':
          result = await api.superAdminSettings.testAnthropic();
          break;
        case 's3':
          result = await api.superAdminSettings.testS3();
          break;
        case 'oidc': {
          // OIDC takes the issuer from THIS setting's current value.
          // Non-secret, so reading value off the row is fine; we DO
          // NOT prompt for it inline (the issuer URL is the
          // setting being tested).
          const issuer = value?.value || '';
          if (!issuer) {
            setActionResult({ ok: false, detail: t('superAdminSettings.setOidcFirst') });
            return;
          }
          result = await api.superAdminSettings.testOIDC(issuer);
          break;
        }
        default:
          return;
      }
    } catch (e) {
      result = { ok: false, detail: e.message };
    }
    setActionResult(result);
  };

  return (
    <section className="border rounded p-4 bg-surface-1">
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div className="min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <h3 className="font-medium text-text-primary">{def.label}</h3>
            <code className="text-xs text-text-secondary bg-surface-2 px-1.5 py-0.5 rounded">
              {def.key}
            </code>
            {isSecret && (
              <span className="text-xs px-1.5 py-0.5 rounded bg-amber-100 text-amber-900">
                {t('superAdminSettings.secretBadge')}
              </span>
            )}
            <span className="text-xs px-1.5 py-0.5 rounded bg-surface-2 text-text-secondary">
              {t('superAdminSettings.sourcePrefix', { source: sourceLabel })}
            </span>
            {!canEditAtScope && (
              <span className="text-xs text-text-secondary italic">
                {t('superAdminSettings.notEditableAtScope', { scope })}
              </span>
            )}
          </div>
          {def.description && (
            <p className="text-sm text-text-secondary mt-1">{def.description}</p>
          )}
          {sourceHint && (
            <p className="text-xs text-text-secondary mt-1">{sourceHint}</p>
          )}
          {/* Live value display */}
          <div className="mt-3">
            {isSecret ? (
              hasValue ? (
                <div className="text-sm text-text-primary">
                  <CheckCircle2 size={14} className="inline mr-1 text-green-600" />
                  {t('superAdminSettings.setBadge')}
                  {value?.updated_at && (
                    <span className="text-text-secondary ml-2 text-xs">
                      {t('superAdminSettings.updatedAt', { date: new Date(value.updated_at).toLocaleString() })}
                    </span>
                  )}
                </div>
              ) : (
                <div className="text-sm text-text-secondary">{t('superAdminSettings.notSet')}</div>
              )
            ) : (
              <div className="text-sm font-mono break-all text-text-primary">
                {hasValue ? (renderNonSecret(value, def.value_type)) : (
                  <span className="text-text-secondary not-italic">{t('superAdminSettings.unsetPlaceholder')}</span>
                )}
              </div>
            )}
          </div>
        </div>

        {/* Test-action button (if applicable) */}
        {def.test_action && (
          <button
            type="button"
            onClick={runTestAction}
            className="text-sm px-3 py-1.5 rounded border border-amber-300 bg-amber-50 hover:bg-amber-100 text-amber-900 flex items-center gap-1"
            title={t('superAdminSettings.testTitle', { action: def.test_action })}
          >
            <Send size={14} /> {t('superAdminSettings.testPrefix', { action: def.test_action })}
          </button>
        )}
      </div>

      {actionResult && (
        <div
          role="status"
          className={`mt-3 p-2 rounded text-sm border ${
            actionResult.ok
              ? 'border-green-300 bg-green-50 text-green-900'
              : 'border-red-300 bg-red-50 text-red-900'
          }`}
        >
          <strong>{actionResult.ok ? t('superAdminSettings.successLabel') : t('superAdminSettings.failedLabel')}:</strong> {actionResult.detail || t('superAdminSettings.noDetail')}
          {actionResult.duration_ms ? (
            <span className="ml-2 text-xs opacity-70">{t('superAdminSettings.durationMs', { ms: actionResult.duration_ms })}</span>
          ) : null}
        </div>
      )}

      {/* Editor */}
      {canEditAtScope && (
        <div className="mt-3 pt-3 border-t border-surface-3">
          {localError && (
            <div role="alert" className="mb-2 text-sm text-red-800">{localError}</div>
          )}
          {isSecret && !showSecretInput ? (
            <div className="flex gap-2 flex-wrap">
              <button
                type="button"
                onClick={() => { setShowSecretInput(true); setDraft(''); }}
                className="text-sm px-3 py-1.5 rounded border border-surface-3 hover:bg-surface-2"
              >
                {hasValue ? t('superAdminSettings.replace') : t('superAdminSettings.setEllipsis')}
              </button>
              {hasValue && (
                <button
                  type="button"
                  onClick={() => setConfirmClear(true)}
                  className="text-sm px-3 py-1.5 rounded border border-red-300 text-red-700 hover:bg-red-50 flex items-center gap-1"
                >
                  <Trash2 size={14} /> {t('superAdminSettings.clear')}
                </button>
              )}
            </div>
          ) : (
            <SettingInput
              valueType={def.value_type}
              draft={draft}
              setDraft={setDraft}
              showText={showSecretText}
              setShowText={setShowSecretText}
              isSecret={isSecret}
            />
          )}
          {(showSecretInput || !isSecret) && (
            <div className="mt-2 flex gap-2 flex-wrap">
              <button
                type="button"
                onClick={handleSave}
                disabled={saving || draft === ''}
                className="text-sm px-3 py-1.5 rounded bg-surface-3 hover:bg-surface-4 disabled:opacity-50 flex items-center gap-1"
              >
                <Save size={14} /> {saving ? t('common.saving') : t('common.save')}
              </button>
              {hasValue && (
                <button
                  type="button"
                  onClick={() => setConfirmClear(true)}
                  disabled={saving}
                  className="text-sm px-3 py-1.5 rounded border border-red-300 text-red-700 hover:bg-red-50 flex items-center gap-1"
                >
                  <Trash2 size={14} /> {t('superAdminSettings.clearAtScope', { scope })}
                </button>
              )}
              {isSecret && (
                <button
                  type="button"
                  onClick={() => { setShowSecretInput(false); setDraft(''); }}
                  className="text-sm px-3 py-1.5 text-text-secondary hover:text-text-primary"
                >
                  {t('common.cancel')}
                </button>
              )}
            </div>
          )}
        </div>
      )}

      {confirmClear && (
        <div
          role="alertdialog"
          aria-labelledby={`clear-${def.key}-title`}
          className="mt-3 p-3 rounded border border-red-300 bg-red-50"
        >
          <div id={`clear-${def.key}-title`} className="font-medium text-red-900 mb-1">
            {t('superAdminSettings.confirmTitle', { label: def.label, scope })}
          </div>
          <p className="text-sm text-red-800 mb-2">
            {t('superAdminSettings.confirmBody', { scope })}
            {isSecret && ` ${t('superAdminSettings.confirmSecretSuffix')}`}{' '}
            {t('superAdminSettings.confirmChainNote')}
          </p>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={handleClear}
              className="text-sm px-3 py-1.5 rounded bg-red-600 text-white hover:bg-red-700"
            >
              {saving ? t('superAdminSettings.clearing') : t('superAdminSettings.yesClear')}
            </button>
            <button
              type="button"
              onClick={() => setConfirmClear(false)}
              className="text-sm px-3 py-1.5 text-red-800 hover:text-red-900"
            >
              {t('common.cancel')}
            </button>
          </div>
        </div>
      )}
    </section>
  );
}

// SettingInput renders the right form control for each value_type.
// String/Int/Secret use <input>; Bool uses <select>; JSON uses <textarea>.
function SettingInput({ valueType, draft, setDraft, showText, setShowText, isSecret }) {
  const { t } = useTranslation();
  if (valueType === 'bool') {
    return (
      <select
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        className="border rounded px-2 py-1 text-sm w-full max-w-sm"
        aria-label={t('superAdminSettings.booleanValue')}
      >
        <option value="">{t('superAdminSettings.selectOption')}</option>
        <option value="true">true</option>
        <option value="false">false</option>
      </select>
    );
  }
  if (valueType === 'json') {
    return (
      <textarea
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        rows={4}
        className="border rounded px-2 py-1 text-sm w-full font-mono"
        placeholder='{"key": "value"}'
        aria-label={t('superAdminSettings.jsonValue')}
      />
    );
  }
  // The aria-label is asserted by the SuperAdminSettingsPage tests against
  // /New string value/i — keep the English phrasing so the suite continues
  // to pass while still routing through i18n for translation.
  return (
    <div className="flex items-center gap-2 max-w-xl">
      <input
        type={isSecret && !showText ? 'password' : valueType === 'int' ? 'number' : 'text'}
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        className="border rounded px-2 py-1 text-sm flex-1"
        autoComplete="off"
        spellCheck="false"
        aria-label={t('superAdminSettings.newValueAria', { valueType })}
      />
      {isSecret && (
        <button
          type="button"
          onClick={() => setShowText((v) => !v)}
          className="text-text-secondary hover:text-text-primary"
          aria-label={showText ? t('superAdminSettings.hide') : t('superAdminSettings.show')}
        >
          {showText ? <EyeOff size={16} /> : <Eye size={16} />}
        </button>
      )}
    </div>
  );
}

// renderNonSecret formats non-secret values for display. Takes the
// FULL effective-value row (not just the string) so it can enforce
// the secret short-circuit defensively: if the row reports
// is_secret=true (server contract violation OR a future catalog/row
// drift), we render the masked placeholder regardless of what the
// catalog definition said. Belt-and-suspenders for the Wave 4 audit
// H1 finding.
function renderNonSecret(ev, valueType) {
  // Note: this helper is intentionally i18n-free so it can be called from
  // anywhere without needing a t() reference. The masked placeholder is
  // a fixed defense-in-depth sentinel, not translated UI copy.
  if (!ev) return '—';
  if (ev.is_secret) {
    // Defense-in-depth: catalog says non-secret but server marked the
    // row secret. Trust the row marker — never render the raw value.
    return '••• (secret)';
  }
  const value = ev.value;
  if (value === null || value === undefined) return '—';
  if (valueType === 'bool') return value === 'true' ? 'true' : 'false';
  if (valueType === 'json') {
    try {
      return JSON.stringify(JSON.parse(value), null, 2);
    } catch {
      return value;
    }
  }
  return value;
}
