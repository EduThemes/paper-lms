/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
import { useEffect } from 'react';
import { X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { useDesigner } from '../state/context';
import { useDesignerT } from '../i18n';

const TONE = {
  info: 'border-border bg-surface-raised',
  success: 'border-accent-success/40 bg-surface-raised',
  warning: 'border-accent-warning/50 bg-surface-raised',
  error: 'border-accent-danger/40 bg-surface-raised',
};

/**
 * One toast at a time, bottom-centre, for designer events with an optional action (Undo). Toasts with an
 * action stay longer. The live region is always present so new toasts are announced. Also hosts the
 * designer's polite announcement region (moves, fixes, undo…).
 */
export function DesignerToast() {
  const { state, hideToast } = useDesigner();
  const { t } = useDesignerT();
  const toast = state.toast;

  useEffect(() => {
    if (!toast) return undefined;
    const timer = setTimeout(hideToast, toast.action ? 10000 : 5000);
    return () => clearTimeout(timer);
  }, [toast, hideToast]);

  return (
    <>
      <div className="pointer-events-none fixed inset-x-0 bottom-4 z-50 flex justify-center px-4" role="status" aria-live="polite">
        {toast && (
          <div
            key={toast.id}
            className={cn('pointer-events-auto flex max-w-lg items-center gap-3 rounded-md border px-4 py-3 text-sm text-text-primary shadow-lg', TONE[toast.variant ?? 'info'])}
          >
            <span className="min-w-0 flex-1">{toast.message}</span>
            {toast.action && (
              <Button
                size="sm"
                variant="outline"
                onClick={() => {
                  toast.action();
                  hideToast();
                }}
              >
                {toast.actionLabel}
              </Button>
            )}
            <button
              type="button"
              onClick={hideToast}
              className="rounded p-1 text-text-secondary hover:text-text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              <X className="h-4 w-4" aria-hidden="true" />
              <span className="sr-only">{t('dismiss')}</span>
            </button>
          </div>
        )}
      </div>
      <div className="sr-only" role="status" aria-live="polite">
        {state.announcement}
      </div>
    </>
  );
}
