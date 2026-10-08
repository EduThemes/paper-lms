// Theme editor (G118): an account's brand colours and logo. Colours are
// allow-listed variables applied as CSS custom properties for the light
// theme; the logo replaces the Paper mark in the navigation.

import React, { useEffect, useState } from 'react';
import { Upload, RotateCcw } from 'lucide-react';
import Layout from '../components/Layout';
import Page from '../components/Page';
import { adminUrls } from '../services/api_admin_accounts';
import { useBrandConfig, useBrandMutation } from '../services/adminAccountsQueries';
import { Section, Field, Button, ErrorNote, StatusNote, inputClass } from '../components/admin/AdminUI';
import { brandScaleFromHex, isHexColor } from '../utils/adminAccounts';

function ColorRow({ variable, value, onChange }) {
  const id = `theme-${variable.name}`;
  const valid = value === '' || isHexColor(value);
  return (
    <div className="grid grid-cols-1 md:grid-cols-[1fr_auto_10rem] gap-3 items-end">
      <Field label={variable.label} id={id} hint={variable.description}>
        <input
          id={id}
          className={`${inputClass} ${valid ? '' : 'border-accent-danger'}`}
          placeholder={variable.default || 'Default'}
          value={value}
          onChange={(e) => onChange(e.target.value.trim())}
        />
      </Field>
      <input
        type="color"
        aria-label={`${variable.label} color picker`}
        className="h-9 w-12 rounded border border-border-strong"
        value={isHexColor(value) ? value : (variable.default || '#000000')}
        onChange={(e) => onChange(e.target.value.toUpperCase())}
      />
      <Button variant="secondary" onClick={() => onChange('')}>Use default</Button>
    </div>
  );
}

function Preview({ variables }) {
  const scale = brandScaleFromHex(variables.brand_primary) || brandScaleFromHex('#1B6EB1');
  const button = variables.button_primary || variables.brand_primary || '#1B6EB1';
  const nav = variables.nav_background || '#2D3B45';
  const navText = variables.nav_text || '#FCFCFA';
  return (
    <div className="flex rounded-md border border-border-default overflow-hidden h-40" aria-label="Theme preview">
      <div className="w-16 flex flex-col items-center gap-3 pt-3" style={{ backgroundColor: nav, color: navText }}>
        <div className="w-8 h-8 rounded" style={{ backgroundColor: `rgb(${scale[600]})` }} />
        <span className="text-[10px]">Nav</span>
      </div>
      <div className="flex-1 p-4 space-y-3 bg-surface-0">
        <div className="flex gap-1">
          {Object.entries(scale).map(([step, rgb]) => (
            <div key={step} title={step} className="h-4 flex-1" style={{ backgroundColor: `rgb(${rgb})` }} />
          ))}
        </div>
        <p className="text-sm"><span style={{ color: `rgb(${scale[600]})` }} className="underline">A link</span> in body text.</p>
        <button type="button" className="rounded-md px-3 py-1.5 text-sm" style={{ backgroundColor: button, color: variables.button_primary_text || '#FFFFFF' }}>
          Primary button
        </button>
      </div>
    </div>
  );
}

const ThemeEditorPage = () => {
  const query = useBrandConfig();
  const [vars, setVars] = useState({});
  const [status, setStatus] = useState('');
  const flash = (msg) => {
    setStatus(msg);
    setTimeout(() => setStatus(''), 1500);
  };
  const mutation = useBrandMutation({ onSuccess: () => flash('Saved') });

  useEffect(() => {
    if (query.data?.config) setVars(query.data.config.variables || {});
  }, [query.data]);

  return (
    <Page query={query} title="Theme">
      {({ config, variables }) => {
        const invalid = Object.values(vars).some((v) => v && !isHexColor(v));
        return (
          <Layout>
            <div className="p-8 max-w-5xl">
              <h1 className="text-2xl font-bold text-text-primary">Theme</h1>
              <p className="text-sm text-text-secondary mt-1 mb-6">Brand colors and logo for everyone in this account. Dark mode keeps its own palette.</p>
              <ErrorNote error={mutation.error} />
              <Section
                title="Colors"
                actions={(
                  <>
                    <StatusNote message={status} />
                    <Button variant="secondary" onClick={() => mutation.mutate({ action: 'reset' })}><RotateCcw className="w-4 h-4" /> Reset theme</Button>
                    <Button disabled={invalid || mutation.isPending} onClick={() => mutation.mutate({ action: 'variables', variables: vars })}>Save colors</Button>
                  </>
                )}
              >
                <div className="space-y-4 mb-6">
                  {variables.map((v) => (
                    <ColorRow key={v.name} variable={v} value={vars[v.name] || ''} onChange={(val) => setVars({ ...vars, [v.name]: val })} />
                  ))}
                </div>
                <Preview variables={vars} />
              </Section>
              <Section title="Logo" description="PNG, JPEG, GIF or WebP, up to 512 KB. Shown in the navigation.">
                <div className="flex items-center gap-4">
                  {config.has_logo ? (
                    <img src={adminUrls.brandLogo(config.version)} alt="Account logo" className="h-12 w-12 object-contain rounded border border-border-default" />
                  ) : (
                    <span className="text-sm text-text-tertiary">Using the Paper logo.</span>
                  )}
                  <label className="inline-flex items-center gap-1.5 rounded-md border border-border-strong px-3 py-1.5 text-sm cursor-pointer hover:bg-surface-1">
                    <Upload className="w-4 h-4" /> Upload logo
                    <input
                      type="file"
                      accept="image/png,image/jpeg,image/gif,image/webp"
                      className="sr-only"
                      onChange={(e) => {
                        const file = e.target.files?.[0];
                        if (file) mutation.mutate({ action: 'logo', file });
                        e.target.value = '';
                      }}
                    />
                  </label>
                  {config.has_logo && <Button variant="danger" onClick={() => mutation.mutate({ action: 'deleteLogo' })}>Remove logo</Button>}
                </div>
              </Section>
            </div>
          </Layout>
        );
      }}
    </Page>
  );
};

export default ThemeEditorPage;
