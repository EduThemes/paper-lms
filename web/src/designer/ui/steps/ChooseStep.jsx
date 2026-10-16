/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
/**
 * Step 1: choose the page type and a look. With a page already open, picking a look of the same page type
 * keeps the teacher's content and only changes the look (one undo step).
 */
import { useMemo, useState } from 'react';
import { Button } from '@/components/ui/button';
import { getTheme } from '../lib/engine';
import { audienceForUiMode } from '../lib/catalog';
import { clearDraft, loadDraft } from '../lib/autosave';
import { toSchemes } from '../lib/schemes';
import { Notice } from '../components/controls';
import { ThemeGallery } from '../components/ThemeGallery';
import { starterDoc, timeAgo } from '../lib/starter';
import { useDesigner } from '../state/context';
import { useDesignerT } from '../i18n';
import { useColourSchemes } from '@/services/api_designer';

export function ChooseStep({ initialKind = 'home' }) {
  const { t } = useDesignerT();
  const { doc, courseId, course, teacher, locale, openDoc, dispatch, setStep } = useDesigner();
  const [kind, setKind] = useState(doc?.kind ?? initialKind);
  const [draftGone, setDraftGone] = useState(false);
  const schemesQuery = useColourSchemes();
  const schemes = useMemo(() => toSchemes(schemesQuery.data), [schemesQuery.data]);
  // A draft is offered only when nothing is open (and it isn't for a page that's open elsewhere).
  const draft = useMemo(() => (doc || draftGone ? null : loadDraft(courseId)), [doc, draftGone, courseId]);

  const current = doc?.palette;
  const colourOptions = [
    { id: '', name: t('choose.coloursTheme') },
    ...(current && !schemes.some((s) => s.palette.id === current.id) ? [{ id: current.id, name: t('choose.coloursCurrent', { name: current.name }), palette: current }] : []),
    ...schemes.map((s) => ({ id: s.palette.id, name: s.name, palette: s.palette })),
  ];

  function use(theme, paletteId, { layout, palette }) {
    if (doc && doc.kind === kind) {
      // Same kind of page: keep the content, change the look (undoable).
      dispatch({ type: 'setDoc', patch: { themeId: theme.id, paletteId, palette, layout: layout || theme.defaultLayout } });
      setStep('customize');
      return;
    }
    // A brand-new page (never saved over the page that was open): course name, code and teacher go into
    // its placeholders.
    const next = starterDoc({ theme, kind, locale, layout, paletteId, palette, course, teacher });
    openDoc(next, { step: 'customize', page: null, saved: null });
  }

  const resume = draft && (
    <Notice
      variant="info"
      title={t('choose.resume')}
      action={
        <div className="flex flex-wrap gap-2">
          <Button
            size="sm"
            onClick={() => openDoc(draft.doc, { step: 'customize', page: draft.pageUrl ? { url: draft.pageUrl } : null, saved: null })}
          >
            {t('choose.resumeButton')}
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => {
              clearDraft(courseId);
              setDraftGone(true);
            }}
          >
            {t('choose.discardDraft')}
          </Button>
        </div>
      }
    >
      {t('choose.resumeBody', {
        kind: t(`kind.${draft.doc.kind}`),
        theme: getTheme(draft.doc.themeId)?.name ?? draft.doc.themeId,
        when: timeAgo(draft.savedAt, t),
      })}
    </Notice>
  );

  return (
    <div className="px-4 py-6 sm:px-6">
      <ThemeGallery
        ctx={{ courseId, course, teacher, locale }}
        kind={kind}
        onKind={setKind}
        defaultAudience={audienceForUiMode(course?.uiMode)}
        colourOptions={colourOptions}
        initialColours={current?.id ?? ''}
        onUse={use}
        lede={
          <>
            <strong className="text-text-primary">{t(`kind.${kind}`)}.</strong> {t(`kindDesc.${kind}`)}{' '}
            {doc && doc.kind === kind ? t('choose.keepContent') : t('choose.lede')}
          </>
        }
        top={resume}
      />
    </div>
  );
}
