/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
/**
 * Custom colours for the Look panel: edit a colourway (simple: 1–3 brand colours, the rest worked out;
 * advanced: every palette colour with live contrast results and "Fix contrast"), and "My colour schemes"
 * (save, reuse on any theme, rename, edit, delete), stored per user on the server so they follow the
 * teacher between courses. Every change to the page's colours is a normal doc edit (undo/redo, autosave);
 * dragging a colour picker is one undo step.
 */
import { useEffect, useId, useMemo, useState } from 'react';
import { ChevronDown } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import { useColourSchemes, useDeleteColourScheme, useSaveColourScheme } from '@/services/api_designer';
import { HEX_RE, PALETTE_KEYS, checkPalette, derivePalette, fixPalette, normHex, sanitizePalette, themeOf } from '../lib/engine';
import { newId } from '../lib/ids';
import { MAX_SCHEMES, cleanSchemeName, schemeIdOf, toScheme, toSchemes } from '../lib/schemes';
import { Spinner, TextField } from './controls';
import { useDesigner } from '../state/context';
import { useDesignerT } from '../i18n';

/* ------------------------------------------------------------------ one colour: picker + hex field */

export function ColourField({ label, hint, value, shown, optional, onChange }) {
  const { t } = useDesignerT();
  const id = useId();
  const [draft, setDraft] = useState(value);
  const [error, setError] = useState(false);
  const [focused, setFocused] = useState(false);
  // Never rewrite the box while the teacher is typing in it.
  useEffect(() => {
    if (focused) return;
    setDraft(value);
    setError(false);
  }, [value, focused]);

  const commit = (raw) => {
    const v = raw.trim();
    const withHash = v && !v.startsWith('#') ? `#${v}` : v;
    if (!v && optional) {
      setError(false);
      onChange(undefined);
    } else if (HEX_RE.test(withHash)) {
      setError(false);
      onChange(normHex(withHash));
    } else return false;
    return true;
  };

  return (
    <div className="flex items-start gap-2">
      <input
        type="color"
        aria-label={t('colours.picker', { label })}
        value={normHex(value) ?? normHex(shown) ?? '#000000'}
        onChange={(e) => onChange(e.target.value)}
        className="mt-7 h-9 w-11 shrink-0 cursor-pointer rounded border border-input bg-surface-0 p-0.5"
      />
      <div className="min-w-0 flex-1 space-y-1">
        <label htmlFor={id} className="text-sm font-medium text-text-primary">
          {optional ? `${label} ${t('optional')}` : label}
        </label>
        <Input
          id={id}
          value={draft}
          placeholder={optional && shown ? shown : '#1f6fb2'}
          spellCheck={false}
          autoComplete="off"
          aria-invalid={error || undefined}
          aria-describedby={`${id}-msg`}
          className="h-9 bg-surface-0 font-mono text-text-primary"
          onFocus={() => setFocused(true)}
          onChange={(e) => {
            const v = e.target.value;
            setDraft(v);
            // Apply only a complete 6-digit code while typing; short codes are accepted on blur.
            if (/^#?[0-9a-f]{6}$/i.test(v.trim()) && commit(v)) setError(false);
          }}
          onBlur={() => {
            setFocused(false);
            if (!commit(draft)) setError(true);
          }}
        />
        <p id={`${id}-msg`} className={cn('text-xs', error ? 'text-accent-danger' : 'text-text-secondary')}>
          {error ? t('colours.badHex') : hint ?? ''}
        </p>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ contrast results */

function ContrastList({ palette }) {
  const { t } = useDesignerT();
  const results = checkPalette(palette);
  return (
    <ul aria-label={t('colours.contrast')} className="space-y-1">
      {results.map((r) => {
        const ratio = (Math.floor(r.ratio * 100) / 100).toFixed(2);
        const verdict = r.pass ? t('colours.pass') : t('colours.fail');
        return (
          <li key={r.id} className="flex items-center gap-2 text-sm">
            <span aria-hidden="true" className="inline-flex h-7 w-9 items-center justify-center rounded border border-border font-semibold" style={{ background: r.bgColor, color: r.fgColor }}>
              Aa
            </span>
            <span className="min-w-0 flex-1 text-text-primary">
              <span aria-hidden="true">{r.label}</span>
              <span className="sr-only">{t('colours.pair', { label: r.label, ratio, verdict })}</span>
            </span>
            <span aria-hidden="true" className="font-mono text-xs text-text-secondary">
              {ratio}:1
            </span>
            <Badge aria-hidden="true" variant={r.pass ? 'secondary' : 'destructive'}>
              {verdict}
            </Badge>
          </li>
        );
      })}
    </ul>
  );
}

function ContrastStatus({ palette }) {
  const { t } = useDesignerT();
  const fails = checkPalette(palette).filter((r) => !r.pass).length;
  return (
    <div role="status" aria-live="polite" className={cn('rounded-md px-3 py-2 text-sm', fails ? 'bg-accent-warning/10 text-text-primary' : 'bg-accent-success/10 text-text-primary')}>
      {fails === 0 ? t('colours.allPass') : t('colours.someFail', { count: fails })}
    </div>
  );
}

/* ------------------------------------------------------------------ editor */

function PaletteEditor({ palette, schemes, onChange, onDone }) {
  const { t } = useDesignerT();
  const { showToast, announce } = useDesigner();
  const headingId = useId();
  const save = useSaveColourScheme();
  const [base, setBase] = useState(() => ({ primary: palette.primary, secondary: palette.secondary, accent: palette.accent }));
  const [name, setName] = useState(palette.name);
  const [advanced, setAdvanced] = useState(false);
  useEffect(() => setName(palette.name), [palette.name]);

  const setBaseField = (k, v) => {
    if (k === 'primary' && !v) return;
    const next = { ...base, [k]: v };
    setBase(next);
    onChange(derivePalette(next, palette.name, palette.id), true);
  };
  const savedId = schemeIdOf(palette.id);
  const saved = savedId ? schemes.find((s) => s.id === savedId) : null;

  async function doSave(asNew) {
    if ((asNew || !saved) && schemes.length >= MAX_SCHEMES) {
      showToast({ message: t('colours.tooMany', { n: MAX_SCHEMES }), variant: 'error' });
      return;
    }
    const clean = cleanSchemeName(palette.name) || t('colours.untitled');
    const valid = sanitizePalette({ ...palette, name: clean });
    if (!valid) {
      showToast({ message: t('colours.invalid'), variant: 'error' });
      return;
    }
    try {
      const res = await save.mutateAsync({ id: asNew || !saved ? undefined : saved.id, name: clean, palette: valid });
      const scheme = toScheme(res);
      if (scheme && scheme.palette.id !== palette.id) onChange(scheme.palette, false);
      announce(t('colours.saved', { name: clean }));
      showToast({ message: t('colours.saved', { name: clean }), variant: 'success' });
    } catch (e) {
      showToast({ message: e?.status === 422 || e?.code === 'too_many' ? t('colours.tooMany', { n: MAX_SCHEMES }) : t('colours.saveFailed'), variant: 'error' });
    }
  }

  return (
    <section aria-labelledby={headingId} className="space-y-3 rounded-md border border-border bg-surface-1 p-3">
      <h4 id={headingId} className="text-sm font-semibold text-text-primary">
        {t('colours.editorTitle')}
      </h4>
      <TextField
        label={t('colours.name')}
        value={name}
        maxLength={60}
        onChange={(v) => {
          setName(v);
          const clean = cleanSchemeName(v);
          if (clean) onChange({ ...palette, name: clean }, true);
        }}
        onBlur={() => setName(palette.name)}
      />
      <ColourField label={t('colours.primary')} hint={t('colours.primaryHint')} value={base.primary} onChange={(v) => setBaseField('primary', v)} />
      <ColourField label={t('colours.secondary')} hint={t('colours.auto')} value={base.secondary ?? ''} shown={palette.secondary} optional onChange={(v) => setBaseField('secondary', v)} />
      <ColourField label={t('colours.accent')} hint={t('colours.accentHint')} value={base.accent ?? ''} shown={palette.accent} optional onChange={(v) => setBaseField('accent', v)} />
      <p className="text-xs text-text-secondary">{t('colours.simpleNote')}</p>
      <ContrastStatus palette={palette} />

      <div className="rounded-md border border-border bg-surface-0">
        <button
          type="button"
          aria-expanded={advanced}
          onClick={() => setAdvanced((x) => !x)}
          className="flex w-full items-center justify-between px-3 py-2 text-left text-sm font-medium text-text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          {t('colours.advanced')}
          <ChevronDown className={cn('h-4 w-4 transition-transform', advanced && 'rotate-180')} aria-hidden="true" />
        </button>
        {advanced && (
          <div className="space-y-3 border-t border-border p-3">
            <p className="text-xs text-text-secondary">{t('colours.advancedNote')}</p>
            {PALETTE_KEYS.map((k) => (
              <ColourField key={k} label={t(`colourField.${k}`)} value={palette[k]} onChange={(v) => v && onChange({ ...palette, [k]: v }, true)} />
            ))}
            <ContrastList palette={palette} />
            <Button
              type="button"
              size="sm"
              variant="outline"
              disabled={checkPalette(palette).every((r) => r.pass)}
              onClick={() => {
                onChange(fixPalette(palette), false);
                announce(t('colours.fixed'));
              }}
            >
              {t('colours.fix')}
            </Button>
          </div>
        )}
      </div>

      <div className="flex flex-wrap gap-2">
        {saved ? (
          <>
            <Button type="button" size="sm" disabled={save.isPending} onClick={() => void doSave(false)}>
              {t('colours.saveChanges', { name: saved.name })}
            </Button>
            <Button type="button" size="sm" variant="outline" disabled={save.isPending} onClick={() => void doSave(true)}>
              {t('colours.saveAsNew')}
            </Button>
          </>
        ) : (
          <Button type="button" size="sm" disabled={save.isPending} onClick={() => void doSave(false)}>
            {t('colours.saveNew')}
          </Button>
        )}
        <Button type="button" size="sm" variant="outline" onClick={onDone}>
          {t('colours.done')}
        </Button>
      </div>
    </section>
  );
}

/* ------------------------------------------------------------------ My colour schemes */

function SchemeRow({ scheme, inUse, onUse, onEdit }) {
  const { t } = useDesignerT();
  const { doc, dispatch, showToast, announce } = useDesigner();
  const save = useSaveColourScheme();
  const del = useDeleteColourScheme();
  const [renaming, setRenaming] = useState(false);
  const [draft, setDraft] = useState(scheme.name);
  const p = scheme.palette;

  async function commitRename() {
    const name = cleanSchemeName(draft);
    if (!name) return;
    try {
      await save.mutateAsync({ id: scheme.id, name, palette: { ...p, name } });
      setRenaming(false);
      // Keep the open page's copy in step when it uses this scheme.
      if (doc?.palette?.id === p.id) dispatch({ type: 'setDoc', patch: { palette: { ...doc.palette, name } } });
    } catch {
      showToast({ message: t('colours.saveFailed'), variant: 'error' });
    }
  }

  async function doDelete() {
    const copy = scheme;
    try {
      await del.mutateAsync(scheme.id);
    } catch (e) {
      if (e?.status !== 404) {
        showToast({ message: t('colours.saveFailed'), variant: 'error' });
        return;
      }
    }
    announce(t('colours.deleted', { name: copy.name }));
    showToast({
      message: t('colours.deleted', { name: copy.name }),
      actionLabel: t('undo'),
      action: () => save.mutate({ name: copy.name, palette: copy.palette }),
    });
  }

  return (
    <li className="flex flex-wrap items-center gap-2 rounded-md border border-border bg-surface-0 p-2">
      <span
        aria-hidden="true"
        className="h-6 w-10 shrink-0 rounded border border-border"
        style={{ background: `linear-gradient(90deg, ${p.primary} 0 40%, ${p.secondary} 40% 75%, ${p.accent} 75% 100%)` }}
      />
      {renaming ? (
        <div className="flex min-w-0 flex-1 flex-wrap items-end gap-2">
          <TextField
            className="min-w-[10rem] flex-1"
            label={t('colours.renameLabel', { name: scheme.name })}
            value={draft}
            maxLength={60}
            onChange={setDraft}
            onKeyDown={(e) => {
              if (e.key === 'Enter') void commitRename();
              if (e.key === 'Escape') setRenaming(false);
            }}
          />
          <Button type="button" size="sm" onClick={() => void commitRename()}>
            {t('colours.save')}
          </Button>
          <Button type="button" size="sm" variant="outline" onClick={() => setRenaming(false)}>
            {t('cancel')}
          </Button>
        </div>
      ) : (
        <>
          <span className="min-w-0 flex-1 truncate text-sm text-text-primary">
            {scheme.name} {inUse && <Badge variant="secondary">{t('colours.inUse')}</Badge>}
          </span>
          <span className="flex flex-wrap gap-1">
            <Button type="button" size="sm" className="h-8" disabled={inUse} onClick={onUse}>
              <span aria-hidden="true">{t('colours.use')}</span>
              <span className="sr-only">{t('colours.useNamed', { name: scheme.name })}</span>
            </Button>
            <Button type="button" size="sm" variant="outline" className="h-8" onClick={onEdit}>
              <span aria-hidden="true">{t('colours.edit')}</span>
              <span className="sr-only">{t('colours.editNamed', { name: scheme.name })}</span>
            </Button>
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="h-8"
              onClick={() => {
                setDraft(scheme.name);
                setRenaming(true);
              }}
            >
              <span aria-hidden="true">{t('colours.rename')}</span>
              <span className="sr-only">{t('colours.renameNamed', { name: scheme.name })}</span>
            </Button>
            <Button type="button" size="sm" variant="outline" className="h-8" onClick={() => void doDelete()}>
              <span aria-hidden="true">{t('colours.delete')}</span>
              <span className="sr-only">{t('colours.deleteNamed', { name: scheme.name })}</span>
            </Button>
          </span>
        </>
      )}
    </li>
  );
}

/* ------------------------------------------------------------------ the Look panel block */

export function CustomColours() {
  const { t } = useDesignerT();
  const { doc, dispatch, announce } = useDesigner();
  const query = useColourSchemes();
  const schemes = useMemo(() => toSchemes(query.data), [query.data]);
  const [editing, setEditing] = useState(false);
  const headingId = useId();
  const listId = useId();

  const theme = themeOf(doc);
  const builtIn = theme.palettes.find((p) => p.id === doc.paletteId) ?? theme.palettes[0];
  const custom = useMemo(() => (doc.palette ? sanitizePalette(doc.palette) : null), [doc.palette]);

  const setPalette = (p, coalesce = false) =>
    dispatch({ type: 'setDoc', patch: { palette: p }, ...(coalesce ? { coalesce: 'palette', at: Date.now() } : {}) });

  function startEditing() {
    // "Edit this colourway" copies the built-in colourway into the doc; built-in palettes never change.
    if (!custom) setPalette({ ...builtIn, id: newId('cs'), name: cleanSchemeName(t('colours.copyOf', { name: builtIn.name })) });
    setEditing(true);
  }

  return (
    <section aria-labelledby={headingId} className="space-y-3">
      <div>
        <h3 id={headingId} className="text-sm font-semibold text-text-primary">
          {t('colours.title')}
        </h3>
        <p className="text-xs text-text-secondary">{t('colours.intro')}</p>
      </div>

      {custom && !editing && <p className="text-sm font-semibold text-text-primary">{t('colours.using', { name: custom.name })}</p>}

      {editing && custom ? (
        <PaletteEditor key={custom.id} palette={custom} schemes={schemes} onChange={(p, c) => setPalette(p, c)} onDone={() => setEditing(false)} />
      ) : (
        <div className="flex flex-wrap gap-2">
          <Button type="button" size="sm" variant="outline" onClick={startEditing}>
            {custom ? t('colours.edit') : t('colours.editThis')}
          </Button>
          {custom && (
            <Button
              type="button"
              size="sm"
              variant="outline"
              onClick={() => {
                setPalette(undefined);
                announce(t('colours.backToTheme'));
              }}
            >
              {t('colours.useTheme')}
            </Button>
          )}
        </div>
      )}

      <div className="space-y-2">
        <h4 id={listId} className="text-sm font-semibold text-text-primary">
          {t('colours.mine')}
        </h4>
        <p className="text-xs text-text-secondary">{t('colours.mineHint')}</p>
        {query.isLoading && <Spinner label={t('colours.loading')} />}
        {query.isError && (
          <div className="flex items-center gap-2 text-sm text-text-primary">
            {t('colours.loadFailed')}
            <Button type="button" size="sm" variant="outline" onClick={() => query.refetch()}>
              {t('app.tryAgain')}
            </Button>
          </div>
        )}
        {query.isSuccess && schemes.length === 0 && <p className="text-sm text-text-primary">{t('colours.none')}</p>}
        {schemes.length > 0 && (
          <ul aria-labelledby={listId} className="space-y-1.5">
            {schemes.map((s) => (
              <SchemeRow
                key={s.id}
                scheme={s}
                inUse={custom?.id === s.palette.id}
                onUse={() => {
                  setPalette(s.palette);
                  announce(t('colours.applied', { name: s.name }));
                }}
                onEdit={() => {
                  if (custom?.id !== s.palette.id) setPalette(s.palette);
                  setEditing(true);
                }}
              />
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}
