/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
// Page Designer — /courses/:courseId/designer
//
//   ?kind=home|start-here|overview|module|instructor  start a new page of that kind (gallery preselected)
//   ?page=<slug>                                       re-open a page the designer made (its `design`)
//
// Lazy-loaded by the router: the theme catalog and icon geometry only load here. A page without a design is
// never opened in the designer (its HTML would be replaced); a designed page whose HTML was changed
// elsewhere opens with a warning.
import { useMemo, useRef, useState } from 'react';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import Layout from '../components/Layout';
import { Button } from '../components/ui/button';
import { useDesignedPage } from '../services/api_designer';
import { useDesignerT } from '../designer/ui/i18n';
import { DesignerProvider } from '../designer/ui/state/DesignerContext';
import { useDesigner } from '../designer/ui/state/context';
import { DesignerShell } from '../designer/ui/DesignerShell';
import { useDesignerCourse } from '../designer/ui/useDesignerCourse';
import { emptyHistory } from '../designer/ui/lib/history';
import { clearDraft, loadDraft } from '../designer/ui/lib/autosave';
import { inspectSavedPage } from '../designer/ui/lib/savePage';
import { KINDS, timeAgo } from '../designer/ui/lib/starter';
import { ErrorState, LoadingState } from '../designer/ui/components/states';
import { Notice } from '../designer/ui/components/controls';

/** Offers a newer local draft of the page being edited. */
function DraftNotice({ draft, page, saved }) {
  const { t } = useDesignerT();
  const { courseId, openDoc } = useDesigner();
  const [gone, setGone] = useState(false);
  if (gone) return null;
  return (
    <Notice
      variant="info"
      title={t('edit.draftTitle')}
      action={
        <div className="flex flex-wrap gap-2">
          <Button
            size="sm"
            onClick={() => {
              openDoc(draft.doc, { step: 'customize', page, saved });
              setGone(true);
            }}
          >
            {t('edit.draftRestore')}
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => {
              clearDraft(courseId);
              setGone(true);
            }}
          >
            {t('edit.draftDiscard')}
          </Button>
        </div>
      }
    >
      {t('edit.draftBody', { when: timeAgo(draft.savedAt, t) })}
    </Notice>
  );
}

function EditDesigner({ courseId, slug, ctx }) {
  const { t } = useDesignerT();
  const pageQ = useDesignedPage(courseId, slug, { refetchOnWindowFocus: false });
  // The page as it was when the designer opened it. Later fetches (after saving) must not re-open it.
  const first = useRef(null);
  if (!first.current && pageQ.data) first.current = pageQ.data;
  const snapshot = first.current;
  const opened = useMemo(() => (snapshot ? inspectSavedPage(snapshot, courseId) : null), [snapshot, courseId]);

  if (!snapshot && pageQ.isLoading) return <LoadingState label={t('edit.loading')} />;
  if (!snapshot && pageQ.isError) {
    if (pageQ.error?.status === 404) return <ErrorState message={t('edit.notFound')} />;
    return <ErrorState message={pageQ.error?.message} onRetry={() => pageQ.refetch()} />;
  }
  const p = snapshot;
  if (!opened || opened.status === 'not-designed') {
    return (
      <div className="mx-auto max-w-2xl p-8">
        <Notice
          variant="warning"
          title={t('edit.notDesignedTitle')}
          action={
            <div className="flex flex-wrap gap-2">
              <Button asChild size="sm">
                <Link to={`/courses/${courseId}/pages/${encodeURIComponent(slug)}`}>{t('edit.openOriginal')}</Link>
              </Button>
              <Button asChild size="sm" variant="outline">
                <Link to={`/courses/${courseId}/designer`}>{t('finish.newPage')}</Link>
              </Button>
            </div>
          }
        >
          {t('edit.notDesignedBody')}
        </Notice>
      </div>
    );
  }

  const doc = opened.doc;
  const page = { url: p.url ?? slug, pageId: p.page_id, title: p.title, published: !!p.published, frontPage: !!p.front_page, verified: true };
  const draft = loadDraft(courseId);
  const draftForThis = draft && draft.pageUrl === page.url && draft.doc.id === doc.id && JSON.stringify(draft.doc) !== JSON.stringify(doc) ? draft : null;
  const initial = { history: { ...emptyHistory, present: doc }, step: 'customize', page, saved: doc };

  return (
    <DesignerProvider key={`${slug}|${doc.id}`} courseId={courseId} course={ctx.course} teacher={ctx.teacher} locale={doc.locale ?? ctx.locale} initial={initial}>
      <DesignerShell
        initialKind={doc.kind}
        notices={
          <>
            {opened.status === 'changed' && <Notice variant="warning" title={t('edit.changedTitle')}>{t('edit.changedBody')}</Notice>}
            {draftForThis && <DraftNotice draft={draftForThis} page={page} saved={doc} />}
          </>
        }
      />
    </DesignerProvider>
  );
}

export default function PageDesignerPage() {
  const { courseId } = useParams();
  const [params] = useSearchParams();
  const { t } = useDesignerT();
  const ctx = useDesignerCourse(courseId);
  const slug = params.get('page');
  const kindParam = params.get('kind');
  const kind = KINDS.includes(kindParam) ? kindParam : 'home';

  let body;
  if (ctx.isLoading) body = <LoadingState />;
  else if (ctx.isError || !ctx.course) body = <ErrorState message={ctx.error?.message || t('app.loadFailed')} onRetry={() => ctx.refetch()} />;
  else if (!ctx.isTeacher) body = <ErrorState message={t('app.teachersOnly')} />;
  else if (slug) body = <EditDesigner courseId={courseId} slug={slug} ctx={ctx} />;
  else
    body = (
      <DesignerProvider key={`new|${kind}`} courseId={courseId} course={ctx.course} teacher={ctx.teacher} locale={ctx.locale}>
        <DesignerShell initialKind={kind} />
      </DesignerProvider>
    );

  return <Layout>{body}</Layout>;
}
