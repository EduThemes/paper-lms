/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
/**
 * Step 3: title, publish, make it the course home page (front page + course default_view 'wiki'),
 * optionally add it to a module, and save. A new design always creates a new page (Paper gives a clashing
 * title a fresh slug); only the page the design was opened from, or created this session, is updated.
 */
import { useState } from 'react';
import { Link } from 'react-router-dom';
import { ExternalLink } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { getDesignedPage, useSaveDesignedPage } from '@/services/api_designer';
import { clearDraft, isPageDoc } from '../lib/autosave';
import { docTitle } from '../lib/course';
import { renderForSave, wikiPagePayload } from '../lib/savePage';
import { Notice, SelectField, Spinner, Switch, TextField } from '../components/controls';
import { ThumbFrame } from '../components/PreviewFrames';
import { useDesignerModules } from '../state/useDesignerModules';
import { useIssues, useRendered } from '../state/derived';
import { useDesigner } from '../state/context';
import { tintOf } from '../lib/engine';
import { useDesignerT } from '../i18n';

const errText = (e) => e?.message || String(e ?? '');

/** The page to update, or null to create one: only a page that holds this same design. */
async function resolveTarget(courseId, page, doc) {
  if (!page?.url) return null;
  if (page.verified) return page.url;
  try {
    const live = await getDesignedPage(courseId, page.url);
    return isPageDoc(live?.design) && live.design.id === doc.id ? live.url ?? page.url : null;
  } catch {
    return null;
  }
}

export function FinishStep() {
  const { t } = useDesignerT();
  const { doc, state, courseId, course, setStep, setTab, setPage, reset } = useDesigner();
  const { html, css } = useRendered();
  const issues = useIssues().filter((i) => i.severity !== 'suggestion');
  const page = state.page;
  const [title, setTitle] = useState(() => page?.title || docTitle(doc, t(`kind.${doc?.kind ?? 'home'}`)));
  const [publish, setPublish] = useState(page?.published ?? false);
  const [frontPage, setFrontPage] = useState(page?.frontPage ?? false);
  const [moduleId, setModuleId] = useState('');
  const [result, setResult] = useState(null);
  const [error, setError] = useState(null);
  const updating = !!page?.url;
  const { modules } = useDesignerModules(courseId, !updating);
  const save = useSaveDesignedPage(courseId);

  if (!doc) return null;

  async function onSave() {
    if (!title.trim()) return;
    setError(null);
    setResult(null);
    try {
      const slug = await resolveTarget(courseId, page, doc);
      const rendered = renderForSave(doc, { courseId, customCssEnabled: !!course.customCssEnabled });
      const wikiPage = wikiPagePayload(rendered.doc, { title, html: rendered.html, css: rendered.css, published: publish, frontPage });
      const res = await save.mutateAsync({ slug, wikiPage, makeHome: frontPage, moduleId: slug ? null : moduleId || null });
      const p = res.page ?? {};
      setPage(
        { url: p.url, pageId: p.page_id, title: p.title ?? wikiPage.title, published: !!(p.published ?? wikiPage.published), frontPage: !!(p.front_page ?? frontPage), verified: true },
        doc,
      );
      if (frontPage) setPublish(true);
      clearDraft(courseId);
      setResult({ ...res, updated: !!slug, module: modules.find((m) => m.id === moduleId) });
    } catch (e) {
      setError(errText(e));
    }
  }

  const pageHref = page?.url ? `/courses/${courseId}/pages/${encodeURIComponent(page.url)}` : null;

  return (
    <div className="mx-auto grid max-w-6xl gap-6 px-4 py-6 sm:px-6 lg:grid-cols-[minmax(0,1fr)_320px]">
      <div className="space-y-5">
        <div>
          <h2 className="text-xl font-semibold text-text-primary">{t('finish.title')}</h2>
          <p className="text-sm text-text-secondary">{docTitle(doc, t(`kind.${doc.kind}`))}</p>
        </div>

        {issues.length > 0 && (
          <Notice
            variant={issues.some((i) => i.severity === 'error') ? 'warning' : 'info'}
            action={
              <Button
                size="sm"
                variant="outline"
                onClick={() => {
                  setStep('customize');
                  setTab('check');
                }}
              >
                {t('finish.reviewIssues')}
              </Button>
            }
          >
            {t('finish.issues', { count: issues.length })}
          </Notice>
        )}

        <section aria-labelledby="pd-save-h" className="space-y-4 rounded-card border border-border bg-surface-0 p-5">
          <div>
            <h3 id="pd-save-h" className="text-base font-semibold text-text-primary">
              {updating ? t('finish.updateTitle', { title: page.title || title }) : t('finish.createTitle')}
            </h3>
            <p className="text-sm text-text-secondary">{updating ? t('finish.updateBody') : t('finish.createBody')}</p>
          </div>
          <TextField label={t('finish.pageTitle')} value={title} onChange={setTitle} required error={!title.trim() ? t('finish.pageTitleRequired') : undefined} />
          <Switch label={t('finish.publish')} checked={publish || frontPage} disabled={frontPage} onChange={setPublish} />
          <Switch label={t('finish.frontPage')} hint={t('finish.frontPageHint')} checked={frontPage} onChange={setFrontPage} />
          {!updating && modules.length > 0 && (
            <SelectField
              label={t('finish.addToModule')}
              hint={t('finish.moduleHint')}
              value={moduleId}
              onChange={setModuleId}
              options={[{ value: '', label: t('finish.noModule') }, ...modules.map((m) => ({ value: m.id, label: m.name }))]}
            />
          )}
          <div className="flex flex-wrap items-center gap-3">
            <Button size="lg" onClick={onSave} disabled={save.isPending || !title.trim()}>
              {save.isPending ? t('finish.saving') : updating ? t('finish.saveChanges') : t('finish.save')}
            </Button>
            {save.isPending && <Spinner label={t('finish.saving')} />}
          </div>
          <div aria-live="polite" className="space-y-2">
            {error && <Notice variant="error">{t('finish.error', { error })}</Notice>}
            {result && (
              <Notice
                variant="success"
                title={result.updated ? t('finish.updated', { title: result.page?.title ?? title }) : t('finish.saved', { title: result.page?.title ?? title })}
                action={
                  <div className="flex flex-wrap gap-2">
                    {pageHref && (
                      <Button asChild size="sm">
                        <Link to={pageHref}>
                          <ExternalLink aria-hidden="true" />
                          {t('finish.openPage')}
                        </Link>
                      </Button>
                    )}
                    <Button size="sm" variant="outline" onClick={() => setStep('customize')}>
                      {t('finish.keepEditing')}
                    </Button>
                    <Button size="sm" variant="outline" onClick={reset}>
                      {t('finish.newPage')}
                    </Button>
                  </div>
                }
              >
                {result.home === true && <p>{t('finish.homeDone')}</p>}
                {result.home instanceof Error && <p>{t('finish.homeFailed', { error: errText(result.home) })}</p>}
                {result.moduleItem && !(result.moduleItem instanceof Error) && result.module && <p>{t('finish.moduleDone', { module: result.module.name })}</p>}
                {result.moduleItem instanceof Error && <p>{t('finish.moduleFailed', { error: errText(result.moduleItem) })}</p>}
              </Notice>
            )}
          </div>
        </section>

        <Button variant="outline" onClick={() => setStep('customize')}>
          {t('steps.back')}
        </Button>
      </div>

      <aside aria-label={t('preview.title')} className="space-y-2">
        <div className="relative aspect-[3/4] overflow-hidden rounded-card border border-border bg-surface-0">
          <ThumbFrame cacheKey={`finish|${html}|${css}`} render={() => ({ html, css })} tint={tintOf(doc)} title={t('preview.frameTitle')} />
        </div>
        <button
          type="button"
          onClick={() => setStep('customize')}
          className="text-sm font-medium text-brand-600 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring dark:text-brand-300"
        >
          {t('finish.keepEditing')}
        </button>
      </aside>
    </div>
  );
}
