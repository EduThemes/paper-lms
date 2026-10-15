import { useState } from 'react';
import { Link } from 'react-router-dom';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Palette, Paintbrush, Wand2, X } from 'lucide-react';
import { Button } from '../ui/button';
import { designerPath, makeoverPath } from '../../utils/pageDesign';

// Teacher-only invitation on the course home to use the Page Designer
// (docs/page-designer/README.md, "Where teachers find it"). Shown when the
// course home is not a front page, or there is no front page yet.
// Dismissible per course; the choice lives in localStorage, and a browser
// that blocks storage simply shows the callout again next time.

const storageKey = (courseId) => `paper.designHomeCallout.dismissed.${courseId}`;

function readDismissed(courseId) {
  try {
    return window.localStorage.getItem(storageKey(courseId)) === '1';
  } catch {
    return false;
  }
}

export default function DesignHomeCallout({ courseId }) {
  const { t } = useTranslation();
  const [dismissed, setDismissed] = useState(() => readDismissed(courseId));
  if (dismissed) return null;

  const dismiss = () => {
    try {
      window.localStorage.setItem(storageKey(courseId), '1');
    } catch {
      /* storage disabled: hide for this visit only */
    }
    setDismissed(true);
  };

  const headingId = `design-home-callout-${courseId}`;
  return (
    <section
      aria-labelledby={headingId}
      className="relative overflow-hidden rounded-card border border-brand-200 bg-gradient-to-br from-brand-50 via-surface-0 to-surface-0 p-5 shadow-sm sm:p-6"
    >
      <button
        type="button"
        onClick={dismiss}
        aria-label={t('designHome.dismiss')}
        title={t('designHome.dismiss')}
        className="absolute end-2 top-2 rounded-md p-1.5 text-text-tertiary transition-colors hover:bg-surface-2 hover:text-text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <X className="h-4 w-4" aria-hidden="true" />
      </button>
      <div className="flex flex-col gap-4 md:flex-row md:items-center">
        <div
          aria-hidden="true"
          className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-brand-600 text-white shadow-sm"
        >
          <Palette className="h-6 w-6" />
        </div>
        <div className="min-w-0 flex-1 pe-8 md:pe-0">
          <h2 id={headingId} className="text-base font-semibold text-text-primary">
            {t('designHome.title')}
          </h2>
          <p className="mt-1 text-sm text-text-secondary">{t('designHome.body')}</p>
        </div>
        <div className="flex flex-wrap gap-2 md:shrink-0 md:pe-8">
          <Button size="sm" asChild className="gap-1.5">
            <Link to={designerPath(courseId, { kind: 'home' })}>
              <Paintbrush className="h-4 w-4" aria-hidden="true" />
              {t('designHome.designHome')}
            </Link>
          </Button>
          <Button size="sm" variant="outline" asChild className="gap-1.5 bg-surface-0">
            <Link to={makeoverPath(courseId)}>
              <Wand2 className="h-4 w-4" aria-hidden="true" />
              {t('designHome.makeover')}
            </Link>
          </Button>
        </div>
      </div>
    </section>
  );
}
DesignHomeCallout.propTypes = {
  courseId: PropTypes.oneOfType([PropTypes.string, PropTypes.number]).isRequired,
};
