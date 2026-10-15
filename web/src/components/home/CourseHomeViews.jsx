import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import PropTypes from 'prop-types';
import { FileText, Megaphone, Paintbrush, Home } from 'lucide-react';
import { api } from '../../services/api';
import { useCourseFrontPage } from '../../services/apiQueries';
import SafeHTML from '../SafeHTML';
import DesignedPageView, { PageContent } from '../pages/DesignedPageView';
import { isDesignedPage, designerPath } from '../../utils/pageDesign';
import { Card, CardHeader, CardTitle, CardContent } from '../ui/card';
import { Button } from '../ui/button';
import { Skeleton } from '../ui/skeleton';

// Course home bodies for the "Front Page" (default_view = "wiki") and
// "Announcements" home settings (B147). Canvas shows the course's front
// page / recent announcements on the course home for these settings.

// DesignedFrontPage: a front page made with the Page Designer. It brings its
// own banner, headings and colours, so it is shown as is in a rounded frame,
// not inside a titled card. Teachers get a quiet edit bar above it.
export function DesignedFrontPage({ page, courseId, isTeacher = false }) {
  return (
    <section aria-label={page.title || 'Course home page'} data-testid="designed-front-page">
      {isTeacher && (
        <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
          <span className="inline-flex items-center gap-1.5 text-xs font-medium text-text-tertiary">
            <Home className="h-3.5 w-3.5" aria-hidden="true" /> Home page
          </span>
          <Button variant="outline" size="sm" asChild className="gap-1.5 rounded-pill">
            <Link to={designerPath(courseId, { page: page.url })}>
              <Paintbrush className="h-4 w-4" aria-hidden="true" />
              Edit design
            </Link>
          </Button>
        </div>
      )}
      <div className="overflow-hidden rounded-card bg-surface-0 shadow-sm ring-1 ring-border">
        <DesignedPageView page={page} />
      </div>
    </section>
  );
}
DesignedFrontPage.propTypes = {
  page: PropTypes.shape({ title: PropTypes.string, url: PropTypes.string }).isRequired,
  courseId: PropTypes.oneOfType([PropTypes.string, PropTypes.number]).isRequired,
  isTeacher: PropTypes.bool,
};

// CourseFrontPageView loads the front page through react-query; CoursePage
// reads the same query (same key, one request) to decide whether a designed
// page gets the full-width slot.
export function CourseFrontPageView({ courseId, isTeacher = false }) {
  const query = useCourseFrontPage(courseId);

  if (query.isLoading) {
    return <Skeleton className="h-40 w-full" data-testid="front-page-loading" />;
  }
  if (query.isError) {
    return (
      <Card>
        <CardContent className="py-10 text-center">
          <p className="mb-3 text-accent-danger">The front page could not be loaded.</p>
          <Button variant="outline" onClick={() => query.refetch()}>Try Again</Button>
        </CardContent>
      </Card>
    );
  }
  const page = query.data;
  if (!page) {
    return (
      <Card>
        <CardContent className="py-10 text-center text-text-tertiary">
          This course has no front page yet.
        </CardContent>
      </Card>
    );
  }
  if (isDesignedPage(page)) {
    return <DesignedFrontPage page={page} courseId={courseId} isTeacher={isTeacher} />;
  }
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg flex items-center gap-2">
          <FileText className="h-5 w-5 text-brand-600" /> {page.title}
        </CardTitle>
      </CardHeader>
      <CardContent>
        {/* SafeHTML: sanitized, with equations and media captions (G007).
            A page stylesheet, if any, is scoped to the page (PageContent). */}
        <PageContent
          page={page}
          fallback={<SafeHTML className="prose max-w-none text-text-primary" html={page.body || ''} />}
        />
      </CardContent>
    </Card>
  );
}
CourseFrontPageView.propTypes = {
  courseId: PropTypes.oneOfType([PropTypes.string, PropTypes.number]).isRequired,
  isTeacher: PropTypes.bool,
};

const RECENT_ANNOUNCEMENTS = 5;

export function CourseAnnouncementsView({ courseId }) {
  const [state, setState] = useState({ loading: true, items: [] });

  useEffect(() => {
    let cancelled = false;
    setState({ loading: true, items: [] });
    api.getCourseAnnouncements(courseId, 1, RECENT_ANNOUNCEMENTS)
      .then((res) => { if (!cancelled) setState({ loading: false, items: res?.data || [] }); })
      .catch(() => { if (!cancelled) setState({ loading: false, items: [] }); });
    return () => { cancelled = true; };
  }, [courseId]);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg flex items-center gap-2">
          <Megaphone className="h-5 w-5 text-brand-600" /> Recent Announcements
        </CardTitle>
      </CardHeader>
      <CardContent>
        {state.loading && <Skeleton className="h-24 w-full" data-testid="announcements-loading" />}
        {!state.loading && state.items.length === 0 && (
          <p className="py-6 text-center text-text-tertiary">No announcements yet.</p>
        )}
        {!state.loading && state.items.length > 0 && (
          <ul className="divide-y">
            {state.items.map((a) => (
              <li key={a.id} className="py-3">
                <p className="font-medium text-text-primary">{a.title}</p>
                {a.message && (
                  <SafeHTML className="prose max-w-none text-sm text-text-secondary mt-1" html={a.message} />
                )}
              </li>
            ))}
          </ul>
        )}
        <Link to={`/courses/${courseId}/announcements`} className="mt-3 inline-block text-sm text-brand-600 hover:underline">
          All announcements
        </Link>
      </CardContent>
    </Card>
  );
}
