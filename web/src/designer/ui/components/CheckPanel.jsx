/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
/** Accessibility checklist: issue, why it matters, and a one-click fix where there is one. */
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { Notice } from './controls';
import { whyKey } from '../lib/starter';
import { useDesigner } from '../state/context';
import { useDesignerT } from '../i18n';

const ORDER = ['error', 'warning', 'suggestion'];

const BORDER = { error: 'border-l-accent-danger', warning: 'border-l-accent-warning', suggestion: 'border-l-accent-info' };

export function CheckPanel({ issues }) {
  const { t } = useDesignerT();
  const { doc, dispatch, select, announce } = useDesigner();
  const sorted = [...issues].sort((a, b) => ORDER.indexOf(a.severity) - ORDER.indexOf(b.severity));

  return (
    <div className="space-y-3">
      <div>
        <h2 className="text-base font-semibold text-text-primary">{t('check.title')}</h2>
        <p className="text-xs text-text-secondary">{t('check.intro')}</p>
      </div>
      {sorted.length === 0 ? (
        <Notice variant="success">{t('check.allClear')}</Notice>
      ) : (
        <ul className="space-y-2">
          {sorted.map((issue) => {
            const hasSection = !!issue.sectionId && !!doc?.sections.some((s) => s.id === issue.sectionId);
            const fix = issue.fix;
            const docFix = fix?.docPatch;
            const canFix = !!fix && (docFix ? true : !!fix.patch && hasSection);
            return (
              <li key={issue.id} className={cn('space-y-1.5 rounded-md border border-l-4 border-border bg-surface-0 p-3', BORDER[issue.severity])}>
                <p className="text-xs font-semibold uppercase tracking-wide text-text-secondary">{t(`check.${issue.severity}`)}</p>
                <p className="text-sm text-text-primary">{issue.message}</p>
                <p className="text-xs text-text-secondary">
                  <strong>{t('check.whyLabel')}:</strong> {t(whyKey(issue))}
                </p>
                {(canFix || hasSection) && (
                  <div className="flex flex-wrap gap-2 pt-1">
                    {canFix && (
                      <Button
                        type="button"
                        size="sm"
                        onClick={() => {
                          // Doc-level fixes (custom colours) are one undo step, like section fixes.
                          if (docFix) dispatch({ type: 'setDoc', patch: { palette: docFix.palette ?? undefined } });
                          else dispatch({ type: 'update', id: issue.sectionId, patch: fix.patch });
                          announce(t('check.fixed', { label: fix.label }));
                        }}
                      >
                        {fix.label}
                      </Button>
                    )}
                    {hasSection && (
                      <Button type="button" size="sm" variant="outline" onClick={() => select(issue.sectionId)}>
                        {t('check.goTo')}
                      </Button>
                    )}
                  </div>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
