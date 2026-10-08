import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { FileText, Megaphone } from 'lucide-react';
import { api } from '../../services/api';
import SafeHTML from '../SafeHTML';
import { Card, CardHeader, CardTitle, CardContent } from '../ui/card';
import { Skeleton } from '../ui/skeleton';

// Course home bodies for the "Front Page" (default_view = "wiki") and
// "Announcements" home settings (B147). Canvas shows the course's front
// page / recent announcements on the course home for these settings.

export function CourseFrontPageView({ courseId }) {
  const [state, setState] = useState({ loading: true, page: null });

  useEffect(() => {
    let cancelled = false;
    setState({ loading: true, page: null });
    api.getCourseFrontPage(courseId)
      .then((page) => { if (!cancelled) setState({ loading: false, page }); })
      .catch(() => { if (!cancelled) setState({ loading: false, page: null }); });
    return () => { cancelled = true; };
  }, [courseId]);

  if (state.loading) {
    return <Skeleton className="h-40 w-full" data-testid="front-page-loading" />;
  }
  if (!state.page) {
    return (
      <Card>
        <CardContent className="py-10 text-center text-text-tertiary">
          This course has no front page yet.
        </CardContent>
      </Card>
    );
  }
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg flex items-center gap-2">
          <FileText className="h-5 w-5 text-brand-600" /> {state.page.title}
        </CardTitle>
      </CardHeader>
      <CardContent>
        {/* SafeHTML: sanitized, with equations and media captions (G007). */}
        <SafeHTML className="prose max-w-none text-text-primary" html={state.page.body || ''} />
      </CardContent>
    </Card>
  );
}

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
