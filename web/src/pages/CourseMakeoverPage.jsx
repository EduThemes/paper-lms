/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
// Whole-course makeover — /courses/:courseId/designer/makeover
//
//   (no query)  pick one look, choose pages (Home, Start Here, Course overview, Meet your instructor, a
//               Module overview per module), create them linked to each other, set Home as the front page
//               (+ course default_view 'wiki') and apply the course look. Nothing existing is overwritten.
//   ?look=1     Course settings → "Change look": pick a look and apply just the course look (header banner
//               and colours), with an offer to make the pages too.
import { useMemo } from 'react';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';
import Layout from '../components/Layout';
import { useDesignerT } from '../designer/ui/i18n';
import { useDesignerCourse } from '../designer/ui/useDesignerCourse';
import { MakeoverFlow } from '../designer/ui/makeover/MakeoverFlow';
import { ErrorState, LoadingState } from '../designer/ui/components/states';
import { courseLabel } from '../designer/ui/lib/course';

export default function CourseMakeoverPage() {
  const { courseId } = useParams();
  const [params] = useSearchParams();
  const { t } = useDesignerT();
  const info = useDesignerCourse(courseId);
  const lookOnly = params.get('look') === '1';
  const ctx = useMemo(
    () => ({ courseId, course: info.course, teacher: info.teacher, locale: info.locale }),
    [courseId, info.course, info.teacher, info.locale],
  );

  let body;
  if (info.isLoading) body = <LoadingState />;
  else if (info.isError || !info.course) body = <ErrorState message={info.error?.message || t('app.loadFailed')} onRetry={() => info.refetch()} />;
  else if (!info.isTeacher) body = <ErrorState message={t('app.teachersOnly')} />;
  else body = <MakeoverFlow key={lookOnly ? 'look' : 'all'} ctx={ctx} lookOnly={lookOnly} />;

  return (
    <Layout>
      <div className="min-h-full bg-surface-0">
        <header className="border-b border-border px-4 py-3 sm:px-6">
          <Link
            to={lookOnly ? `/courses/${courseId}/settings` : `/courses/${courseId}`}
            className="inline-flex items-center gap-1 text-xs font-medium text-text-secondary hover:text-text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
            {t('app.backToCourse')}
          </Link>
          <h1 className="text-lg font-semibold text-text-primary">{lookOnly ? t('makeover.lookOnlyTitle') : t('makeover.title')}</h1>
          {info.course && <p className="text-xs text-text-secondary">{courseLabel(info.course)}</p>}
        </header>
        <div className="px-4 py-6 sm:px-6">{body}</div>
      </div>
    </Layout>
  );
}
