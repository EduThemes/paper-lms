/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
import { Link } from 'react-router-dom';
import { ArrowLeft, Redo2, Undo2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { canRedo, canUndo } from '../lib/history';
import { courseLabel } from '../lib/course';
import { useDesigner } from '../state/context';
import { useDesignerT } from '../i18n';

const STEPS = ['choose', 'customize', 'finish'];

/** Title row, the three steps, undo/redo and the save status. */
export function StepHeader() {
  const { state, doc, course, courseId, setStep, undo, redo } = useDesigner();
  const { t } = useDesignerT();
  const label = courseLabel(course);
  const dirty = !!doc && state.saved !== doc;

  return (
    <header className="border-b border-border bg-surface-0">
      <div className="flex flex-wrap items-center gap-x-6 gap-y-3 px-4 py-3 sm:px-6">
        <div className="min-w-0 flex-1">
          <Link
            to={`/courses/${courseId}/pages`}
            className="inline-flex items-center gap-1 text-xs font-medium text-text-secondary hover:text-text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
            {t('app.backToPages')}
          </Link>
          <h1 className="truncate text-lg font-semibold text-text-primary">
            {t('app.name')}
            {doc && <span className="font-normal text-text-secondary"> · {t(`kind.${doc.kind}`)}</span>}
          </h1>
          {label && <p className="truncate text-xs text-text-secondary">{label}</p>}
        </div>

        <nav aria-label={t('steps.label')}>
          <ol className="flex items-center gap-1">
            {STEPS.map((s, i) => {
              const current = state.step === s;
              return (
                <li key={s} className="flex items-center">
                  {i > 0 && <span aria-hidden="true" className="mx-1 h-px w-4 bg-border-strong sm:w-6" />}
                  <button
                    type="button"
                    aria-current={current ? 'step' : undefined}
                    disabled={s !== 'choose' && !doc}
                    onClick={() => setStep(s)}
                    className={cn(
                      'inline-flex items-center gap-2 rounded-full px-2.5 py-1 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50',
                      current ? 'bg-brand-600 text-white' : 'text-text-secondary hover:bg-surface-1 hover:text-text-primary',
                    )}
                  >
                    <span
                      aria-hidden="true"
                      className={cn(
                        'inline-flex h-5 w-5 items-center justify-center rounded-full text-xs',
                        current ? 'bg-white/20' : 'border border-border-strong',
                      )}
                    >
                      {i + 1}
                    </span>
                    <span>
                      <span className="sr-only">{t('steps.stepOf', { n: i + 1 })}: </span>
                      {t(`steps.${s}`)}
                    </span>
                  </button>
                </li>
              );
            })}
          </ol>
        </nav>

        {doc && (
          <div className="flex items-center gap-1">
            <span className="mr-2 hidden text-xs text-text-secondary md:inline" role="status" aria-live="polite">
              {dirty ? (state.page ? t('save.unsaved') : t('save.savedLocal')) : ''}
            </span>
            <Button variant="ghost" size="icon" className="h-9 w-9" onClick={undo} disabled={!canUndo(state.history)} title={`${t('undo')} (Ctrl+Z)`}>
              <Undo2 aria-hidden="true" />
              <span className="sr-only">{t('undo')}</span>
            </Button>
            <Button variant="ghost" size="icon" className="h-9 w-9" onClick={redo} disabled={!canRedo(state.history)} title={`${t('redo')} (Ctrl+Shift+Z)`}>
              <Redo2 aria-hidden="true" />
              <span className="sr-only">{t('redo')}</span>
            </Button>
          </div>
        )}
      </div>
    </header>
  );
}
