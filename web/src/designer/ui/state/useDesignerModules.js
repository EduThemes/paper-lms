import { useMemo } from 'react';
import { useCourseModules } from '@/services/apiQueries';
import { useAssignmentDueDates } from '@/services/api_designer';
import { normalizeModules } from '../lib/modules';

/** The course's modules (with items and assignment due dates), normalized for the designer. */
export function useDesignerModules(courseId, enabled = true) {
  const modulesQ = useCourseModules(courseId, 1, 100, true, { enabled: !!courseId && enabled });
  const dueQ = useAssignmentDueDates(courseId, { enabled: !!courseId && enabled });
  const modules = useMemo(() => normalizeModules(modulesQ.data?.data ?? [], dueQ.data ?? {}), [modulesQ.data, dueQ.data]);
  return { modules, isLoading: modulesQ.isLoading, isError: modulesQ.isError, refetch: modulesQ.refetch };
}
