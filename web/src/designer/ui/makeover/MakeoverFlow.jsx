/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
/**
 * Whole-course makeover UI: pick one look (the shared gallery), choose the pages, create them with progress
 * and retries, then the home page and course look. `lookOnly` (Course settings → Change look) applies just
 * the course look, with an offer to make pages too.
 */
import { useEffect, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { CheckCircle2, CircleDashed, Loader2, XCircle } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { addPageToModule, createDesignedPage, setCourseHomeToFrontPage, updateCourseLook, useColourSchemes, useUpdateCourseLook } from '@/services/api_designer';
import { courseLook, getTheme, renderDesign, renderOptions, tintOf } from '../lib/engine';
import { audienceForUiMode } from '../lib/catalog';
import { COURSE_PAGES, defaultPicks, planMakeover } from '../lib/makeover';
import { isPublished } from '../lib/modules';
import { toSchemes } from '../lib/schemes';
import { Checkbox, Notice, Spinner, Switch } from '../components/controls';
import { ThemeGallery } from '../components/ThemeGallery';
import { starterDoc } from '../lib/starter';
import { ThumbFrame } from '../components/PreviewFrames';
import { useDesignerModules } from '../state/useDesignerModules';
import { createMakeoverRunner, initialRun } from './runner';
import { useDesignerT } from '../i18n';
import { lookCssVars, normalizeLook } from '@/utils/courseLook';

const designerHref = (courseId, slug) => `/courses/${courseId}/designer?page=${encodeURIComponent(slug)}`;

/** Focus the stage heading whenever the stage changes. */
function useStageHeading(stage) {
  const ref = useRef(null);
  useEffect(() => {
    ref.current?.focus();
  }, [stage]);
  return ref;
}

function LookSummary({ look, onChange, onCancel, disabled }) {
  const { t } = useDesignerT();
  const theme = getTheme(look.themeId);
  return (
    <div className="flex flex-wrap items-center gap-2">
      <span className="text-sm font-semibold text-text-primary">{t('makeover.look', { theme: theme?.name ?? '' })}</span>
      <Button size="sm" variant="outline" disabled={disabled} onClick={onChange}>
        {t('makeover.changeLook')}
      </Button>
      {onCancel && (
        <Button size="sm" variant="ghost" disabled={disabled} onClick={onCancel}>
          {t('cancel')}
        </Button>
      )}
    </div>
  );
}

function PreviewGrid({ pages, ctx }) {
  const { t } = useDesignerT();
  if (!pages.length) return null;
  return (
    <section aria-labelledby="pd-mk-preview" className="space-y-3">
      <h2 id="pd-mk-preview" className="text-base font-semibold text-text-primary">
        {t('makeover.previewTitle')}
      </h2>
      <ul className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
        {pages.map((p) => {
          const title = p.doc.sections.find((s) => s.type === 'title')?.text || t(`kind.${p.kind}`);
          const { doc } = p;
          // The plan is rebuilt on every change (fresh ids), so key the cached render on what it shows.
          const key = `mk|${p.role}|${doc.themeId}|${doc.paletteId}|${doc.palette ? Object.values(doc.palette).join(',') : ''}|${doc.layout}|${ctx.locale}|${ctx.courseId}|${title}|${p.module?.items?.length ?? ''}`;
          return (
            <li key={p.role} className="overflow-hidden rounded-card border border-border bg-surface-0">
              <div className="relative aspect-[3/4] border-b border-border">
                <ThumbFrame cacheKey={key} render={() => renderDesign(doc, renderOptions(ctx.courseId))} tint={tintOf(doc)} title={t('preview.thumbTitle', { name: title })} lang={ctx.locale} />
              </div>
              <div className="p-3">
                <h3 className="truncate text-sm font-semibold text-text-primary">{title}</h3>
                <p className="text-xs text-text-secondary">{t(`kind.${p.kind}`)}</p>
              </div>
            </li>
          );
        })}
      </ul>
    </section>
  );
}

function PagesStage({ ctx, look, onChangeLook, onStart }) {
  const { t } = useDesignerT();
  const heading = useStageHeading('pages');
  const { modules, isLoading, isError, refetch } = useDesignerModules(ctx.courseId);
  const [picks, setPicks] = useState(null);
  const [options, setOptions] = useState({ publish: false, addToModules: true, makeHome: true, applyLook: true });
  const defaults = useMemo(() => defaultPicks(modules), [modules]);
  const current = picks ?? defaults;
  const theme = getTheme(look.themeId);

  const plan = useMemo(
    () =>
      theme
        ? planMakeover({
            theme,
            paletteId: look.paletteId,
            palette: look.palette,
            layout: look.layout,
            course: ctx.course,
            teacher: ctx.teacher,
            modules,
            picks: current,
            locale: ctx.locale,
            openLabel: (m) => t('modules.openModule', { module: m.name }),
          })
        : [],
    [theme, look, ctx, modules, current, t],
  );
  const toggle = (list, v, on) => (on ? [...new Set([...list, v])] : list.filter((x) => x !== v));
  const opt = (k) => (v) => setOptions((o) => ({ ...o, [k]: v }));
  const wantsHome = current.pages.includes('home');

  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <h2 ref={heading} tabIndex={-1} className="text-xl font-semibold text-text-primary outline-none">
          {t('makeover.pagesTitle')}
        </h2>
        <LookSummary look={look} onChange={onChangeLook} />
      </div>
      {isError ? (
        <Notice
          variant="error"
          action={
            <Button size="sm" variant="outline" onClick={() => refetch()}>
              {t('app.tryAgain')}
            </Button>
          }
        >
          {t('makeover.courseError')}
        </Notice>
      ) : isLoading ? (
        <Spinner label={t('makeover.loadingCourse')} />
      ) : (
        <>
          <div className="grid gap-6 rounded-card border border-border bg-surface-0 p-5 md:grid-cols-2">
            <fieldset className="space-y-2">
              <legend className="mb-1 text-sm font-semibold text-text-primary">{t('makeover.coursePages')}</legend>
              {COURSE_PAGES.map((k) => (
                <Checkbox key={k} label={t(`kind.${k}`)} checked={current.pages.includes(k)} onChange={(c) => setPicks({ ...current, pages: toggle(current.pages, k, c) })} />
              ))}
            </fieldset>
            <fieldset className="space-y-2">
              <legend className="mb-1 text-sm font-semibold text-text-primary">{t('makeover.modulePages')}</legend>
              {modules.length === 0 ? (
                <p className="text-sm text-text-secondary">{t('makeover.noModules')}</p>
              ) : (
                <>
                  {modules.map((m) => (
                    <Checkbox
                      key={m.id}
                      label={isPublished(m) ? m.name : t('modules.unpublishedModule', { module: m.name })}
                      checked={current.modules.includes(m.id)}
                      onChange={(c) => setPicks({ ...current, modules: toggle(current.modules, m.id, c) })}
                    />
                  ))}
                  <p className="text-xs text-text-secondary">{t('makeover.moduleHint')}</p>
                </>
              )}
            </fieldset>
            <fieldset className="space-y-3 md:col-span-2">
              <legend className="mb-1 text-sm font-semibold text-text-primary">{t('makeover.options')}</legend>
              <Switch label={t('makeover.addToModules')} checked={options.addToModules} onChange={opt('addToModules')} />
              <Switch label={t('makeover.publish')} checked={options.publish} onChange={opt('publish')} />
              {wantsHome && <Switch label={t('makeover.makeHome')} hint={t('finish.frontPageHint')} checked={options.makeHome} onChange={opt('makeHome')} />}
              <Switch label={t('makeover.applyLook')} checked={options.applyLook} onChange={opt('applyLook')} />
            </fieldset>
            <div className="md:col-span-2">
              <Button size="lg" disabled={!plan.length} onClick={() => onStart(plan, { ...options, makeHome: wantsHome && options.makeHome })}>
                {t('makeover.create', { count: plan.length })}
              </Button>
              {!plan.length && <p className="mt-2 text-sm text-text-secondary">{t('makeover.nothingPicked')}</p>}
            </div>
          </div>
          <PreviewGrid pages={plan} ctx={ctx} />
        </>
      )}
    </div>
  );
}

const STATE_ICON = { waiting: CircleDashed, working: Loader2, done: CheckCircle2, failed: XCircle };
const STATE_COLOR = { waiting: 'text-text-tertiary', working: 'text-accent-info animate-spin', done: 'text-accent-success', failed: 'text-accent-danger' };

function StatusRow({ p, courseId, running, onRetry }) {
  const { t } = useDesignerT();
  const Icon = STATE_ICON[p.state];
  const moduleProblem = p.moduleItem === 'failed';
  return (
    <li className="space-y-1 rounded-md border border-border bg-surface-0 p-3">
      <div className="flex flex-wrap items-center gap-2">
        <Icon className={cn('h-4 w-4 shrink-0', STATE_COLOR[p.state])} aria-hidden="true" />
        {p.state === 'done' && p.pageUrl ? (
          <Link to={`/courses/${courseId}/pages/${encodeURIComponent(p.pageUrl)}`} className="font-semibold text-brand-600 hover:underline">
            {p.title}
            <span className="sr-only"> ({t('makeover.openPage', { title: p.title })})</span>
          </Link>
        ) : (
          <span className="font-semibold text-text-primary">{p.title}</span>
        )}
        <span className="text-xs text-text-secondary">{t(`makeover.status.${p.state}`)}</span>
        {p.state === 'done' && p.pageUrl && (
          <Link to={designerHref(courseId, p.pageUrl)} className="ml-auto text-xs text-brand-600 hover:underline">
            {t('makeover.editPage', { title: p.title })}
          </Link>
        )}
      </div>
      {p.error && p.state === 'failed' && <p className="text-xs text-accent-danger">{p.error}</p>}
      {p.moduleItem !== 'none' && (
        <p className={cn('text-xs', moduleProblem ? 'text-accent-danger' : 'text-text-secondary')}>
          {t(`makeover.module.${p.moduleItem}`)}
          {moduleProblem && p.moduleError ? `: ${p.moduleError}` : ''}
        </p>
      )}
      {(p.state === 'failed' || (p.state === 'done' && moduleProblem)) && (
        <Button size="sm" variant="outline" disabled={running} onClick={onRetry}>
          <span aria-hidden="true">{t('makeover.retry')}</span>
          <span className="sr-only">{t('makeover.retryNamed', { title: p.title })}</span>
        </Button>
      )}
    </li>
  );
}

function ProgressStage({ run, runner, courseId, onAgain }) {
  const { t } = useDesignerT();
  const finished = run.stage === 'done';
  const heading = useStageHeading(finished ? 'done' : 'creating');
  const rows = run.plan.map((p) => run.progress[p.role]).filter(Boolean);
  const total = rows.length;
  const done = rows.filter((r) => r.state === 'done').length;
  const f = run.finish;

  return (
    <div className="space-y-4">
      <div className="space-y-1">
        <h2 ref={heading} tabIndex={-1} className="text-xl font-semibold text-text-primary outline-none">
          {finished ? t('makeover.doneTitle') : t('makeover.creatingTitle')}
        </h2>
        <p className="text-sm text-text-secondary">{finished ? t('makeover.doneBody', { count: done }) : t('makeover.creatingBody')}</p>
        {finished && !run.options.publish && <p className="text-sm text-text-primary">{t('makeover.unpublishedNote')}</p>}
      </div>
      <div className="h-2 w-full overflow-hidden rounded-full bg-surface-2" role="progressbar" aria-label={t('makeover.creatingTitle')} aria-valuemin={0} aria-valuemax={Math.max(total, 1)} aria-valuenow={done}>
        <div className="h-full bg-brand-600 transition-all" style={{ width: `${total ? (done / total) * 100 : 0}%` }} />
      </div>
      <p role="status" aria-live="polite" className="text-sm text-text-secondary">
        {run.stage === 'finishing' ? t('makeover.finishing') : t('makeover.progress', { done, total })}
      </p>
      {run.stopped && !run.running && <Notice variant="error">{t('makeover.stopped')}</Notice>}
      <ol className="space-y-2">
        {rows.map((p) => (
          <StatusRow key={p.role} p={p} courseId={courseId} running={run.running} onRetry={() => void runner.retry(p.role)} />
        ))}
      </ol>
      {finished && (
        <div className="space-y-2" aria-live="polite">
          {f.home === 'done' && <Notice variant="success">{t('makeover.homeDone')}</Notice>}
          {f.look === 'done' && <Notice variant="success">{t('makeover.lookDone')}</Notice>}
          {(f.home === 'failed' || f.look === 'failed') && (
            <Notice
              variant="error"
              action={
                <Button size="sm" variant="outline" disabled={run.running} onClick={() => void runner.retryFinish()}>
                  {t('makeover.retryFinish')}
                </Button>
              }
            >
              {f.home === 'failed' && <p>{t('makeover.homeFailed', { error: f.homeError })}</p>}
              {f.look === 'failed' && <p>{t('makeover.lookFailed', { error: f.lookError })}</p>}
            </Notice>
          )}
          <div className="flex flex-wrap gap-2 pt-2">
            <Button asChild>
              <Link to={`/courses/${courseId}`}>{t('makeover.goHome')}</Link>
            </Button>
            <Button variant="outline" onClick={onAgain}>
              {t('makeover.again')}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}

/** ?look=1: apply only the course look (header banner and colours). */
function LookOnlyStage({ ctx, look, onChangeLook, onMakePages }) {
  const { t } = useDesignerT();
  const heading = useStageHeading('lookOnly');
  const update = useUpdateCourseLook(ctx.courseId);
  const theme = getTheme(look.themeId);
  const doc = useMemo(
    () => (theme ? starterDoc({ theme, kind: 'home', locale: ctx.locale, layout: look.layout, paletteId: look.paletteId, palette: look.palette, course: ctx.course, teacher: ctx.teacher }) : null),
    [theme, look, ctx],
  );
  if (!doc) return null;
  const value = courseLook(doc);
  // Preview the header as the course will show it: .course-look picks the look's light or dark colourway
  // from Paper's theme, like CourseHero does.
  const preview = normalizeLook(value);
  return (
    <div className="space-y-5">
      <div className="space-y-2">
        <h2 ref={heading} tabIndex={-1} className="text-xl font-semibold text-text-primary outline-none">
          {t('makeover.lookOnlyTitle')}
        </h2>
        <p className="text-sm text-text-secondary">{t('makeover.lookOnlyBody')}</p>
        <LookSummary look={look} onChange={onChangeLook} />
      </div>
      <figure className="overflow-hidden rounded-card border border-border">
        {value.banner && <img src={value.banner.src} alt="" className="block max-h-48 w-full object-cover" />}
        <figcaption className="course-look px-4 py-3 text-lg font-semibold" style={lookCssVars(preview)}>
          {ctx.course?.name}
        </figcaption>
      </figure>
      <div className="flex flex-wrap items-center gap-2">
        <Button size="lg" disabled={update.isPending} onClick={() => update.mutate(value)}>
          {update.isPending ? t('makeover.applyingLook') : t('makeover.applyLookButton')}
        </Button>
        {update.isPending && <Spinner label={t('makeover.applyingLook')} />}
      </div>
      <div aria-live="polite" className="space-y-2">
        {update.isError && <Notice variant="error">{t('makeover.lookFailed', { error: update.error?.message ?? '' })}</Notice>}
        {update.isSuccess && (
          <Notice
            variant="success"
            title={t('makeover.lookDone')}
            action={
              <div className="flex flex-wrap gap-2">
                <Button asChild size="sm">
                  <Link to={`/courses/${ctx.courseId}`}>{t('makeover.goHome')}</Link>
                </Button>
                <Button size="sm" variant="outline" onClick={onMakePages}>
                  {t('makeover.alsoMakePages')}
                </Button>
              </div>
            }
          />
        )}
      </div>
    </div>
  );
}

/** ctx: { courseId, course, teacher, locale }. lookOnly: start from Course settings → Change look. */
export function MakeoverFlow({ ctx, lookOnly = false, api: apiOverride }) {
  const { t } = useDesignerT();
  const [stage, setStage] = useState('look'); // 'look' | 'pages' | 'lookOnly' | 'run'
  const [look, setLook] = useState(null);
  const [onlyLook, setOnlyLook] = useState(lookOnly);
  const schemesQ = useColourSchemes();
  const schemes = useMemo(() => toSchemes(schemesQ.data), [schemesQ.data]);

  const [run, setRunState] = useState(initialRun);
  const runRef = useRef(run);
  const set = (patch) => {
    runRef.current = typeof patch === 'function' ? patch(runRef.current) : { ...runRef.current, ...patch };
    setRunState(runRef.current);
  };
  const api = apiOverride ?? { createPage: createDesignedPage, addToModule: addPageToModule, setHome: setCourseHomeToFrontPage, setLook: updateCourseLook };
  const runner = useMemo(
    () => createMakeoverRunner({ api, courseId: ctx.courseId, customCssEnabled: !!ctx.course?.customCssEnabled, get: () => runRef.current, set, kindTitle: (k) => t(`kind.${k}`) }),
    [ctx.courseId, ctx.course, t], // eslint-disable-line react-hooks/exhaustive-deps
  );

  if (stage === 'run') {
    return (
      <ProgressStage
        run={run}
        runner={runner}
        courseId={ctx.courseId}
        onAgain={() => {
          set(initialRun);
          setOnlyLook(false);
          setStage('look');
        }}
      />
    );
  }
  if (stage === 'pages' && look) {
    return (
      <PagesStage
        ctx={ctx}
        look={look}
        onChangeLook={() => setStage('look')}
        onStart={(plan, options) => {
          setStage('run');
          void runner.start(plan, options);
        }}
      />
    );
  }
  if (stage === 'lookOnly' && look) {
    return (
      <LookOnlyStage
        ctx={ctx}
        look={look}
        onChangeLook={() => setStage('look')}
        onMakePages={() => {
          setOnlyLook(false);
          setStage('pages');
        }}
      />
    );
  }

  const colourOptions = [{ id: '', name: t('choose.coloursTheme') }, ...schemes.map((s) => ({ id: s.palette.id, name: s.name, palette: s.palette }))];
  return (
    <ThemeGallery
      ctx={ctx}
      kind="home"
      defaultAudience={audienceForUiMode(ctx.course?.uiMode)}
      colourOptions={colourOptions}
      heading={onlyLook ? t('makeover.lookOnlyPick') : t('makeover.pickLookTitle')}
      lede={onlyLook ? t('makeover.lookOnlyBody') : t('makeover.pickLookBody')}
      top={!onlyLook && <p className="text-sm text-text-primary">{t('makeover.entryBody')}</p>}
      onUse={(theme, paletteId, { layout, palette }) => {
        setLook({ themeId: theme.id, paletteId, palette, layout });
        setStage(onlyLook ? 'lookOnly' : 'pages');
      }}
    />
  );
}
