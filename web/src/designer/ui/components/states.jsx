/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
import { Button } from '@/components/ui/button';
import { Notice, Spinner } from './controls';
import { useDesignerT } from '../i18n';

export function LoadingState({ label }) {
  const { t } = useDesignerT();
  return (
    <div className="flex items-center justify-center py-16">
      <Spinner label={label ?? t('app.loading')} className="h-8" />
    </div>
  );
}

export function ErrorState({ message, onRetry }) {
  const { t } = useDesignerT();
  return (
    <div className="mx-auto max-w-2xl p-8">
      <Notice
        variant="error"
        action={
          onRetry && (
            <Button size="sm" onClick={onRetry}>
              {t('app.tryAgain')}
            </Button>
          )
        }
      >
        {message || t('app.loadFailed')}
      </Notice>
    </div>
  );
}
