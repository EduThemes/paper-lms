/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
/**
 * Step 2: the editing panel (Content / Look / Check / Advanced tabs) beside the live preview (desktop,
 * tablet or phone; light or dark).
 */
import { useMemo, useRef } from 'react';
import { ArrowRight, Monitor, Moon, Smartphone, Sun, Tablet } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { tintOf } from '../lib/engine';
import { Outline } from '../components/Outline';
import { ModuleTools } from '../components/ModuleTools';
import { LookPanel } from '../components/LookPanel';
import { CheckPanel } from '../components/CheckPanel';
import { CustomCssPanel } from '../components/CustomCssPanel';
import { LivePreview } from '../components/PreviewFrames';
import { Segmented } from '../components/controls';
import { SectionEditor } from '../editors/SectionEditor';
import { useIssues, useRendered } from '../state/derived';
import { useDesigner } from '../state/context';
import { useDesignerT } from '../i18n';

function Tabs({ tabs, current, onChange, idBase }) {
  const refs = useRef({});
  const onKey = (e, i) => {
    const n = tabs.length;
    const to = e.key === 'ArrowRight' ? (i + 1) % n : e.key === 'ArrowLeft' ? (i - 1 + n) % n : e.key === 'Home' ? 0 : e.key === 'End' ? n - 1 : -1;
    if (to < 0) return;
    e.preventDefault();
    onChange(tabs[to].id);
    refs.current[tabs[to].id]?.focus();
  };
  return (
    <div role="tablist" className="flex gap-1 border-b border-border px-2">
      {tabs.map((tab, i) => {
        const selected = tab.id === current;
        return (
          <button
            key={tab.id}
            ref={(el) => (refs.current[tab.id] = el)}
            type="button"
            role="tab"
            id={`${idBase}-tab-${tab.id}`}
            aria-selected={selected}
            aria-controls={`${idBase}-panel-${tab.id}`}
            tabIndex={selected ? 0 : -1}
            onClick={() => onChange(tab.id)}
            onKeyDown={(e) => onKey(e, i)}
            className={cn(
              '-mb-px inline-flex items-center gap-1.5 border-b-2 px-3 py-2.5 text-sm font-medium focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring',
              selected ? 'border-brand-600 text-text-primary' : 'border-transparent text-text-secondary hover:text-text-primary',
            )}
          >
            {tab.label}
          </button>
        );
      })}
    </div>
  );
}

function CheckLabel({ issues }) {
  const { t } = useDesignerT();
  const counted = issues.filter((i) => i.severity !== 'suggestion');
  const errors = issues.some((i) => i.severity === 'error');
  return (
    <>
      {t('customize.check')}
      {counted.length > 0 && (
        <span className={cn('inline-flex min-w-[1.25rem] justify-center rounded-full px-1.5 text-xs font-semibold text-white', errors ? 'bg-accent-danger' : 'bg-accent-warning')}>
          <span aria-hidden="true">{counted.length}</span>
          <span className="sr-only">, {t('customize.checkCount', { count: counted.length })}</span>
        </span>
      )}
    </>
  );
}

export function CustomizeStep() {
  const { t } = useDesignerT();
  const { doc, state, course, locale, select, setTab, setDevice, setDark, setStep } = useDesigner();
  const { html, css } = useRendered();
  const issues = useIssues();
  const tint = useMemo(() => (doc ? tintOf(doc) : null), [doc?.themeId, doc?.paletteId, doc?.palette]); // eslint-disable-line react-hooks/exhaustive-deps
  if (!doc) return null;
  const selected = doc.sections.find((s) => s.id === state.selectedId) ?? null;
  const tabs = [
    { id: 'content', label: t('customize.content') },
    { id: 'look', label: t('customize.look') },
    { id: 'check', label: <CheckLabel issues={issues} /> },
    ...(course.customCssEnabled ? [{ id: 'advanced', label: t('customize.advanced') }] : []),
  ];
  const tab = tabs.some((x) => x.id === state.panelTab) ? state.panelTab : 'content';

  return (
    <div className="flex flex-col lg:flex-row">
      <aside aria-label={t('customize.panelLabel')} className="w-full shrink-0 border-b border-border bg-surface-0 lg:sticky lg:top-0 lg:h-[calc(100vh-4rem)] lg:w-[400px] lg:overflow-y-auto lg:border-b-0 lg:border-r">
        <Tabs tabs={tabs} current={tab} onChange={setTab} idBase="pd" />
        <div role="tabpanel" id={`pd-panel-${tab}`} aria-labelledby={`pd-tab-${tab}`} className="space-y-5 p-4">
          {tab === 'content' &&
            (selected ? (
              <SectionEditor section={selected} onDone={() => select(null)} />
            ) : (
              <>
                <ModuleTools />
                <Outline />
              </>
            ))}
          {tab === 'look' && <LookPanel />}
          {tab === 'check' && <CheckPanel issues={issues} />}
          {tab === 'advanced' && <CustomCssPanel />}
        </div>
      </aside>

      <section aria-labelledby="pd-preview-h" className="min-w-0 flex-1 space-y-3 bg-surface-1 p-4 sm:p-6">
        <div className="flex flex-wrap items-end justify-between gap-3">
          <h2 id="pd-preview-h" className="sr-only">
            {t('preview.title')}
          </h2>
          <div className="flex flex-wrap items-end gap-3">
            <Segmented
              legend={t('preview.device')}
              legendHidden
              name="pd-device"
              size="sm"
              value={state.device}
              onChange={setDevice}
              options={[
                { value: 'desktop', label: t('preview.desktop'), icon: Monitor },
                { value: 'tablet', label: t('preview.tablet'), icon: Tablet },
                { value: 'phone', label: t('preview.phone'), icon: Smartphone },
              ]}
            />
            <Segmented
              legend={t('preview.mode')}
              legendHidden
              name="pd-mode"
              size="sm"
              value={state.dark ? 'dark' : 'light'}
              onChange={(v) => setDark(v === 'dark')}
              options={[
                { value: 'light', label: t('preview.light'), icon: Sun },
                { value: 'dark', label: t('preview.dark'), icon: Moon },
              ]}
            />
          </div>
          <Button onClick={() => setStep('finish')}>
            {t('steps.nextFinish')}
            <ArrowRight aria-hidden="true" />
          </Button>
        </div>
        {state.dark && <p className="text-xs text-text-secondary">{t('preview.darkNote')}</p>}
        <LivePreview
          html={html}
          css={css}
          device={state.device}
          dark={state.dark}
          selectedId={state.selectedId}
          onSelect={select}
          tint={tint}
          title={t('preview.frameTitle')}
          lang={locale}
        />
        <p className="text-center text-xs text-text-secondary">{t('preview.clickHint')}</p>
      </section>
    </div>
  );
}
