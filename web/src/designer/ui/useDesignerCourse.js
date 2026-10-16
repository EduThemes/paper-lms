/**
 * What the designer needs about the course and the user: the course (name, code, K-12 mode, whether
 * hand-written CSS is allowed), the teacher for instructor cards, the starter locale, and whether the
 * user may design pages here.
 */
import { useMemo } from 'react';
import { useCourse } from '@/services/apiQueries';
import { useCourseTeachers } from '@/services/api_designer';
import { useAuth } from '@/contexts/AuthContext';
import useIsTeacher from '@/hooks/useIsTeacher';
import { starterLocale } from './lib/course';
import { useDesignerT } from './i18n';

export function useDesignerCourse(courseId) {
  const { i18n } = useDesignerT();
  const { user } = useAuth();
  const courseQ = useCourse(courseId);
  const teachersQ = useCourseTeachers(courseId);
  const isTeacher = useIsTeacher(courseId);
  const raw = courseQ.data;

  const course = useMemo(
    () =>
      raw
        ? {
            id: raw.id,
            name: raw.name ?? '',
            code: raw.course_code ?? '',
            uiMode: raw.ui_mode ?? 'standard',
            customCssEnabled: raw.custom_css_enabled === true,
            defaultView: raw.default_view,
            look: raw.look ?? null,
          }
        : null,
    [raw],
  );

  const teacher = useMemo(() => {
    const list = teachersQ.data ?? [];
    const me = user && list.find((x) => String(x.id) === String(user.id));
    const pick = me ?? list[0];
    return pick ? { name: pick.name, email: pick.email } : null;
  }, [teachersQ.data, user]);

  return {
    course,
    teacher,
    locale: starterLocale(i18n.resolvedLanguage || i18n.language),
    isTeacher,
    isLoading: courseQ.isLoading || isTeacher === null,
    isError: courseQ.isError,
    error: courseQ.error,
    refetch: courseQ.refetch,
  };
}
