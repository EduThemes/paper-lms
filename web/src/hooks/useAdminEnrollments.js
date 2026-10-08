// React Query bindings for the admin-enrollments lane
// (services/api_admin_enrollments.js).

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { enrollmentsAdminApi } from '../services/api_admin_enrollments';

export const adminEnrollmentKeys = {
  all: ['adminEnrollments'],
  roster: (courseId, states = []) => ['adminEnrollments', 'roster', String(courseId), states.join(',')],
  rosterAll: (courseId) => ['adminEnrollments', 'roster', String(courseId)],
  sections: (courseId) => ['adminEnrollments', 'sections', String(courseId)],
  invitations: () => ['adminEnrollments', 'invitations'],
  myCourses: () => ['adminEnrollments', 'myCourses'],
};

export function useRoster(courseId, states = [], options = {}) {
  return useQuery({
    queryKey: adminEnrollmentKeys.roster(courseId, states),
    queryFn: () => enrollmentsAdminApi.listRoster(courseId, states),
    enabled: !!courseId,
    ...options,
  });
}

export function useCourseSections(courseId, options = {}) {
  return useQuery({
    queryKey: adminEnrollmentKeys.sections(courseId),
    queryFn: () => enrollmentsAdminApi.listSections(courseId),
    enabled: !!courseId,
    ...options,
  });
}

export function useMyCourses(options = {}) {
  return useQuery({
    queryKey: adminEnrollmentKeys.myCourses(),
    queryFn: () => enrollmentsAdminApi.listMyCourses(),
    ...options,
  });
}

// Every roster / section change invalidates the course's roster and
// sections (section moves change both).
function useInvalidateCourse(courseId) {
  const qc = useQueryClient();
  return () => {
    qc.invalidateQueries({ queryKey: adminEnrollmentKeys.rosterAll(courseId) });
    qc.invalidateQueries({ queryKey: adminEnrollmentKeys.sections(courseId) });
  };
}

/** Lifecycle action on one enrollment: conclude | deactivate | delete | reactivate. */
export function useEnrollmentAction(courseId, options = {}) {
  const invalidate = useInvalidateCourse(courseId);
  const { onSuccess, ...rest } = options;
  return useMutation({
    mutationFn: ({ enrollmentId, action }) => (action === 'reactivate'
      ? enrollmentsAdminApi.reactivate(courseId, enrollmentId)
      : enrollmentsAdminApi.applyTask(courseId, enrollmentId, action)),
    ...rest,
    onSuccess: (...args) => {
      invalidate();
      onSuccess?.(...args);
    },
  });
}

export function useUpdateEnrollment(courseId, options = {}) {
  const invalidate = useInvalidateCourse(courseId);
  const { onSuccess, ...rest } = options;
  return useMutation({
    mutationFn: ({ enrollmentId, patch }) => enrollmentsAdminApi.updateEnrollment(courseId, enrollmentId, patch),
    ...rest,
    onSuccess: (...args) => {
      invalidate();
      onSuccess?.(...args);
    },
  });
}

export function useCreateEnrollment(courseId, options = {}) {
  const invalidate = useInvalidateCourse(courseId);
  const { onSuccess, ...rest } = options;
  return useMutation({
    mutationFn: (enrollment) => enrollmentsAdminApi.createEnrollment(courseId, enrollment),
    ...rest,
    onSuccess: (...args) => {
      invalidate();
      onSuccess?.(...args);
    },
  });
}

/** Section mutations: { kind: create|update|delete|crosslist|uncrosslist, ... }. */
export function useSectionMutation(courseId, options = {}) {
  const invalidate = useInvalidateCourse(courseId);
  const qc = useQueryClient();
  const { onSuccess, ...rest } = options;
  return useMutation({
    mutationFn: ({ kind, sectionId, section, targetCourseId }) => {
      switch (kind) {
        case 'create': return enrollmentsAdminApi.createSection(courseId, section);
        case 'update': return enrollmentsAdminApi.updateSection(sectionId, section);
        case 'delete': return enrollmentsAdminApi.deleteSection(sectionId);
        case 'crosslist': return enrollmentsAdminApi.crosslistSection(sectionId, targetCourseId);
        case 'uncrosslist': return enrollmentsAdminApi.uncrosslistSection(sectionId);
        default: throw new Error(`unknown section change: ${kind}`);
      }
    },
    ...rest,
    onSuccess: (data, vars, ...more) => {
      invalidate();
      // A cross-listed section lands in (or leaves) another course too.
      if (vars?.kind === 'crosslist' || vars?.kind === 'uncrosslist') {
        qc.invalidateQueries({ queryKey: adminEnrollmentKeys.all });
      }
      onSuccess?.(data, vars, ...more);
    },
  });
}
