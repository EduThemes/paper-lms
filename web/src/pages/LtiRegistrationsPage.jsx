import React, { useEffect, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Puzzle, Plus, Pencil, Trash2, Link2, RotateCw, X, AlertTriangle, Copy, Check } from 'lucide-react';
import Layout from '../components/Layout';
import useAccountId from '../hooks/useAccountId';
import { useAuth } from '../contexts/AuthContext';
import {
  ltiApi,
  ltiQueryKeys,
  LTI_PLACEMENTS,
  LTI_SCOPES,
  LTI_PRIVACY_LEVELS,
  buildRegistrationPayload,
  registrationToForm,
  dynamicRegistrationUrl,
} from '../services/api_lti';

const inputCls =
  'w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500';
const labelCls = 'block text-sm font-medium text-text-secondary mb-1';

function emptyForm() {
  return registrationToForm(null);
}

function CopyValue({ value }) {
  const [copied, setCopied] = useState(false);
  if (!value) return null;
  return (
    <span className="inline-flex items-center gap-1">
      <code className="break-all rounded bg-surface-1 px-1.5 py-0.5 text-xs">{value}</code>
      <button
        type="button"
        className="text-text-tertiary hover:text-brand-600"
        aria-label="Copy"
        onClick={() => {
          navigator.clipboard?.writeText(value).then(() => {
            setCopied(true);
            setTimeout(() => setCopied(false), 1500);
          }).catch(() => {});
        }}
      >
        {copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
      </button>
    </span>
  );
}

function PlatformCard({ accountId, isSuperAdmin, onError }) {
  const qc = useQueryClient();
  const query = useQuery({ queryKey: ltiQueryKeys.platform(accountId), queryFn: () => ltiApi.platformInfo(accountId), enabled: Boolean(accountId) });
  const rotate = useMutation({
    mutationFn: () => ltiApi.rotatePlatformKey(),
    onSuccess: () => qc.invalidateQueries({ queryKey: ltiQueryKeys.platform(accountId) }),
    onError: (e) => onError(e.message),
  });
  const p = query.data;
  if (!p) return null;
  const rows = [
    ['Issuer', p.issuer],
    ['OIDC authorization endpoint', p.authorization_endpoint],
    ['Token endpoint', p.token_endpoint],
    ['Public JWKS', p.jwks_uri],
    ['Deep linking return URL', p.deep_link_return_url],
    ['OpenID configuration', p.openid_configuration],
  ];
  return (
    <section className="mb-6 rounded-lg bg-surface-0 p-6 shadow" aria-labelledby="lti-platform-heading">
      <h2 id="lti-platform-heading" className="mb-1 font-semibold text-text-primary">Platform details</h2>
      <p className="mb-4 text-sm text-text-secondary">Give these to the tool vendor when registering Paper as an LTI 1.3 platform.</p>
      <dl className="grid grid-cols-1 gap-x-6 gap-y-2 text-sm md:grid-cols-[max-content_1fr]">
        {rows.map(([k, v]) => (
          <React.Fragment key={k}>
            <dt className="text-text-tertiary">{k}</dt>
            <dd><CopyValue value={v} /></dd>
          </React.Fragment>
        ))}
        <dt className="text-text-tertiary">Signing key</dt>
        <dd className="flex flex-wrap items-center gap-2">
          <CopyValue value={p.keys?.current_kid} />
          <span className="text-xs text-text-tertiary">({p.keys?.published_kids?.length || 1} published)</span>
          {isSuperAdmin && p.keys?.rotatable && (
            <button
              type="button"
              disabled={rotate.isPending}
              onClick={() => {
                if (window.confirm('Rotate the platform signing key? The upcoming key starts signing launches; the current one stays published until the next rotation.')) rotate.mutate();
              }}
              className="inline-flex items-center gap-1 rounded-md border border-border-strong px-2 py-1 text-xs hover:bg-surface-1 disabled:opacity-50"
            >
              <RotateCw className="h-3.5 w-3.5" />
              {rotate.isPending ? 'Rotating…' : 'Rotate key'}
            </button>
          )}
        </dd>
      </dl>
    </section>
  );
}

function RegistrationForm({ initial, saving, onSubmit, onCancel }) {
  const [form, setForm] = useState(initial);
  useEffect(() => setForm(initial), [initial]);
  const set = (k) => (e) => setForm((f) => ({ ...f, [k]: e.target.value }));
  const setPlacement = (i, patch) =>
    setForm((f) => ({ ...f, placements: f.placements.map((p, j) => (i === j ? { ...p, ...patch } : p)) }));
  const toggleScope = (scope) =>
    setForm((f) => ({ ...f, scopes: f.scopes.includes(scope) ? f.scopes.filter((s) => s !== scope) : [...f.scopes, scope] }));

  return (
    <form
      className="space-y-5"
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit(form);
      }}
    >
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div>
          <label className={labelCls} htmlFor="lti-title">Tool name <span className="text-accent-danger">*</span></label>
          <input id="lti-title" required className={inputCls} value={form.title} onChange={set('title')} />
        </div>
        <div>
          <label className={labelCls} htmlFor="lti-email">Contact email</label>
          <input id="lti-email" type="email" className={inputCls} value={form.email} onChange={set('email')} />
        </div>
        <div>
          <label className={labelCls} htmlFor="lti-target">Target link URI <span className="text-accent-danger">*</span></label>
          <input id="lti-target" required type="url" className={inputCls} value={form.target_link_uri} onChange={set('target_link_uri')} placeholder="https://tool.example.com/lti/launch" />
        </div>
        <div>
          <label className={labelCls} htmlFor="lti-oidc">OpenID Connect login URL <span className="text-accent-danger">*</span></label>
          <input id="lti-oidc" required type="url" className={inputCls} value={form.oidc_initiation_url} onChange={set('oidc_initiation_url')} placeholder="https://tool.example.com/lti/login" />
        </div>
        <div>
          <label className={labelCls} htmlFor="lti-jwks">Public JWK set URL</label>
          <input id="lti-jwks" type="url" className={inputCls} value={form.public_jwk_url} onChange={set('public_jwk_url')} placeholder="https://tool.example.com/.well-known/jwks.json" />
          <p className="mt-1 text-xs text-text-tertiary">Or paste the tool's public key below. One of the two is required.</p>
        </div>
        <div>
          <label className={labelCls} htmlFor="lti-privacy">Privacy level</label>
          <select id="lti-privacy" className={inputCls} value={form.privacy_level} onChange={set('privacy_level')}>
            {LTI_PRIVACY_LEVELS.map((p) => (
              <option key={p.value} value={p.value}>{p.label}</option>
            ))}
          </select>
        </div>
        <div>
          <label className={labelCls} htmlFor="lti-domain">Domain</label>
          <input id="lti-domain" className={inputCls} value={form.domain} onChange={set('domain')} placeholder="tool.example.com" />
        </div>
        <div>
          <label className={labelCls} htmlFor="lti-icon">Icon URL</label>
          <input id="lti-icon" type="url" className={inputCls} value={form.icon_url} onChange={set('icon_url')} />
        </div>
      </div>
      <div>
        <label className={labelCls} htmlFor="lti-desc">Description</label>
        <textarea id="lti-desc" rows={2} className={inputCls} value={form.description} onChange={set('description')} />
      </div>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div>
          <label className={labelCls} htmlFor="lti-redirects">Redirect URIs (one per line)</label>
          <textarea id="lti-redirects" rows={3} className={`${inputCls} font-mono`} value={form.redirect_uris} onChange={set('redirect_uris')} />
          <p className="mt-1 text-xs text-text-tertiary">The target link URI is always allowed.</p>
        </div>
        <div>
          <label className={labelCls} htmlFor="lti-jwk">Public JWK (JSON)</label>
          <textarea id="lti-jwk" rows={3} className={`${inputCls} font-mono`} value={form.public_jwk} onChange={set('public_jwk')} placeholder='{"kty":"RSA","kid":"…","n":"…","e":"AQAB"}' />
        </div>
      </div>
      <div>
        <label className={labelCls} htmlFor="lti-custom">Custom fields (JSON object)</label>
        <textarea id="lti-custom" rows={3} className={`${inputCls} font-mono`} value={form.custom_fields} onChange={set('custom_fields')} placeholder={'{\n  "user_id": "$Canvas.user.id",\n  "course": "$Context.title"\n}'} />
        <p className="mt-1 text-xs text-text-tertiary">Values starting with $ are substitution variables, e.g. $Canvas.user.id, $Person.name.full, $Context.title.</p>
      </div>

      <fieldset>
        <legend className={labelCls}>Placements</legend>
        <div className="space-y-2">
          {form.placements.map((p, i) => {
            const meta = LTI_PLACEMENTS.find((x) => x.value === p.placement);
            return (
              <div key={p.placement} className="rounded-md border border-border-default p-3">
                <label className="flex items-center gap-2 text-sm font-medium text-text-primary">
                  <input type="checkbox" checked={p.enabled} onChange={(e) => setPlacement(i, { enabled: e.target.checked })} />
                  {meta?.label || p.placement}
                </label>
                {p.enabled && (
                  <div className="mt-2 grid grid-cols-1 gap-2 md:grid-cols-3">
                    <input aria-label={`${meta?.label} label`} className={inputCls} placeholder="Label shown to users" value={p.text} onChange={(e) => setPlacement(i, { text: e.target.value })} />
                    <select aria-label={`${meta?.label} message type`} className={inputCls} value={p.message_type || meta?.messageType} onChange={(e) => setPlacement(i, { message_type: e.target.value })}>
                      <option value="LtiResourceLinkRequest">Resource link launch</option>
                      <option value="LtiDeepLinkingRequest">Deep linking (content selection)</option>
                    </select>
                    <input aria-label={`${meta?.label} target link URI`} className={inputCls} type="url" placeholder="Target link URI (optional)" value={p.target_link_uri} onChange={(e) => setPlacement(i, { target_link_uri: e.target.value })} />
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </fieldset>

      <fieldset>
        <legend className={labelCls}>LTI Advantage services</legend>
        <div className="grid grid-cols-1 gap-1 md:grid-cols-2">
          {LTI_SCOPES.map((s) => (
            <label key={s.value} className="flex items-center gap-2 text-sm text-text-primary">
              <input type="checkbox" checked={form.scopes.includes(s.value)} onChange={() => toggleScope(s.value)} />
              {s.label}
            </label>
          ))}
        </div>
      </fieldset>

      <div className="flex gap-3">
        <button type="submit" disabled={saving} className="rounded-md bg-brand-600 px-4 py-2 text-sm text-on-brand hover:bg-brand-700 disabled:opacity-50">
          {saving ? 'Saving…' : 'Save tool'}
        </button>
        <button type="button" onClick={onCancel} className="px-4 py-2 text-sm text-text-tertiary hover:text-text-secondary">Cancel</button>
      </div>
    </form>
  );
}

function DynamicRegistrationDialog({ accountId, onDone, onClose, onError }) {
  const [toolUrl, setToolUrl] = useState('');
  const [frameSrc, setFrameSrc] = useState('');
  const [starting, setStarting] = useState(false);
  const frameRef = useRef(null);

  useEffect(() => {
    if (!frameSrc) return undefined;
    // The tool signals completion with postMessage {subject: "org.imsglobal.lti.close"}.
    const handler = (event) => {
      if (frameRef.current && event.source !== frameRef.current.contentWindow) return;
      if (event.data && event.data.subject === 'org.imsglobal.lti.close') onDone();
    };
    window.addEventListener('message', handler);
    return () => window.removeEventListener('message', handler);
  }, [frameSrc, onDone]);

  const start = async (e) => {
    e.preventDefault();
    setStarting(true);
    try {
      const tok = await ltiApi.dynamicRegistrationToken(accountId);
      setFrameSrc(dynamicRegistrationUrl(toolUrl, tok));
    } catch (err) {
      onError(err.message || 'Could not start the registration');
    } finally {
      setStarting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" role="dialog" aria-modal="true" aria-label="Register a tool by URL">
      <div className="flex w-full max-w-4xl flex-col rounded-lg bg-surface-0 shadow-xl">
        <div className="flex items-center justify-between border-b border-border-default px-4 py-3">
          <h2 className="font-semibold text-text-primary">Register a tool by URL (Dynamic Registration)</h2>
          <button type="button" onClick={onClose} aria-label="Close" className="p-1 text-text-tertiary hover:text-text-primary"><X className="h-4 w-4" /></button>
        </div>
        <div className="p-4">
          {!frameSrc ? (
            <form onSubmit={start} className="space-y-3">
              <label className={labelCls} htmlFor="lti-dr-url">Tool registration URL</label>
              <input id="lti-dr-url" required type="url" className={inputCls} value={toolUrl} onChange={(e) => setToolUrl(e.target.value)} placeholder="https://tool.example.com/lti/register" />
              <p className="text-xs text-text-tertiary">The tool registers itself with Paper and is deployed to every course of this account. You can change its placements afterwards.</p>
              <button type="submit" disabled={starting} className="rounded-md bg-brand-600 px-4 py-2 text-sm text-on-brand hover:bg-brand-700 disabled:opacity-50">
                {starting ? 'Starting…' : 'Start registration'}
              </button>
            </form>
          ) : (
            <iframe ref={frameRef} src={frameSrc} title="Tool registration" className="h-[560px] w-full rounded-md border border-border-default" />
          )}
        </div>
      </div>
    </div>
  );
}

function AccountLti11Tools({ accountId, onError }) {
  const qc = useQueryClient();
  const query = useQuery({ queryKey: ltiQueryKeys.accountTools(accountId), queryFn: () => ltiApi.listAccountTools(accountId), enabled: Boolean(accountId) });
  const [showForm, setShowForm] = useState(false);
  const invalidate = () => qc.invalidateQueries({ queryKey: ltiQueryKeys.accountTools(accountId) });
  const create = useMutation({
    mutationFn: (tool) => ltiApi.createAccountTool(accountId, tool),
    onSuccess: () => {
      setShowForm(false);
      invalidate();
    },
    onError: (e) => onError(e.message),
  });
  const remove = useMutation({ mutationFn: (id) => ltiApi.deleteAccountTool(accountId, id), onSuccess: invalidate, onError: (e) => onError(e.message) });
  const tools = (query.data || []).filter((t) => t.lti_version === '1.1');

  return (
    <section className="mt-8 rounded-lg bg-surface-0 p-6 shadow" aria-labelledby="lti11-heading">
      <div className="mb-3 flex items-center justify-between">
        <div>
          <h2 id="lti11-heading" className="font-semibold text-text-primary">LTI 1.1 tools</h2>
          <p className="text-sm text-text-secondary">Legacy tools launched with an OAuth 1.0a consumer key and secret, available in every course.</p>
        </div>
        <button type="button" onClick={() => setShowForm((s) => !s)} className="inline-flex items-center gap-1 rounded-md border border-border-strong px-3 py-1.5 text-sm hover:bg-surface-1">
          <Plus className="h-4 w-4" /> Add LTI 1.1 tool
        </button>
      </div>
      {showForm && (
        <form
          className="mb-4 grid grid-cols-1 gap-3 md:grid-cols-2"
          onSubmit={(e) => {
            e.preventDefault();
            const fd = new FormData(e.target);
            create.mutate({
              name: fd.get('name'),
              url: fd.get('url'),
              consumer_key: fd.get('consumer_key'),
              shared_secret: fd.get('shared_secret'),
              privacy_level: fd.get('privacy_level'),
            });
          }}
        >
          <input name="name" required aria-label="Tool name" placeholder="Tool name" className={inputCls} />
          <input name="url" required type="url" aria-label="Launch URL" placeholder="https://tool.example.com/lti" className={inputCls} />
          <input name="consumer_key" required aria-label="Consumer key" placeholder="Consumer key" className={inputCls} />
          <input name="shared_secret" required type="password" aria-label="Shared secret" placeholder="Shared secret" className={inputCls} autoComplete="new-password" />
          <select name="privacy_level" aria-label="Privacy level" className={inputCls} defaultValue="anonymous">
            {LTI_PRIVACY_LEVELS.map((p) => <option key={p.value} value={p.value}>{p.label}</option>)}
          </select>
          <button type="submit" disabled={create.isPending} className="rounded-md bg-brand-600 px-4 py-2 text-sm text-on-brand hover:bg-brand-700 disabled:opacity-50">
            {create.isPending ? 'Adding…' : 'Add tool'}
          </button>
        </form>
      )}
      {tools.length === 0 ? (
        <p className="text-sm text-text-tertiary">No LTI 1.1 tools.</p>
      ) : (
        <ul className="divide-y divide-border-default">
          {tools.map((t) => (
            <li key={t.id} className="flex items-center justify-between py-2 text-sm">
              <span>
                <span className="font-medium text-text-primary">{t.name}</span>
                <span className="ml-2 text-text-tertiary">{t.url}</span>
              </span>
              <button type="button" aria-label={`Remove ${t.name}`} onClick={() => window.confirm(`Remove ${t.name}?`) && remove.mutate(t.id)} className="p-1 text-accent-danger">
                <Trash2 className="h-4 w-4" />
              </button>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

/**
 * Admin page for LTI 1.3 tools (G123, G126, G132, G134): register tools
 * manually or by Dynamic Registration, edit placements / privacy /
 * services, deploy a tool to every course of the account, and read the
 * platform details a vendor needs.
 */
export default function LtiRegistrationsPage() {
  const accountId = useAccountId();
  const { user } = useAuth();
  const qc = useQueryClient();
  const [editing, setEditing] = useState(null); // null | 'new' | registration
  const [showDynamic, setShowDynamic] = useState(false);
  const [error, setError] = useState(null);

  const query = useQuery({
    queryKey: ltiQueryKeys.registrations(accountId),
    queryFn: () => ltiApi.listRegistrations(accountId),
    enabled: Boolean(accountId),
  });
  const invalidate = () => qc.invalidateQueries({ queryKey: ltiQueryKeys.registrations(accountId) });

  const save = useMutation({
    mutationFn: ({ id, form }) => {
      const payload = buildRegistrationPayload(form);
      return id ? ltiApi.updateRegistration(accountId, id, payload) : ltiApi.createRegistration(accountId, payload);
    },
    onSuccess: () => {
      setEditing(null);
      invalidate();
    },
    onError: (e) => setError(e.message),
  });
  const remove = useMutation({ mutationFn: (id) => ltiApi.deleteRegistration(accountId, id), onSuccess: invalidate, onError: (e) => setError(e.message) });
  const install = useMutation({
    mutationFn: ({ id, enabled }) => ltiApi.setAccountInstall(accountId, id, enabled),
    onSuccess: invalidate,
    onError: (e) => setError(e.message),
  });
  const toggleState = useMutation({
    mutationFn: (reg) => {
      const form = registrationToForm(reg);
      form.workflow_state = reg.workflow_state === 'active' ? 'inactive' : 'active';
      return ltiApi.updateRegistration(accountId, reg.id, buildRegistrationPayload(form));
    },
    onSuccess: invalidate,
    onError: (e) => setError(e.message),
  });

  const regs = query.data || [];
  const formInitial = editing === 'new' ? emptyForm() : editing ? registrationToForm(editing) : null;

  return (
    <Layout>
      <div className="p-8">
        <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 className="text-2xl font-bold text-text-primary">LTI Tools</h1>
            <p className="mt-1 text-text-secondary">Register LTI 1.3 tools and deploy them to courses.</p>
          </div>
          <div className="flex gap-2">
            <button type="button" onClick={() => setShowDynamic(true)} className="inline-flex items-center gap-2 rounded-md border border-border-strong px-4 py-2 text-sm hover:bg-surface-1">
              <Link2 className="h-4 w-4" /> Register by URL
            </button>
            <button type="button" onClick={() => setEditing('new')} className="inline-flex items-center gap-2 rounded-md bg-brand-600 px-4 py-2 text-sm text-on-brand hover:bg-brand-700">
              <Plus className="h-4 w-4" /> Register tool
            </button>
          </div>
        </div>

        {error && (
          <div role="alert" className="mb-6 flex items-center gap-2 rounded-md border border-accent-danger/30 bg-accent-danger/10 px-4 py-3 text-accent-danger">
            <AlertTriangle className="h-4 w-4 flex-shrink-0" />
            {error}
            <button type="button" className="ml-auto" aria-label="Dismiss" onClick={() => setError(null)}><X className="h-4 w-4" /></button>
          </div>
        )}

        {formInitial && (
          <section className="mb-6 rounded-lg bg-surface-0 p-6 shadow">
            <h2 className="mb-4 font-semibold text-text-primary">{editing === 'new' ? 'Register an LTI 1.3 tool' : `Edit ${editing.name}`}</h2>
            <RegistrationForm
              initial={formInitial}
              saving={save.isPending}
              onCancel={() => setEditing(null)}
              onSubmit={(form) => {
                setError(null);
                try {
                  buildRegistrationPayload(form);
                } catch (e) {
                  setError(e.message);
                  return;
                }
                save.mutate({ id: editing === 'new' ? null : editing.id, form });
              }}
            />
          </section>
        )}

        {accountId && <PlatformCard accountId={accountId} isSuperAdmin={user?.role === 'super_admin'} onError={setError} />}

        <section className="overflow-hidden rounded-lg bg-surface-0 shadow" aria-label="Registered tools">
          {query.isLoading ? (
            <p className="p-6 text-sm text-text-tertiary">Loading tools…</p>
          ) : query.isError ? (
            <p className="p-6 text-sm text-accent-danger">{query.error?.message || 'Could not load tools'}</p>
          ) : regs.length === 0 ? (
            <div className="p-12 text-center">
              <Puzzle className="mx-auto mb-4 h-12 w-12 text-text-tertiary" />
              <p className="text-text-tertiary">No LTI 1.3 tools registered yet.</p>
            </div>
          ) : (
            <table className="min-w-full divide-y divide-border-default">
              <thead className="bg-surface-1">
                <tr>
                  {['Tool', 'Client ID', 'Placements', 'Privacy', 'Status', 'All courses', ''].map((h) => (
                    <th key={h} className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-text-tertiary">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y divide-border-default">
                {regs.map((reg) => (
                  <tr key={reg.id}>
                    <td className="px-4 py-3 text-sm font-medium text-text-primary">{reg.tool_configuration?.title || reg.name}</td>
                    <td className="px-4 py-3 text-sm"><CopyValue value={reg.client_id} /></td>
                    <td className="px-4 py-3 text-xs text-text-secondary">
                      {(reg.tool_configuration?.placements || []).map((p) => p.placement).join(', ') || '—'}
                    </td>
                    <td className="px-4 py-3 text-xs text-text-secondary">{reg.tool_configuration?.privacy_level}</td>
                    <td className="px-4 py-3">
                      <button
                        type="button"
                        onClick={() => toggleState.mutate(reg)}
                        className={`rounded-full px-2 py-1 text-xs ${reg.workflow_state === 'active' ? 'bg-accent-success/20 text-accent-success' : 'bg-surface-2 text-text-secondary'}`}
                        aria-label={`${reg.workflow_state === 'active' ? 'Disable' : 'Enable'} ${reg.name}`}
                      >
                        {reg.workflow_state}
                      </button>
                    </td>
                    <td className="px-4 py-3 text-sm">
                      <label className="flex items-center gap-2">
                        <input
                          type="checkbox"
                          checked={Boolean(reg.account_install)}
                          onChange={(e) => install.mutate({ id: reg.id, enabled: e.target.checked })}
                          aria-label={`Deploy ${reg.name} to every course`}
                        />
                        {reg.deployment_id ? <span className="text-xs text-text-tertiary">deployment {reg.deployment_id}</span> : null}
                      </label>
                    </td>
                    <td className="px-4 py-3 text-right">
                      <button type="button" aria-label={`Edit ${reg.name}`} onClick={() => setEditing(reg)} className="p-1 text-text-tertiary hover:text-brand-600"><Pencil className="h-4 w-4" /></button>
                      <button
                        type="button"
                        aria-label={`Delete ${reg.name}`}
                        onClick={() => window.confirm(`Delete ${reg.name}? Every launch and deployment of it stops working.`) && remove.mutate(reg.id)}
                        className="p-1 text-accent-danger"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>

        {accountId && <AccountLti11Tools accountId={accountId} onError={setError} />}

        {showDynamic && accountId && (
          <DynamicRegistrationDialog
            accountId={accountId}
            onError={setError}
            onClose={() => setShowDynamic(false)}
            onDone={() => {
              setShowDynamic(false);
              invalidate();
            }}
          />
        )}
      </div>
    </Layout>
  );
}
