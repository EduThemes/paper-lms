import { Link } from 'react-router-dom';
import PropTypes from 'prop-types';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Palette, Wand2, Trash2 } from 'lucide-react';
import { api } from '../../services/api';
import { queryKeys } from '../../services/apiQueries';
import { normalizeLook, lookCssVars } from '../../utils/courseLook';
import { makeoverPath } from '../../utils/pageDesign';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';

// Course settings → Look (docs/page-designer/README.md, "Where teachers
// find it"): the course header banner and colours, picked in the Page
// Designer. Shows the current look and lets a teacher change or remove it.
//
// "Change look" opens the course makeover with ?look=1, which asks the
// designer to start at the look step (the header art and colours) rather
// than a full makeover of the course pages.
export default function CourseLookCard({ courseId, course, onCourseChange, disabledReason }) {
  const look = normalizeLook(course?.look);
  const queryClient = useQueryClient();
  const remove = useMutation({
    mutationFn: () => api.updateCourse(courseId, { look: null }),
    onSuccess: (updated) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.courses.detail(courseId) });
      onCourseChange?.(updated && typeof updated === 'object' ? { ...updated, look: null } : { look: null });
    },
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg">Look</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {look ? (
          <div className="flex flex-col gap-4 sm:flex-row sm:items-center">
            <div
              className="course-look relative h-20 w-full shrink-0 overflow-hidden rounded-md ring-1 ring-border sm:w-48"
              style={lookCssVars(look)}
              data-testid="course-look-thumbnail"
            >
              {look.banner && (
                <img
                  src={look.banner.src}
                  srcSet={look.banner.srcSet}
                  alt=""
                  className="absolute inset-0 h-full w-full object-cover"
                />
              )}
              <div className="course-look-scrim absolute inset-0" aria-hidden="true" />
              <span className="course-look-text relative block p-2 text-xs font-semibold">
                {course?.name || 'Course'}
              </span>
            </div>
            <div className="min-w-0 space-y-2">
              <p className="text-sm text-text-secondary">
                The course header uses this look&apos;s banner and colours
                {look.themeId ? <> (theme <span className="font-medium text-text-primary">{look.themeId}</span>)</> : null}.
                In dark mode it switches to the theme&apos;s dark colours.
              </p>
              {look.swatches.length > 0 && (
                <ul className="flex flex-wrap gap-1.5" aria-label="Look colours">
                  {look.swatches.map((s) => (
                    <li key={s.key} className="flex items-center gap-1 text-xs text-text-tertiary">
                      <span
                        className="inline-block h-5 w-5 rounded-full ring-1 ring-border"
                        style={{ backgroundColor: s.color }}
                        aria-hidden="true"
                      />
                      <span className="sr-only">{s.key} {s.color}</span>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </div>
        ) : (
          <p className="text-sm text-text-secondary">
            The course header uses Paper&apos;s generated colours. Pick a look from the page designer&apos;s themes
            to give it banner art and matching colours; a whole-course makeover applies one too.
          </p>
        )}

        <div className="flex flex-wrap gap-2">
          <Button size="sm" variant="outline" asChild className="gap-1.5">
            <Link to={makeoverPath(courseId, { look: '1' })}>
              {look ? <Palette className="h-4 w-4" aria-hidden="true" /> : <Wand2 className="h-4 w-4" aria-hidden="true" />}
              {look ? 'Change look' : 'Choose a look'}
            </Link>
          </Button>
          {look && (
            <Button
              size="sm"
              variant="outline"
              className="gap-1.5 text-accent-danger"
              disabled={remove.isPending || Boolean(disabledReason)}
              title={disabledReason || undefined}
              onClick={() => remove.mutate()}
            >
              <Trash2 className="h-4 w-4" aria-hidden="true" />
              {remove.isPending ? 'Removing…' : 'Remove look'}
            </Button>
          )}
        </div>
        {look && disabledReason && <p className="text-xs text-text-tertiary">{disabledReason}</p>}
        {remove.isError && (
          <div role="alert" className="flex flex-wrap items-center gap-2 text-sm text-accent-danger">
            <span>The look could not be removed: {remove.error?.message || 'unknown error'}.</span>
            <Button size="sm" variant="outline" onClick={() => remove.mutate()}>Try Again</Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
CourseLookCard.propTypes = {
  courseId: PropTypes.oneOfType([PropTypes.string, PropTypes.number]).isRequired,
  course: PropTypes.object,
  onCourseChange: PropTypes.func,
  disabledReason: PropTypes.string,
};
